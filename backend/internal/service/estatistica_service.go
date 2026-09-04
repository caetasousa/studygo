package service

import (
	"context"
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// EstatisticaService responde "como está indo": a série histórica, o resumo por
// semana e o balanceamento por disciplina.
type EstatisticaService struct {
	carregador
}

func NewEstatisticaService(deps Dependencias) *EstatisticaService {
	return &EstatisticaService{deps.carregador()}
}

func (s *EstatisticaService) Estatisticas(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) (Estatisticas, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return Estatisticas{}, err
	}

	cfg := c.Plano.Config.Normalizar()

	codigos := make([]string, 0, len(c.Concurso.Disciplinas))
	for _, d := range c.Concurso.Disciplinas {
		codigos = append(codigos, d.Codigo)
	}

	res := plano.Gerar(cfg, &c.Concurso)
	plano.AplicarNosDias(res.Dias, c.Atividades)

	stats := plano.CalcularStats(res.Dias, codigos, c.Atividades, c.Registros)

	serie := []PontoDaSerie{}
	porSemana := map[int]*ResumoDaSemana{}
	ordemSemanas := []int{}

	for _, d := range res.Dias {
		dt := plano.DayOf(d.Data)

		rs, ok := porSemana[d.Semana]
		if !ok {
			rs = &ResumoDaSemana{Semana: d.Semana}
			porSemana[d.Semana] = rs
			ordemSemanas = append(ordemSemanas, d.Semana)
		}

		rs.HorasPrevisto += cfg.HorasDia

		horas, questoes, acertos := plano.TotaisDoDia(c.Atividades, c.Registros, dt)
		if horas == nil && questoes == nil && acertos == nil {
			continue
		}

		h := valorFloat(horas)
		q := valorInt(questoes)
		a := valorInt(acertos)

		rs.Horas += h
		rs.Questoes += q
		rs.Acertos += a

		serie = append(serie, PontoDaSerie{
			Data:     dt.Format(formatoISO),
			Horas:    arredondar1(h),
			Questoes: q,
			Acertos:  a,
		})
	}

	semanas := make([]ResumoDaSemana, 0, len(ordemSemanas))

	for _, sm := range ordemSemanas {
		rs := porSemana[sm]
		rs.Horas = arredondar1(rs.Horas)
		rs.HorasPrevisto = arredondar1(rs.HorasPrevisto)
		semanas = append(semanas, *rs)
	}

	var acertoPct *int

	if stats.QuestoesTotal > 0 {
		v := stats.AcertosTotal * 100 / stats.QuestoesTotal
		acertoPct = &v
	}

	return Estatisticas{
		Serie:         serie,
		PorSemana:     semanas,
		PorDisciplina: montarBalanceamento(c.Concurso, cfg, res, stats),
		Streak: calcularStreak(
			res.Dias, c.Atividades, c.Registros, plano.DayOf(s.relogio.Now()),
		),
		HorasTotal:    arredondar1(stats.HorasTotal),
		QuestoesTotal: stats.QuestoesTotal,
		AcertoPct:     acertoPct,
	}, nil
}

// calcularStreak conta dias concluídos consecutivos, andando para trás a partir
// do dia mais recente que não está no futuro.
func calcularStreak(
	dias []plano.Dia,
	atividades []plano.Atividade,
	registros plano.Registros,
	hoje time.Time,
) int {
	fim := -1

	for i, d := range dias {
		if plano.DayOf(d.Data).After(hoje) {
			break
		}

		fim = i
	}

	streak := 0

	for i := fim; i >= 0; i-- {
		if !plano.DiaConcluido(atividades, registros, plano.DayOf(dias[i].Data)) {
			break
		}

		streak++
	}

	return streak
}

func valorFloat(p *float64) float64 {
	if p == nil {
		return 0
	}

	return *p
}

func valorInt(p *int) int {
	if p == nil {
		return 0
	}

	return *p
}
