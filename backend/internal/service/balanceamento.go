package service

import (
	"math"
	"strconv"
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
	atividades []plano.Atividade,
	concluida func(uuid.UUID) bool,
) []LinhaBalanceamento {
	intervalos := intervalosDeRevisita(res.Dias)
	visitas := plano.VisitasPorDisciplina(res.Dias)
	horasPorBloco := cfg.HorasDia / float64(max(cfg.BlocosPorDia, 1))

	out := make([]LinhaBalanceamento, 0, len(c.Disciplinas))

	for i, d := range c.Disciplinas {
		sd := stats.Disciplina[d.Codigo]

		// "Aprendo" e o aviso de matéria incompleta vêm do cronograma gravado:
		// o que foi antecipado, reorganizado ou tirado por repetição conta como
		// está, e cada tópico conta dentro dos blocos que juntam vários.
		adiada := cfg.NaRetaFinal(d.Codigo)
		cobertura := plano.CoberturaDaMateria(res.Dias, d.Codigo, d.Temas, adiada)
		passadas := passadasDe(res.Slots[d.Codigo], len(d.Temas))
		cobertos := len(d.Temas)

		if len(d.Temas) > 0 {
			passadas = arredondar1(float64(cobertura.Aparicoes) / float64(len(d.Temas)))
			cobertos = cobertura.Cobertos
		}

		revisoes := revisoesRetaDe(res.SlotsReta[d.Codigo], len(d.Temas))
		if adiada {
			revisoes = 0
		}

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
			Cor:            concurso.Cor(i),
			Questoes:       cfg.Questoes[d.Codigo],
			QuestoesEdital: d.QuestoesPadrao,
			Delta:          cfg.Questoes[d.Codigo] - d.QuestoesPadrao,
			Modo:           string(cfg.ModoDe(d.Codigo)),
			SoNaRetaFinal:  adiada,
			Peso:           d.Peso,
			Pontos:         res.Pontos[d.Codigo],
			PctIdeal:       arredondar1(pctIdeal),
			BlocosConteudo: res.Slots[d.Codigo],
			BlocosReta:     res.SlotsReta[d.Codigo],
			Temas:          len(d.Temas),
			TemasEstudados: plano.TopicosEstudados(d.Codigo, d.Temas, atividades, concluida),
			Passadas:       passadas,
			TemasCobertos:  cobertos,
			Visitas:        visitas[d.Codigo],
			RevisoesGerais: revisoes,
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

// montarAlertas avisa só dos três prazos que importam a quem estuda: o período
// de inscrições, o último dia de pagamento da inscrição e a prova. A cobertura
// e o orçamento de questões têm lugar na tela de balanceamento, e os demais
// marcos (isenção, recursos, divulgações), na de datas — no topo de toda tela,
// eram avisos demais para os três que de fato não podem passar.
//
// Cada um aparece até a data passar (ou o prazo ser marcado como cumprido), e
// o tom sobe conforme ela chega.
func montarAlertas(
	c concurso.Concurso,
	checks map[uuid.UUID]bool,
	prova time.Time,
	agora time.Time,
) []Alerta {
	agora = plano.DayOf(agora)
	p := concurso.PrazosChave(c.Marcos)
	out := []Alerta{}

	if m := p.Inscricao; m != nil && !checks[m.ID] {
		inicio, fim := plano.DayOf(m.DataInicio), plano.DayOf(fimDoMarco(*m))

		switch {
		case fim.Before(agora):
		case !agora.Before(inicio):
			out = append(out, Alerta{
				Nivel:  nivelPorProximidade(plano.DiffDays(agora, fim), true),
				Titulo: "Inscrições abertas até " + dataCurta(fim),
				Texto:  "Encerram " + quando(plano.DiffDays(agora, fim)) + ".",
			})
		default:
			titulo := "Inscrições em " + dataCurta(inicio)
			if !fim.Equal(inicio) {
				titulo = "Inscrições de " + dataCurta(inicio) + " a " + dataCurta(fim)
			}

			out = append(out, Alerta{
				Nivel:  nivelPorProximidade(plano.DiffDays(agora, inicio), false),
				Titulo: titulo,
				Texto:  "Abrem " + quando(plano.DiffDays(agora, inicio)) + ".",
			})
		}
	}

	if m := p.Pagamento; m != nil && !checks[m.ID] {
		if fim := plano.DayOf(fimDoMarco(*m)); !fim.Before(agora) {
			out = append(out, Alerta{
				Nivel:  nivelPorProximidade(plano.DiffDays(agora, fim), true),
				Titulo: "Pagamento da inscrição até " + dataCurta(fim),
				Texto:  "O boleto vence " + quando(plano.DiffDays(agora, fim)) + ".",
			})
		}
	}

	if prova.IsZero() && p.Prova != nil {
		prova = p.Prova.DataInicio
	}

	if dia := plano.DayOf(prova); !prova.IsZero() && !dia.Before(agora) {
		out = append(out, Alerta{
			Nivel:  nivelPorProximidade(plano.DiffDays(agora, dia), false),
			Titulo: "Prova em " + dia.Format("02/01/2006"),
			Texto:  "É " + quando(plano.DiffDays(agora, dia)) + ".",
		})
	}

	return out
}

func fimDoMarco(m concurso.Marco) time.Time {
	if m.DataFim != nil {
		return *m.DataFim
	}

	return m.DataInicio
}

// nivelPorProximidade: o prazo que encerra (inscrição aberta, boleto) fica
// vermelho nos últimos 3 dias; qualquer um fica amarelo na última semana.
func nivelPorProximidade(dias int, encerra bool) string {
	switch {
	case encerra && dias <= 3:
		return string(plano.SeveridadePerigo)
	case dias <= 7:
		return string(plano.SeveridadeAviso)
	default:
		return string(plano.SeveridadeInfo)
	}
}

func quando(dias int) string {
	switch dias {
	case 0:
		return "hoje"
	case 1:
		return "amanhã"
	default:
		return "em " + strconv.Itoa(dias) + " dias"
	}
}

func dataCurta(t time.Time) string {
	return t.Format("02/01")
}
