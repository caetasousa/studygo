package service

import (
	"strings"
	"testing"

	"studygo/internal/domain/concurso"
	"studygo/internal/port"
)

func ptrInt(n int) *int { return &n }

func ptrFloat(f float64) *float64 { return &f }

func TestEstruturaParaResposta(t *testing.T) {
	t.Parallel()

	base := port.EditalEstrutura{
		NomeSugerido: "TCE-GO — Técnico de Controle Externo (TI)",
		DataProva:    "2027-01-17",
		GruposGerais: []port.EditalGrupo{
			{
				Kind:       "ger",
				Rotulo:     "Conhecimentos Gerais",
				Total:      ptrInt(25),
				Peso:       ptrFloat(1),
				PesoEscopo: "group",
				Disciplinas: []port.EditalDisciplina{
					{Nome: "Língua Portuguesa"},
					{Nome: "Matemática"},
				},
			},
		},
		GruposEspecificos: []port.EditalGrupo{
			{
				Kind:   "esp",
				Rotulo: "Conhecimentos Específicos",
				Total:  ptrInt(45),
				Peso:   ptrFloat(2),
				Disciplinas: []port.EditalDisciplina{
					{Nome: "Engenharia de Software", Questoes: ptrInt(10)},
				},
			},
		},
		Discursivas: []port.EditalDiscursiva{
			{Modalidade: "estudo_de_caso", Rotulo: "Estudo de Caso", Questoes: ptrInt(1)},
		},
		Duracao: &port.EditalDuracao{Minutos: 270, Escopo: "exam_set"},
		Marcos: []port.EditalMarco{
			{Data: "2026-10-05", DataFim: "2026-11-06", Titulo: "Inscrições", ExigeAcao: true},
			{Data: "", Titulo: "lixo sem data"},
		},
		Alertas: []port.EditalAlerta{
			{Codigo: "questions_not_broken_down", Gravidade: "blocker", Mensagem: "informe a estimativa"},
		},
	}

	t.Run("mapeia grupos, duração e alertas — sem inventar questões", func(t *testing.T) {
		t.Parallel()

		got := estruturaParaResposta(base)

		if got.Prova != "2027-01-17" || got.Nome == "" {
			t.Errorf("cabeçalho: %+v", got)
		}
		if len(got.Gerais) != 1 || got.Gerais[0].Kind != "ger" {
			t.Fatalf("grupos gerais: %+v", got.Gerais)
		}
		if got.Gerais[0].Total == nil || *got.Gerais[0].Total != 25 {
			t.Errorf("total geral: %+v", got.Gerais[0].Total)
		}
		// The edital gave only the group total — every discipline stays null.
		for _, d := range got.Gerais[0].Disciplinas {
			if d.Questoes != nil {
				t.Errorf("disciplina %q recebeu questões inventadas: %v", d.Nome, *d.Questoes)
			}
		}
		// Where the edital did break it down, the value is kept.
		if got.Especificas[0].Disciplinas[0].Questoes == nil ||
			*got.Especificas[0].Disciplinas[0].Questoes != 10 {
			t.Errorf("questões específicas não preservadas: %+v", got.Especificas[0].Disciplinas[0])
		}

		if got.Duracao == nil || got.Duracao.Minutos != 270 || got.Duracao.Escopo != "exam_set" {
			t.Errorf("duração: %+v", got.Duracao)
		}
		if len(got.Discursivas) != 1 || got.Discursivas[0].Modalidade != "estudo_de_caso" {
			t.Errorf("discursivas: %+v", got.Discursivas)
		}
		if len(got.Marcos) != 1 {
			t.Errorf("marco sem data deveria ter sido descartado: %+v", got.Marcos)
		}
		if len(got.Alertas) != 1 || got.Alertas[0].Gravidade != "blocker" {
			t.Errorf("alertas: %+v", got.Alertas)
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
			t.Errorf("pesos default = %d/%d, quero 2/1", c.Disciplinas[0].Peso, c.Disciplinas[1].Peso)
		}
		if c.Disciplinas[1].Bloco != concurso.BlocoGeral {
			t.Errorf("bloco normalizado = %q", c.Disciplinas[1].Bloco)
		}
		if err := c.Validar(); err != nil {
			t.Errorf("Validar: %v", err)
		}
	})

	t.Run("peso do usuário sobrepõe o default do bloco", func(t *testing.T) {
		t.Parallel()

		in := base
		in.Disciplinas = []DisciplinaInput{
			{Nome: "Constitucional", Bloco: "esp", Questoes: 15, Peso: 3},
			{Nome: "Português", Bloco: "ger", Questoes: 10}, // no peso -> default 1
		}

		c, _ := concursoFromInput(in)
		if c.Disciplinas[0].Peso != 3 {
			t.Errorf("peso do usuário = %d, quero 3", c.Disciplinas[0].Peso)
		}
		if c.Disciplinas[1].Peso != 1 {
			t.Errorf("peso default = %d, quero 1", c.Disciplinas[1].Peso)
		}
	})

	t.Run("cadernoUrl é preservada e trimada", func(t *testing.T) {
		t.Parallel()

		in := base
		in.Disciplinas = []DisciplinaInput{
			{Nome: "Constitucional", Bloco: "esp", Questoes: 15, CadernoURL: "  https://tec.com/caderno/1  "},
			{Nome: "Português", Bloco: "ger", Questoes: 10},
		}

		c, _ := concursoFromInput(in)
		if c.Disciplinas[0].CadernoURL != "https://tec.com/caderno/1" {
			t.Errorf("cadernoUrl = %q, quero a URL trimada", c.Disciplinas[0].CadernoURL)
		}
		if c.Disciplinas[1].CadernoURL != "" {
			t.Errorf("cadernoUrl vazia = %q", c.Disciplinas[1].CadernoURL)
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
