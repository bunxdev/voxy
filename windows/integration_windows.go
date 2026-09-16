package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Always use a fresh disk, separate from the user's normal VM.
func (a *app) testVM() (err error) {
	dir, err := os.MkdirTemp(filepath.Dir(a.state), "voxy-test-")
	if err != nil {
		return err
	}
	test := *a
	test.noAutoBackup = true
	test.state = filepath.Join(dir, "Debian con espacios")
	if err = test.secureState(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	test.port = listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	fmt.Println("Evidencias:", test.state)
	defer func() {
		if err != nil {
			fmt.Println("TEST FALLÓ:", err)
			fmt.Println("Datos conservados en", test.state)
			if _, alive, _ := test.running(); alive {
				if e := test.stop(); e != nil {
					fmt.Println("Apagado pendiente:", e)
				}
			}
		}
	}()
	for _, action := range []string{"init", "start", "wait"} {
		if err = test.dispatch([]string{action}); err != nil {
			return err
		}
	}
	if err = test.remote(`set -eu; test "$(uname -m)" = x86_64; . /etc/os-release; test "$ID" = debian; test "$VERSION_ID" = 12; ! command -v docker; getent hosts deb.debian.org; apt-get -o APT::Update::Error-Mode=any update -qq; echo voxy-windows-persistence > /root/voxy-test; test -z "$(systemctl --failed --no-legend --plain)"`, os.Stdout); err != nil {
		return err
	}
	before, err := test.capture("cat /proc/sys/kernel/random/boot_id")
	if err != nil {
		return err
	}
	hashes := map[string]string{}
	for _, name := range []string{"kernel", "initramfs"} {
		hashes[name], err = hashFile(test.path(name))
		if err != nil {
			return err
		}
	}
	if err = test.syncKernel(); err != nil {
		return err
	}
	for name, want := range hashes {
		got, e := hashFile(test.path(name))
		if e != nil {
			return e
		}
		if got != want {
			return fmt.Errorf("Checksum de %s cambió", name)
		}
	}
	if test.resize("2G") == nil {
		return fmt.Errorf("Se permitió resize con VM encendida")
	}
	if err = test.stop(); err != nil {
		return err
	}
	for name, want := range hashes {
		got, e := hashFile(test.path(name))
		if e != nil {
			return e
		}
		if got != want {
			return fmt.Errorf("Checksum de %s cambió después del apagado", name)
		}
	}
	if _, err = test.image("check", "disk.qcow2"); err != nil {
		return err
	}
	if err = test.resize("2G"); err != nil {
		return err
	}
	if test.resize("1G") == nil {
		return fmt.Errorf("Se permitió reducir el disco")
	}
	if err = test.start(); err != nil {
		return err
	}
	if err = test.wait(); err != nil {
		return err
	}
	after, err := test.capture("cat /proc/sys/kernel/random/boot_id")
	if err != nil {
		return err
	}
	if before == after {
		return fmt.Errorf("No hubo nuevo arranque")
	}
	if err = test.remote(`set -eu; test "$(cat /root/voxy-test)" = voxy-windows-persistence; test "$(df -kP / | awk 'NR==2 {print $2}')" -gt 1900000; test -z "$(systemctl --failed --no-legend --plain)"; free -m; df -h /`, os.Stdout); err != nil {
		return err
	}
	if err = test.stop(); err != nil {
		return err
	}
	if _, err = test.image("check", "disk.qcow2"); err != nil {
		return err
	}
	message := fmt.Sprintf("PASS %s Windows executable (%s): SSH, DNS, APT, kernel, resize, persistencia, qcow2, apagado.\n", time.Now().UTC().Format(time.RFC3339), test.accel)
	fmt.Print(message)
	return os.WriteFile(test.path("PASS.txt"), []byte(message), 0600)
}
