package service

import (
	"context"
	"fmt"

	"annygo/internal/domain/plano"
	"annygo/internal/port"
)

// distancias are the spaced-review offsets, matching the artifact's D-1/D-7/D-30.
var distancias = []int{1, 7, 30}

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
		c, err := s.concursos.ConcursoByID(ctx, pce.ConcursoID)
		if err != nil {
			return enviados, fmt.Errorf("loading concurso %s: %w", pce.ConcursoID, err)
		}

		res := plano.Gerar(pce.Plano.Config, &c)
		plano.AplicarReordenacoes(res.Dias, pce.Plano.Reordenacoes)

		hojeIdx := -1
		for i, d := range res.Dias {
			if plano.DayOf(d.Data).Equal(hoje) {
				hojeIdx = i
				break
			}
		}

		if hojeIdx < 0 {
			continue
		}

		itens := lembreteItens(res.Dias, hojeIdx)
		if len(itens) == 0 {
			continue
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

func lembreteItens(dias []plano.Dia, hojeIdx int) []port.LembreteItem {
	itens := []port.LembreteItem{}

	for _, k := range distancias {
		j := hojeIdx - k
		if j < 0 {
			continue
		}

		d := dias[j]

		if len(d.Itens) == 0 {
			itens = append(itens, port.LembreteItem{
				Distancia:  k,
				Disciplina: "—",
				Tema:       d.Tema,
			})

			continue
		}

		for _, it := range d.Itens {
			itens = append(itens, port.LembreteItem{
				Distancia:  k,
				Disciplina: it.Disciplina,
				Tema:       it.Tema,
			})
		}
	}

	return itens
}
