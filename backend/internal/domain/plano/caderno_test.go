package plano_test

import (
	"testing"
	"time"

	"studygo/internal/domain/plano"
)

// dia builds a UTC date. It lived in revisao_test.go, which went away with the
// spaced-review queue; the schedule tests still need it.
func dia(a int, m time.Month, d int) time.Time {
	return time.Date(a, m, d, 0, 0, 0, 0, time.UTC)
}

func res(disc, tema, data string, q, a int) plano.ResultadoTema {
	return plano.ResultadoTema{Disciplina: disc, Tema: tema, Data: data, Questoes: q, Acertos: a}
}

func TestCaderno(t *testing.T) {
	t.Parallel()

	t.Run("só entra o tema que foi mal", func(t *testing.T) {
		t.Parallel()

		// 9/10 é bom, 3/10 é fraco: só o segundo vira caderno.
		cad := plano.Caderno([]plano.ResultadoTema{
			res("POR", "Crase", "2026-09-01", 10, 9),
			res("POR", "Regência", "2026-09-01", 10, 3),
		})

		if len(cad["POR"]) != 1 {
			t.Fatalf("itens = %d, quer 1", len(cad["POR"]))
		}

		if cad["POR"][0].Tema != "Regência" {
			t.Errorf("tema = %q, quer Regência", cad["POR"][0].Tema)
		}
	})

	t.Run("é por matéria, não um caderno só", func(t *testing.T) {
		t.Parallel()

		cad := plano.Caderno([]plano.ResultadoTema{
			res("POR", "Crase", "2026-09-01", 10, 2),
			res("MAT", "Frações", "2026-09-01", 10, 1),
		})

		if len(cad) != 2 || len(cad["POR"]) != 1 || len(cad["MAT"]) != 1 {
			t.Fatalf("cadernos = %v", cad)
		}
	})

	t.Run("acumula tentativas do mesmo tema", func(t *testing.T) {
		t.Parallel()

		cad := plano.Caderno([]plano.ResultadoTema{
			res("POR", "Crase", "2026-09-01", 10, 2),
			res("POR", "Crase", "2026-09-08", 10, 6),
		})

		it := cad["POR"][0]
		if it.Questoes != 20 || it.Acertos != 8 {
			t.Fatalf("acumulado = %d/%d, quer 20/8", it.Acertos, it.Questoes)
		}

		// Só a primeira tentativa foi fraca (20%); a segunda, 60%, não é.
		if it.Erros != 1 {
			t.Errorf("erros = %d, quer 1", it.Erros)
		}

		if it.UltimaData != "2026-09-08" {
			t.Errorf("última data = %q", it.UltimaData)
		}
	})

	t.Run("um tema que melhorou continua no caderno", func(t *testing.T) {
		t.Parallel()

		// O caderno é registro do que foi difícil, não uma fila que esvazia.
		cad := plano.Caderno([]plano.ResultadoTema{
			res("POR", "Crase", "2026-09-01", 10, 1),
			res("POR", "Crase", "2026-09-20", 10, 10),
		})

		if len(cad["POR"]) != 1 {
			t.Fatalf("o tema saiu do caderno ao melhorar: %v", cad)
		}
	})

	t.Run("ordena pelo que precisa de mais trabalho", func(t *testing.T) {
		t.Parallel()

		cad := plano.Caderno([]plano.ResultadoTema{
			res("POR", "Um erro", "2026-09-01", 10, 5),
			res("POR", "Dois erros", "2026-09-01", 10, 2),
			res("POR", "Dois erros", "2026-09-02", 10, 1),
		})

		if cad["POR"][0].Tema != "Dois erros" {
			t.Errorf("ordem = %v; o mais errado devia vir primeiro", cad["POR"])
		}
	})

	t.Run("ignora entrada sem questões", func(t *testing.T) {
		t.Parallel()

		cad := plano.Caderno([]plano.ResultadoTema{
			res("POR", "Crase", "2026-09-01", 0, 0),
		})

		if len(cad) != 0 {
			t.Errorf("caderno = %v, quer vazio", cad)
		}
	})
}

func TestTemasDoDia(t *testing.T) {
	t.Parallel()

	cad := map[string][]plano.ItemCaderno{
		"POR": {
			{Disciplina: "POR", Tema: "p1"},
			{Disciplina: "POR", Tema: "p2"},
		},
		"MAT": {
			{Disciplina: "MAT", Tema: "m1"},
		},
		"DIR": {
			{Disciplina: "DIR", Tema: "d1"},
		},
	}

	t.Run("alterna entre as matérias do dia", func(t *testing.T) {
		t.Parallel()

		got := plano.TemasDoDia(cad, []string{"POR", "MAT"}, 3)

		// Um de cada antes de repetir a mesma matéria.
		quer := []string{"p1", "m1", "p2"}
		for i := range quer {
			if got[i].Tema != quer[i] {
				t.Fatalf("ordem = %v, quer %v", temas(got), quer)
			}
		}
	})

	t.Run("não puxa matéria que não é do dia", func(t *testing.T) {
		t.Parallel()

		for _, it := range plano.TemasDoDia(cad, []string{"POR"}, 5) {
			if it.Disciplina != "POR" {
				t.Errorf("veio %q, que não é do dia", it.Disciplina)
			}
		}
	})

	t.Run("respeita o limite", func(t *testing.T) {
		t.Parallel()

		if n := len(plano.TemasDoDia(cad, []string{"POR", "MAT"}, 2)); n != 2 {
			t.Errorf("itens = %d, quer 2", n)
		}

		if n := len(plano.TemasDoDia(cad, []string{"POR"}, 0)); n != 0 {
			t.Errorf("limite 0 devia devolver nada, veio %d", n)
		}
	})

	t.Run("para quando os cadernos acabam", func(t *testing.T) {
		t.Parallel()

		// Pede 10, mas POR+MAT só têm 3 temas juntos.
		if n := len(plano.TemasDoDia(cad, []string{"POR", "MAT"}, 10)); n != 3 {
			t.Errorf("itens = %d, quer 3", n)
		}
	})

	t.Run("disciplina repetida no dia não ganha duas vezes", func(t *testing.T) {
		t.Parallel()

		got := plano.TemasDoDia(cad, []string{"POR", "POR", "MAT"}, 2)

		if got[0].Disciplina != "POR" || got[1].Disciplina != "MAT" {
			t.Errorf("ordem = %v; POR repetida devia contar uma vez", temas(got))
		}
	})
}

func temas(itens []plano.ItemCaderno) []string {
	out := make([]string, len(itens))
	for i, it := range itens {
		out[i] = it.Tema
	}

	return out
}
