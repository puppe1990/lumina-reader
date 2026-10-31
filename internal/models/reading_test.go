package models

import "testing"

func TestStreak(t *testing.T) {
	casos := []struct {
		nome string
		dias []string
		hoje string
		quer int
	}{
		{"vazio", nil, "2026-10-31", 0},
		{"só hoje", []string{"2026-10-31"}, "2026-10-31", 1},
		{"hoje e ontem", []string{"2026-10-31", "2026-10-30"}, "2026-10-31", 2},
		{"buraco para", []string{"2026-10-31", "2026-10-29"}, "2026-10-31", 1},
		{"ontem sem hoje", []string{"2026-10-30", "2026-10-29"}, "2026-10-31", 2},
		{"anteontem zera", []string{"2026-10-29", "2026-10-28"}, "2026-10-31", 0},
		{"virada de mês", []string{"2026-11-01", "2026-10-31", "2026-10-30"}, "2026-11-01", 3},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			if got := Streak(caso.dias, caso.hoje); got != caso.quer {
				t.Errorf("Streak(%v, %s) = %d, want %d", caso.dias, caso.hoje, got, caso.quer)
			}
		})
	}
}
