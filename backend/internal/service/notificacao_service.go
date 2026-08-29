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

		itens := lembreteItens(res.Dias, hoje, pce.Plano.Config.Perfil.Normalizar().Intervalos)
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

// lembreteItens collects what is due for review today, one entry per interval.
// The offsets are calendar days, not positions in the plan: for someone who
// studies Monday to Friday, "seven days ago" is not seven plan days back.
func lembreteItens(dias []plano.Dia, hoje time.Time, intervalos []int) []port.LembreteItem {
	itens := []port.LembreteItem{}

	porData := make(map[time.Time]plano.Dia, len(dias))
	for _, d := range dias {
		porData[plano.DayOf(d.Data)] = d
	}

	for _, k := range intervalos {
		d, ok := porData[plano.AddDays(hoje, -k)]
		if !ok {
			continue
		}

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
