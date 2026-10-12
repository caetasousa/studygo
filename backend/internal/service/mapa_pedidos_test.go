//go:build integration

package service

import (
	"context"
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
