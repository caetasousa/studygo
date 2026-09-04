package service

import (
	"math"
	"strconv"
	"strings"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

func montarBalanceamento(
	c concurso.Concurso,
	cfg plano.Config,
	res plano.Resultado,
	stats plano.Stats,
) []LinhaBalanceamento {
	intervalos := intervalosDeRevisita(res.Dias)
	visitas := plano.VisitasPorDisciplina(res.Dias)
	horasPorBloco := cfg.HorasDia / float64(max(cfg.BlocosPorDia, 1))

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
			Cor:            i % concurso.TotalCoresDisciplinas,
			Questoes:       cfg.Questoes[d.Codigo],
			QuestoesEdital: d.QuestoesPadrao,
			Delta:          cfg.Questoes[d.Codigo] - d.QuestoesPadrao,
			Modo:           string(cfg.ModoDe(d.Codigo)),
			Peso:           d.Peso,
			Pontos:         res.Pontos[d.Codigo],
			PctIdeal:       arredondar1(pctIdeal),
			BlocosConteudo: res.Slots[d.Codigo],
			BlocosReta:     res.SlotsReta[d.Codigo],
			Temas:          len(d.Temas),
			Passadas:       passadasDe(res.Slots[d.Codigo], len(d.Temas)),
			Visitas:        visitas[d.Codigo],
			RevisoesGerais: revisoesRetaDe(res.SlotsReta[d.Codigo], len(d.Temas)),
			IntervaloDias:  intervalos[d.Codigo],
			HorasPrevisto: arredondar1(
				float64(res.Slots[d.Codigo]+res.SlotsReta[d.Codigo]) * horasPorBloco,
			),
			HorasLancado: arredondar1(sd.Horas),
			Desvio:       arredondar1(tempoPct - pctIdeal),
			AcertoPct:    acerto,
		})
	}

	return out
}

// intervalosDeRevisita mede de quantos em quantos dias, em média, a mesma
// matéria volta.
//
// Medido no cronograma real, e não por fórmula, para que reflita o que o plano
// de fato faz: reforço, dias de descanso e a reta final entortam o espaçamento,
// e uma fórmula discordaria em silêncio do calendário que o estudante vê.
func intervalosDeRevisita(dias []plano.Dia) map[string]float64 {
	ultimo := map[string]time.Time{}
	soma := map[string]float64{}
	vaos := map[string]int{}

	for _, d := range dias {
		// Uma matéria agendada duas vezes no mesmo dia ainda é uma visita só.
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
			out[cod] = arredondar1(total / float64(vaos[cod]))
		}
	}

	return out
}

// passadasDe é quantas vezes um conjunto de blocos cobre a lista inteira de
// temas de uma matéria — uma passada completa pela MATÉRIA, não por tema.
//
// Uma matéria sem temas próprios é encabeçada pelo próprio nome (ver o motor),
// então um bloco é uma passada completa.
func passadasDe(slots, temas int) float64 {
	if temas <= 0 {
		if slots > 0 {
			return float64(slots)
		}

		return 0
	}

	return arredondar1(float64(slots) / float64(temas))
}

// revisoesRetaDe é quantas vezes a reta final percorre uma matéria.
//
// A reta funciona diferente da fase de conteúdo, e é isso que tornava a divisão
// ingênua errada: quando a matéria recebe menos blocos do que tem temas,
// `reparte` PARTICIONA a lista de temas entre esses blocos — um bloco cobrindo
// "T1 · T2 · T3" — em vez de descartar o resto. Então qualquer bloco já
// significa a matéria coberta uma vez, e blocos extras são passadas extras.
func revisoesRetaDe(slots, temas int) float64 {
	switch {
	case slots <= 0:
		return 0
	case temas <= 0:
		return float64(slots)
	case slots <= temas:
		return 1
	default:
		return arredondar1(float64(slots) / float64(temas))
	}
}

func montarProps(
	cfg plano.Config,
	dias []plano.Dia,
	stats plano.Stats,
	agora time.Time,
) ResumoDoPlano {
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

	return ResumoDoPlano{
		FaltamDias:     max(plano.DiffDays(agora, cfg.Prova), 0),
		Progresso:      progresso,
		HorasTotal:     arredondar1(stats.HorasTotal),
		HorasAlvo:      arredondar1(float64(total) * cfg.HorasDia),
		AcertoPct:      acerto,
		TotalDias:      total,
		DiasConcluidos: stats.Feitos,
		VoltasRevisao:  arredondar1(plano.VoltasRevisao(dias, temasPorRevisao(cfg))),
	}
}

// montarAlertas redige em português as decisões que o domínio tomou.
//
// A REGRA de quando avisar mora em domain/plano/alerta.go; aqui só se escolhe a
// frase. Separar os dois é o que permite mudar o texto sem tocar na regra, e
// testar a regra sem depender da redação.
func montarAlertas(
	c concurso.Concurso,
	checks map[uuid.UUID]bool,
	linhas []LinhaBalanceamento,
	agora time.Time,
) []Alerta {
	out := []Alerta{}

	cobertura := make([]plano.LinhaCobertura, 0, len(linhas))
	for _, l := range linhas {
		cobertura = append(cobertura, plano.LinhaCobertura{
			Codigo:         l.Codigo,
			Nome:           l.Nome,
			Temas:          l.Temas,
			Passadas:       l.Passadas,
			Questoes:       l.Questoes,
			QuestoesEdital: l.QuestoesEdital,
			Delta:          l.Delta,
		})
	}

	if a := plano.CoberturaDoPlano(cobertura); a != nil {
		out = append(out, textoDaCobertura(*a))
	}

	if a := plano.OrcamentoDoPlano(cobertura); a != nil {
		out = append(out, textoDoOrcamento(*a))
	}

	return append(out, alertasDeMarco(c, checks, agora)...)
}

func textoDaCobertura(a plano.AlertaCobertura) Alerta {
	var b strings.Builder

	for i, d := range a.Incompletas {
		if i > 0 {
			b.WriteString("; ")
		}

		b.WriteString(d.Nome)

		if d.Passadas == 0 {
			b.WriteString(" (não entra no plano)")

			continue
		}

		b.WriteString(" (")
		b.WriteString(strconv.Itoa(int(d.Passadas * 100)))
		b.WriteString("% do conteúdo)")
	}

	titulo := "O plano não cobre todo o conteúdo"
	if a.SemNenhuma > 0 {
		titulo = "Há matéria que não entra no plano"
	}

	return Alerta{
		Nivel:  string(a.Severidade),
		Titulo: titulo,
		Texto: "Não há dias suficientes até a prova para percorrer estas matérias " +
			"inteiras uma vez: " + b.String() + ". Adiante a data de início, " +
			"acrescente dias de estudo na semana, aumente os blocos por dia — ou " +
			"aceite e escolha por onde cortar, sabendo do buraco.",
	}
}

func textoDoOrcamento(a plano.AlertaOrcamento) Alerta {
	verbo := "tire"
	if a.Sobra < 0 {
		verbo = "distribua mais"
	}

	sobra := a.Sobra
	if sobra < 0 {
		sobra = -sobra
	}

	var b strings.Builder

	b.WriteString("As mais fora do eixo: ")

	for i, d := range a.MaisForaDoEixo {
		if i > 0 {
			b.WriteString(", ")
		}

		b.WriteString(d.Nome)
		b.WriteString(" (")

		if d.Delta > 0 {
			b.WriteString("+")
		}

		b.WriteString(strconv.Itoa(d.Delta))
		b.WriteString(")")
	}

	if len(a.MaisForaDoEixo) == 0 {
		b.Reset()
		b.WriteString("Ajuste as questões por matéria na tela de balanceamento.")
	} else {
		b.WriteString(". O motor divide o tempo em proporção estrita, então " +
			"aumentar uma matéria tira tempo de todas as outras.")
	}

	return Alerta{
		Nivel: string(a.Severidade),
		Titulo: "Você distribuiu " + strconv.Itoa(a.Distribuidas) + " de " +
			strconv.Itoa(a.NoEdital) + " questões — " + verbo + " " + strconv.Itoa(sobra),
		Texto: b.String(),
	}
}

// alertasDeMarco avisa dos prazos do edital que ainda exigem ação. No máximo
// dois: uma lista longa de avisos deixa de ser aviso.
func alertasDeMarco(
	c concurso.Concurso,
	checks map[uuid.UUID]bool,
	agora time.Time,
) []Alerta {
	out := []Alerta{}

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
			out = append(out, Alerta{
				Nivel:  string(plano.SeveridadePerigo),
				Titulo: "Prazo aberto agora — encerra em " + dataCurta(fim),
				Texto:  m.Titulo,
			})
		case dist <= 7:
			out = append(out, Alerta{
				Nivel: string(plano.SeveridadeAviso),
				Titulo: "Faltam " + strconv.Itoa(dist) + " dias — abre em " +
					dataCurta(m.DataInicio),
				Texto: m.Titulo,
			})
		}

		if len(out) == 2 {
			break
		}
	}

	return out
}

func dataCurta(t time.Time) string {
	return t.Format("02/01")
}
