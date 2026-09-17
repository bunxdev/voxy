//go:build darwin || linux

// Native macOS helper; the application bundles this binary, with no Go runtime installation.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bunxdev/voxy/windows/internal/autoports"
)

func save(path string, s autoports.Status) error {
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".ports-state-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 3 && os.Args[1] == "validate" {
		if !autoports.ValidAddress(os.Args[2]) {
			return fmt.Errorf("IPv4 inválida")
		}
		return nil
	}
	if len(os.Args) == 3 && os.Args[1] == "status" {
		state := os.Args[2]
		address, e := autoports.LoadConfig(filepath.Join(state, "ports-auto.conf"))
		if e != nil {
			return e
		}
		if address == "" {
			fmt.Println("Automático: desactivado")
		} else {
			fmt.Println("Automático:", address)
		}
		var s autoports.Status
		b, e := os.ReadFile(filepath.Join(state, "ports-auto.json"))
		session, _ := os.ReadFile(filepath.Join(state, "ports-session"))
		if e != nil || json.Unmarshal(b, &s) != nil || s.Session != strings.TrimSpace(string(session)) {
			return nil
		}
		fmt.Println("Última lectura del detector:", s.Updated.Local().Format(time.RFC3339))
		for _, r := range s.Active {
			fmt.Printf("  %s %s:%d -> Debian:%d\n", r.Protocol, r.Address, r.Host, r.Guest)
		}
		for _, c := range s.Conflicts {
			fmt.Println("  Omitido:", c)
		}
		if s.Error != "" {
			fmt.Println("  Detector:", s.Error)
		}
		if time.Since(s.Updated) > 15*time.Second {
			fmt.Println("  Detector sin actualización reciente; ejecuta start para recuperarlo.")
		}
		return nil
	}

	if len(os.Args) != 3 {
		return fmt.Errorf("voxy-ports STATE LAUNCHER")
	}
	state := os.Args[1]
	path := func(name string) string { return filepath.Join(state, name) }
	f, e := os.OpenFile(path("ports-worker.lck"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	lockErr := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	for i := 0; lockErr != nil && i < 5; i++ {
		time.Sleep(time.Second)
		lockErr = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	}
	if lockErr != nil {
		return nil
	}
	sessionBytes, e := os.ReadFile(path("ports-session"))
	if e != nil {
		return e
	}
	session := strings.TrimSpace(string(sessionBytes))
	q, e := autoports.Dial(path("ports.sock"))
	if e != nil {
		return e
	}
	defer q.Close()
	engine := autoports.Engine{State: autoports.Status{Session: session}, Save: func(s autoports.Status) error { return save(path("ports-auto.json"), s) }, HMP: q.HMP}
	b, e := os.ReadFile(path("ports.active"))
	if e != nil {
		return e
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r autoports.Rule
		if _, e = fmt.Sscan(line, &r.Protocol, &r.Address, &r.Host, &r.Guest); e != nil {
			return e
		}
		engine.Manual = append(engine.Manual, r)
	}
	portBytes, e := os.ReadFile(path("ssh-port"))
	if e != nil {
		return e
	}
	var port int
	if _, e = fmt.Sscan(string(portBytes), &port); e != nil {
		return e
	}
	engine.Manual = append(engine.Manual, autoports.Rule{Protocol: "tcp", Address: "127.0.0.1", Host: port, Guest: 22})
	var old autoports.Status
	if b, e := os.ReadFile(path("ports-auto.json")); e == nil && json.Unmarshal(b, &old) == nil && old.Session == session {
		engine.State = old
	}
	if e = engine.Recover(); e != nil {
		return e
	}
	for {
		b, e := os.ReadFile(path("ports-session"))
		if e != nil || strings.TrimSpace(string(b)) != session {
			return e
		}
		if e = q.Call("query-status", nil, nil); e != nil {
			return e
		}
		address, probeErr := autoports.LoadConfig(path("ports-auto.conf"))
		var output []byte
		if address != "" && probeErr == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			// Execute ssh directly: no shell child may outlive a cancelled probe.
			cmd := exec.CommandContext(ctx, "/usr/bin/ssh", "-i", path("id_ed25519"), "-p", strings.TrimSpace(string(portBytes)), "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile="+path("known_hosts"), "root@127.0.0.1", autoports.Probe)
			output, probeErr = cmd.Output()
			cancel()
		}
		if e = engine.Reconcile(address, string(output), probeErr); e != nil {
			return e
		}
		time.Sleep(3 * time.Second)
	}
}
