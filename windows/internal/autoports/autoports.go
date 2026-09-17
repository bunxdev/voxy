// Package autoports reconciles guest listeners with QEMU host forwarding.
package autoports

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const Probe = "LC_ALL=C ss -H -lntu -e"

type Rule struct {
	Protocol, Address string
	Host, Guest       int
}
type Status struct {
	Session   string
	Updated   time.Time
	Address   string
	Active    []Rule
	Conflicts []string
	Error     string
}

func ValidAddress(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil && ip.String() == s && !ip.IsMulticast() && s != "255.255.255.255"
}

// Missing configuration means disabled. Never interpret malformed data as a bind.
func LoadConfig(path string) (string, error) {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	s := strings.TrimSpace(string(b))
	if s == "off" {
		return "", nil
	}
	if !ValidAddress(s) {
		return "", fmt.Errorf("Configuración automática inválida: %q", s)
	}
	return s, nil
}
func Overlap(a, b Rule) bool {
	return a.Protocol == b.Protocol && a.Host == b.Host && (a.Address == b.Address || a.Address == "0.0.0.0" || b.Address == "0.0.0.0")
}

// Include dual-stack wildcard sockets, but never IPv6-only or loopback listeners.
func Listeners(output, address string) ([]Rule, error) {
	unique := map[Rule]bool{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 6 || (f[0] != "tcp" && f[0] != "udp") {
			return nil, fmt.Errorf("Salida ss no reconocida: %s", line)
		}
		if (f[0] == "tcp" && f[1] != "LISTEN") || (f[0] == "udp" && f[1] != "UNCONN") {
			continue
		}
		host, port, e := net.SplitHostPort(f[4])
		if e != nil {
			return nil, e
		}
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("Puerto ss inválido")
		}
		host = strings.SplitN(host, "%", 2)[0]
		ip := net.ParseIP(host)
		if strings.Contains(line, "v6only:1") {
			continue
		}
		if ip != nil && ip.To4() == nil && !(ip.IsUnspecified() && strings.Contains(line, "v6only:0")) {
			continue
		}
		if host != "*" && (ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast()) {
			continue
		}
		if (f[0] == "tcp" && n == 22) || (f[0] == "udp" && (n == 67 || n == 68)) {
			continue
		}
		unique[Rule{f[0], address, n, n}] = true
	}
	result := []Rule{}
	for r := range unique {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Protocol != result[j].Protocol {
			return result[i].Protocol < result[j].Protocol
		}
		return result[i].Host < result[j].Host
	})
	return result, nil
}

type Engine struct {
	State  Status
	Manual []Rule
	// Save must atomically replace the status, also used as an ownership journal.
	Save func(Status) error
	HMP  func(string) (string, error)
}

func (e *Engine) save() error { e.State.Updated = time.Now().UTC(); return e.Save(e.State) }

type transportError struct{ error }

func (e *Engine) command(add bool, r Rule) error {
	cmd := fmt.Sprintf("hostfwd_remove net %s:%s:%d", r.Protocol, r.Address, r.Host)
	if add {
		cmd = fmt.Sprintf("hostfwd_add net %s:%s:%d-:%d", r.Protocol, r.Address, r.Host, r.Guest)
	}
	result, err := e.HMP(cmd)
	if err != nil {
		return transportError{err}
	}
	if add && strings.TrimSpace(result) != "" {
		return fmt.Errorf("%s", strings.TrimSpace(result))
	}
	if !add && !strings.Contains(result, "removed") {
		return fmt.Errorf("%s", strings.TrimSpace(result))
	}
	return nil
}
func (e *Engine) Reconcile(address, output string, probeErr error) error {
	e.State.Address = address
	e.State.Error = ""
	e.State.Conflicts = nil
	desired := []Rule{}
	var err error
	if address != "" && probeErr == nil {
		desired, err = Listeners(output, address)
		probeErr = err
	}
	if probeErr != nil {
		// Preserve existing mappings on transient SSH failure; a failed probe is not
		// evidence that services have stopped. Disabling still removes owned rules.
		e.State.Error = probeErr.Error()
		if address != "" {
			return e.save()
		}
	}
	wanted := map[Rule]bool{}
	for _, r := range desired {
		blocked := false
		for _, m := range e.Manual {
			if Overlap(r, m) {
				blocked = true
				break
			}
		}
		if blocked {
			e.State.Conflicts = append(e.State.Conflicts, fmt.Sprintf("%s %s:%d: reservado por regla manual/SSH", r.Protocol, r.Address, r.Host))
			continue
		}
		wanted[r] = true
	}
	kept := []Rule{}
	for _, r := range e.State.Active {
		if !wanted[r] {
			if err = e.command(false, r); err != nil {
				// A journaled add may have failed before a crash. QEMU reports not found.
				if !strings.Contains(err.Error(), "not found") {
					kept = append(kept, r)
					e.State.Conflicts = append(e.State.Conflicts, err.Error())
				}
			}
		} else {
			kept = append(kept, r)
		}
	}
	e.State.Active = kept
	if err = e.save(); err != nil {
		return err
	}
	for _, r := range desired {
		if !wanted[r] {
			continue
		}
		exists := false
		for _, a := range e.State.Active {
			if a == r {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		// Journal ownership before mutation: a crash cannot orphan a successful add.
		e.State.Active = append(e.State.Active, r)
		if err = e.save(); err != nil {
			return err
		}
		if err = e.command(true, r); err != nil {
			var transport transportError
			if errors.As(err, &transport) {
				return err
			}
			e.State.Active = e.State.Active[:len(e.State.Active)-1]
			e.State.Conflicts = append(e.State.Conflicts, fmt.Sprintf("%s %s:%d: %s", r.Protocol, r.Address, r.Host, err))
		}
		if err = e.save(); err != nil {
			return err
		}
	}
	return e.save()
}

// Recover removes only this worker's journaled mappings, never manual rules.
func (e *Engine) Recover() error {
	for _, r := range e.State.Active {
		if (r.Protocol != "tcp" && r.Protocol != "udp") || !ValidAddress(r.Address) || r.Host < 1 || r.Host > 65535 || r.Guest != r.Host {
			return fmt.Errorf("Estado automático inválido")
		}
		for _, m := range e.Manual {
			if Overlap(r, m) {
				return fmt.Errorf("Estado automático solapa regla manual")
			}
		}
		if err := e.command(false, r); err != nil && !strings.Contains(err.Error(), "not found") {
			return err
		}
	}
	e.State.Active = nil
	return e.save()
}

type QMP struct {
	net.Conn
	dec *json.Decoder
	enc *json.Encoder
}

func Dial(path string) (*QMP, error) {
	c, err := net.DialTimeout("unix", path, 3*time.Second)
	if err != nil {
		return nil, err
	}
	q := &QMP{c, json.NewDecoder(c), json.NewEncoder(c)}
	c.SetDeadline(time.Now().Add(5 * time.Second))
	var greeting map[string]json.RawMessage
	if err = q.dec.Decode(&greeting); err == nil && greeting["QMP"] == nil {
		err = fmt.Errorf("Saludo QMP inválido")
	}
	if err == nil {
		err = q.Call("qmp_capabilities", nil, nil)
	}
	if err != nil {
		c.Close()
		return nil, err
	}
	return q, nil
}
func (q *QMP) Call(command string, args any, out any) error {
	q.SetDeadline(time.Now().Add(5 * time.Second))
	request := map[string]any{"execute": command, "id": "ports"}
	if args != nil {
		request["arguments"] = args
	}
	if err := q.enc.Encode(request); err != nil {
		return err
	}
	for {
		var r struct {
			ID     string
			Return json.RawMessage
			Error  *struct{ Desc string }
		}
		if err := q.dec.Decode(&r); err != nil {
			return err
		}
		if r.ID != "ports" {
			continue
		}
		if r.Error != nil {
			return fmt.Errorf("QMP: %s", r.Error.Desc)
		}
		if out != nil {
			return json.Unmarshal(r.Return, out)
		}
		return nil
	}
}
func (q *QMP) HMP(command string) (string, error) {
	var out string
	err := q.Call("human-monitor-command", map[string]string{"command-line": command}, &out)
	return out, err
}
