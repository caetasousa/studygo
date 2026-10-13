//go:build integration

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"studygo/internal/adapter/crypto"
	"studygo/internal/adapter/postgres"
	"studygo/internal/domain/mapa"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// processadorQueVeOPedidoIrEmbora é o processador de mapas (a fronteira de fora)
// no instante em que quem enviou o PDF vai embora: a aba fechada ou a página
// recarregada cancelam a requisição do upload. Ele responde como um cliente HTTP
// abortado — com o erro do contexto, se o contexto que recebeu morreu.
type processadorQueVeOPedidoIrEmbora struct {
	port.ProcessadorDeMapas
	irEmbora context.CancelFunc
}

func (p processadorQueVeOPedidoIrEmbora) Processar(ctx context.Context, _ port.TrabalhoDeMapa) error {
	p.irEmbora()

	return ctx.Err()
}

// M37: o PDF já está gravado quando quem o enviou sai da página. A entrega ao
// processador não pode depender de a requisição continuar viva: o pedido
// ficava "na fila" para sempre, sem motivo e sem ninguém processando.
func TestPedidosDeMapa_QuemSaiDaPaginaNaoTravaOPedido(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	repo := postgres.NewMapaRepo(ce.pool)

	cifra, err := crypto.NewCifra("segredo-do-teste-com-mais-de-trinta-e-dois-caracteres")
	if err != nil {
		t.Fatal(err)
	}

	ctx, irEmbora := context.WithCancel(t.Context())
	defer irEmbora()

	svc := NewPedidosDeMapaService(
		repo, ce.deps.Concursos, NewMapaService(repo, ce.deps.Concursos),
		processadorQueVeOPedidoIrEmbora{irEmbora: irEmbora},
		NewTokenDoClaudeService(ce.deps.Usuarios, cifra),
	)

	p, err := svc.Pedir(ctx, ce.usuario, "", uuid.NullUUID{}, "aula.pdf", []byte("%PDF-1.7 aula"))
	if err != nil {
		t.Fatalf("pedir: %v", err)
	}

	gravado, err := repo.Pedido(t.Context(), ce.usuario, p.ID)
	if err != nil {
		t.Fatalf("lendo o pedido: %v", err)
	}

	if gravado.Situacao != mapa.Processando {
		t.Fatalf("o pedido ficou %q (relatório %q); quer processando", gravado.Situacao, gravado.Relatorio)
	}
}

// processadorQueAnota é o processador de mapas que recebe e anota o trabalho.
type processadorQueAnota struct {
	port.ProcessadorDeMapas
	trabalhos *[]port.TrabalhoDeMapa
}

func (p processadorQueAnota) Processar(_ context.Context, t port.TrabalhoDeMapa) error {
	*p.trabalhos = append(*p.trabalhos, t)

	return nil
}

// M38: o tópico que nenhuma aula cobre vira pedido de mapa pelo assunto. Vai
// ao processador com o assunto e sem PDF, e o que falhou volta à fila sem
// pedir um PDF que nunca existiu.
func TestPedidosDeMapa_AssuntoSemAulaViraPedido(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	repo := postgres.NewMapaRepo(ce.pool)

	cifra, err := crypto.NewCifra("segredo-do-teste-com-mais-de-trinta-e-dois-caracteres")
	if err != nil {
		t.Fatal(err)
	}

	var trabalhos []port.TrabalhoDeMapa

	svc := NewPedidosDeMapaService(
		repo, ce.deps.Concursos, NewMapaService(repo, ce.deps.Concursos),
		processadorQueAnota{trabalhos: &trabalhos},
		NewTokenDoClaudeService(ce.deps.Usuarios, cifra),
	)

	for _, ruim := range []string{"  ", "ab", strings.Repeat("x", mapa.MaxAssunto+1)} {
		if _, err := svc.PedirAssunto(t.Context(), ce.usuario, "", uuid.NullUUID{}, ruim); !errors.Is(err, mapa.ErrAssuntoInvalido) {
			t.Fatalf("assunto %d caracteres: erro %v; quer ErrAssuntoInvalido", len(ruim), err)
		}
	}

	if len(trabalhos) != 0 {
		t.Fatalf("o assunto recusado foi ao processador: %+v", trabalhos)
	}

	c, err := ce.deps.Concursos.PorSlug(t.Context(), ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	banda := c.Disciplinas[1].ID

	p, err := svc.PedirAssunto(t.Context(), ce.usuario, ce.slug, uuid.NullUUID{UUID: banda, Valid: true}, "  Transações:\n ACID  ")
	if err != nil {
		t.Fatalf("pedir pelo assunto: %v", err)
	}

	if p.Situacao != mapa.Processando || p.Assunto != "Transações: ACID" || p.TemPDF || p.DisciplinaNome != "Banco de Dados" {
		t.Fatalf("o pedido ficou %+v", p)
	}

	if len(trabalhos) != 1 || trabalhos[0].Assunto != "Transações: ACID" || trabalhos[0].PDF != nil ||
		trabalhos[0].Materia != "BANDA — Banco de Dados" {
		t.Fatalf("o processador recebeu %+v", trabalhos)
	}

	if err := svc.Falhar(t.Context(), p.ID, "não achei fonte oficial"); err != nil {
		t.Fatalf("falhar: %v", err)
	}

	if err := svc.Reenfileirar(t.Context(), ce.usuario, p.ID); err != nil {
		t.Fatalf("pôr de novo na fila o assunto que falhou: %v", err)
	}

	if len(trabalhos) != 2 || trabalhos[1].Assunto != "Transações: ACID" {
		t.Fatalf("o reenvio não levou o assunto: %+v", trabalhos)
	}
}
