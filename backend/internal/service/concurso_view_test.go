package service

import (
	"strings"
	"testing"

	"annygo/internal/domain/concurso"
	"annygo/internal/port"
)

func TestEstruturaParaResposta(t *testing.T) {
	t.Parallel()

	base := port.EditalEstrutura{
		Nome:             "TCE-GO — Técnico de Controle Externo (TI)",
		Prova:            "2027-01-17",
		ProvaDiscursiva:  true,
		TotalGerais:      25,
		TotalEspecificas: 45,
		Gerais: []port.EditalDisciplina{
			{Nome: "Língua Portuguesa", Questoes: 0},
			{Nome: "Matemática", Questoes: 0},
			{Nome: "Legislação Institucional", Questoes: 0},
		},
		Especificas: []port.EditalDisciplina{
			{Nome: "Engenharia de Software", Questoes: 0},
			{Nome: "Banco de Dados", Questoes: 0},
		},
		Marcos: []port.EditalMarco{
			{Data: "2026-10-05", DataFim: "2026-11-06", Titulo: "Inscrições", ExigeAcao: true},
			{Data: "", Titulo: "lixo sem data"},
		},
	}

	t.Run("blocos rotulados e total distribuído", func(t *testing.T) {
		t.Parallel()

		got := estruturaParaResposta(base)

		if got.Prova != "2027-01-17" || !got.ProvaDiscursiva {
			t.Errorf("cabeçalho: %+v", got)
		}
		for _, d := range got.Gerais {
			if d.Bloco != "ger" {
				t.Errorf("disciplina geral com bloco %q", d.Bloco)
			}
		}
		for _, d := range got.Especificas {
			if d.Bloco != "esp" {
				t.Errorf("disciplina específica com bloco %q", d.Bloco)
			}
		}

		ger, esp := 0, 0
		for _, d := range got.Gerais {
			ger += d.Questoes
		}
		for _, d := range got.Especificas {
			esp += d.Questoes
		}
		if ger != 25 || esp != 45 {
			t.Errorf("distribuição ger=%d esp=%d, quero 25/45", ger, esp)
		}

		if len(got.Marcos) != 1 {
			t.Errorf("marco sem data deveria ter sido descartado: %+v", got.Marcos)
		}
		if !hasAvisoContendo(got.Avisos, "distribuí igualmente") {
			t.Errorf("faltou aviso da distribuição: %v", got.Avisos)
		}
	})

	t.Run("sem prova gera aviso", func(t *testing.T) {
		t.Parallel()

		e := base
		e.Prova = ""

		got := estruturaParaResposta(e)
		if !hasAvisoContendo(got.Avisos, "data de prova") {
			t.Errorf("avisos = %v", got.Avisos)
		}
	})

	t.Run("não distribui bloco já preenchido", func(t *testing.T) {
		t.Parallel()

		e := port.EditalEstrutura{
			Prova:       "2027-01-17",
			TotalGerais: 25,
			Gerais: []port.EditalDisciplina{
				{Nome: "Português", Questoes: 15},
				{Nome: "Matemática", Questoes: 0},
			},
		}

		got := estruturaParaResposta(e)
		if got.Gerais[1].Questoes != 0 {
			t.Errorf("não deveria distribuir: %+v", got.Gerais)
		}
	})
}

func TestConcursoFromInput(t *testing.T) {
	t.Parallel()

	base := ConcursoInput{
		Nome:  "TJ-SP Escrevente",
		Prova: "2026-05-10",
		Disciplinas: []DisciplinaInput{
			{Nome: "Direito Constitucional", Bloco: "esp", Questoes: 15},
			{Nome: "Português", Bloco: "GER", Questoes: 10, Temas: []string{"Crase"}},
		},
	}

	t.Run("aplica peso, defaults e valida", func(t *testing.T) {
		t.Parallel()

		c, _ := concursoFromInput(base)

		if c.RetaPadraoDias != 28 {
			t.Errorf("reta default = %d", c.RetaPadraoDias)
		}
		if c.ProvaPadrao.Format("2006-01-02") != "2026-05-10" {
			t.Errorf("prova = %v", c.ProvaPadrao)
		}
		if c.Disciplinas[0].Peso != 2 || c.Disciplinas[1].Peso != 1 {
			t.Errorf("pesos = %d/%d", c.Disciplinas[0].Peso, c.Disciplinas[1].Peso)
		}
		if c.Disciplinas[1].Bloco != concurso.BlocoGeral {
			t.Errorf("bloco normalizado = %q", c.Disciplinas[1].Bloco)
		}
		if err := c.Validar(); err != nil {
			t.Errorf("Validar: %v", err)
		}
	})

	t.Run("Validar rejeita soma de pontos zero", func(t *testing.T) {
		t.Parallel()

		in := base
		in.Disciplinas = []DisciplinaInput{{Nome: "X", Bloco: "esp", Questoes: 0}}

		c, _ := concursoFromInput(in)
		if err := c.Validar(); err == nil {
			t.Fatal("esperava erro de pontos zero")
		}
	})

	t.Run("Validar rejeita sem prova", func(t *testing.T) {
		t.Parallel()

		in := base
		in.Prova = ""

		c, _ := concursoFromInput(in)
		if err := c.Validar(); err == nil {
			t.Fatal("esperava erro de prova obrigatória")
		}
	})

	t.Run("fontes vazias são descartadas", func(t *testing.T) {
		t.Parallel()

		in := base
		in.Disciplinas[0].Fontes = []FonteInput{
			{Titulo: "Lei 8.112", URL: "http://x"},
			{Titulo: "", URL: ""},
		}

		c, _ := concursoFromInput(in)
		if len(c.Disciplinas[0].Fontes) != 1 {
			t.Errorf("fontes = %d, queria 1", len(c.Disciplinas[0].Fontes))
		}
	})
}

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, prefix string }{
		{"TCE-GO — Técnico de Controle Externo (TI)", "tce-go-tecnico-de-controle-externo-ti-"},
		{"TJ/SP  Escrevente", "tj-sp-escrevente-"},
		{"!!!", "concurso-"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got := slugify(tt.in)
			if !strings.HasPrefix(got, tt.prefix) {
				t.Errorf("slugify(%q) = %q, queria prefixo %q", tt.in, got, tt.prefix)
			}
			if len(got) <= len(tt.prefix) {
				t.Errorf("slug %q sem sufixo aleatório", got)
			}
		})
	}
}

func hasAvisoContendo(avisos []string, sub string) bool {
	for _, a := range avisos {
		if strings.Contains(a, sub) {
			return true
		}
	}

	return false
}
