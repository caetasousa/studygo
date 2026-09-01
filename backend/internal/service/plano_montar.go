package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

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

	// Normalize once here so every downstream reader — montarConfig, the block
	// breakdown, balanceamento — sees the same HorasDia (recomputed from
	// MinutosBloco) and clamped method fields.
	salvo.Config = salvo.Config.Normalizar()

	codes := make([]string, 0, len(c.Disciplinas))
	nomes := map[string]string{}

	for _, d := range c.Disciplinas {
		codes = append(codes, d.Codigo)
		nomes[d.Codigo] = d.Nome
	}

	res := plano.Gerar(salvo.Config, &c)
	plano.AplicarReordenacoes(res.Dias, salvo.Reordenacoes)

	// Individually moved activities win over the generated layout, for the days
	// they cover. A plan the user never rearranged reads exactly as before.
	atividades, err := s.planos.ListAtividades(ctx, salvo.ID)
	if err != nil {
		return PlanoResposta{}, err
	}

	plano.AplicarAtividades(res.Dias, atividades)

	// A plan the user never rearranged has no stored activities. Rather than
	// materialising them here (which would make a GET write), fall back to the
	// engine's deterministic slot ids, so the very first drag has something to
	// address. The first move replaces these with real uuids.
	if len(atividades) == 0 {
		atividades = plano.DerivarAtividades(res.Dias)
	}

	porDia := map[time.Time][]plano.Atividade{}
	for _, a := range atividades {
		k := plano.DayOf(a.Data)
		porDia[k] = append(porDia[k], a)
	}

	stats := plano.CalcularStats(res.Dias, codes, salvo.Registros)

	balanceamento := montarBalanceamento(c, salvo.Config, res, stats)

	// A fila é datada; cada dia do plano recebe o que vence nele. As atrasadas
	// caem todas no primeiro dia não-passado, para não sumirem da tela.
	ctxBlocos := plano.BlocoCtx{
		Cfg:      salvo.Config,
		Nomes:    nomes,
		Simulado: res.Simulado,
		Cadernos: plano.Caderno(resultadosDoPlano(res.Dias, salvo)),
		Revisao:  plano.FilaRevisao(res.Dias, temasPorRevisao(salvo.Config)),
	}

	// Loaded once for every day's review tail below, rather than per day: the
	// list is small, and re-querying it per day would be one query per day of
	// the whole plan.
	anotacoes, err := s.planos.ListAnotacoes(ctx, salvo.ID)
	if err != nil {
		return PlanoResposta{}, err
	}

	dias := make([]DiaResposta, 0, len(res.Dias))
	var hojeIndex *int

	// The days arrive already merged: AplicarAtividades collapses a repeated
	// topic before anything reads the items, so the pile can never reach either
	// the response or AtividadesFaltantes. Merging again here would be a second
	// copy of that rule, free to drift out of step with the one that matters.
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

		// Every item already carries the id it was reconciled to — a stored
		// activity's uuid, or the engine's slot id for one that has never been
		// arranged. Pairing by index instead would mis-address a day whose item
		// count no longer matches what was stored (raising blocosPorDia does
		// exactly that), leaving the extra subject with no id to move.
		porID := map[string]plano.Atividade{}
		for _, a := range porDia[plano.DayOf(d.Data)] {
			porID[a.ID] = a
		}

		for _, it := range d.Itens {
			item := ItemResposta{
				ID:         it.AtividadeID,
				Disciplina: it.Disciplina,
				Tema:       it.Tema,
				Passada:    it.Passada,
			}

			if a, ok := porID[it.AtividadeID]; ok {
				item.Movida = a.Movida()
			}

			dr.Itens = append(dr.Itens, item)
		}

		for _, b := range plano.Blocos(d, ctxBlocos) {
			dr.Blocos = append(dr.Blocos, BlocoResposta{
				Minutos: b.Minutos,
				Titulo:  b.Titulo,
				Detalhe: b.Detalhe,
			})

			if b.Disciplina != "" {
				dr.Revisao = montarRevisao(b.Disciplina, plano.DayOf(d.Data), salvo.Revisoes, anotacoes)
			}
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
		Balanceamento: balanceamento,
		Props:         montarProps(salvo.Config, res.Dias, stats, agora),
		Alertas: append(
			append(alertaCobertura(balanceamento), alertaOrcamento(balanceamento)...),
			montarAlertas(c, salvo.Marcos, agora)...,
		),
		HojeIndex:   hojeIndex,
		Reordenados: datasOrdenadas(salvo.Reordenacoes),
		GeradoEm:    s.clock.Now(),
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
			Codigo:     d.Codigo,
			Nome:       d.Nome,
			Bloco:      string(d.Bloco),
			Peso:       d.Peso,
			Cor:        i % 13,
			CadernoURL: d.CadernoURL,
			Temas:      d.Temas,
			Fontes:     fontes,
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
	cfg := salvo.Config.Normalizar()

	modos := make(map[string]string, len(cfg.Modos))
	for codigo, m := range cfg.Modos {
		modos[codigo] = string(m)
	}

	reforcos := make(map[string]float64, len(cfg.Reforcos))
	for codigo := range cfg.Reforcos {
		reforcos[codigo] = cfg.ReforcoDe(codigo)
	}

	ciclo := make([]CicloItemInput, 0, len(cfg.CicloRevisao))
	for _, it := range cfg.CicloRevisao {
		ciclo = append(ciclo, CicloItemInput{Titulo: it.Titulo, Questoes: it.Questoes})
	}

	// MinutosBloco is what the config screen edits. When the plan has never had
	// an explicit length (old rows), report the one implied by HorasDia so the
	// screen shows a real number and the first save solidifies it.
	minutos := cfg.MinutosBloco
	if minutos == 0 {
		minutos = minutosDe(cfg)
	}

	return ConfigResposta{
		Inicio:        cfg.Inicio.Format(isoDate),
		Prova:         cfg.Prova.Format(isoDate),
		HorasDia:      cfg.HorasDia,
		DiasEstudo:    cfg.DiasEstudo,
		DiaRevisao:    cfg.DiaRevisao,
		RetaFinalDias: cfg.RetaFinalDias,
		TemaUI:        salvo.TemaUI,
		Questoes:      cfg.Questoes,

		BlocosPorDia:   cfg.BlocosPorDia,
		MinutosBloco:   minutos,
		Reforcos:       reforcos,
		MinutosRevisao: cfg.MinutosRevisao,
		CicloRevisao:   ciclo,
		RevisaoSemanal: cfg.RevisaoSemanal,
		Simulados:      string(cfg.Simulados),
		Discursiva:     cfg.Discursiva,
		Modos:          modos,
		PctQuestoes:    cfg.PctQuestoes,
		LimiarFraco:    cfg.LimiarFraco,
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

// temasPorRevisao is how many topics one review block covers.
//
// Derived from the block's own length rather than configured separately: a
// 20-minute block cannot honestly ask for six topics. Roughly ten minutes each,
// with at least one whenever there is a block at all.
func temasPorRevisao(cfg plano.Config) int {
	cfg = cfg.Normalizar()

	if cfg.MinutosRevisao <= 0 {
		return 0
	}

	n := cfg.MinutosRevisao / 10
	if n < 1 {
		return 1
	}

	if n > 6 {
		return 6
	}

	return n
}

// resultadosDoPlano collects every answered battery in the plan, which is what
// the error notebook is built from.
//
// Two sources, because a topic can be answered in either: the day records
// (what was studied that day) and the spaced-review queue (what came back).
// Day records only name the discipline, so the topics come from the day the
// engine generated — that is the same pairing the schedule shows.
func resultadosDoPlano(dias []plano.Dia, salvo plano.Salvo) []plano.ResultadoTema {
	out := []plano.ResultadoTema{}

	for _, d := range dias {
		reg, ok := salvo.Registros[plano.DayOf(d.Data)]
		if !ok {
			continue
		}

		for _, b := range reg.Blocos {
			if b.Questoes == nil || *b.Questoes <= 0 {
				continue
			}

			acertos := 0
			if b.Acertos != nil {
				acertos = *b.Acertos
			}

			// The battery covers whatever that discipline studied that day.
			for _, it := range d.Itens {
				if it.Disciplina != b.Disciplina {
					continue
				}

				out = append(out, plano.ResultadoTema{
					Disciplina: b.Disciplina,
					Tema:       it.Tema,
					Data:       d.Data.Format(isoDate),
					Questoes:   *b.Questoes,
					Acertos:    acertos,
				})
			}
		}
	}

	return out
}

// intervalosDeRevisita measures how many days pass, on average, between two
// days that study the same discipline.
//
// Measured from the generated schedule rather than derived from a formula, so
// it reflects what the plan actually does — reinforcement, rest days and the
// reta final all bend the spacing, and a formula would quietly disagree with
// the calendar the student is looking at.
func intervalosDeRevisita(dias []plano.Dia) map[string]float64 {
	ultimo := map[string]time.Time{}
	soma := map[string]float64{}
	vaos := map[string]int{}

	for _, d := range dias {
		// A discipline scheduled twice in one day is still one visit.
		vistas := map[string]bool{}

		for _, it := range d.Itens {
			if it.Disciplina == "" || vistas[it.Disciplina] {
				continue
			}

			vistas[it.Disciplina] = true

			if ant, ok := ultimo[it.Disciplina]; ok {
				soma[it.Disciplina] += float64(plano.DiffDays(ant, d.Data))
				vaos[it.Disciplina]++
			}

			ultimo[it.Disciplina] = d.Data
		}
	}

	out := make(map[string]float64, len(soma))

	for cod, total := range soma {
		if vaos[cod] > 0 {
			out[cod] = round1(total / float64(vaos[cod]))
		}
	}

	return out
}

// revisoesRetaDe is how many times the reta final goes over a discipline.
//
// The reta works differently from the content phase, which is what made the
// naive slots/topics wrong: when a discipline gets fewer blocks than it has
// topics, `reparte` PARTITIONS the topic list across those blocks — one block
// covering "T1 · T2 · T3 · T4" — instead of dropping the rest. So any block at
// all means the whole subject is covered once, and extra blocks are extra full
// passes.
//
// The old figure said 0.1x for a subject the reta actually reviews completely.
func revisoesRetaDe(slots, temas int) float64 {
	if slots <= 0 {
		return 0
	}

	if temas <= 0 {
		return float64(slots)
	}

	if slots <= temas {
		// The blocks partition the whole list: exactly one complete pass.
		return 1
	}

	return round1(float64(slots) / float64(temas))
}

// passadasDe is how many times a set of slots covers a discipline's whole topic
// list — one complete pass over the subject, not per topic.
//
// A discipline with no topics of its own is headlined by its name (see the
// engine), so one slot is one full pass.
func passadasDe(slots, temas int) float64 {
	if temas <= 0 {
		if slots > 0 {
			return float64(slots)
		}

		return 0
	}

	return round1(float64(slots) / float64(temas))
}

func montarBalanceamento(
	c concurso.Concurso,
	cfg plano.Config,
	res plano.Resultado,
	stats plano.Stats,
) []LinhaBalanceamento {
	cfg = cfg.Normalizar()
	intervalos := intervalosDeRevisita(res.Dias)
	visitas := plano.VisitasPorDisciplina(res.Dias)
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
			QuestoesEdital: d.QuestoesPadrao,
			Delta:          cfg.Questoes[d.Codigo] - d.QuestoesPadrao,
			Modo:           string(cfg.ModoDe(d.Codigo)),
			Peso:           d.Peso,
			Pontos:         res.Pontos[d.Codigo],
			PctIdeal:       round1(pctIdeal),
			BlocosConteudo: res.Slots[d.Codigo],
			BlocosReta:     res.SlotsReta[d.Codigo],
			Temas:          len(d.Temas),
			Passadas:       passadasDe(res.Slots[d.Codigo], len(d.Temas)),
			Visitas:        visitas[d.Codigo],
			RevisoesGerais: revisoesRetaDe(res.SlotsReta[d.Codigo], len(d.Temas)),
			IntervaloDias:  intervalos[d.Codigo],
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
		VoltasRevisao:  round1(plano.VoltasRevisao(dias, temasPorRevisao(cfg))),
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
// alertaCobertura warns when the plan does not have time to go through a
// discipline even once.
//
// This is the failure a study plan must never hide: with a near exam date, or
// too many topics for the days available, the engine simply runs out of slots
// and whole subjects never appear. The schedule looked complete either way —
// the missing subject was only absent, and absence is hard to notice.
func alertaCobertura(linhas []LinhaBalanceamento) []AlertaResposta {
	incompletas := make([]LinhaBalanceamento, 0, len(linhas))
	nenhuma := 0

	for _, l := range linhas {
		if l.Temas == 0 || l.Passadas >= 1 {
			continue
		}

		incompletas = append(incompletas, l)

		if l.Passadas == 0 {
			nenhuma++
		}
	}

	if len(incompletas) == 0 {
		return []AlertaResposta{}
	}

	// A subject that never appears at all is a different order of problem from
	// one that is merely cut short.
	nivel := "warn"
	if nenhuma > 0 {
		nivel = "danger"
	}

	var b strings.Builder

	for i, l := range incompletas {
		if i > 0 {
			b.WriteString("; ")
		}

		b.WriteString(l.Nome)

		if l.Passadas == 0 {
			b.WriteString(" (não entra no plano)")

			continue
		}

		b.WriteString(" (")
		b.WriteString(strconv.Itoa(int(l.Passadas * 100)))
		b.WriteString("% do conteúdo)")
	}

	titulo := "O plano não cobre todo o conteúdo"
	if nenhuma > 0 {
		titulo = "Há matéria que não entra no plano"
	}

	return []AlertaResposta{{
		Nivel:  nivel,
		Titulo: titulo,
		Texto: "Não há dias suficientes até a prova para percorrer estas matérias " +
			"inteiras uma vez: " + b.String() + ". Adiante a data de início, " +
			"acrescente dias de estudo na semana, aumente os blocos por dia — ou " +
			"aceite e escolha por onde cortar, sabendo do buraco.",
	}}
}

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
