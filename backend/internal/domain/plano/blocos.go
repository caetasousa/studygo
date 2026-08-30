package plano

import (
	"math"
	"strconv"
)

// Bloco is one time slice of a day's routine: minutes, a label, and what to do.
type Bloco struct {
	Minutos int    `json:"minutos"`
	Titulo  string `json:"titulo"`
	Detalhe string `json:"detalhe"`
}

// BlocoCtx carries everything the breakdown needs beyond the day itself.
type BlocoCtx struct {
	Cfg      Config            // the user's plan settings (dates, rhythm, method)
	Nomes    map[string]string // discipline codigo -> display name
	Simulado Composicao        // question split of a full mock exam
}

// Composicao is how many questions of each bloco a full exam has.
type Composicao struct {
	Gerais      int
	Especificas int
}

// Blocos returns the timed breakdown for a day, mirroring the artifact's
// blocos(): content days get two study blocks plus a spaced-review tail; the
// special days get their own fixed splits.
func Blocos(d Dia, ctx BlocoCtx) []Bloco {
	cfg := ctx.Cfg.Normalizar()
	h := cfg.HorasDia * 60

	if len(d.Itens) > 0 {
		rev := d.Tipo == TipoRevisaoDirigida

		// Sem nada vencendo, o tempo da revisão volta para o conteúdo em vez de
		// mandar revisar o vazio.
		pctRevisao := cfg.PctRevisao
		if len(d.Revisoes) == 0 {
			pctRevisao = 0
		}

		minutos := repartirMinutos(h*(1-pctRevisao), d.Itens, cfg)
		out := make([]Bloco, 0, len(d.Itens)+1)

		for idx, it := range d.Itens {
			// As questões do dia seguem o mesmo peso dos minutos: a matéria
			// reforçada leva um bloco maior e uma bateria maior junto.
			porBloco := 0
			if h > 0 {
				porBloco = int(math.Round(float64(d.Meta) * float64(minutos[idx]) / (h * (1 - pctRevisao))))
			}

			out = append(out, Bloco{
				Minutos: minutos[idx],
				Titulo:  rotuloBloco(idx) + " — " + ctx.Nomes[it.Disciplina],
				Detalhe: detalheDoBloco(cfg.ModoDe(it.Disciplina), rev, porBloco),
			})
		}

		if pctRevisao == 0 {
			return out
		}

		out = append(out, Bloco{
			Minutos: m5(h * pctRevisao),
			Titulo:  "Revisão espaçada — " + strconv.Itoa(len(d.Revisoes)) + " temas",
			Detalhe: revisaoDetalhe(cfg, d.Revisoes),
		})

		return out
	}

	switch d.Tipo {
	case TipoSimulado:
		return []Bloco{
			{
				Minutos: m5(h * 0.70),
				Titulo:  "Simulado cronometrado",
				Detalhe: strconv.Itoa(ctx.Simulado.Gerais) + " gerais + " +
					strconv.Itoa(ctx.Simulado.Especificas) +
					" específicos, sem pausa e sem consulta",
			},
			{Minutos: m5(h * 0.30), Titulo: "Correção comentada", Detalhe: "cada erro vira uma linha no caderno de erros"},
		}
	case TipoDiscursiva:
		return []Bloco{
			{Minutos: m5(h * 0.20), Titulo: "Leitura e roteiro", Detalhe: "identifique o comando e monte o esqueleto da resposta"},
			{Minutos: m5(h * 0.55), Titulo: "Redação completa", Detalhe: "escreva a discursiva inteira dentro do tempo"},
			{Minutos: m5(h * 0.25), Titulo: "Autocorreção", Detalhe: "confronte com a estrutura esperada e reescreva o pior trecho"},
		}
	case TipoVespera:
		return []Bloco{
			{Minutos: m5(h * 0.5), Titulo: "Leitura leve", Detalhe: "só resumos e mapas; nada de assunto novo"},
			{Minutos: m5(h * 0.2), Titulo: "Logística", Detalhe: "documento com foto, comprovante, local, horário e trajeto"},
			{Minutos: m5(h * 0.3), Titulo: "Descanso", Detalhe: "durma cedo; nesta altura, sono rende mais que revisão"},
		}
	default: // weekly review — foco em resolução de questões
		meta := d.Meta
		if meta <= 0 {
			meta = 40
		}

		return []Bloco{
			{
				Minutos: m5(h * 0.55),
				Titulo:  "Resolução de questões — bateria no peso da prova",
				Detalhe: strconv.Itoa(meta) + " questões, mais específicas que gerais, cronometradas e corrigidas uma a uma",
			},
			{
				Minutos: m5(h * 0.30),
				Titulo:  "Revisão ativa dos erros",
				Detalhe: "releia só o resumo dos temas que você errou na bateria",
			},
			{
				Minutos: m5(h * 0.15),
				Titulo:  "Caderno de erros",
				Detalhe: "anote o porquê de cada erro, não só a resposta",
			},
		}
	}
}

// m5 rounds to the nearest 5 minutes.
func m5(x float64) int {
	return int(math.Round(x/5)) * 5
}

// detalheDoBloco writes what to do in one study block. A discipline studied by
// questions only never gets the "teoria com resumo" instruction, and one studied
// by theory only never gets a question count.
func detalheDoBloco(modo Modo, revisaoDirigida bool, questoes int) string {
	q := strconv.Itoa(questoes)

	if revisaoDirigida {
		if modo == ModoTeoria {
			return "reconstrua o assunto de memória, sem consultar, e confira o resumo depois"
		}

		return "reconstrua o assunto de memória, sem consultar, e resolva " + q + " questões só dele"
	}

	switch modo {
	case ModoQuestoes:
		return q + " questões do tema, corrigidas uma a uma; a teoria vem da correção"
	case ModoTeoria:
		return "teoria com resumo de própria autoria, sem bateria de questões"
	default:
		return "teoria com resumo de própria autoria e " + q + " questões do tema, corrigidas uma a uma"
	}
}

// revisaoDetalhe says what is actually due today. Retrieval beats re-reading, so
// by default it asks for questions on each topic before any consulting.
func revisaoDetalhe(cfg Config, vencendo []Revisao) string {
	if cfg.RevisaoPorQuestoes {
		por := cfg.QuestoesPorRevisao

		return "resolva " + strconv.Itoa(por) + " questões de cada tema ao lado sem consultar antes, " +
			"e só depois confira o resumo no que errar (" +
			strconv.Itoa(por*len(vencendo)) + " questões no total)"
	}

	return "reconstrua de memória cada tema ao lado e confira o resumo depois"
}

// ordinais names the study blocks of a day; past the sixth the number is used.
var ordinais = []string{"1º bloco", "2º bloco", "3º bloco", "4º bloco", "5º bloco", "6º bloco"}

func rotuloBloco(idx int) string {
	if idx < len(ordinais) {
		return ordinais[idx]
	}

	return strconv.Itoa(idx+1) + "º bloco"
}

// repartirMinutos splits the content time across the day's blocks in proportion
// to each discipline's reforço, rounding to 5 minutes and giving the leftover to
// the heaviest block so the day still adds up.
func repartirMinutos(total float64, itens []ItemDia, cfg Config) []int {
	pesos := make([]float64, len(itens))
	soma := 0.0

	for i, it := range itens {
		pesos[i] = cfg.ReforcoDe(it.Disciplina)
		soma += pesos[i]
	}

	if soma == 0 {
		soma = float64(len(itens))
		for i := range pesos {
			pesos[i] = 1
		}
	}

	out := make([]int, len(itens))
	usado := 0

	for i := range itens {
		out[i] = m5(total * pesos[i] / soma)
		usado += out[i]
	}

	if sobra := m5(total) - usado; sobra != 0 {
		maior := 0
		for i := range pesos {
			if pesos[i] > pesos[maior] {
				maior = i
			}
		}

		if out[maior]+sobra > 0 {
			out[maior] += sobra
		}
	}

	return out
}
