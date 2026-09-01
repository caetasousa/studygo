package plano_test

import (
	"testing"

	"studygo/internal/domain/plano"
)

// diaEstudo builds one base-phase study day.
func diaEstudo(n int, itens ...plano.ItemDia) plano.Dia {
	return plano.Dia{N: n, Tipo: plano.TipoEstudo, Fase: plano.FaseBase, Itens: itens}
}

func item(disc, tema string) plano.ItemDia {
	return plano.ItemDia{Disciplina: disc, Tema: tema}
}

func TestFilaRevisao(t *testing.T) {
	t.Parallel()

	t.Run("o primeiro dia não revisa nada", func(t *testing.T) {
		t.Parallel()

		dias := []plano.Dia{diaEstudo(1, item("POR", "Crase"))}

		if got := plano.FilaRevisao(dias, 2, nil); len(got) != 0 {
			t.Errorf("dia 1 revisou %v; não há o que revisar ainda", got)
		}
	})

	t.Run("no dia seguinte já revisa o que foi estudado", func(t *testing.T) {
		t.Parallel()

		// Estudou Português e Matemática no dia 1: o dia 2 já volta ao primeiro.
		dias := []plano.Dia{
			diaEstudo(1, item("POR", "Crase"), item("MAT", "Frações")),
			diaEstudo(2, item("POR", "Regência"), item("MAT", "Razões")),
			diaEstudo(3, item("POR", "Concordância"), item("MAT", "Juros")),
		}

		got := plano.FilaRevisao(dias, 1, nil)

		if len(got[2]) != 1 || got[2][0].Tema != "Crase" {
			t.Fatalf("dia 2 = %v, quer Crase", got[2])
		}

		// E no dia 3 segue para o próximo da fila, não repete.
		if len(got[3]) != 1 || got[3][0].Tema != "Frações" {
			t.Fatalf("dia 3 = %v, quer Frações", got[3])
		}
	})

	t.Run("uma matéria por dia", func(t *testing.T) {
		t.Parallel()

		dias := []plano.Dia{
			diaEstudo(1, item("POR", "p1"), item("MAT", "m1")),
			diaEstudo(2, item("POR", "p2"), item("MAT", "m2")),
			diaEstudo(3, item("POR", "p3"), item("MAT", "m3")),
		}

		for n, itens := range plano.FilaRevisao(dias, 4, nil) {
			for _, it := range itens {
				if it.Disciplina != itens[0].Disciplina {
					t.Errorf("dia %d misturou matérias: %v", n, itens)
				}
			}
		}
	})

	t.Run("dá a volta e recomeça", func(t *testing.T) {
		t.Parallel()

		// Fila curta e muitos dias: tem de circular, não acabar.
		dias := []plano.Dia{
			diaEstudo(1, item("POR", "p1")),
			diaEstudo(2), diaEstudo(3), diaEstudo(4),
		}

		got := plano.FilaRevisao(dias, 1, nil)

		for _, n := range []int{2, 3, 4} {
			if len(got[n]) != 1 || got[n][0].Tema != "p1" {
				t.Errorf("dia %d = %v, quer p1 de novo", n, got[n])
			}
		}
	})

	t.Run("não entra na reta final", func(t *testing.T) {
		t.Parallel()

		dias := []plano.Dia{
			diaEstudo(1, item("POR", "p1")),
			{N: 2, Tipo: plano.TipoRevisaoDirigida, Fase: plano.FaseReta, Itens: []plano.ItemDia{item("POR", "p1")}},
		}

		if got := plano.FilaRevisao(dias, 1, nil); len(got) != 0 {
			t.Errorf("a fila entrou na reta final: %v", got)
		}
	})

	t.Run("um tópico não se repete dentro do mesmo dia", func(t *testing.T) {
		t.Parallel()

		dias := []plano.Dia{diaEstudo(1, item("POR", "p1")), diaEstudo(2)}

		if got := plano.FilaRevisao(dias, 5, nil); len(got[2]) != 1 {
			t.Errorf("dia 2 = %v, quer só p1 uma vez", got[2])
		}
	})
}

func TestVoltasRevisao(t *testing.T) {
	t.Parallel()

	t.Run("conta as voltas completas sobre o estudado", func(t *testing.T) {
		t.Parallel()

		// 3 dias, 1 tópico por dia, 1 revisão por dia: os dias 2 e 3 revisam,
		// são 2 revisões sobre 3 tópicos estudados.
		dias := []plano.Dia{
			diaEstudo(1, item("POR", "p1")),
			diaEstudo(2, item("POR", "p2")),
			diaEstudo(3, item("POR", "p3")),
		}

		if got := plano.VoltasRevisao(dias, 1); got < 0.6 || got > 0.7 {
			t.Errorf("voltas = %v, quer ~0,67", got)
		}
	})

	t.Run("mais revisões por dia dão mais voltas", func(t *testing.T) {
		t.Parallel()

		dias := []plano.Dia{
			diaEstudo(1, item("POR", "p1")),
			diaEstudo(2, item("POR", "p2")),
			diaEstudo(3, item("POR", "p3")),
			diaEstudo(4, item("POR", "p4")),
		}

		uma := plano.VoltasRevisao(dias, 1)
		duas := plano.VoltasRevisao(dias, 2)

		if !(duas > uma) {
			t.Errorf("2 por dia = %v, devia superar 1 por dia = %v", duas, uma)
		}
	})

	t.Run("sem estudo não há voltas", func(t *testing.T) {
		t.Parallel()

		if got := plano.VoltasRevisao([]plano.Dia{diaEstudo(1)}, 2); got != 0 {
			t.Errorf("voltas = %v, quer 0", got)
		}
	})
}

func TestRevisoesPorDisciplina(t *testing.T) {
	t.Parallel()

	t.Run("conta as voltas sobre cada matéria", func(t *testing.T) {
		t.Parallel()

		// POR tem 1 tópico e é revisado nos dias 2, 3 e 4: 3 voltas sobre ele.
		dias := []plano.Dia{
			diaEstudo(1, item("POR", "p1")),
			diaEstudo(2), diaEstudo(3), diaEstudo(4),
		}

		got := plano.RevisoesPorDisciplina(dias, 1)

		if got["POR"] != 3 {
			t.Errorf("POR = %v, quer 3", got["POR"])
		}
	})

	t.Run("matéria estudada cedo é revisada mais que a estudada tarde", func(t *testing.T) {
		t.Parallel()

		// POR entra no dia 1, MAT só no dia 4: POR roda mais vezes na fila.
		dias := []plano.Dia{
			diaEstudo(1, item("POR", "p1")),
			diaEstudo(2), diaEstudo(3),
			diaEstudo(4, item("MAT", "m1")),
			diaEstudo(5), diaEstudo(6),
		}

		got := plano.RevisoesPorDisciplina(dias, 1)

		if !(got["POR"] > got["MAT"]) {
			t.Errorf("POR = %v, MAT = %v; a estudada cedo devia liderar", got["POR"], got["MAT"])
		}
	})

	t.Run("matéria nunca estudada não aparece", func(t *testing.T) {
		t.Parallel()

		dias := []plano.Dia{diaEstudo(1, item("POR", "p1")), diaEstudo(2)}

		if _, ok := plano.RevisoesPorDisciplina(dias, 1)["MAT"]; ok {
			t.Error("MAT apareceu sem nunca ter sido estudada")
		}
	})
}

func TestVisitasPorDisciplina(t *testing.T) {
	t.Parallel()

	t.Run("conta os dias em que a matéria aparece", func(t *testing.T) {
		t.Parallel()

		dias := []plano.Dia{
			diaEstudo(1, item("POR", "p1"), item("MAT", "m1")),
			diaEstudo(2, item("POR", "p2")),
			diaEstudo(3, item("POR", "p3"), item("MAT", "m2")),
		}

		got := plano.VisitasPorDisciplina(dias)

		if got["POR"] != 3 {
			t.Errorf("POR = %d, quer 3", got["POR"])
		}

		if got["MAT"] != 2 {
			t.Errorf("MAT = %d, quer 2", got["MAT"])
		}
	})

	t.Run("duas vezes no mesmo dia contam como uma visita", func(t *testing.T) {
		t.Parallel()

		// Voltar à matéria é o que importa, não quantos blocos ela ocupa.
		dias := []plano.Dia{diaEstudo(1, item("POR", "p1"), item("POR", "p2"))}

		if got := plano.VisitasPorDisciplina(dias); got["POR"] != 1 {
			t.Errorf("POR = %d, quer 1", got["POR"])
		}
	})

	t.Run("a reta final não entra na conta", func(t *testing.T) {
		t.Parallel()

		dias := []plano.Dia{
			diaEstudo(1, item("POR", "p1")),
			{N: 2, Tipo: plano.TipoRevisaoDirigida, Fase: plano.FaseReta, Itens: []plano.ItemDia{item("POR", "p1")}},
		}

		if got := plano.VisitasPorDisciplina(dias); got["POR"] != 1 {
			t.Errorf("POR = %d, quer 1 — a reta final é outra fase", got["POR"])
		}
	})
}
