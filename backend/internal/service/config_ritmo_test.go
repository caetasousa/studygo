package service

import (
	"testing"

	"studygo/internal/domain/plano"
)

// Only a change to how a day is filled — blocks per day or block length — has to
// release the materialised future days back to the engine. Everything else
// (dates, question counts, the review cycle) leaves the layout alone.
func TestRitmoMudou(t *testing.T) {
	t.Parallel()

	base := plano.ConfigPadrao()
	base.BlocosPorDia = 2
	base.MinutosBloco = 50

	tests := []struct {
		nome string
		muda func(*plano.Config)
		quer bool
	}{
		{"nada", func(*plano.Config) {}, false},
		{"blocosPorDia", func(c *plano.Config) { c.BlocosPorDia = 4 }, true},
		{"minutosBloco", func(c *plano.Config) { c.MinutosBloco = 90 }, true},
		{"simulados", func(c *plano.Config) { c.Simulados = plano.SimuladoNunca }, false},
		{"pctQuestoes", func(c *plano.Config) { c.PctQuestoes = 0.7 }, false},
	}

	for _, tc := range tests {
		t.Run(tc.nome, func(t *testing.T) {
			t.Parallel()

			depois := base
			tc.muda(&depois)

			if got := ritmoMudou(base, depois); got != tc.quer {
				t.Errorf("ritmoMudou = %v, quer %v", got, tc.quer)
			}
		})
	}
}
