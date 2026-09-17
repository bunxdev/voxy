package main

import (
	"os"
	"strings"
	"testing"
)

func TestPortsPersistenceAndAtomicValidation(t *testing.T) {
	a := &app{state: t.TempDir(), port: 22222}
	if err := a.ports([]string{"add", "tcp", "33033-33035", "8000-8002"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(a.path("ports.conf"))
	for _, args := range [][]string{
		{"add", "tcp", "33033", "9", "0.0.0.0"},
		{"add", "tcp", "22222", "22"},
		{"add", "udp", "0", "9"},
		{"add", "tcp", "1-65535", "1-65535"},
		{"add", "tcp", "5000-5002", "8000-8001"},
		{"add", "tcp", "5000", "5000", "127.0.0.1,hostfwd=tcp::1-:1"},
		{"add", "tcp", "5000", "5000", "224.0.0.1"},
		{"remove", "tcp", "33033-33036"},
	} {
		if a.ports(args) == nil {
			t.Fatalf("accepted %v", args)
		}
		after, _ := os.ReadFile(a.path("ports.conf"))
		if string(after) != string(before) {
			t.Fatalf("changed configuration after %v", args)
		}
	}
	if err := a.ports([]string{"add", "udp", "33033", "9000"}); err != nil {
		t.Fatal(err)
	}
	if err := a.ports([]string{"add", "tcp", "33033", "9000", "100.85.206.59"}); err != nil {
		t.Fatal(err)
	}
	rules, err := a.loadPorts()
	if err != nil || len(rules) != 5 {
		t.Fatal(rules, err)
	}
	netdev := portNetdev(a.port, rules)
	if !strings.Contains(netdev, "hostfwd=tcp:127.0.0.1:33035-:8002") || !strings.Contains(netdev, "hostfwd=udp:127.0.0.1:33033-:9000") {
		t.Fatal(netdev)
	}
	if err := a.ports([]string{"remove", "tcp", "33033-33035"}); err != nil {
		t.Fatal(err)
	}
	rules, err = a.loadPorts()
	if err != nil || len(rules) != 2 {
		t.Fatal(rules, err)
	}
	// A corrupt file must not prevent stop, and clear can recover it.
	if err := os.WriteFile(a.path("ports.conf"), []byte("tcp 0.0.0.0 99999 80\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = a.loadPorts(); err == nil {
		t.Fatal("accepted corrupt config")
	}
	if err = a.ports([]string{"clear"}); err != nil {
		t.Fatal(err)
	}
	if rules, err = a.loadPorts(); err != nil || len(rules) != 0 {
		t.Fatal(rules, err)
	}
}
func TestPortsLimitAndChangedSSHPort(t *testing.T) {
	a := &app{state: t.TempDir(), port: 22222}
	if err := a.ports([]string{"add", "tcp", "40000-40255", "40000-40255"}); err != nil {
		t.Fatal(err)
	}
	if a.ports([]string{"add", "udp", "50000", "50000"}) == nil {
		t.Fatal("accepted more than 256 mappings")
	}
	a.port = 40000
	if _, err := a.loadPorts(); err == nil {
		t.Fatal("SSH conflict not detected at startup")
	}
}

func TestPortListingWhileControlLockHeld(t *testing.T) {
	a := &app{state: t.TempDir(), port: 22222}
	unlock, err := a.lock("control")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if err = a.dispatch([]string{"ports", "list"}); err != nil {
		t.Fatal("read-only listing blocked", err)
	}
	if err = a.dispatch([]string{"ports", "add", "tcp", "33033", "33033"}); err == nil {
		t.Fatal("configuration changed without acquiring control lock")
	}
}
