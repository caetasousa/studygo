package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
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

	balanceamento := montarBalanceamento(c, salvo.Config, res, stats)

	// A fila é datada; cada dia do plano recebe o que vence nele. As atrasadas
	// caem todas no primeiro dia não-passado, para não sumirem da tela.
	vencendo := distribuirRevisoes(res.Dias, salvo.Revisoes, agora)

	ctxBlocos := plano.BlocoCtx{
		HorasDia: salvo.Config.HorasDia,
		Nomes:    nomes,
		Simulado: res.Simulado,
		Perfil:   salvo.Config.Perfil,
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

		d.Revisoes = vencendo[plano.DayOf(d.Data)]

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

		dr.Revisoes = revisoesDoDia(vencendo[plano.DayOf(d.Data)], salvo.Config, agora)

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
		Balanceamento: balanceamento,
		Props:         montarProps(salvo.Config, res.Dias, stats, agora),
		Alertas:       append(alertaOrcamento(balanceamento), montarAlertas(c, salvo.Marcos, agora)...),
		HojeIndex:     hojeIndex,
		Reordenados:   datasOrdenadas(salvo.Reordenacoes),
		GeradoEm:      s.clock.Now(),
	}

	return resp, nil
}

func montarConcurso(c concurso.Concurso) ConcursoResposta {
	discs := make([]DisciplinaResposta, 0, len(c.Disciplinas))

	for i, d := range c.Disciplinas {
		fontes := make([]FonteResposta, 0, len(d.Fontes))
		for _, f := range d.Fontes {
			fontes = append(fontes, FonteResposta{Titulo: f.Titulo, URL: f.URL, Tipo: f.Tipo})
		}

		discs = append(discs, DisciplinaResposta{
			Codigo: d.Codigo,
			Nome:   d.Nome,
			Bloco:  string(d.Bloco),
			Peso:   d.Peso,
			Cor:    i % 13,
			Temas:  d.Temas,
			Fontes: fontes,
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
		Perfil:        montarPerfil(salvo.Config.Perfil, salvo.Config.HorasDia),
	}
}

func montarPerfil(p plano.Perfil, horasDia float64) PerfilResposta {
	p = p.Normalizar()

	modos := make(map[string]string, len(p.Modos))
	for codigo, m := range p.Modos {
		modos[codigo] = string(m)
	}

	reforcos := make(map[string]float64, len(p.Reforcos))
	for codigo := range p.Reforcos {
		reforcos[codigo] = p.ReforcoDe(codigo)
	}

	ciclo := make([]CicloItemInput, 0, len(p.CicloRevisao))
	for _, it := range p.CicloRevisao {
		ciclo = append(ciclo, CicloItemInput{Titulo: it.Titulo, Questoes: it.Questoes})
	}

	return PerfilResposta{
		BlocosPorDia:       p.BlocosPorDia,
		PctRevisao:         p.PctRevisao,
		Reforcos:           reforcos,
		CicloRevisao:       ciclo,
		MinutosPorBloco:    minutosPorBloco(horasDia, p),
		Simulados:          string(p.Simulados),
		Discursiva:         p.Discursiva,
		Intervalos:         p.Intervalos,
		PctQuestoes:        p.PctQuestoes,
		RevisaoPorQuestoes: p.RevisaoPorQuestoes,
		QuestoesPorRevisao: p.QuestoesPorRevisao,
		LimiarFraco:        p.LimiarFraco,
		Modos:              modos,
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
	perfil := cfg.Perfil.Normalizar()
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
			QuestoesEdital: d.QuestoesPadrao,
			Delta:          cfg.Questoes[d.Codigo] - d.QuestoesPadrao,
			Modo:           string(perfil.ModoDe(d.Codigo)),
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

// alertaOrcamento warns when the user's question estimates no longer add up to
// the edital's totals. The engine splits time strictly in proportion, so raising
// one discipline silently takes time from every other one — this is what says so
// out loud, and names where the excess (or the gap) is.
func alertaOrcamento(linhas []LinhaBalanceamento) []AlertaResposta {
	var (
		total  int
		edital int
	)

	for _, l := range linhas {
		total += l.Questoes
		edital += l.QuestoesEdital
	}

	if edital == 0 || total == edital {
		return []AlertaResposta{}
	}

	sobra := total - edital

	nivel := "warn"
	if abs(sobra)*4 > edital {
		nivel = "danger"
	}

	verbo, direcao := "tire", 1
	if sobra < 0 {
		verbo, direcao = "distribua mais", -1
	}

	titulo := fmt.Sprintf(
		"Você distribuiu %d de %d questões — %s %d",
		total, edital, verbo, abs(sobra),
	)

	return []AlertaResposta{{
		Nivel:  nivel,
		Titulo: titulo,
		Texto:  textoOrcamento(linhas, direcao),
	}}
}

// textoOrcamento names the two disciplines furthest from the edital in the
// direction that caused the imbalance.
func textoOrcamento(linhas []LinhaBalanceamento, direcao int) string {
	fora := make([]LinhaBalanceamento, 0, len(linhas))
	for _, l := range linhas {
		if l.Delta*direcao > 0 {
			fora = append(fora, l)
		}
	}

	if len(fora) == 0 {
		return "nenhuma disciplina está fora do edital — confira se o total de questões está certo"
	}

	sort.SliceStable(fora, func(i, j int) bool {
		return abs(fora[i].Delta) > abs(fora[j].Delta)
	})

	if len(fora) > 2 {
		fora = fora[:2]
	}

	partes := make([]string, 0, len(fora))
	for _, l := range fora {
		partes = append(partes, fmt.Sprintf("%s (%+d)", l.Nome, l.Delta))
	}

	return "provavelmente de " + strings.Join(partes, " e ")
}

func abs(x int) int {
	if x < 0 {
		return -x
	}

	return x
}

// distribuirRevisoes buckets the open queue by the plan day it falls on.
// Anything already overdue lands on the first day that is not in the past, so a
// missed review resurfaces instead of disappearing behind the calendar.
func distribuirRevisoes(
	dias []plano.Dia,
	fila []plano.Revisao,
	agora time.Time,
) map[time.Time][]plano.Revisao {
	out := map[time.Time][]plano.Revisao{}

	diasComTema := map[time.Time]bool{}
	primeiroAberto := time.Time{}

	for _, d := range dias {
		if len(d.Itens) == 0 {
			continue
		}

		dia := plano.DayOf(d.Data)
		diasComTema[dia] = true

		if primeiroAberto.IsZero() && !dia.Before(agora) {
			primeiroAberto = dia
		}
	}

	for _, r := range fila {
		if r.FeitaEm != nil {
			continue
		}

		alvo := plano.DayOf(r.VenceEm)

		if alvo.Before(agora) || !diasComTema[alvo] {
			if primeiroAberto.IsZero() {
				continue
			}

			alvo = primeiroAberto
		}

		out[alvo] = append(out[alvo], r)
	}

	return out
}

func revisoesDoDia(rs []plano.Revisao, cfg plano.Config, agora time.Time) []RevisaoResposta {
	perfil := cfg.Perfil.Normalizar()
	out := make([]RevisaoResposta, 0, len(rs))

	for _, r := range rs {
		intervalo := 0
		if r.Etapa < len(perfil.Intervalos) {
			intervalo = perfil.Intervalos[r.Etapa]
		}

		atraso := plano.DiffDays(plano.DayOf(r.VenceEm), agora)
		if atraso < 0 {
			atraso = 0
		}

		out = append(out, RevisaoResposta{
			ID:         r.ID,
			Disciplina: r.Disciplina,
			Tema:       r.Tema,
			Etapa:      r.Etapa,
			Intervalo:  intervalo,
			VenceEm:    r.VenceEm.Format(isoDate),
			Atraso:     atraso,
			Questoes:   perfil.QuestoesPorRevisao,
		})
	}

	return out
}

// minutosPorBloco is what one normal (reforço 1) study block lasts, rounded to
// five minutes — the number the profile screen shows next to blocos por dia.
func minutosPorBloco(horasDia float64, p plano.Perfil) int {
	if p.BlocosPorDia <= 0 {
		return 0
	}

	m := horasDia * 60 * (1 - p.PctRevisao) / float64(p.BlocosPorDia)

	return int(math.Round(m/5)) * 5
}
