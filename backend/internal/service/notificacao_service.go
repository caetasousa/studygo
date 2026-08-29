package service

import (
	"context"
	"fmt"
	"time"

	"annygo/internal/domain/plano"
	"annygo/internal/port"
)

// NotificacaoService computes and dispatches each user's spaced-review reminder
// for the current day. It is driven by cmd/worker.
type NotificacaoService struct {
	planos    port.PlanoRepository
	concursos port.ConcursoRepository
	notifier  port.Notifier
	clock     port.Clock
}

func NewNotificacaoService(
	planos port.PlanoRepository,
	concursos port.ConcursoRepository,
	notifier port.Notifier,
	clock port.Clock,
) *NotificacaoService {
	return &NotificacaoService{
		planos:    planos,
		concursos: concursos,
		notifier:  notifier,
		clock:     clock,
	}
}

// EnviarLembretesDoDia walks every plan, works out which topics fall due for
// review today, and hands each reminder to the notifier.
func (s *NotificacaoService) EnviarLembretesDoDia(ctx context.Context) (int, error) {
	planos, err := s.planos.ListPlanosParaLembrete(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing planos: %w", err)
	}

	hoje := plano.DayOf(s.clock.Now())
	enviados := 0

	for _, pce := range planos {
		itens := lembreteItens(pce.Plano, hoje)
		if len(itens) == 0 {
			continue
		}

		c, err := s.concursos.ConcursoByID(ctx, pce.ConcursoID)
		if err != nil {
			return enviados, fmt.Errorf("loading concurso %s: %w", pce.ConcursoID, err)
		}

		dica := ""
		for _, it := range itens {
			if d := c.DisciplinaByCodigo(it.Disciplina); d != nil && len(d.Fontes) > 0 {
				dica = "ouça o Áudio Overview de " + d.Nome + " no NotebookLM durante o trajeto"
				break
			}
		}

		if err := s.notifier.EnviarLembrete(ctx, port.Lembrete{
			Email:   pce.Email,
			Nome:    pce.Nome,
			DataISO: hoje.Format(isoDate),
			Itens:   itens,
			Dica:    dica,
		}); err != nil {
			return enviados, fmt.Errorf("sending lembrete to %s: %w", pce.Email, err)
		}

		enviados++
	}

	return enviados, nil
}

// lembreteItens collects what the spaced-review queue owes today — overdue
// entries included, since a missed review is exactly the one worth chasing.
func lembreteItens(salvo plano.Salvo, hoje time.Time) []port.LembreteItem {
	itens := []port.LembreteItem{}

	for _, r := range plano.VencidasAte(salvo.Revisoes, hoje) {
		itens = append(itens, port.LembreteItem{
			Distancia:  plano.DiffDays(r.OrigemData, hoje),
			Disciplina: r.Disciplina,
			Tema:       r.Tema,
		})
	}

	return itens
}
