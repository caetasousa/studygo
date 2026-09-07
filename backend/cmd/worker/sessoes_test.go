package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

// A faxina das sessões é a última coisa que a virada do dia faz, e é a que
// menos pode atrapalhar as outras: se ela falhar, a tabela cresce um dia a mais
// e a próxima virada tenta de novo. O que não pode é derrubar o worker.

type sessoesFalsas struct {
	apagados int64
	err      error

	chamadas int
	recebido time.Time
}

func (s *sessoesFalsas) LimparRefreshTokens(_ context.Context, agora time.Time) (int64, error) {
	s.chamadas++
	s.recebido = agora

	return s.apagados, s.err
}

func TestLimparSessoes_passaOInstanteDaVirada(t *testing.T) {
	t.Parallel()

	sessoes := &sessoesFalsas{apagados: 7}
	agora := time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC)

	limparSessoes(t.Context(), quieto(), sessoes, agora)

	if sessoes.chamadas != 1 {
		t.Fatalf("chamadas = %d, quer 1", sessoes.chamadas)
	}

	if !sessoes.recebido.Equal(agora) {
		t.Errorf("instante = %v, quer %v", sessoes.recebido, agora)
	}
}

// Falhar aqui não pode escalar: o worker segue vivo para a próxima virada.
func TestLimparSessoes_engoleOErro(t *testing.T) {
	t.Parallel()

	sessoes := &sessoesFalsas{err: errors.New("banco indisponível")}

	limparSessoes(t.Context(), quieto(), sessoes, time.Now())

	if sessoes.chamadas != 1 {
		t.Errorf("chamadas = %d, quer 1", sessoes.chamadas)
	}
}

func quieto() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
