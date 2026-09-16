package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

// Kernel-owned byte-range locks are released even if the process or host dies.
func (a *app) lock(name string) (func(), error) {
	f, e := os.OpenFile(a.path(name+".lck"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	ov := new(windows.Overlapped)
	e = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ov)
	if e != nil {
		f.Close()
		return nil, fmt.Errorf("Otra operación está activa (%s): %w", name, e)
	}
	return func() { windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ov); f.Close() }, nil
}
func durableJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".commit-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	from, e := windows.UTF16PtrFromString(tmp)
	if e != nil {
		return e
	}
	to, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return e
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
