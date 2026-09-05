package port_test

import (
	"testing"
	"time"

	"studygo/internal/domain/plano"
	"studygo/internal/port"
)

// O cronograma é feito de datas: "o que estudo hoje" tem que responder o mesmo
// das 00:00 às 23:59 do MESMO dia em Brasília. Com o relógio em UTC, das 21:00
// à meia-noite o dia já era o seguinte.
func TestFuso_ODiaViraNaMeiaNoiteDeBrasilia(t *testing.T) {
	casos := []struct {
		nome string
		hora int
		quer int // dia do mês esperado
	}{
		{"manhã", 8, 5},
		{"noite, antes da antiga virada", 20, 5},
		{"22h — onde o UTC já tinha virado", 22, 5},
		{"23h59", 23, 5},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			instante := time.Date(2026, 9, 5, c.hora, 0, 0, 0, port.Fuso)

			if got := plano.DayOf(instante).Day(); got != c.quer {
				t.Errorf("DayOf(%02dh) = dia %d, quer dia %d", c.hora, got, c.quer)
			}
		})
	}

	// E a madrugada já é o dia novo, que é o ponto de quem estuda tarde.
	madrugada := time.Date(2026, 9, 6, 1, 0, 0, 0, port.Fuso)
	if got := plano.DayOf(madrugada).Day(); got != 6 {
		t.Errorf("DayOf(01h do dia 6) = dia %d, quer dia 6", got)
	}
}

// O offset tem que ser o de Brasília, venha do tzdata ou do fallback.
func TestFuso_EstaEmMenosTres(t *testing.T) {
	_, offset := time.Date(2026, 9, 5, 12, 0, 0, 0, port.Fuso).Zone()

	if want := -3 * 60 * 60; offset != want {
		t.Errorf("offset = %ds, quer %ds (UTC-3)", offset, want)
	}
}
