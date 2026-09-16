package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type qmpClient struct {
	net.Conn
	dec *json.Decoder
	enc *json.Encoder
}

func (a *app) qmp() (*qmpClient, error) {
	c, e := net.DialTimeout("unix", a.path("qmp.sock"), 5*time.Second)
	if e != nil {
		return nil, fmt.Errorf("QMP no disponible; inicia la VM con Voxy 0.4.0: %w", e)
	}
	q := &qmpClient{c, json.NewDecoder(c), json.NewEncoder(c)}
	c.SetDeadline(time.Now().Add(15 * time.Second))
	var greeting map[string]json.RawMessage
	if e = q.dec.Decode(&greeting); e != nil {
		c.Close()
		return nil, e
	}
	if greeting["QMP"] == nil {
		c.Close()
		return nil, fmt.Errorf("Saludo QMP inválido")
	}
	if e = q.call("qmp_capabilities", nil, nil); e != nil {
		c.Close()
		return nil, e
	}
	return q, nil
}
func (q *qmpClient) call(command string, args any, result any) error {
	q.SetDeadline(time.Now().Add(30 * time.Second))
	req := map[string]any{"execute": command, "id": "voxy"}
	if args != nil {
		req["arguments"] = args
	}
	if e := q.enc.Encode(req); e != nil {
		return e
	}
	for {
		var msg struct {
			Return json.RawMessage `json:"return"`
			Error  *struct {
				Desc string `json:"desc"`
			} `json:"error"`
			ID string `json:"id"`
		}
		if e := q.dec.Decode(&msg); e != nil {
			return e
		}
		if msg.ID != "voxy" {
			continue
		}
		if msg.Error != nil {
			return fmt.Errorf("QMP %s: %s", command, msg.Error.Desc)
		}
		if result != nil {
			return json.Unmarshal(msg.Return, result)
		}
		return nil
	}
}
