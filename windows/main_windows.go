package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

const version = "0.6.0"

type app struct {
	noAutoBackup       bool
	root, state, accel string
	port, ram, cpus    int
}
type runState struct {
	PID         uint32
	Created     uint64
	Executable  string
	Port        int
	Accelerator string
	Ports       []portRule
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func (a *app) path(name string) string   { return filepath.Join(a.state, name) }
func (a *app) binary(name string) string { return filepath.Join(a.root, "qemu", name+".exe") }
func writeJSON(path string, v any) error { return durableJSON(path, v) }
func (a *app) secureState() error {
	if err := os.MkdirAll(a.state, 0700); err != nil {
		return err
	}
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(a.state, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}
func newApp() (*app, error) {
	exe, e := os.Executable()
	if e != nil {
		return nil, e
	}
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil, errors.New("LOCALAPPDATA no está definido")
	}
	a := &app{root: filepath.Dir(exe), state: env("VOXY_DATA_DIR", filepath.Join(local, "Voxy", "amd64")), accel: env("VOXY_ACCEL", "auto")}
	a.state, e = filepath.Abs(a.state)
	if e != nil {
		return nil, e
	}
	if strings.Contains(a.state, ",") || filepath.Dir(a.state) == a.state || strings.EqualFold(a.state, a.root) {
		return nil, errors.New("Directorio de datos no válido; usa una carpeta dedicada sin comas")
	}
	for _, setting := range []struct {
		name, def string
		target    *int
		min, max  int
	}{{"VOXY_SSH_PORT", "22222", &a.port, 1, 65535}, {"VOXY_RAM_MB", "512", &a.ram, 256, 1048576}, {"VOXY_CPUS", "1", &a.cpus, 1, 256}} {
		n, err := strconv.Atoi(env(setting.name, setting.def))
		if err != nil || n < setting.min || n > setting.max {
			return nil, fmt.Errorf("%s fuera de rango", setting.name)
		}
		*setting.target = n
	}
	if a.accel != "auto" && a.accel != "whpx" && a.accel != "tcg" {
		return nil, errors.New("VOXY_ACCEL debe ser auto, whpx o tcg")
	}
	return a, a.secureState()
}
func processIdentity(pid uint32) (string, uint64, bool, error) {
	h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if errors.Is(e, windows.ERROR_INVALID_PARAMETER) {
		return "", 0, false, nil
	}
	if e != nil {
		return "", 0, false, e
	}
	defer windows.CloseHandle(h)
	var code uint32
	if e = windows.GetExitCodeProcess(h, &code); e != nil {
		return "", 0, false, e
	}
	if code != 259 {
		return "", 0, false, nil
	}
	var created, exited, kernel, user windows.Filetime
	if e = windows.GetProcessTimes(h, &created, &exited, &kernel, &user); e != nil {
		return "", 0, false, e
	}
	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	if e = windows.QueryFullProcessImageName(h, 0, &buf[0], &size); e != nil {
		return "", 0, false, e
	}
	return windows.UTF16ToString(buf[:size]), uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime), true, nil
}
func (a *app) running() (*runState, bool, error) {
	b, e := os.ReadFile(a.path("process.json"))
	if os.IsNotExist(e) {
		return nil, false, nil
	}
	if e != nil {
		return nil, false, e
	}
	var r runState
	if e = json.Unmarshal(b, &r); e != nil {
		return nil, false, e
	}
	exe, created, alive, e := processIdentity(r.PID)
	if e != nil {
		return nil, false, e
	}
	return &r, alive && created == r.Created && strings.EqualFold(exe, r.Executable), nil
}
func whpxAvailable() bool {
	p := windows.NewLazySystemDLL("WinHvPlatform.dll").NewProc("WHvGetCapability")
	if p.Find() != nil {
		return false
	}
	var present uint32
	var written uint32
	result, _, _ := p.Call(0, uintptr(unsafe.Pointer(&present)), 4, uintptr(unsafe.Pointer(&written)))
	return result == 0 && present != 0
}
func hashFile(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), e
}
func copyFile(from, to string) error {
	in, e := os.Open(from)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	if e == nil {
		e = out.Sync()
	}
	ce := out.Close()
	if e != nil {
		return e
	}
	return ce
}
func (a *app) init() error {
	if _, e := os.Stat(a.path("disk.qcow2")); !os.IsNotExist(e) {
		return errors.New("Ya existe un disco o no se puede acceder; no se sobrescribe")
	}
	var hashes map[string]string
	b, e := os.ReadFile(filepath.Join(a.root, "image", "SHA256.json"))
	if e != nil {
		return e
	}
	if e = json.Unmarshal(b, &hashes); e != nil {
		return e
	}
	stage, e := os.MkdirTemp(a.state, ".init-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	for _, name := range []string{"kernel", "initramfs", "disk.qcow2"} {
		src := filepath.Join(a.root, "image", name)
		sum, e := hashFile(src)
		if e != nil {
			return e
		}
		if len(hashes[name]) != 64 || sum != hashes[name] {
			return fmt.Errorf("Checksum incorrecto: %s", name)
		}
		if e = copyFile(src, filepath.Join(stage, name)); e != nil {
			return e
		}
	}
	// Commit disk last, allowing an interrupted initial copy to be retried.
	for _, name := range []string{"kernel", "initramfs", "disk.qcow2"} {
		if e = os.Rename(filepath.Join(stage, name), a.path(name)); e != nil {
			return e
		}
	}
	fmt.Println("Debian instalado en", a.state)
	return nil
}
func (a *app) key() (ssh.Signer, error) {
	path := a.path("id_ed25519")
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		block, err := ssh.MarshalPrivateKey(priv, "voxy")
		if err != nil {
			return nil, err
		}
		b = pem.EncodeToMemory(block)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, err
		}
		_, e = f.Write(b)
		ce := f.Close()
		if e == nil {
			e = ce
		}
	}
	if e != nil {
		return nil, e
	}
	return ssh.ParsePrivateKey(b)
}
func (a *app) start() error {
	_, alive, e := a.running()
	if e != nil {
		return e
	}
	if alive {
		fmt.Println("Voxy ya está encendido")
		return a.startBackupWorker()
	}
	rules, e := a.loadPorts()
	if e != nil {
		return e
	}
	if e = a.applyRestore(); e != nil {
		return e
	}
	if e = a.applyKernelUpdate(); e != nil {
		return e
	}
	accel := a.accel
	if accel == "auto" {
		if !whpxAvailable() {
			return errors.New("WHPX no disponible. Habilita Plataforma de hipervisor de Windows y reinicia, o selecciona explícitamente VOXY_ACCEL=tcg")
		}
		accel = "whpx"
	}
	if accel == "whpx" && !whpxAvailable() {
		return errors.New("Windows Hypervisor Platform no disponible; ejecuta Voxy.exe doctor")
	}
	for _, name := range []string{"kernel", "initramfs", "disk.qcow2"} {
		f, e := os.Stat(a.path(name))
		if e != nil || f.Size() == 0 {
			return errors.New("Ejecuta Voxy.exe init primero")
		}
	}
	if _, e = a.image("check", "-f", "qcow2", "disk.qcow2"); e != nil {
		return fmt.Errorf("El disco requiere revisión; no se modifica automáticamente. Usa backups / restore si necesitas recuperar: %w", e)
	}
	_ = os.Remove(a.path("qmp.sock"))
	signer, e := a.key()
	if e != nil {
		return e
	}
	if e = os.WriteFile(a.path("authorized_keys"), ssh.MarshalAuthorizedKey(signer.PublicKey()), 0600); e != nil {
		return e
	}
	if e = a.prepareFirmware(); e != nil {
		return e
	}
	// CreateProcessW sets a Unicode working directory. Relative ASCII filenames
	// avoid this QEMU build's narrow Win32 file-path conversion for accented names.
	args := []string{"-name", "voxy-amd64", "-L", "firmware", "-machine", "q35", "-accel", accel, "-m", strconv.Itoa(a.ram), "-smp", strconv.Itoa(a.cpus), "-kernel", "kernel", "-initrd", "initramfs", "-append", "console=ttyS0 root=/dev/vda rootfstype=ext4 rw quiet", "-drive", "file=disk.qcow2,format=qcow2,if=none,id=rootdisk,discard=unmap,cache=writeback", "-device", "virtio-blk-pci,drive=rootdisk", "-device", "virtio-rng-pci", "-fw_cfg", "name=opt/vm/ssh-key,file=authorized_keys", "-netdev", portNetdev(a.port, rules), "-device", "virtio-net-pci,netdev=net,romfile=", "-display", "none", "-monitor", "none", "-qmp", "unix:qmp.sock,server=on,wait=off", "-serial", "file:serial.log"}
	if accel == "tcg" {
		args = append(args, "-cpu", "max")
	}
	log, e := os.Create(a.path("qemu.log"))
	if e != nil {
		return e
	}
	defer log.Close()
	cmd := exec.Command(a.binary("qemu-system-x86_64"), args...)
	cmd.Dir = a.state
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &windowsSysProcAttr
	if e = cmd.Start(); e != nil {
		return e
	}
	exe, created, alive, e := processIdentity(uint32(cmd.Process.Pid))
	if e != nil || !alive {
		_ = cmd.Wait()
		return a.qemuError("QEMU no inició")
	}
	r := runState{PID: uint32(cmd.Process.Pid), Created: created, Executable: exe, Port: a.port, Accelerator: accel, Ports: rules}
	if e = writeJSON(a.path("process.json"), r); e != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return e
	}
	_ = cmd.Process.Release()
	time.Sleep(2 * time.Second)
	_, alive, e = a.running()
	if e != nil {
		return e
	}
	if !alive {
		return a.qemuError("QEMU terminó")
	}
	fmt.Printf("Voxy iniciado (%s), SSH 127.0.0.1:%d\n", accel, a.port)
	return a.startBackupWorker()
}
func (a *app) client() (*ssh.Client, error) {
	r, alive, e := a.running()
	if e != nil {
		return nil, e
	}
	if !alive {
		return nil, errors.New("Voxy está apagado")
	}
	b, e := os.ReadFile(a.path("id_ed25519"))
	if e != nil {
		return nil, e
	}
	signer, e := ssh.ParsePrivateKey(b)
	if e != nil {
		return nil, e
	}
	verify := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		path := a.path("host_key.pub")
		b, e := os.ReadFile(path)
		if os.IsNotExist(e) {
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err == nil {
				_, err = f.Write(ssh.MarshalAuthorizedKey(key))
				ce := f.Close()
				if err != nil {
					return err
				}
				return ce
			}
			if !os.IsExist(err) {
				return err
			}
			b, e = os.ReadFile(path)
		}
		if e != nil {
			return e
		}
		saved, _, _, _, e := ssh.ParseAuthorizedKey(b)
		if e != nil {
			return e
		}
		if !bytes.Equal(saved.Marshal(), key.Marshal()) {
			return errors.New("La clave SSH del invitado cambió; revisa host_key.pub antes de continuar")
		}
		return nil
	}
	cfg := &ssh.ClientConfig{User: "root", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: verify, Timeout: 5 * time.Second}
	addr := fmt.Sprintf("127.0.0.1:%d", r.Port)
	conn, e := net.DialTimeout("tcp", addr, 5*time.Second)
	if e != nil {
		return nil, e
	}
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	c, ch, req, e := ssh.NewClientConn(conn, addr, cfg)
	if e != nil {
		conn.Close()
		return nil, e
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(c, ch, req), nil
}
func (a *app) remote(command string, out io.Writer) error {
	c, e := a.client()
	if e != nil {
		return e
	}
	defer c.Close()
	s, e := c.NewSession()
	if e != nil {
		return e
	}
	defer s.Close()
	s.Stdout = out
	s.Stderr = os.Stderr
	return s.Run(command)
}
func (a *app) capture(command string) (string, error) {
	var b bytes.Buffer
	e := a.remote(command, &b)
	return strings.TrimSpace(b.String()), e
}
func (a *app) wait() error {
	deadline := time.Now().Add(5 * time.Minute)
	var last error
	for time.Now().Before(deadline) {
		_, alive, e := a.running()
		if e != nil {
			return e
		}
		if !alive {
			return a.qemuError("QEMU terminó")
		}
		last = a.remote("systemctl is-active --quiet vm-prepare ssh", io.Discard)
		if last == nil {
			fmt.Println("Debian listo")
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("Tiempo de espera agotado; revisa serial.log y qemu.log. Último error: %v", last)
}

func (a *app) qemuError(message string) error {
	b, _ := os.ReadFile(a.path("qemu.log"))
	if len(b) > 16384 {
		b = b[len(b)-16384:]
	}
	return fmt.Errorf("%s; registro %s:\n%s", message, a.path("qemu.log"), b)
}
func (a *app) stop() error {
	_, alive, e := a.running()
	if e != nil {
		return e
	}
	if !alive {
		if e = a.applyKernelUpdate(); e != nil {
			return e
		}
		fmt.Println("Voxy apagado")
		return nil
	}
	if e = a.remote("systemctl --no-block --no-wall start poweroff.target", io.Discard); e != nil {
		return e
	}
	for i := 0; i < 120; i++ {
		_, alive, e = a.running()
		if e != nil {
			return e
		}
		if !alive {
			if e = a.applyKernelUpdate(); e != nil {
				return e
			}
			fmt.Println("Voxy apagado")
			return nil
		}
		time.Sleep(time.Second)
	}
	return errors.New("No se confirmó el apagado; no se forzó la terminación")
}
func (a *app) image(args ...string) ([]byte, error) {
	cmd := exec.Command(a.binary("qemu-img"), args...)
	cmd.Dir = a.state
	b, e := cmd.CombinedOutput()
	if e != nil {
		return nil, fmt.Errorf("qemu-img: %w: %s", e, b)
	}
	return b, nil
}
func (a *app) resize(size string) error {
	_, alive, e := a.running()
	if e != nil {
		return e
	}
	if alive {
		return errors.New("Apaga primero la VM")
	}
	want, e := parseSize(size)
	if e != nil {
		return e
	}
	b, e := a.image("info", "--output=json", "disk.qcow2")
	if e != nil {
		return e
	}
	var info struct {
		Size uint64 `json:"virtual-size"`
	}
	if e = json.Unmarshal(b, &info); e != nil {
		return e
	}
	if want <= info.Size {
		return errors.New("Solo se permite ampliar el disco")
	}
	b, e = a.image("resize", "-f", "qcow2", "disk.qcow2", size)
	if e == nil {
		fmt.Print(string(b))
	}
	return e
}
func parseSize(size string) (uint64, error) {
	if !regexp.MustCompile(`^[1-9][0-9]*[MGT]$`).MatchString(size) {
		return 0, errors.New("Usa un tamaño como 2G o 8G")
	}
	n, e := strconv.ParseUint(size[:len(size)-1], 10, 64)
	shift := map[byte]uint{'M': 20, 'G': 30, 'T': 40}[size[len(size)-1]]
	if e != nil || n > (1<<63-1)>>shift {
		return 0, errors.New("Tamaño fuera de rango")
	}
	return n << shift, nil
}
func (a *app) syncKernel() error {
	if _, e := os.Stat(a.path("kernel-update.json")); !os.IsNotExist(e) {
		return errors.New("Hay una actualización pendiente; apaga primero la VM para aplicarla")
	}
	hashes := map[string]string{}
	for _, f := range []struct{ name, remote string }{{"kernel", "/vmlinuz"}, {"initramfs", "/initrd.img"}} {
		next := a.path(f.name + ".next")
		out, e := os.Create(next)
		if e != nil {
			return e
		}
		e = a.remote("cat "+f.remote, out)
		ce := out.Close()
		if e != nil {
			os.Remove(next)
			return e
		}
		if ce != nil {
			return ce
		}
		stat, e := os.Stat(next)
		if e != nil || stat.Size() == 0 {
			return errors.New("Kernel/initramfs vacío")
		}
		hashes[f.name], e = hashFile(next)
		if e != nil {
			return e
		}
	}
	if e := writeJSON(a.path("kernel-update.json"), hashes); e != nil {
		return e
	}
	fmt.Println("Kernel e initramfs descargados; se aplicarán después del apagado")
	return nil
}

// QEMU holds initramfs open on Windows. Keep both staged originals until the
// complete update commits, so an interrupted replacement can be retried.
func (a *app) applyKernelUpdate() error {
	b, e := os.ReadFile(a.path("kernel-update.json"))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	var hashes map[string]string
	if e = json.Unmarshal(b, &hashes); e != nil {
		return e
	}
	for _, name := range []string{"kernel", "initramfs"} {
		sum, err := hashFile(a.path(name + ".next"))
		if err != nil {
			return err
		}
		if len(hashes[name]) != 64 || hashes[name] != sum {
			return fmt.Errorf("Actualización de %s alterada; se conserva el kernel anterior", name)
		}
	}
	for _, name := range []string{"kernel", "initramfs"} {
		temp := a.path(name + ".apply")
		if e = os.Remove(temp); e != nil && !os.IsNotExist(e) {
			return e
		}
		if e = copyFile(a.path(name+".next"), temp); e != nil {
			return e
		}
		if e = os.Rename(temp, a.path(name)); e != nil {
			return e
		}
	}
	if e = os.Remove(a.path("kernel-update.json")); e != nil {
		return e
	}
	for _, name := range []string{"kernel", "initramfs"} {
		_ = os.Remove(a.path(name + ".next"))
	}
	fmt.Println("Actualización del kernel aplicada con la VM apagada")
	return nil
}
func (a *app) shell() error {
	restoreOutput, e := enableVirtualTerminal(os.Stdout, os.Stderr)
	if e != nil {
		return e
	}
	defer restoreOutput()
	c, e := a.client()
	if e != nil {
		return e
	}
	defer c.Close()
	s, e := c.NewSession()
	if e != nil {
		return e
	}
	defer s.Close()
	w, h := 80, 24
	if tw, th, err := terminalSize(); err == nil && tw > 0 && th > 0 {
		w, h = tw, th
	}
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		old, err := term.MakeRaw(fd)
		if err != nil {
			return err
		}
		defer term.Restore(fd, old)
	}
	if e = s.RequestPty("xterm", h, w, ssh.TerminalModes{ssh.ECHO: 1}); e != nil {
		return e
	}
	s.Stdin = os.Stdin
	s.Stdout = os.Stdout
	s.Stderr = os.Stderr
	if e = s.Shell(); e != nil {
		return e
	}
	done := make(chan struct{})
	defer close(done)
	go watchTerminalSize(done, s.WindowChange, w, h)
	return s.Wait()
}
func (a *app) status() error {
	r, alive, e := a.running()
	if e != nil {
		return e
	}
	if alive {
		fmt.Printf("Voxy encendido (%s), PID %d, SSH 127.0.0.1:%d\n", r.Accelerator, r.PID, r.Port)
	} else {
		fmt.Println("Voxy apagado")
	}
	fmt.Println("Datos:", a.state)
	if alive && len(r.Ports) > 0 {
		fmt.Println("Puertos activos:")
		printPorts(r.Ports)
	}
	if os.Getenv("VOXY_AUTO_BACKUP") == "0" {
		fmt.Println("Copias automáticas desactivadas en esta sesión")
	} else {
		s, err := a.loadBackupSettings()
		if err != nil {
			fmt.Println("Configuración de copias inválida; solo copia inicial:", err)
		} else if s.Periodic {
			fmt.Printf("Copia inicial y copias periódicas cada %d min con escrituras; 3 puntos locales\n", s.Minutes)
		} else {
			fmt.Println("Copia inicial al arrancar; copias periódicas desactivadas; 3 puntos locales")
		}
	}
	if b, e := os.ReadFile(a.path("backup-status.json")); e == nil {
		var status backupStatus
		if json.Unmarshal(b, &status) == nil {
			fmt.Println("Recuperación:", status.Message)
			if !status.LastSuccess.IsZero() {
				fmt.Println("Última copia:", status.LastSuccess.Local().Format(time.RFC3339))
			}
		}
	}
	return nil
}
func (a *app) doctor() error {
	fmt.Println("Voxy", version, "Windows x64")
	fmt.Println("WHPX disponible:", whpxAvailable())
	fmt.Println("Acelerador solicitado:", a.accel)
	fmt.Println("Si falta WHPX: habilita Plataforma de hipervisor de Windows en optionalfeatures.exe y reinicia.")
	fmt.Println("TCG explícito en PowerShell: $env:VOXY_ACCEL='tcg'")
	for _, cmd := range []struct {
		name string
		args []string
	}{{"qemu-system-x86_64", []string{"--version"}}, {"qemu-system-x86_64", []string{"-accel", "help"}}, {"qemu-img", []string{"--version"}}} {
		c := exec.Command(a.binary(cmd.name), cmd.args...)
		c.Dir = filepath.Join(a.root, "qemu")
		b, e := c.CombinedOutput()
		fmt.Print(string(b))
		if e != nil {
			return e
		}
	}
	return a.status()
}
func (a *app) dispatch(args []string) error {
	action := args[0]
	if action == "ports" && len(args) > 1 && args[1] != "list" {
		unlock, err := a.lock("control")
		if err != nil {
			return err
		}
		defer unlock()
	}
	switch action {
	case "init", "start", "stop", "resize", "sync-kernel", "restore":
		unlock, e := a.lock("control")
		if e != nil {
			return e
		}
		defer unlock()
	}
	switch action {
	case "init":
		return a.init()
	case "start":
		return a.start()
	case "wait":
		return a.wait()
	case "stop":
		return a.stop()
	case "status":
		return a.status()
	case "doctor":
		return a.doctor()
	case "ports":
		return a.ports(args[1:])
	case "resize":
		if len(args) != 2 {
			return errors.New("Uso: Voxy.exe resize 8G")
		}
		return a.resize(args[1])
	case "sync-kernel":
		return a.syncKernel()
	case "ssh":
		if len(args) == 1 {
			return a.shell()
		}
		return a.remote(strings.Join(args[1:], " "), os.Stdout)
	case "backup":
		return a.backup(false)
	case "backups":
		return a.listBackups()
	case "backup-worker":
		return a.backupWorker()
	case "restore":
		if len(args) != 2 {
			return errors.New("Uso: Voxy.exe restore ID (VM apagada)")
		}
		return a.restore(args[1])
	case "test":
		return a.testVM()
	default:
		return errors.New("Uso: Voxy.exe {init|start|wait|ssh [comando]|status|stop|resize 8G|sync-kernel|doctor|backup|backups|restore ID|ports [operación]|test}")
	}
}
func (a *app) menu() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Voxy - Debian para Windows (experimental)")
	for {
		fmt.Println()
		_ = a.status()
		fmt.Print("\n1) Iniciar Debian\n2) Terminal Debian\n3) Apagar\n4) Diagnóstico\n5) Cerrar panel (la VM sigue encendida)\n6) Crear punto de recuperación\n7) Ver puntos de recuperación\n8) Restaurar un punto (VM apagada)\n9) Configurar copias periódicas\n10) Configurar puertos\n> ")
		line, e := reader.ReadString('\n')
		if e != nil {
			return
		}
		var err error
		switch strings.TrimSpace(line) {
		case "1":
			if _, e := os.Stat(a.path("disk.qcow2")); os.IsNotExist(e) {
				err = a.dispatch([]string{"init"})
			}
			if err == nil {
				err = a.dispatch([]string{"start"})
			}
			if err == nil {
				err = a.wait()
			}
		case "2":
			err = a.shell()
		case "3":
			err = a.dispatch([]string{"stop"})
		case "4":
			err = a.doctor()
		case "6":
			err = a.dispatch([]string{"backup"})
		case "7":
			err = a.listBackups()
		case "9":
			err = a.configureBackups(reader)
		case "10":
			err = a.configurePorts(reader)
		case "8":
			_ = a.listBackups()
			fmt.Print("ID a restaurar (Enter cancela): ")
			id, _ := reader.ReadString('\n')
			id = strings.TrimSpace(id)
			if id != "" {
				fmt.Print("Se volverá a ese punto y se conservará el disco actual. Escribe RESTAURAR: ")
				answer, _ := reader.ReadString('\n')
				if strings.TrimSpace(answer) == "RESTAURAR" {
					err = a.dispatch([]string{"restore", id})
				}
			}
		case "5":
			return
		default:
			continue
		}
		if err != nil {
			fmt.Println("ERROR:", err)
		}
	}
}
func main() {
	a, e := newApp()
	if e == nil {
		if len(os.Args) == 1 {
			a.menu()
			return
		}
		e = a.dispatch(os.Args[1:])
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", e)
		os.Exit(1)
	}
}

func (a *app) prepareFirmware() error {
	dir := a.path("firmware")
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	for _, name := range []string{"bios-256k.bin", "bios.bin", "kvmvapic.bin", "linuxboot_dma.bin", "vgabios-stdvga.bin"} {
		source := filepath.Join(a.root, "qemu", "share", name)
		dest := filepath.Join(dir, name)
		want, e := hashFile(source)
		if e != nil {
			return e
		}
		if got, e := hashFile(dest); e == nil && got == want {
			continue
		}
		next := dest + ".next"
		if e = os.Remove(next); e != nil && !os.IsNotExist(e) {
			return e
		}
		if e = copyFile(source, next); e != nil {
			return e
		}
		if e = os.Rename(next, dest); e != nil {
			return e
		}
	}
	return nil
}
