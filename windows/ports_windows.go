package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const maxForwardedPorts = 256

type portRule struct {
	Protocol string
	Address  string
	Host     int
	Guest    int
}

func portRange(s string) (int, int, error) {
	parts := strings.Split(s, "-")
	if len(parts) > 2 {
		return 0, 0, errors.New("Puerto o rango inválido")
	}
	values := []int{}
	for _, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return 0, 0, errors.New("Puerto inválido")
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return 0, 0, errors.New("Puerto fuera de 1–65535")
		}
		values = append(values, n)
	}
	end := values[0]
	if len(values) == 2 {
		end = values[1]
	}
	if end < values[0] || end-values[0]+1 > maxForwardedPorts {
		return 0, 0, errors.New("Rango invertido o mayor de 256 puertos")
	}
	return values[0], end, nil
}

func validBind(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil && ip.String() == s && !ip.IsMulticast() && s != "255.255.255.255"
}

func validatePorts(rules []portRule, sshPort int) error {
	if len(rules) > maxForwardedPorts {
		return errors.New("Máximo 256 puertos reenviados por VM")
	}
	for i, r := range rules {
		if (r.Protocol != "tcp" && r.Protocol != "udp") || !validBind(r.Address) || r.Host < 1 || r.Host > 65535 || r.Guest < 1 || r.Guest > 65535 {
			return fmt.Errorf("Regla de puertos inválida: %v", r)
		}
		if r.Protocol == "tcp" && r.Host == sshPort && (r.Address == "0.0.0.0" || r.Address == "127.0.0.1") {
			return errors.New("La regla entra en conflicto con el puerto SSH de Voxy")
		}
		for _, other := range rules[:i] {
			if r.Protocol == other.Protocol && r.Host == other.Host && (r.Address == other.Address || r.Address == "0.0.0.0" || other.Address == "0.0.0.0") {
				return errors.New("Reglas duplicadas o direcciones de escucha solapadas")
			}
		}
	}
	return nil
}
func encodePorts(rules []portRule) []byte {
	var b strings.Builder
	for _, r := range rules {
		fmt.Fprintf(&b, "%s %s %d %d\n", r.Protocol, r.Address, r.Host, r.Guest)
	}
	return []byte(b.String())
}
func (a *app) loadPorts() ([]portRule, error) {
	b, err := os.ReadFile(a.path("ports.conf"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rules []portRule
	scanner := bufio.NewScanner(strings.NewReader(string(b)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 4 {
			return nil, errors.New("ports.conf: se esperan protocolo dirección puerto-host puerto-Debian")
		}
		h, hEnd, err := portRange(f[2])
		if err != nil || h != hEnd {
			return nil, errors.New("ports.conf: puerto host inválido")
		}
		g, gEnd, err := portRange(f[3])
		if err != nil || g != gEnd {
			return nil, errors.New("ports.conf: puerto Debian inválido")
		}
		rules = append(rules, portRule{f[0], f[1], h, g})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rules, validatePorts(rules, a.port)
}
func portNetdev(sshPort int, rules []portRule) string {
	s := fmt.Sprintf("user,id=net,hostfwd=tcp:127.0.0.1:%d-:22", sshPort)
	for _, r := range rules {
		s += fmt.Sprintf(",hostfwd=%s:%s:%d-:%d", r.Protocol, r.Address, r.Host, r.Guest)
	}
	return s
}
func printPorts(rules []portRule) {
	if len(rules) == 0 {
		fmt.Println("  (ninguno)")
	}
	for _, r := range rules {
		fmt.Printf("  %s %s:%d -> Debian:%d\n", r.Protocol, r.Address, r.Host, r.Guest)
	}
}
func (a *app) ports(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}
	if args[0] == "list" && len(args) == 1 {
		rules, err := a.loadPorts()
		if err != nil {
			return err
		}
		fmt.Println("Configurados para el próximo arranque:")
		printPorts(rules)
		r, alive, err := a.running()
		if err != nil {
			return err
		}
		if alive {
			fmt.Println("Activos en la VM actual:")
			printPorts(r.Ports)
		}
		if err := a.printAutoPorts(alive, r); err != nil {
			return err
		}
		fmt.Println("Archivo:", a.path("ports.conf"))
		return nil
	}
	if args[0] == "auto" {
		return a.autoPortsConfig(args[1:])
	}
	var rules []portRule
	if args[0] != "clear" {
		var err error
		rules, err = a.loadPorts()
		if err != nil {
			return err
		}
	}
	switch args[0] {
	case "add":
		if len(args) != 4 && len(args) != 5 {
			return portsUsage()
		}
		addr := "127.0.0.1"
		if len(args) == 5 {
			addr = args[4]
		}
		h, end, err := portRange(args[2])
		if err != nil {
			return err
		}
		g, gEnd, err := portRange(args[3])
		if err != nil {
			return err
		}
		if end-h != gEnd-g {
			return errors.New("Los rangos host y Debian deben tener igual longitud")
		}
		for p := h; p <= end; p++ {
			rules = append(rules, portRule{args[1], addr, p, g + p - h})
		}
	case "remove":
		if len(args) != 3 && len(args) != 4 {
			return portsUsage()
		}
		addr := "127.0.0.1"
		if len(args) == 4 {
			addr = args[3]
		}
		h, end, err := portRange(args[2])
		if err != nil {
			return err
		}
		if !validBind(addr) || (args[1] != "tcp" && args[1] != "udp") {
			return portsUsage()
		}
		kept := []portRule{}
		found := 0
		for _, r := range rules {
			if r.Protocol == args[1] && r.Address == addr && r.Host >= h && r.Host <= end {
				found++
			} else {
				kept = append(kept, r)
			}
		}
		if found != end-h+1 {
			return errors.New("No existen todos los puertos solicitados; no se modificó la configuración")
		}
		rules = kept
	case "clear":
		if len(args) != 1 {
			return portsUsage()
		}
	default:
		return portsUsage()
	}
	if err := validatePorts(rules, a.port); err != nil {
		return err
	}
	if err := durableBytes(a.path("ports.conf"), encodePorts(rules)); err != nil {
		return err
	}
	fmt.Println("Puertos guardados. Se aplican al apagar e iniciar Debian.")
	return nil
}
func portsUsage() error {
	return errors.New("Uso: ports list | add tcp|udp HOST[-FIN] DEBIAN[-FIN] [IPv4] | remove tcp|udp HOST[-FIN] [IPv4] | clear | auto on [IPv4] | auto off. IPv4 por defecto: 127.0.0.1; 0.0.0.0: todas las interfaces")
}
func (a *app) configurePorts(reader *bufio.Reader) error {
	if err := a.ports([]string{"list"}); err != nil {
		fmt.Println(err)
	}
	fmt.Println(portsUsage())
	fmt.Print("Operación (ejemplo: add tcp 33033 33033 0.0.0.0; Enter cancela): ")
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	return a.dispatch(append([]string{"ports"}, fields...))
}
