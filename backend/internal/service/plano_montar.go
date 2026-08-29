package service

import (
	"context"
	"math"
	"sort"
	"strconv"
	"time"

	"annygo/internal/domain/concurso"
	"annygo/internal/domain/plano"

	"github.com/google/uuid"
)

// montar runs the engine, folds in the persisted state and builds the fat
// response payload.
func (s *PlanoService) montar(
	ctx context.Context,
	c concurso.Concurso,
	salvo plano.Salvo,
) (PlanoResposta, error) {
	agora := plano.DayOf(s.clock.Now())

	codes := make([]string, 0, len(c.Disciplinas))
	nomes := map[string]string{}

	for _, d := range c.Disciplinas {
		codes = append(codes, d.Codigo)
		nomes[d.Codigo] = d.Nome
	}

	res := plano.Gerar(salvo.Config, &c)
	plano.AplicarReordenacoes(res.Dias, salvo.Reordenacoes)

	stats := plano.CalcularStats(res.Dias, codes, salvo.Registros)

	ctxBlocos := plano.BlocoCtx{
		HorasDia: salvo.Config.HorasDia,
		Nomes:    nomes,
		Simulado: res.Simulado,
	}

	dias := make([]DiaResposta, 0, len(res.Dias))
	var hojeIndex *int

	for i, d := range res.Dias {
		dr := DiaResposta{
			N:      d.N,
			Data:   d.Data.Format(isoDate),
			Semana: d.Semana,
			Fase:   string(d.Fase),
			Tipo:   string(d.Tipo),
			Tema:   d.Tema,
			Meta:   d.Meta,
			Itens:  make([]ItemResposta, 0, len(d.Itens)),
		}

		for _, it := range d.Itens {
			dr.Itens = append(dr.Itens, ItemResposta{
				Disciplina: it.Disciplina,
				Tema:       it.Tema,
				Passada:    it.Passada,
			})
		}

		for _, b := range plano.Blocos(d, ctxBlocos) {
			dr.Blocos = append(dr.Blocos, BlocoResposta{
				Minutos: b.Minutos,
				Titulo:  b.Titulo,
				Detalhe: b.Detalhe,
			})
		}

		if r, ok := salvo.Registros[plano.DayOf(d.Data)]; ok {
			dr.Registro = registroToResposta(r)
		}

		if _, ok := salvo.Reordenacoes[plano.DayOf(d.Data)]; ok {
			dr.Reordenado = true
		}

		if hojeIndex == nil && !d.Data.Before(agora) {
			idx := i
			hojeIndex = &idx
		}

		dias = append(dias, dr)
	}

	resp := PlanoResposta{
		Concurso:      montarConcurso(c),
		Config:        montarConfig(salvo),
		Dias:          dias,
		Marcos:        montarMarcos(c, salvo.Marcos),
		Balanceamento: montarBalanceamento(c, salvo.Config, res, stats),
		Props:         montarProps(salvo.Config, res.Dias, stats, agora),
		Alertas:       montarAlertas(c, salvo.Marcos, agora),
		HojeIndex:     hojeIndex,
		Reordenados:   datasOrdenadas(salvo.Reordenacoes),
		GeradoEm:      s.clock.Now(),
	}

	return resp, nil
}

func montarConcurso(c concurso.Concurso) ConcursoResposta {
	discs := make([]DisciplinaResposta, 0, len(c.Disciplinas))

	for i, d := range c.Disciplinas {
		discs = append(discs, DisciplinaResposta{
			Codigo: d.Codigo,
			Nome:   d.Nome,
			Bloco:  string(d.Bloco),
			Peso:   d.Peso,
			Cor:    i % 13,
			Temas:  d.Temas,
		})
	}

	conteudo := make([]ConteudoResposta, 0, len(c.Conteudo))
	for _, item := range c.Conteudo {
		conteudo = append(conteudo, ConteudoResposta{Tipo: item.Tipo, Texto: item.Texto})
	}

	return ConcursoResposta{
		Slug:        c.Slug,
		Nome:        c.Nome,
		Banca:       c.Banca,
		Cargo:       c.Cargo,
		Emoji:       c.Emoji,
		Resumo:      c.Resumo,
		Disciplinas: discs,
		Conteudo:    conteudo,
	}
}

func montarConfig(salvo plano.Salvo) ConfigResposta {
	return ConfigResposta{
		Inicio:        salvo.Config.Inicio.Format(isoDate),
		Prova:         salvo.Config.Prova.Format(isoDate),
		HorasDia:      salvo.Config.HorasDia,
		DiasEstudo:    salvo.Config.DiasEstudo,
		DiaRevisao:    salvo.Config.DiaRevisao,
		RetaFinalDias: salvo.Config.RetaFinalDias,
		TemaUI:        salvo.TemaUI,
		Questoes:      salvo.Config.Questoes,
	}
}

func montarMarcos(c concurso.Concurso, checks map[uuid.UUID]bool) []MarcoResposta {
	out := make([]MarcoResposta, 0, len(c.Marcos))

	for _, m := range c.Marcos {
		var fim *string
		if m.DataFim != nil {
			s := m.DataFim.Format(isoDate)
			fim = &s
		}

		out = append(out, MarcoResposta{
			ID:         m.ID,
			Rotulo:     m.Rotulo,
			DataInicio: m.DataInicio.Format(isoDate),
			DataFim:    fim,
			Titulo:     m.Titulo,
			ExigeAcao:  m.ExigeAcao,
			EProva:     m.EProva,
			Cumprido:   checks[m.ID],
		})
	}

	return out
}

func montarBalanceamento(
	c concurso.Concurso,
	cfg plano.Config,
	res plano.Resultado,
	stats plano.Stats,
) []LinhaBalanceamento {
	hBloco := cfg.HorasDia / 2
	out := make([]LinhaBalanceamento, 0, len(c.Disciplinas))

	for i, d := range c.Disciplinas {
		sd := stats.Disciplina[d.Codigo]

		var pctIdeal float64
		if res.SomaPontos != 0 {
			pctIdeal = float64(res.Pontos[d.Codigo]) / float64(res.SomaPontos) * 100
		}

		var tempoPct float64
		if stats.HorasTotal != 0 {
			tempoPct = sd.Horas / stats.HorasTotal * 100
		}

		var acerto *int
		if sd.Questoes > 0 {
			v := int(math.Round(sd.Acertos / sd.Questoes * 100))
			acerto = &v
		}

		out = append(out, LinhaBalanceamento{
			Codigo:         d.Codigo,
			Nome:           d.Nome,
			Bloco:          string(d.Bloco),
			Cor:            i % 13,
			Questoes:       cfg.Questoes[d.Codigo],
			Peso:           d.Peso,
			Pontos:         res.Pontos[d.Codigo],
			PctIdeal:       round1(pctIdeal),
			BlocosConteudo: res.Slots[d.Codigo],
			BlocosReta:     res.SlotsReta[d.Codigo],
			HorasPrevisto:  round1(float64(res.Slots[d.Codigo]+res.SlotsReta[d.Codigo]) * hBloco),
			HorasLancado:   round1(sd.Horas),
			Desvio:         round1(tempoPct - pctIdeal),
			AcertoPct:      acerto,
		})
	}

	return out
}

func montarProps(cfg plano.Config, dias []plano.Dia, stats plano.Stats, agora time.Time) PropsResposta {
	total := len(dias)

	progresso := 0
	if total > 0 {
		progresso = int(math.Round(float64(stats.Feitos) / float64(total) * 100))
	}

	var acerto *int
	if stats.QuestoesTotal > 0 {
		v := int(math.Round(float64(stats.AcertosTotal) / float64(stats.QuestoesTotal) * 100))
		acerto = &v
	}

	faltam := plano.DiffDays(agora, cfg.Prova)
	if faltam < 0 {
		faltam = 0
	}

	return PropsResposta{
		FaltamDias:     faltam,
		Progresso:      progresso,
		HorasTotal:     round1(stats.HorasTotal),
		HorasAlvo:      round1(float64(total) * cfg.HorasDia),
		AcertoPct:      acerto,
		TotalDias:      total,
		DiasConcluidos: stats.Feitos,
	}
}

func montarAlertas(c concurso.Concurso, checks map[uuid.UUID]bool, agora time.Time) []AlertaResposta {
	out := []AlertaResposta{}

	for _, m := range c.Marcos {
		if !m.ExigeAcao || checks[m.ID] {
			continue
		}

		fim := m.DataInicio
		if m.DataFim != nil {
			fim = *m.DataFim
		}

		if plano.DayOf(fim).Before(agora) {
			continue
		}

		dist := plano.DiffDays(agora, m.DataInicio)

		switch {
		case dist <= 0:
			out = append(out, AlertaResposta{
				Nivel:  "danger",
				Titulo: "Prazo aberto agora — encerra em " + fmtCurto(fim),
				Texto:  m.Titulo,
			})
		case dist <= 7:
			out = append(out, AlertaResposta{
				Nivel:  "warn",
				Titulo: "Faltam " + strconv.Itoa(dist) + " dias — abre em " + fmtCurto(m.DataInicio),
				Texto:  m.Titulo,
			})
		}

		if len(out) == 2 {
			break
		}
	}

	return out
}

func datasOrdenadas(reord map[time.Time]plano.Reordenacao) []string {
	out := make([]string, 0, len(reord))
	for d := range reord {
		out = append(out, d.Format(isoDate))
	}

	sort.Strings(out)

	return out
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func fmtCurto(t time.Time) string {
	return t.Format("02/01")
}
