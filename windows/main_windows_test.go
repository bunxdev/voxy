package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSizeValidation(t *testing.T) {
	for _, s := range []string{"", "0G", "-1G", "1", "1g", "1.5G", "8G --shrink", "999999999999999999999T"} {
		if _, e := parseSize(s); e == nil {
			t.Errorf("accepted %q", s)
		}
	}
	if n, e := parseSize("2G"); e != nil || n != 2147483648 {
		t.Fatalf("2G: %d %v", n, e)
	}
}
func TestInitPreservesExistingDisk(t *testing.T) {
	a := &app{state: t.TempDir(), root: t.TempDir()}
	path := a.path("disk.qcow2")
	if e := os.WriteFile(path, []byte("user data"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := a.init(); e == nil {
		t.Fatal("overwrote existing disk")
	}
	b, e := os.ReadFile(path)
	if e != nil || string(b) != "user data" {
		t.Fatal("disk modified")
	}
}
func TestInitRejectsCorruptImageAndCanRetry(t *testing.T) {
	a := &app{state: t.TempDir(), root: t.TempDir()}
	base := filepath.Join(a.root, "image")
	if e := os.Mkdir(base, 0700); e != nil {
		t.Fatal(e)
	}
	hashes := map[string]string{}
	for _, name := range []string{"kernel", "initramfs", "disk.qcow2"} {
		path := filepath.Join(base, name)
		if e := os.WriteFile(path, []byte(name), 0600); e != nil {
			t.Fatal(e)
		}
		hashes[name], _ = hashFile(path)
	}
	if e := writeJSON(filepath.Join(base, "SHA256.json"), hashes); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(base, "disk.qcow2"), []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := a.init(); e == nil {
		t.Fatal("corrupt image accepted")
	}
	if _, e := os.Stat(a.path("disk.qcow2")); !os.IsNotExist(e) {
		t.Fatal("partial disk committed")
	}
	if e := os.WriteFile(filepath.Join(base, "disk.qcow2"), []byte("disk.qcow2"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(a.path("kernel"), []byte("interrupted previous init"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := a.init(); e != nil {
		t.Fatal("retry failed:", e)
	}
	for name, want := range hashes {
		got, e := hashFile(a.path(name))
		if e != nil || got != want {
			t.Fatal("bad imported file:", name, e)
		}
	}
}
func TestPIDReuseIsNotOurVM(t *testing.T) {
	a := &app{state: t.TempDir()}
	pid := uint32(os.Getpid())
	exe, created, alive, e := processIdentity(pid)
	if e != nil || !alive {
		t.Fatal(e)
	}
	r := runState{PID: pid, Created: created - 1, Executable: exe}
	if e := writeJSON(a.path("process.json"), r); e != nil {
		t.Fatal(e)
	}
	if _, alive, e := a.running(); e != nil || alive {
		t.Fatal("PID reuse accepted", e)
	}
}

func TestKernelUpdateValidatesBothFilesBeforeReplacingEither(t *testing.T) {
	a := &app{state: t.TempDir()}
	hashes := map[string]string{}
	for _, name := range []string{"kernel", "initramfs"} {
		if e := os.WriteFile(a.path(name), []byte("old-"+name), 0600); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(a.path(name+".next"), []byte("new-"+name), 0600); e != nil {
			t.Fatal(e)
		}
		hashes[name], _ = hashFile(a.path(name + ".next"))
	}
	if e := writeJSON(a.path("kernel-update.json"), hashes); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(a.path("initramfs.next"), []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := a.applyKernelUpdate(); e == nil {
		t.Fatal("accepted corrupt pending kernel")
	}
	b, _ := os.ReadFile(a.path("kernel"))
	if string(b) != "old-kernel" {
		t.Fatal("replaced kernel before validating initramfs")
	}
	if e := os.WriteFile(a.path("initramfs.next"), []byte("new-initramfs"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := a.applyKernelUpdate(); e != nil {
		t.Fatal(e)
	}
	for name, want := range hashes {
		got, e := hashFile(a.path(name))
		if e != nil || got != want {
			t.Fatal(name, e)
		}
	}
	if _, e := os.Stat(a.path("kernel-update.json")); !os.IsNotExist(e) {
		t.Fatal("pending marker not removed")
	}
}
