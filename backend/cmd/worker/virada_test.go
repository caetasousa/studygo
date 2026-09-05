package main

import (
	"testing"
	"time"
)

// A virada tem que cair exatamente em 00:00 do dia seguinte — em qualquer hora
// do dia, na virada de mês e sob horário de verão, que é onde um ticker de 24h
// deriva.
func TestProximaVirada(t *testing.T) {
	sp, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skipf("tzdata indisponível: %v", err)
	}

	casos := []struct {
		nome  string
		agora time.Time
	}{
		{"logo depois da meia-noite", time.Date(2026, 9, 5, 0, 0, 1, 0, time.UTC)},
		{"meio do dia", time.Date(2026, 9, 5, 12, 30, 0, 0, time.UTC)},
		{"um segundo antes", time.Date(2026, 9, 5, 23, 59, 59, 0, time.UTC)},
		{"virada de mês", time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC)},
		{"virada de ano", time.Date(2026, 12, 31, 18, 0, 0, 0, time.UTC)},
		{"fuso negativo", time.Date(2026, 9, 5, 22, 0, 0, 0, sp)},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			chegada := c.agora.Add(proximaVirada(c.agora))

			if h, m, s := chegada.Clock(); h != 0 || m != 0 || s != 0 {
				t.Errorf("chegou às %02d:%02d:%02d, quer 00:00:00", h, m, s)
			}

			if chegada.Day() == c.agora.Day() {
				t.Errorf("chegou no mesmo dia (%s), quer o seguinte", chegada)
			}

			if d := proximaVirada(c.agora); d <= 0 || d > 24*time.Hour {
				t.Errorf("espera = %s, quer entre 0 e 24h", d)
			}
		})
	}
}
