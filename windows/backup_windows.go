package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const backupInterval = 10 * time.Minute
const backupRetention = 3

var backupID = regexp.MustCompile(`^\d{8}T\d{6}\.\d{9}Z$`)
var backupFiles = []string{"disk.qcow2", "kernel", "initramfs"}

type checkpoint struct {
	ID          string
	Created     time.Time
	Hashes      map[string]string
	QEMUCreated uint64
	Writes      uint64
}
type backupStatus struct {
	LastAttempt time.Time
	LastSuccess time.Time
	Message     string
}

func (a *app) checkpoints() ([]checkpoint, error) {
	entries, e := os.ReadDir(a.path("backups"))
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var all []checkpoint
	for _, entry := range entries {
		if !entry.IsDir() || !backupID.MatchString(entry.Name()) {
			continue
		}
		var c checkpoint
		b, e := os.ReadFile(a.path(filepath.Join("backups", entry.Name(), "manifest.json")))
		if e == nil && json.Unmarshal(b, &c) == nil && c.ID == entry.Name() {
			all = append(all, c)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return all, nil
}
func (a *app) listBackups() error {
	all, e := a.checkpoints()
	if e != nil {
		return e
	}
	fmt.Println("Puntos de recuperación locales (disco, sin memoria RAM):")
	for _, c := range all {
		fmt.Println(c.ID, c.Created.Local().Format(time.RFC3339))
	}
	if len(all) == 0 {
		fmt.Println("Todavía no hay copias completas")
	}
	if b, e := os.ReadFile(a.path("backup-status.json")); e == nil {
		fmt.Println(string(b))
	}
	return nil
}
func (q *qmpClient) writes() (uint64, error) {
	var list []struct {
		Device string `json:"device"`
		Stats  struct {
			Writes uint64 `json:"wr_operations"`
			Unmaps uint64 `json:"unmap_operations"`
		} `json:"stats"`
	}
	if e := q.call("query-blockstats", nil, &list); e != nil {
		return 0, e
	}
	for _, v := range list {
		if v.Device == "rootdisk" {
			return v.Stats.Writes + v.Stats.Unmaps, nil
		}
	}
	return 0, errors.New("No se encontró rootdisk")
}
func (a *app) backup(automatic bool) (err error) {
	unlock, e := a.lock("control")
	if e != nil {
		return e
	}
	defer unlock()
	status := backupStatus{LastAttempt: time.Now().UTC()}
	if b, e := os.ReadFile(a.path("backup-status.json")); e == nil {
		var old backupStatus
		json.Unmarshal(b, &old)
		status.LastSuccess = old.LastSuccess
	}
	defer func() {
		if err != nil {
			status.Message = err.Error()
		} else if status.Message == "" {
			status.Message = "Copia completa y verificada"
			status.LastSuccess = time.Now().UTC()
		}
		_ = durableJSON(a.path("backup-status.json"), status)
	}()
	r, alive, e := a.running()
	if e != nil {
		return e
	}
	if !alive {
		return errors.New("Inicia Debian para crear una copia")
	}
	q, e := a.qmp()
	if e != nil {
		return e
	}
	defer q.Close()
	writes, e := q.writes()
	if e != nil {
		return e
	}
	all, e := a.checkpoints()
	if e != nil {
		return e
	}
	if automatic && len(all) > 0 {
		last := all[len(all)-1]
		if last.QEMUCreated == r.Created && last.Writes == writes {
			status.Message = "Sin escrituras nuevas; se conserva el último punto"
			return nil
		}
	}
	// Reap a concluded job left behind if the previous worker was interrupted.
	var oldJobs []struct{ ID, Status string }
	if e = q.call("query-jobs", nil, &oldJobs); e != nil {
		return e
	}
	for _, job := range oldJobs {
		if job.ID == "voxy-backup" && job.Status == "concluded" {
			if e = q.call("job-dismiss", map[string]any{"id": job.ID}, nil); e != nil {
				return e
			}
		}
	}
	var jobs []json.RawMessage
	if e = q.call("query-block-jobs", nil, &jobs); e != nil {
		return e
	}
	if len(jobs) > 0 {
		return errors.New("Hay un trabajo QEMU pendiente; se conserva la copia anterior")
	}
	_ = q.call("blockdev-del", map[string]any{"node-name": "voxy-backup-target"}, nil)
	var blocks []struct {
		Device   string `json:"device"`
		Inserted struct {
			Image struct {
				Size uint64 `json:"virtual-size"`
			} `json:"image"`
		} `json:"inserted"`
	}
	if e = q.call("query-block", nil, &blocks); e != nil {
		return e
	}
	var size uint64
	for _, b := range blocks {
		if b.Device == "rootdisk" {
			size = b.Inserted.Image.Size
		}
	}
	if size == 0 {
		return errors.New("Tamaño del disco desconocido")
	}
	if e = os.MkdirAll(a.path("backups"), 0700); e != nil {
		return e
	}
	// With no active QEMU jobs and the control lock held, incomplete copies cannot be in use.
	partials, _ := filepath.Glob(a.path("backups/.partial-*"))
	for _, p := range partials {
		if e = os.RemoveAll(p); e != nil {
			return e
		}
	}
	path, _ := windows.UTF16PtrFromString(a.state)
	var free uint64
	if e = windows.GetDiskFreeSpaceEx(path, &free, nil, nil); e != nil {
		return e
	}
	if free < size+(512<<20) {
		return fmt.Errorf("Espacio insuficiente: se requieren %d MiB libres antes de copiar; no se borran puntos anteriores", (size+(512<<20))>>20)
	}

	c := checkpoint{ID: time.Now().UTC().Format("20060102T150405.000000000Z"), Created: time.Now().UTC(), Hashes: map[string]string{}, QEMUCreated: r.Created, Writes: writes}
	relative := filepath.Join("backups", ".partial-"+c.ID)
	stage := a.path(relative)
	if e = os.Mkdir(stage, 0700); e != nil {
		return e
	}
	// Capture boot files from the guest, then verify they did not change during the backup.
	for _, f := range []struct{ name, remote string }{{"kernel", "/vmlinuz"}, {"initramfs", "/initrd.img"}} {
		out, e := os.Create(filepath.Join(stage, f.name))
		if e != nil {
			return e
		}
		e = a.backupRemote("cat "+f.remote, out)
		if e == nil {
			e = out.Sync()
		}
		ce := out.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		c.Hashes[f.name], e = hashFile(filepath.Join(stage, f.name))
		if e != nil {
			return e
		}
	}
	if e = a.backupRemote("sync", io.Discard); e != nil {
		return e
	}
	c.Writes, e = q.writes()
	if e != nil {
		return e
	}
	target := filepath.ToSlash(filepath.Join(relative, "disk.qcow2"))
	if _, e = a.image("create", "-f", "qcow2", target, strconv.FormatUint(size, 10)); e != nil {
		return e
	}
	// Full point-in-time backup; the active disk remains writable. No qemu-img reads of a live disk.
	if e = q.call("blockdev-add", map[string]any{"driver": "qcow2", "node-name": "voxy-backup-target", "file": map[string]any{"driver": "file", "filename": target}}, nil); e != nil {
		return e
	}
	defer q.call("blockdev-del", map[string]any{"node-name": "voxy-backup-target"}, nil)
	if e = q.call("blockdev-backup", map[string]any{"device": "rootdisk", "target": "voxy-backup-target", "job-id": "voxy-backup", "sync": "full", "auto-dismiss": false, "speed": 32 << 20}, nil); e != nil {
		return e
	}
	finished := false
	defer func() {
		if !finished {
			_ = q.call("job-cancel", map[string]any{"id": "voxy-backup"}, nil)
		}
	}()
	deadline := time.Now().Add(30 * time.Minute)
	for !finished {
		if time.Now().After(deadline) {
			return errors.New("La copia excedió 30 minutos; la anterior sigue disponible")
		}
		var jobs []struct{ ID, Status, Error string }
		if e = q.call("query-jobs", nil, &jobs); e != nil {
			return e
		}
		found := false
		for _, job := range jobs {
			if job.ID != "voxy-backup" {
				continue
			}
			found = true
			if job.Status == "concluded" {
				finished = true
				_ = q.call("job-dismiss", map[string]any{"id": job.ID}, nil)
				if job.Error != "" {
					return errors.New(job.Error)
				}
			}
		}
		if !found {
			return errors.New("El trabajo de copia desapareció sin confirmar resultado")
		}
		if !finished {
			time.Sleep(time.Second)
		}
	}
	if e = q.call("blockdev-del", map[string]any{"node-name": "voxy-backup-target"}, nil); e != nil {
		return e
	}
	for _, f := range []struct{ name, remote string }{{"kernel", "/vmlinuz"}, {"initramfs", "/initrd.img"}} {
		var output bytes.Buffer
		e := a.backupRemote("sha256sum "+f.remote+" | cut -d ' ' -f1", &output)
		sum := strings.TrimSpace(output.String())
		if e != nil {
			return e
		}
		if sum != c.Hashes[f.name] {
			return errors.New("El kernel cambió durante la copia; se reintentará")
		}
	}
	if _, e = a.image("check", "-f", "qcow2", target); e != nil {
		return e
	}
	// Flush the completed independent disk before publishing its manifest.
	f, e := os.OpenFile(filepath.Join(stage, "disk.qcow2"), os.O_RDWR, 0)
	if e != nil {
		return e
	}
	e = f.Sync()
	f.Close()
	if e != nil {
		return e
	}
	c.Hashes["disk.qcow2"], e = hashFile(filepath.Join(stage, "disk.qcow2"))
	if e != nil {
		return e
	}
	if e = durableJSON(filepath.Join(stage, "manifest.json"), c); e != nil {
		return e
	}
	if e = os.Rename(stage, a.path(filepath.Join("backups", c.ID))); e != nil {
		return e
	}
	fmt.Println("Punto de recuperación:", c.ID)
	all = append(all, c)
	for len(all) > backupRetention {
		if e = os.RemoveAll(a.path(filepath.Join("backups", all[0].ID))); e != nil {
			return e
		}
		all = all[1:]
	}
	return nil
}
func (a *app) startBackupWorker() error {
	if a.noAutoBackup || os.Getenv("VOXY_AUTO_BACKUP") == "0" {
		return nil
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	cmd := exec.Command(exe, "backup-worker")
	cmd.Env = append(os.Environ(), "VOXY_DATA_DIR="+a.state)
	cmd.SysProcAttr = &windowsSysProcAttr
	if e = cmd.Start(); e != nil {
		return e
	}
	return cmd.Process.Release()
}
func (a *app) backupWorker() error {
	unlock, e := a.lock("backup-worker")
	for i := 0; e != nil && i < 15; i++ {
		time.Sleep(time.Second)
		unlock, e = a.lock("backup-worker")
	}
	if e != nil {
		return nil
	}
	defer unlock()
	r, alive, e := a.running()
	if e != nil || !alive {
		return e
	}
	identity := r.Created
	// First backup soon after SSH is ready, then every ten minutes; no console or network connection required.
	if e = a.wait(); e != nil {
		return e
	}
	next := time.Now()
	for {
		r, alive, e = a.running()
		if e != nil || !alive || r.Created != identity {
			return e
		}
		if !time.Now().Before(next) {
			attempt := time.Now()
			err := a.backup(true)
			next = attempt.Add(backupInterval)
			if next.Before(time.Now()) {
				next = time.Now().Add(backupInterval)
			}
			if err != nil {
				next = time.Now().Add(time.Minute)
			}
		}
		time.Sleep(10 * time.Second)
	}
}

func (a *app) backupRemote(command string, out io.Writer) error {
	c, e := a.client()
	if e != nil {
		return e
	}
	defer c.Close()
	timer := time.AfterFunc(2*time.Minute, func() { c.Close() })
	defer timer.Stop()
	session, e := c.NewSession()
	if e != nil {
		return e
	}
	defer session.Close()
	session.Stdout = out
	session.Stderr = os.Stderr
	return session.Run(command)
}
