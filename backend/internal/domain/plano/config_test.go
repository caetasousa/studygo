package plano_test

import (
	"testing"

	"studygo/internal/domain/plano"
)

// A saved blocosPorDia must survive Normalizar even on a plan whose study
// method was never chosen — otherwise raising it to 3 was reverted to 2 on the
// next load and the schedule kept showing two disciplines a day.
func TestNormalizar_PreservaBlocosPorDia(t *testing.T) {
	t.Parallel()

	tests := []struct {
		nome     string
		blocos   int
		esperado int
	}{
		{"valor salvo pelo usuário é mantido", 3, 3},
		{"nunca definido cai no padrão", 0, 2},
		{"acima do máximo cai no padrão", 99, 2},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			t.Parallel()

			// Simulados vazio: o ramo que antes sobrescrevia tudo.
			c := plano.Config{BlocosPorDia: tt.blocos}

			if got := c.Normalizar().BlocosPorDia; got != tt.esperado {
				t.Errorf("BlocosPorDia = %d, quer %d", got, tt.esperado)
			}
		})
	}
}

// HorasDia is derived from the block count, so raising the blocks must lengthen
// the day rather than leave it stale.
func TestNormalizar_HorasDiaSegueOsBlocos(t *testing.T) {
	t.Parallel()

	// MinutosBloco has to be set: without a block length there is nothing to
	// multiply, and Normalizar keeps HorasDia as-is.
	dois := plano.Config{BlocosPorDia: 2, MinutosBloco: 50}.Normalizar()
	tres := plano.Config{BlocosPorDia: 3, MinutosBloco: 50}.Normalizar()

	if !(tres.HorasDia > dois.HorasDia) {
		t.Errorf("HorasDia com 3 blocos = %v, devia ser maior que com 2 (%v)",
			tres.HorasDia, dois.HorasDia)
	}
}
