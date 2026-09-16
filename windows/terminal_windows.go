package main

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// SSH sends xterm control sequences. Windows console buffers only interpret
// them after VT output is enabled. Pipes/redirected streams remain untouched.
func enableVirtualTerminal(files ...*os.File) (func(), error) {
	type savedMode struct {
		handle windows.Handle
		mode   uint32
	}
	var saved []savedMode
	restore := func() {
		for i := len(saved) - 1; i >= 0; i-- {
			_ = windows.SetConsoleMode(saved[i].handle, saved[i].mode)
		}
	}
	for _, file := range files {
		handle := windows.Handle(file.Fd())
		var mode uint32
		if windows.GetConsoleMode(handle, &mode) != nil {
			continue
		}
		if err := windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING|windows.ENABLE_PROCESSED_OUTPUT); err != nil {
			restore()
			return nil, fmt.Errorf("No se pudo activar la terminal ANSI: %w", err)
		}
		saved = append(saved, savedMode{handle, mode})
	}
	return restore, nil
}

// On Windows the size belongs to the screen buffer (stdout), not stdin.
func terminalSize() (int, int, error) { return term.GetSize(int(os.Stdout.Fd())) }

func watchTerminalSize(done <-chan struct{}, resize func(int, int) error, width, height int) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			w, h, err := terminalSize()
			if err == nil && w > 0 && h > 0 && (w != width || h != height) {
				if resize(h, w) != nil {
					return
				}
				width, height = w, h
			}
		}
	}
}
