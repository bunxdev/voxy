package main

import (
	"io"
	"os"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestVirtualTerminalLeavesPipesUnchanged(t *testing.T) {
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	restore, e := enableVirtualTerminal(w)
	if e != nil {
		t.Fatal(e)
	}
	defer restore()
	want := "\x1b[?2004hVOXY\x1b[?2004l"
	if _, e = w.WriteString(want); e != nil {
		t.Fatal(e)
	}
	w.Close()
	b, e := io.ReadAll(r)
	if e != nil || string(b) != want {
		t.Fatalf("redirected output modified: %q %v", b, e)
	}
}

func TestWindowsConsoleRendersControlSequences(t *testing.T) {
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	// SSH test runners have pipes and no console. Allocate one only for this test.
	name, _ := windows.UTF16PtrFromString("CONOUT$")
	open := func() (windows.Handle, error) {
		return windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	}
	handle, err := open()
	if err != nil {
		ids := []uint32{windows.STD_INPUT_HANDLE, windows.STD_OUTPUT_HANDLE, windows.STD_ERROR_HANDLE}
		original := make([]windows.Handle, len(ids))
		for i, id := range ids {
			original[i], _ = windows.GetStdHandle(id)
		}
		ok, _, e := kernel.NewProc("AllocConsole").Call()
		if ok == 0 {
			t.Fatalf("AllocConsole: %v", e)
		}
		defer func() {
			kernel.NewProc("FreeConsole").Call()
			for i, id := range ids {
				_ = windows.SetStdHandle(id, original[i])
			}
		}()
		handle, err = open()
	}
	if err != nil {
		t.Fatal(err)
	}
	file := os.NewFile(uintptr(handle), "CONOUT$")
	defer file.Close()
	var original uint32
	if err = windows.GetConsoleMode(handle, &original); err != nil {
		t.Fatal(err)
	}
	defer windows.SetConsoleMode(handle, original)
	legacy := original &^ windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if err = windows.SetConsoleMode(handle, legacy); err != nil {
		t.Fatal(err)
	}
	read := func(row uint16) string {
		buf := make([]uint16, 100)
		var count uint32
		ok, _, e := kernel.NewProc("ReadConsoleOutputCharacterW").Call(uintptr(handle), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(uint32(row)<<16), uintptr(unsafe.Pointer(&count)))
		if ok == 0 {
			t.Fatalf("ReadConsoleOutputCharacterW: %v", e)
		}
		return strings.TrimSpace(windows.UTF16ToString(buf[:count]))
	}
	sequence := "\x1b[?2004h\x1b[32mVOXY\x1b[0m\x1b[?2004l"
	if err = windows.SetConsoleCursorPosition(handle, windows.Coord{X: 0, Y: 0}); err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString(sequence); err != nil {
		t.Fatal(err)
	}
	before := read(0)
	if !strings.Contains(before, "?2004") {
		t.Fatalf("could not reproduce legacy console symptom: %q", before)
	}
	restore, err := enableVirtualTerminal(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = windows.SetConsoleCursorPosition(handle, windows.Coord{X: 0, Y: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString("\x1b[2K" + sequence); err != nil {
		t.Fatal(err)
	}
	after := read(2)
	restore()
	if after != "VOXY" {
		t.Fatalf("control sequences visible: %q", after)
	}
	var got uint32
	if err = windows.GetConsoleMode(handle, &got); err != nil || got != legacy {
		t.Fatalf("mode not restored: %x, wanted %x: %v", got, legacy, err)
	}
	t.Logf("Legacy: %q; with VT: %q; console mode restored", before, after)
}
