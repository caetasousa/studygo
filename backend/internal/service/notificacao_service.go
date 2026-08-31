package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"studygo/internal/domain/plano"
	"studygo/internal/port"
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
		// The notebook is built from what the plan scheduled, so the concurso has
		// to be loaded before there is anything to remind about.
		c, err := s.concursos.ConcursoByID(ctx, pce.ConcursoID)
		if err != nil {
			return enviados, fmt.Errorf("loading concurso %s: %w", pce.ConcursoID, err)
		}

		res := plano.Gerar(pce.Plano.Config, &c)

		itens := lembreteItens(pce.Plano, res.Dias, hoje)
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

// lembreteItens is what the reminder chases: the error notebook.
//
// It used to be the spaced-review queue, which no longer exists — review is a
// block of every study day now, drilling what went wrong. The reminder follows
// the same rule, naming the topics the student has actually been missing.
func lembreteItens(salvo plano.Salvo, dias []plano.Dia, hoje time.Time) []port.LembreteItem {
	itens := []port.LembreteItem{}

	cadernos := plano.Caderno(resultadosDoPlano(dias, salvo))

	for _, disc := range ordenarChaves(cadernos) {
		for _, it := range cadernos[disc] {
			itens = append(itens, port.LembreteItem{
				Distancia:  distanciaDe(it.UltimaData, hoje),
				Disciplina: it.Disciplina,
				Tema:       it.Tema,
			})
		}
	}

	return itens
}

// ordenarChaves keeps the reminder's order stable across runs, which map
// iteration would not.
func ordenarChaves(m map[string][]plano.ItemCaderno) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

// distanciaDe is how many days ago the topic was last answered.
func distanciaDe(ultima string, hoje time.Time) int {
	d, err := time.Parse(isoDate, ultima)
	if err != nil {
		return 0
	}

	return plano.DiffDays(d, hoje)
}
