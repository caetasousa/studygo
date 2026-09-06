package concurso_test

import (
	"errors"
	"testing"
	"time"

	"studygo/internal/domain/concurso"
)

// base é um cadastro mínimo válido, para que cada teste altere só o que
// investiga.
func base(discs ...concurso.Disciplina) concurso.Concurso {
	return concurso.Concurso{
		Nome:        "TCE-GO",
		ProvaPadrao: time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC),
		Disciplinas: discs,
	}
}

func materia(nome, codigo string) concurso.Disciplina {
	return concurso.Disciplina{
		Nome: nome, Codigo: codigo,
		Bloco: concurso.BlocoGeral, QuestoesPadrao: 10,
	}
}

// A tag escolhida pelo usuário é o que vale — o nome só entra quando não há
// tag nenhuma.
func TestNormalizar_TagEscolhidaSubstituiADerivadaDoNome(t *testing.T) {
	t.Parallel()

	c := base(
		materia("Matemática e Raciocínio Lógico", "rlm"),
		materia("Língua Portuguesa", ""),
	)

	c.Normalizar()

	if got := c.Disciplinas[0].Codigo; got != "RLM" {
		t.Errorf("tag escolhida = %q, quer RLM", got)
	}

	if got := c.Disciplinas[1].Codigo; got != "LINPO" {
		t.Errorf("tag derivada do nome = %q, quer LINPO", got)
	}
}

// Uma tag escolhida entra na disputa por unicidade como qualquer outra: quem
// chega sem tag desvia dela.
func TestNormalizar_TagDerivadaDesviaDaEscolhida(t *testing.T) {
	t.Parallel()

	c := base(
		materia("Direito Constitucional", "DIRAD"),
		materia("Direito Administrativo", ""),
	)

	c.Normalizar()

	if a, b := c.Disciplinas[0].Codigo, c.Disciplinas[1].Codigo; a == b {
		t.Fatalf("as duas matérias ficaram com a mesma tag %q", a)
	}

	if got := c.Disciplinas[1].Codigo; got != "DIRAD2" {
		t.Errorf("tag derivada = %q, quer DIRAD2", got)
	}
}

func TestValidar_RecusaTagRepetida(t *testing.T) {
	t.Parallel()

	c := base(
		materia("Matemática e Raciocínio Lógico", "RLM"),
		materia("Redação Oficial", "rlm"),
	)

	c.Normalizar()

	var repetida concurso.ErrCodigoRepetido
	if err := c.Validar(); !errors.As(err, &repetida) {
		t.Fatalf("Validar() = %v, quer ErrCodigoRepetido", err)
	}

	// A mensagem diz QUAL tag repetiu — é o que o formulário mostra.
	if repetida.Codigo != "RLM" {
		t.Errorf("tag no erro = %q, quer RLM", repetida.Codigo)
	}
}

func TestValidar_AceitaTagsDistintas(t *testing.T) {
	t.Parallel()

	c := base(
		materia("Matemática e Raciocínio Lógico", "RLM"),
		materia("Língua Portuguesa", "PORT"),
	)

	c.Normalizar()

	if err := c.Validar(); err != nil {
		t.Fatalf("Validar() = %v, quer nil", err)
	}
}
