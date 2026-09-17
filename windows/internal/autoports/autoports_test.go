package autoports

import (
	"errors"
	"strings"
	"testing"
)

const probe = "tcp LISTEN 0 128 0.0.0.0:8080 0.0.0.0:*"

func TestListeners(t *testing.T) {
	input := probe + "\ntcp LISTEN 0 128 10.0.2.15:8080 0.0.0.0:*\ntcp LISTEN 0 128 127.0.0.1:9000 0.0.0.0:*\ntcp LISTEN 0 128 0.0.0.0:22 0.0.0.0:*\nudp UNCONN 0 0 0.0.0.0:8082 0.0.0.0:*\nudp UNCONN 0 0 0.0.0.0:68 0.0.0.0:*"
	r, e := Listeners(input, "127.0.0.1")
	if e != nil || len(r) != 2 || r[0].Host != 8080 || r[1].Protocol != "udp" {
		t.Fatal(r, e)
	}
	if _, e = Listeners("invalid ss output", "127.0.0.1"); e == nil {
		t.Fatal("invalid probe accepted")
	}
}
func TestLifecycle(t *testing.T) {
	var commands []string
	e := Engine{Save: func(Status) error { return nil }, HMP: func(c string) (string, error) {
		commands = append(commands, c)
		if strings.HasPrefix(c, "hostfwd_remove") {
			return "host forwarding rule removed", nil
		}
		return "", nil
	}}
	if err := e.Reconcile("127.0.0.1", probe, nil); err != nil {
		t.Fatal(err)
	}
	e.Reconcile("127.0.0.1", probe, nil)
	if len(commands) != 1 {
		t.Fatal("duplicate add")
	}
	e.Reconcile("127.0.0.1", "", errors.New("ssh failed"))
	if len(commands) != 1 || len(e.State.Active) != 1 {
		t.Fatal("lost rules on probe failure")
	}
	e.Reconcile("0.0.0.0", probe, nil)
	if len(commands) != 3 || e.State.Active[0].Address != "0.0.0.0" {
		t.Fatal(commands)
	}
	e.Reconcile("", "", nil)
	if len(e.State.Active) != 0 || len(commands) != 4 {
		t.Fatal(e.State, commands)
	}
}
func TestConflictsAndRetry(t *testing.T) {
	busy := true
	e := Engine{Manual: []Rule{{"tcp", "0.0.0.0", 8080, 8080}}, Save: func(Status) error { return nil }, HMP: func(string) (string, error) {
		if busy {
			return "Could not set up host forwarding rule", nil
		}
		return "", nil
	}}
	input := probe + "\nudp UNCONN 0 0 0.0.0.0:8082 0.0.0.0:*"
	e.Reconcile("127.0.0.1", input, nil)
	if len(e.State.Active) != 0 || len(e.State.Conflicts) != 2 {
		t.Fatal(e.State)
	}
	busy = false
	e.Reconcile("127.0.0.1", input, nil)
	if len(e.State.Active) != 1 || e.State.Active[0].Protocol != "udp" || len(e.State.Conflicts) != 1 {
		t.Fatal(e.State)
	}
}
func TestJournalBeforeMutation(t *testing.T) {
	e := Engine{Save: func(s Status) error {
		if len(s.Active) > 0 {
			return errors.New("disk full")
		}
		return nil
	}, HMP: func(string) (string, error) { t.Fatal("published before ownership saved"); return "", nil }}
	if e.Reconcile("127.0.0.1", probe, nil) == nil {
		t.Fatal("expected write failure")
	}
}
func TestTransportFailureKeepsJournal(t *testing.T) {
	e := Engine{Save: func(Status) error { return nil }, HMP: func(string) (string, error) { return "", errors.New("connection lost") }}
	if e.Reconcile("127.0.0.1", probe, nil) == nil || len(e.State.Active) != 1 {
		t.Fatal("ambiguous operation must remain journaled")
	}
}
func TestRecoverOnlyOwned(t *testing.T) {
	count := 0
	e := Engine{State: Status{Active: []Rule{{"tcp", "127.0.0.1", 8080, 8080}}}, Save: func(Status) error { return nil }, HMP: func(c string) (string, error) {
		count++
		if c != "hostfwd_remove net tcp:127.0.0.1:8080" {
			t.Fatal(c)
		}
		return "host forwarding rule removed", nil
	}}
	if err := e.Recover(); err != nil || count != 1 || len(e.State.Active) != 0 {
		t.Fatal(err, e.State)
	}
}

func TestDualStack(t *testing.T) {
	output := "tcp LISTEN 0 4096 *:8080 *:* v6only:0\nudp UNCONN 0 0 *:8082 *:* v6only:0\ntcp LISTEN 0 128 [::]:9000 [::]:* v6only:1\ntcp LISTEN 0 128 [::1]:9001 [::]:* v6only:0"
	r, err := Listeners(output, "127.0.0.1")
	if err != nil || len(r) != 2 {
		t.Fatal(r, err)
	}
}
