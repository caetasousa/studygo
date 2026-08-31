package plano_test

import (
	"errors"
	"testing"
	"time"

	"studygo/internal/domain/plano"
)

// Três dias de estudo seguidos, com duas atividades cada.
func diasReplan() []plano.Dia {
	return []plano.Dia{
		{N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo},
		{N: 2, Data: dia(2026, 9, 2), Tipo: plano.TipoEstudo},
		{N: 3, Data: dia(2026, 9, 3), Tipo: plano.TipoEstudo},
		{N: 4, Data: dia(2026, 9, 4), Tipo: plano.TipoEstudo},
	}
}

func atividadesReplan() []plano.Atividade {
	return []plano.Atividade{
		{ID: "a1", Data: dia(2026, 9, 1), Posicao: 0, Disciplina: "POR", Tema: "p1"},
		{ID: "a2", Data: dia(2026, 9, 1), Posicao: 1, Disciplina: "MAT", Tema: "m1"},
		{ID: "b1", Data: dia(2026, 9, 2), Posicao: 0, Disciplina: "POR", Tema: "p2"},
		{ID: "b2", Data: dia(2026, 9, 2), Posicao: 1, Disciplina: "MAT", Tema: "m2"},
		{ID: "c1", Data: dia(2026, 9, 3), Posicao: 0, Disciplina: "POR", Tema: "p3"},
	}
}

func idsDoDia(as []plano.Atividade, dt time.Time) []string {
	out := []string{}
	for _, a := range plano.AtividadesDoDia(as, dt) {
		out = append(out, a.ID)
	}

	return out
}

func TestAdiarDia(t *testing.T) {
	t.Parallel()

	t.Run("empurra o conteúdo e desloca o resto", func(t *testing.T) {
		t.Parallel()

		// Perdi o dia 2: o que era dele vai para o 3, e o que era do 3 vai para o 4.
		got, err := plano.AdiarDia(
			atividadesReplan(), diasReplan(), dia(2026, 9, 2), nadaConcluido,
		)
		if err != nil {
			t.Fatalf("AdiarDia: %v", err)
		}

		if ids := idsDoDia(got, dia(2026, 9, 2)); len(ids) != 0 {
			t.Errorf("o dia adiado ficou com %v, devia esvaziar", ids)
		}

		if ids := idsDoDia(got, dia(2026, 9, 3)); len(ids) != 2 || ids[0] != "b1" {
			t.Errorf("dia 3 = %v, quer [b1 b2]", ids)
		}

		if ids := idsDoDia(got, dia(2026, 9, 4)); len(ids) != 1 || ids[0] != "c1" {
			t.Errorf("dia 4 = %v, quer [c1]", ids)
		}
	})

	t.Run("não mexe no que veio antes", func(t *testing.T) {
		t.Parallel()

		got, err := plano.AdiarDia(
			atividadesReplan(), diasReplan(), dia(2026, 9, 2), nadaConcluido,
		)
		if err != nil {
			t.Fatalf("AdiarDia: %v", err)
		}

		if ids := idsDoDia(got, dia(2026, 9, 1)); len(ids) != 2 || ids[0] != "a1" {
			t.Errorf("dia 1 = %v, devia seguir intacto", ids)
		}
	})

	t.Run("nada se perde", func(t *testing.T) {
		t.Parallel()

		got, err := plano.AdiarDia(
			atividadesReplan(), diasReplan(), dia(2026, 9, 2), nadaConcluido,
		)
		if err != nil {
			t.Fatalf("AdiarDia: %v", err)
		}

		if len(got) != len(atividadesReplan()) {
			t.Errorf("total = %d, quer %d", len(got), len(atividadesReplan()))
		}
	})

	t.Run("recusa adiar um dia já concluído", func(t *testing.T) {
		t.Parallel()

		concluido := func(d time.Time) bool { return d.Equal(dia(2026, 9, 2)) }

		_, err := plano.AdiarDia(atividadesReplan(), diasReplan(), dia(2026, 9, 2), concluido)
		if !errors.Is(err, plano.ErrDiaConcluido) {
			t.Errorf("erro = %v, quer ErrDiaConcluido", err)
		}
	})

	t.Run("recusa quando não há para onde empurrar", func(t *testing.T) {
		t.Parallel()

		_, err := plano.AdiarDia(atividadesReplan(), diasReplan(), dia(2026, 9, 4), nadaConcluido)
		if !errors.Is(err, plano.ErrDestinoInvalido) {
			t.Errorf("erro = %v, quer ErrDestinoInvalido", err)
		}
	})
}

func TestAntecipouAtividade(t *testing.T) {
	t.Parallel()

	t.Run("traz o assunto adiantado para hoje", func(t *testing.T) {
		t.Parallel()

		// Estou no dia 1 e terminei c1, que estava no dia 3.
		got, err := plano.AntecipouAtividade(
			atividadesReplan(), diasReplan(), "c1", dia(2026, 9, 1), nadaConcluido,
		)
		if err != nil {
			t.Fatalf("AntecipouAtividade: %v", err)
		}

		ids := idsDoDia(got, dia(2026, 9, 1))
		if len(ids) != 3 || ids[2] != "c1" {
			t.Errorf("dia 1 = %v, quer c1 no fim", ids)
		}

		if ids := idsDoDia(got, dia(2026, 9, 3)); len(ids) != 0 {
			t.Errorf("dia 3 = %v, devia ficar vazio", ids)
		}
	})

	t.Run("a ordem do que ficou é preservada e densa", func(t *testing.T) {
		t.Parallel()

		// Trago b1 (dia 2, posição 0): b2 sobe para a posição 0.
		got, err := plano.AntecipouAtividade(
			atividadesReplan(), diasReplan(), "b1", dia(2026, 9, 1), nadaConcluido,
		)
		if err != nil {
			t.Fatalf("AntecipouAtividade: %v", err)
		}

		restantes := plano.AtividadesDoDia(got, dia(2026, 9, 2))
		if len(restantes) != 1 || restantes[0].ID != "b2" || restantes[0].Posicao != 0 {
			t.Errorf("dia 2 = %+v, quer só b2 na posição 0", restantes)
		}
	})

	t.Run("nada se perde nem se duplica", func(t *testing.T) {
		t.Parallel()

		got, err := plano.AntecipouAtividade(
			atividadesReplan(), diasReplan(), "c1", dia(2026, 9, 1), nadaConcluido,
		)
		if err != nil {
			t.Fatalf("AntecipouAtividade: %v", err)
		}

		if len(got) != len(atividadesReplan()) {
			t.Errorf("total = %d, quer %d", len(got), len(atividadesReplan()))
		}
	})

	t.Run("assunto de hoje ou do passado não se move", func(t *testing.T) {
		t.Parallel()

		got, err := plano.AntecipouAtividade(
			atividadesReplan(), diasReplan(), "a1", dia(2026, 9, 2), nadaConcluido,
		)
		if err != nil {
			t.Fatalf("AntecipouAtividade: %v", err)
		}

		if ids := idsDoDia(got, dia(2026, 9, 1)); len(ids) != 2 {
			t.Errorf("dia 1 = %v, não devia mudar", ids)
		}
	})

	t.Run("id desconhecido é recusado", func(t *testing.T) {
		t.Parallel()

		_, err := plano.AntecipouAtividade(
			atividadesReplan(), diasReplan(), "zzz", dia(2026, 9, 1), nadaConcluido,
		)
		if !errors.Is(err, plano.ErrAtividadeNaoEncontrada) {
			t.Errorf("erro = %v, quer ErrAtividadeNaoEncontrada", err)
		}
	})
}
