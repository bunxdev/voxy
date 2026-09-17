package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"time"

	"github.com/bunxdev/voxy/windows/internal/autoports"
)

func (a *app) autoPortsConfig(args []string) error {
	value := "off"
	if len(args) == 1 && args[0] == "off" {
	} else if (len(args) == 1 || len(args) == 2) && args[0] == "on" {
		value = "127.0.0.1"
		if len(args) == 2 {
			value = args[1]
		}
		if !autoports.ValidAddress(value) {
			return fmt.Errorf("IPv4 inválida")
		}
	} else {
		return fmt.Errorf("Uso: ports auto on [IPv4] | ports auto off")
	}
	if err := durableBytes(a.path("ports-auto.conf"), []byte(value+"\n")); err != nil {
		return err
	}
	fmt.Println("Modo automático guardado:", value, "(se aplica en unos segundos si Debian está encendido)")
	return a.startPortsWorker()
}
func (a *app) printAutoPorts(alive bool, r *runState) error {
	address, err := autoports.LoadConfig(a.path("ports-auto.conf"))
	if err != nil {
		return err
	}
	if address == "" {
		fmt.Println("Automático: desactivado")
	} else {
		fmt.Println("Automático:", address)
	}
	if !alive {
		return nil
	}
	var s autoports.Status
	b, err := os.ReadFile(a.path("ports-auto.json"))
	if err != nil || json.Unmarshal(b, &s) != nil || s.Session != fmt.Sprint(r.Created) {
		if address != "" {
			fmt.Println("  Esperando al detector; usa start para iniciarlo.")
		}
		return nil
	}
	fmt.Println("Automáticos activos (última lectura", s.Updated.Local().Format(time.RFC3339)+"):")
	for _, p := range s.Active {
		fmt.Printf("  %s %s:%d -> Debian:%d\n", p.Protocol, p.Address, p.Host, p.Guest)
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
func (a *app) startPortsWorker() error {
	_, alive, err := a.running()
	if err != nil || !alive {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "ports-worker")
	cmd.Env = append(os.Environ(), "VOXY_DATA_DIR="+a.state)
	cmd.SysProcAttr = &windowsSysProcAttr
	log, err := os.OpenFile(a.path("ports-worker.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd.Stdout = log
	cmd.Stderr = log
	if err = cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
func (a *app) portsWorker() error {
	unlock, err := a.lock("ports-worker")
	for i := 0; err != nil && i < 30; i++ {
		current, alive, e := a.running()
		if e != nil || !alive {
			return e
		}
		var status autoports.Status
		if b, e := os.ReadFile(a.path("ports-auto.json")); e == nil && json.Unmarshal(b, &status) == nil && status.Session == fmt.Sprint(current.Created) {
			return nil
		}
		time.Sleep(time.Second)
		unlock, err = a.lock("ports-worker")
	}
	if err != nil {
		return nil
	}
	defer unlock()
	r, alive, err := a.running()
	if err != nil || !alive {
		return err
	}
	session := fmt.Sprint(r.Created)
	q, err := autoports.Dial(a.path("ports.sock"))
	// On Windows CreateProcess returns before QEMU has opened its monitors.
	for i := 0; err != nil && i < 30; i++ {
		current, alive, e := a.running()
		if e != nil || !alive || current.Created != r.Created {
			return e
		}
		time.Sleep(time.Second)
		q, err = autoports.Dial(a.path("ports.sock"))
	}
	if err != nil {
		return err
	}
	defer q.Close()
	engine := autoports.Engine{State: autoports.Status{Session: session}, Save: a.saveAutoPorts, HMP: q.HMP}
	for _, p := range r.Ports {
		engine.Manual = append(engine.Manual, autoports.Rule{Protocol: p.Protocol, Address: p.Address, Host: p.Host, Guest: p.Guest})
	}
	engine.Manual = append(engine.Manual, autoports.Rule{Protocol: "tcp", Address: "127.0.0.1", Host: r.Port, Guest: 22})
	var old autoports.Status
	if b, e := os.ReadFile(a.path("ports-auto.json")); e == nil && json.Unmarshal(b, &old) == nil && old.Session == session {
		engine.State = old
	}
	if err = engine.Recover(); err != nil {
		return err
	}
	for {
		current, alive, err := a.running()
		if err != nil || !alive || current.Created != r.Created {
			return err
		}
		if err = q.Call("query-status", nil, nil); err != nil {
			return err
		}
		address, configErr := autoports.LoadConfig(a.path("ports-auto.conf"))
		var output string
		probeErr := configErr
		if address != "" && configErr == nil {
			// Closing the SSH transport bounds a stuck remote command as well as reads.
			c, e := a.client()
			probeErr = e
			if e == nil {
				timer := time.AfterFunc(8*time.Second, func() { c.Close() })
				s, e := c.NewSession()
				probeErr = e
				if e == nil {
					b, e := s.Output(autoports.Probe)
					output = string(b)
					probeErr = e
					s.Close()
				}
				timer.Stop()
				c.Close()
			}
		}
		if err = engine.Reconcile(address, output, probeErr); err != nil {
			return err
		}
		time.Sleep(3 * time.Second)
	}
}

// Readers and virus scanners may briefly deny atomic replacement on Windows.
// Preserve the ownership journal and retry only transient sharing failures.
func (a *app) saveAutoPorts(s autoports.Status) error {
	var err error
	for i := 0; i < 25; i++ {
		err = durableJSON(a.path("ports-auto.json"), s)
		if err == nil {
			return nil
		}
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			break
		}
		time.Sleep(40 * time.Millisecond)
	}
	return fmt.Errorf("Guardar estado de puertos automáticos: %w", err)
}
