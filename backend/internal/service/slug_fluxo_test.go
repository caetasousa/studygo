//go:build integration

package service

import (
	"errors"
	"strings"
	"testing"

	"studygo/internal/domain/concurso"
)

// A colisão de slug é rara e por isso ninguém a vê acontecer — o que torna o
// teste o único lugar onde o comportamento fica visível.
//
// O slug é o nome normalizado mais dois bytes de acaso, e o UNIQUE é do banco
// inteiro: dois estudantes cadastrando o mesmo concurso disputam o mesmo espaço
// de sufixos. Antes, a colisão virava 500 "erro interno" numa operação que só
// precisava sortear de novo.

func comandoValido() ConcursoCommand {
	return ConcursoCommand{
		Nome:  "Polícia Federal",
		Prova: "2026-12-15",
		Disciplinas: []DisciplinaCommand{
			{Nome: "Língua Portuguesa", Bloco: "ger", Questoes: 20},
		},
	}
}

// Dois estudantes cadastram o mesmo concurso: cada um ganha o seu slug, os dois
// derivados do nome.
func TestConcursoService_Criar_mesmoNomeGanhaSlugsDiferentes(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	svc := NewConcursoService(ce.deps.Concursos, nil)

	primeiro, _, err := svc.Criar(t.Context(), ce.usuario, comandoValido())
	if err != nil {
		t.Fatalf("Criar o primeiro: %v", err)
	}

	segundo, _, err := svc.Criar(t.Context(), novoDono(t, ce.pool), comandoValido())
	if err != nil {
		t.Fatalf("Criar o segundo: %v", err)
	}

	if primeiro.Slug == segundo.Slug {
		t.Errorf("os dois ficaram com o slug %q", primeiro.Slug)
	}

	base := concurso.BaseSlug("Polícia Federal") + "-"
	for _, s := range []string{primeiro.Slug, segundo.Slug} {
		if !strings.HasPrefix(s, base) {
			t.Errorf("slug %q não deriva de %q", s, base)
		}
	}
}

// Colidir sempre não pode virar laço infinito nem 500: com todos os sufixos
// ocupados no banco, o erro que sobe é o de conflito, que o adapter traduz em 409.
func TestConcursoService_Criar_desisteQuandoTodosOsSlugsEstaoEmUso(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)

	// Os 65.536 sufixos de dois bytes, todos tomados.
	if _, err := ce.pool.Exec(t.Context(), `
		INSERT INTO concursos (dono_id, slug, nome, prova_padrao)
		SELECT $1, $2 || lpad(to_hex(g), 4, '0'), 'Ocupado', DATE '2026-12-15'
		  FROM generate_series(0, 65535) AS g`,
		ce.usuario, concurso.BaseSlug("Polícia Federal")+"-",
	); err != nil {
		t.Fatalf("ocupando os slugs: %v", err)
	}

	_, _, err := NewConcursoService(ce.deps.Concursos, nil).Criar(t.Context(), ce.usuario, comandoValido())
	if !errors.Is(err, concurso.ErrSlugEmUso) {
		t.Fatalf("erro = %v, quer ErrSlugEmUso", err)
	}
}

// Falha do banco que não é colisão não vira conflito: o estudante veria "esse
// nome já existe" para um problema que não é dele.
func TestConcursoService_Criar_falhaDoBancoNaoViraConflito(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.travarEscrita(t, "concursos")

	_, _, err := NewConcursoService(ce.deps.Concursos, nil).Criar(t.Context(), ce.usuario, comandoValido())
	if err == nil {
		t.Fatal("o banco recusou e o Criar respondeu como se tivesse gravado")
	}

	if errors.Is(err, concurso.ErrSlugEmUso) {
		t.Errorf("erro = %v: uma falha do banco virou conflito de slug", err)
	}
}
