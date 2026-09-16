package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type restoreJournal struct{ Snapshot, Previous string }

func (a *app) validateCheckpoint(id string) (checkpoint, error) {
	var c checkpoint
	if !backupID.MatchString(id) {
		return c, errors.New("Identificador de copia inválido")
	}
	dir := a.path(filepath.Join("backups", id))
	b, e := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	if c.ID != id {
		return c, errors.New("Manifiesto de copia inválido")
	}
	for _, name := range backupFiles {
		hash, e := hashFile(filepath.Join(dir, name))
		if e != nil {
			return c, e
		}
		if len(c.Hashes[name]) != 64 || hash != c.Hashes[name] {
			return c, fmt.Errorf("Copia alterada: %s; el disco actual no se modifica", name)
		}
	}
	return c, nil
}
func (a *app) restore(id string) error {
	_, alive, e := a.running()
	if e != nil {
		return e
	}
	if alive {
		return errors.New("Apaga Debian antes de restaurar")
	}
	if _, e = os.Stat(a.path("restore-pending.json")); e == nil {
		return errors.New("Hay una restauración pendiente; ejecuta start para completarla")
	}
	if _, e = a.validateCheckpoint(id); e != nil {
		return e
	}
	if _, e = a.image("check", "-f", "qcow2", filepath.ToSlash(filepath.Join("backups", id, "disk.qcow2"))); e != nil {
		return e
	}
	journal := restoreJournal{id, "before-restore-" + time.Now().UTC().Format("20060102T150405.000000000Z")}
	if e = durableJSON(a.path("restore-pending.json"), journal); e != nil {
		return e
	}
	return a.applyRestore()
}
func (a *app) applyRestore() error {
	b, e := os.ReadFile(a.path("restore-pending.json"))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	var j restoreJournal
	if e = json.Unmarshal(b, &j); e != nil {
		return e
	}
	if !strings.HasPrefix(j.Previous, "before-restore-") || !backupID.MatchString(strings.TrimPrefix(j.Previous, "before-restore-")) {
		return errors.New("Diario de restauración inválido")
	}
	c, e := a.validateCheckpoint(j.Snapshot)
	if e != nil {
		return e
	}
	previous := a.path(j.Previous)
	if e = os.MkdirAll(previous, 0700); e != nil {
		return e
	}
	// Preserve every old artifact once. A power loss can interrupt any individual move/copy;
	// the immutable source and durable journal allow the complete operation to be retried.
	for _, name := range append(append([]string{}, backupFiles...), "kernel-update.json", "kernel.next", "initramfs.next") {
		saved := filepath.Join(previous, name)
		if _, e = os.Stat(saved); os.IsNotExist(e) {
			if _, e = os.Stat(a.path(name)); e == nil {
				if e = os.Rename(a.path(name), saved); e != nil {
					return e
				}
			} else if !os.IsNotExist(e) {
				return e
			}
		} else if e != nil {
			return e
		}
		if _, ok := c.Hashes[name]; !ok {
			continue
		}
		tmp := a.path(name + ".restore")
		if e = os.Remove(tmp); e != nil && !os.IsNotExist(e) {
			return e
		}
		if e = copyFile(a.path(filepath.Join("backups", j.Snapshot, name)), tmp); e != nil {
			return e
		}
		f, e := os.OpenFile(tmp, os.O_RDWR, 0)
		if e != nil {
			return e
		}
		e = f.Sync()
		f.Close()
		if e != nil {
			return e
		}
		if e = os.Rename(tmp, a.path(name)); e != nil {
			return e
		}
	}
	if e = os.Remove(a.path("restore-pending.json")); e != nil {
		return e
	}
	fmt.Println("Restaurado:", j.Snapshot, "; estado anterior conservado en", previous)
	return nil
}
