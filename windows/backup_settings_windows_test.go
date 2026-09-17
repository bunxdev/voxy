package main

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBackupScheduleDefaultsAndLiveSettings(t *testing.T) {
	a := &app{state: t.TempDir()}
	s, err := a.loadBackupSettings()
	if err != nil || s.Periodic {
		t.Fatalf("default: %+v %v", s, err)
	}
	now := time.Now()
	if !backupDue(s, time.Time{}, now) || backupDue(s, now, now.Add(24*time.Hour)) {
		t.Fatal("expected initial copy only")
	}
	if err = a.configureBackups(bufio.NewReader(strings.NewReader("1\n25\n"))); err != nil {
		t.Fatal(err)
	}
	s, err = a.loadBackupSettings()
	if err != nil || !s.Periodic || s.Minutes != 25 {
		t.Fatalf("saved settings: %+v %v", s, err)
	}
	if backupDue(s, now, now.Add(24*time.Minute)) || !backupDue(s, now, now.Add(25*time.Minute)) {
		t.Fatal("wrong interval")
	}
	if err = a.configureBackups(bufio.NewReader(strings.NewReader("2\n"))); err != nil {
		t.Fatal(err)
	}
	s, err = a.loadBackupSettings()
	if err != nil || backupDue(s, now, now.Add(48*time.Hour)) {
		t.Fatal("disable did not persist", err)
	}
	if !backupDue(s, time.Time{}, now) {
		t.Fatal("initial copy disabled")
	}
}

func TestBackupSettingsRejectInvalidInput(t *testing.T) {
	a := &app{state: t.TempDir()}
	for _, input := range []string{"1\n0\n", "1\n-1\n", "1\n10081\n", "1\nabc\n", "wrong\n"} {
		if err := a.configureBackups(bufio.NewReader(strings.NewReader(input))); err == nil {
			t.Fatal("accepted", input)
		}
	}
	if _, err := os.Stat(a.path("backup-settings.json")); !os.IsNotExist(err) {
		t.Fatal("invalid settings were saved")
	}
	if err := os.WriteFile(a.path("backup-settings.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.loadBackupSettings(); err == nil {
		t.Fatal("corrupt settings accepted")
	}
}
