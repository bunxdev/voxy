package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type backupSettings struct {
	Periodic bool `json:"periodic"`
	Minutes  int  `json:"interval_minutes"`
}

func (a *app) loadBackupSettings() (backupSettings, error) {
	s := backupSettings{Minutes: 10}
	b, err := os.ReadFile(a.path("backup-settings.json"))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(b, &s); err != nil {
		return s, err
	}
	return s, s.validate()
}

func (s backupSettings) validate() error {
	if s.Minutes < 1 || s.Minutes > 10080 {
		return errors.New("El intervalo debe ser de 1 a 10080 minutos")
	}
	return nil
}

func (a *app) saveBackupSettings(s backupSettings) error {
	if err := s.validate(); err != nil {
		return err
	}
	return durableJSON(a.path("backup-settings.json"), s)
}

// The initial attempt is independent of periodic scheduling. Settings are read
// again on each worker tick so changing them does not require a VM restart.
func backupDue(s backupSettings, last, now time.Time) bool {
	return last.IsZero() || (s.Periodic && !now.Before(last.Add(time.Duration(s.Minutes)*time.Minute)))
}

func (a *app) configureBackups(reader *bufio.Reader) error {
	fmt.Print("Copias periódicas: 1) Activar  2) Desactivar (solo copia inicial)  Enter) Cancelar\n> ")
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		return nil
	}
	if choice != "1" && choice != "2" {
		return errors.New("Elige 1 o 2")
	}
	s, err := a.loadBackupSettings()
	if err != nil {
		return err
	}
	s.Periodic = choice == "1"
	if s.Periodic {
		fmt.Print("Intervalo en minutos (1–10080): ")
		line, err = reader.ReadString('\n')
		if err != nil {
			return err
		}
		s.Minutes, err = strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			return errors.New("Escribe un número entero de minutos")
		}
	}
	if err = a.saveBackupSettings(s); err != nil {
		return err
	}
	fmt.Println("Preferencias guardadas. La copia inicial se mantiene; una copia en curso terminará.")
	return nil
}
