package main

import (
	"encoding/json"
	"github.com/bunxdev/voxy/windows/internal/autoports"
	"golang.org/x/sys/windows"
	"os"
	"testing"
	"time"
)

func TestAutoPortsConfiguration(t *testing.T) {
	a := &app{state: t.TempDir(), port: 22222}
	if address, e := autoports.LoadConfig(a.path("ports-auto.conf")); e != nil || address != "" {
		t.Fatal(address, e)
	}
	if e := a.ports([]string{"auto", "on"}); e != nil {
		t.Fatal(e)
	}
	if address, _ := autoports.LoadConfig(a.path("ports-auto.conf")); address != "127.0.0.1" {
		t.Fatal(address)
	}
	for _, ip := range []string{"::1", "bad", "0.0.0.0,hostfwd=tcp::1-:1", "224.0.0.1"} {
		if a.ports([]string{"auto", "on", ip}) == nil {
			t.Fatal("invalid IP accepted")
		}
	}
	if address, _ := autoports.LoadConfig(a.path("ports-auto.conf")); address != "127.0.0.1" {
		t.Fatal("invalid input changed config")
	}
	if e := a.ports([]string{"auto", "off"}); e != nil {
		t.Fatal(e)
	}
}
func TestAutoPortsJournalRetriesSharingViolation(t *testing.T) {
	a := &app{state: t.TempDir()}
	if e := a.saveAutoPorts(autoports.Status{Session: "before"}); e != nil {
		t.Fatal(e)
	}
	name, e := windows.UTF16PtrFromString(a.path("ports-auto.json"))
	if e != nil {
		t.Fatal(e)
	}
	// Deliberately omit FILE_SHARE_DELETE to reproduce a transient reader/scanner.
	h, e := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan struct{})
	go func() { time.Sleep(120 * time.Millisecond); windows.CloseHandle(h); close(done) }()
	e = a.saveAutoPorts(autoports.Status{Session: "after"})
	<-done
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(a.path("ports-auto.json"))
	if e != nil {
		t.Fatal(e)
	}
	var s autoports.Status
	if json.Unmarshal(b, &s) != nil || s.Session != "after" {
		t.Fatal(string(b))
	}
}
