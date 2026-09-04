package concurso_test

import (
	"testing"

	"studygo/internal/domain/concurso"
)

func TestSigla(t *testing.T) {
	t.Parallel()

	tests := []struct {
		nome     string
		esperado string
	}{
		{"Língua Portuguesa", "LINPO"},
		{"Português", "PORT"},
		{"Matemática e Raciocínio Lógico", "MATRA"},
		{"Banco de Dados", "BANDA"},
		{"Legislação Institucional", "LEGIN"},
		{"Engenharia de Software", "ENGSO"},
		// "Noções de" is generic: what identifies the discipline is what follows.
		{"Noções de Direito Administrativo", "DIRAD"},
		{"Noções de Controle Externo", "CONEX"},
		{"Licitações e Contratos Administrativos", "LICCO"},
		{"Segurança da Informação", "SEGIN"},
		// A single significant word gives four letters, not three: "AUDI" reads
		// better than "AUD".
		{"Auditoria", "AUDI"},
		// Nothing to build on.
		{"", ""},
		{"   ", ""},
		{"123", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			t.Parallel()

			if got := concurso.Sigla(tt.nome); got != tt.esperado {
				t.Errorf("Sigla(%q) = %q, quer %q", tt.nome, got, tt.esperado)
			}
		})
	}
}

func TestSiglas_Unicas(t *testing.T) {
	t.Parallel()

	// Three disciplines that collide on the same prefix — the case an edital
	// full of "Direito ..." actually produces.
	nomes := []string{
		"Direito Administrativo",
		"Direito Administrativo",
		"Noções de Direito Administrativo",
		"Língua Portuguesa",
	}

	got := concurso.Siglas(nomes)

	vistos := map[string]bool{}
	for i, c := range got {
		if c == "" {
			t.Fatalf("código %d ficou vazio: %v", i, got)
		}

		if vistos[c] {
			t.Fatalf("código repetido %q em %v", c, got)
		}

		vistos[c] = true
	}

	if got[0] != "DIRAD" || got[1] != "DIRAD2" || got[2] != "DIRAD3" {
		t.Errorf("desempate = %v, quer DIRAD, DIRAD2, DIRAD3 nos três primeiros", got)
	}
}

func TestSiglas_NomeSemLetrasCaiNoPosicional(t *testing.T) {
	t.Parallel()

	got := concurso.Siglas([]string{"---", "Português"})

	if got[0] != "D01" {
		t.Errorf("código sem letras = %q, quer D01", got[0])
	}

	if got[1] != "PORT" {
		t.Errorf("código = %q, quer PORT", got[1])
	}
}

// O código de uma disciplina é o mnemônico que aparece em cada chip do
// cronograma, e é o que o estudante aprende a reconhecer. Estes são os casos
// que a regra precisa acertar — nomes reais de edital, incluindo os que colidem
// no começo ("Direito Administrativo" e "Direito Constitucional").
//
// Mudar a regra é possível; mudar sem querer, não. Por isso os casos ficam aqui.
func TestSigla_CasosDeEdital(t *testing.T) {
	t.Parallel()

	casos := map[string]string{
		"Língua Portuguesa":            "LINPO",
		"Direito Administrativo":       "DIRAD",
		"Noções de Informática":        "INFO",
		"Raciocínio Lógico-Matemático": "RACLO",
		"Banco de Dados":               "BANDA",
		"Introdução à Computação":      "COMP",
		"Ética no Serviço Público":     "ETISE",
		"Análise de Dados":             "ANADA",
		"Matemática":                   "MATE",
	}

	for nome, quer := range casos {
		t.Run(nome, func(t *testing.T) {
			t.Parallel()

			if got := concurso.Sigla(nome); got != quer {
				t.Errorf("Sigla(%q) = %q, quer %q", nome, got, quer)
			}
		})
	}
}
