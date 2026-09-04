package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"studygo/internal/domain/plano"
	"studygo/internal/port"
)

// NotificacaoService calcula e despacha o lembrete de revisão de cada usuário
// no dia. Quem o aciona é o cmd/worker.
type NotificacaoService struct {
	planos     port.PlanoRepository
	cronograma port.CronogramaRepository
	concursos  port.ConcursoRepository
	notifier   port.Notifier
	relogio    port.Clock
}

func NewNotificacaoService(
	planos port.PlanoRepository,
	cronograma port.CronogramaRepository,
	concursos port.ConcursoRepository,
	notifier port.Notifier,
	relogio port.Clock,
) *NotificacaoService {
	return &NotificacaoService{
		planos:     planos,
		cronograma: cronograma,
		concursos:  concursos,
		notifier:   notifier,
		relogio:    relogio,
	}
}

// EnviarLembretesDoDia percorre cada plano, descobre o que vence hoje e entrega
// o lembrete ao notifier.
func (s *NotificacaoService) EnviarLembretesDoDia(ctx context.Context) (int, error) {
	planos, err := s.planos.ParaLembrete(ctx)
	if err != nil {
		return 0, fmt.Errorf("listando planos: %w", err)
	}

	hoje := plano.DayOf(s.relogio.Now())
	enviados := 0

	for _, pu := range planos {
		itens, dica, err := s.lembreteDe(ctx, pu, hoje)
		if err != nil {
			return enviados, err
		}

		if len(itens) == 0 {
			continue
		}

		if err := s.notifier.EnviarLembrete(ctx, port.Lembrete{
			Email:   pu.Email,
			Nome:    pu.Nome,
			DataISO: hoje.Format(formatoISO),
			Itens:   itens,
			Dica:    dica,
		}); err != nil {
			return enviados, fmt.Errorf("enviando lembrete para %s: %w", pu.Email, err)
		}

		enviados++
	}

	return enviados, nil
}

// lembreteDe monta o que um usuário deve revisar hoje: o caderno de erros dele.
//
// Já foi a fila de revisão espaçada de intervalos fixos, que não existe mais —
// revisão agora é um bloco de todo dia de estudo, focado no que deu errado. O
// lembrete segue a mesma regra, nomeando os temas que o estudante vem errando.
func (s *NotificacaoService) lembreteDe(
	ctx context.Context,
	pu port.PlanoDoUsuario,
	hoje time.Time,
) ([]port.ItemLembrete, string, error) {
	c, err := s.concursos.PorID(ctx, pu.ConcursoID)
	if err != nil {
		return nil, "", fmt.Errorf("carregando concurso %s: %w", pu.ConcursoID, err)
	}

	atividades, err := s.cronograma.Atividades(ctx, pu.Plano.ID)
	if err != nil {
		return nil, "", err
	}

	registros, err := s.cronograma.Registros(ctx, pu.Plano.ID)
	if err != nil {
		return nil, "", err
	}

	res := plano.Gerar(pu.Plano.Config, &c)
	plano.AplicarNosDias(res.Dias, atividades)

	cx := contexto{
		Concurso:   c,
		Plano:      pu.Plano,
		Atividades: atividades,
		Registros:  registros,
	}

	cadernos := plano.Caderno(resultadosDoPlano(res.Dias, cx))

	itens := []port.ItemLembrete{}

	for _, disc := range chavesOrdenadas(cadernos) {
		for _, it := range cadernos[disc] {
			itens = append(itens, port.ItemLembrete{
				Distancia:  diasDesde(it.UltimaData, hoje),
				Disciplina: it.Disciplina,
				Tema:       it.Tema,
			})
		}
	}

	dica := ""

	for _, it := range itens {
		if d := c.DisciplinaPorCodigo(it.Disciplina); d != nil && len(d.Fontes) > 0 {
			dica = "ouça o Áudio Overview de " + d.Nome + " no NotebookLM durante o trajeto"

			break
		}
	}

	return itens, dica, nil
}

// chavesOrdenadas mantém a ordem do lembrete estável entre execuções, o que a
// iteração de um map não faria.
func chavesOrdenadas(m map[string][]plano.ItemCaderno) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

// diasDesde é há quantos dias o tema foi respondido pela última vez.
func diasDesde(ultima string, hoje time.Time) int {
	d, err := time.Parse(formatoISO, ultima)
	if err != nil {
		return 0
	}

	return plano.DiffDays(d, hoje)
}
