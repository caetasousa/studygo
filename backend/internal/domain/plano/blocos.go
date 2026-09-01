package plano

import (
	"math"
	"strconv"
	"strings"
)

// Bloco is one time slice of a day's routine: minutes, a label, and what to do.
type Bloco struct {
	Minutos int    `json:"minutos"`
	Titulo  string `json:"titulo"`
	Detalhe string `json:"detalhe"`
	// Disciplina is set only on the review tail (see caudaRevisao), once the
	// queue actually has something to name — the codigo, not the display
	// name, so a caller can key a record or a caderno link on it without
	// reverse-engineering it back out of Titulo.
	Disciplina string `json:"-"`
}

// BlocoCtx carries everything the breakdown needs beyond the day itself.
type BlocoCtx struct {
	Cfg      Config            // the user's plan settings (dates, rhythm, method)
	Nomes    map[string]string // discipline codigo -> display name
	Simulado Composicao        // question split of a full mock exam
	// Cadernos is the error notebook per discipline codigo. What went wrong
	// outranks the queue's normal order: a topic in the notebook comes back
	// first.
	Cadernos map[string][]ItemCaderno
	// Revisao is what each day's review block should cover, keyed by plan-day
	// number — the rolling queue over everything studied so far (see
	// FilaRevisao).
	Revisao map[int][]ItemRevisao
}

// Composicao is how many questions of each bloco a full exam has.
type Composicao struct {
	Gerais      int
	Especificas int
}

// MesclarItensIguais collapses a run of consecutive items on a day that name the
// same discipline AND the same topic into a single item.
//
// This is what a discipline with few topics but many daily blocks produces: the
// engine's reparte fills the leftover slots with repeats of the same topic, so
// a day ends up scheduling "Fundamentos da IA" six times in a row, and the day's
// fixed time budget is then split six ways into 10-minute slivers. Merged, the
// day shows one "Fundamentos da IA" block whose minutes are the sum — which is
// what Blocos computes anyway, since it splits by item.
//
// The first item's fields win, so its AtividadeID survives and the block stays
// addressable. Only an exact (discipline, topic) match merges: two different
// topics of one discipline, or a genuine second pass with a different label,
// stay separate.
func MesclarItensIguais(itens []ItemDia) []ItemDia {
	if len(itens) < 2 {
		return itens
	}

	out := make([]ItemDia, 0, len(itens))

	for _, it := range itens {
		if n := len(out); n > 0 &&
			out[n-1].Disciplina == it.Disciplina &&
			out[n-1].Tema == it.Tema {
			continue
		}

		out = append(out, it)
	}

	return out
}

// Blocos returns the timed breakdown for a day, mirroring the artifact's
// blocos(): content days get two study blocks plus a spaced-review tail; the
// special days get their own fixed splits.
func Blocos(d Dia, ctx BlocoCtx) []Bloco {
	cfg := ctx.Cfg.Normalizar()
	h := cfg.HorasDia * 60

	if len(d.Itens) > 0 {
		rev := d.Tipo == TipoRevisaoDirigida

		// Content blocks have a length of their own; the review block sits beside
		// them with its own. Nothing is a percentage of anything else, so moving
		// one does not silently resize the other.
		revMin := cfg.MinutosRevisao
		conteudoMin := h - float64(revMin)

		if conteudoMin < 0 {
			conteudoMin = 0
		}

		minutos := repartirMinutos(conteudoMin, d.Itens, cfg)
		out := make([]Bloco, 0, len(d.Itens)+1)

		for idx, it := range d.Itens {
			// As questões do dia acompanham os minutos do bloco: blocos de tamanho
			// igual, baterias de tamanho igual.
			porBloco := 0
			if conteudoMin > 0 {
				porBloco = int(math.Round(float64(d.Meta) * float64(minutos[idx]) / conteudoMin))
			}

			out = append(out, Bloco{
				Minutos: minutos[idx],
				Titulo:  rotuloBloco(idx) + " — " + ctx.Nomes[it.Disciplina],
				Detalhe: detalheDoBloco(cfg.ModoDe(it.Disciplina), rev, porBloco),
			})
		}

		if revMin <= 0 {
			return out
		}

		out = append(out, caudaRevisao(d, ctx, revMin))

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

// caudaRevisao is the review block that closes a study day.
//
// It drills the ERROR NOTEBOOK of the day's own disciplines: the topics that
// went badly, accumulated, which is the method's whole point — by the end of
// the cycle the notebook holds the part of the edital that resisted you.
//
// Before anything has gone wrong the notebook is empty, and the block says so
// rather than inventing work: the time is there to go back over the day's own
// subjects.
func caudaRevisao(d Dia, ctx BlocoCtx, minutos int) Bloco {
	itens := ctx.Revisao[d.N]
	if len(itens) == 0 {
		return Bloco{
			Minutos: minutos,
			Titulo:  "Revisão",
			Detalhe: "ainda não há matéria estudada para revisar — siga o conteúdo de hoje",
		}
	}

	// A topic already answered badly outranks the queue's own order: the
	// notebook is the reason to come back sooner.
	nome := ctx.Nomes[itens[0].Disciplina]
	if nome == "" {
		nome = itens[0].Disciplina
	}

	return Bloco{
		Minutos:    minutos,
		Titulo:     "Revisão — " + nome,
		Detalhe:    revisaoDetalhe(itens, ctx.Cadernos),
		Disciplina: itens[0].Disciplina,
	}
}

// revisaoDetalhe names the topics coming back today, marking the ones the
// student has already got wrong so the time goes to them first.
func revisaoDetalhe(itens []ItemRevisao, cadernos map[string][]ItemCaderno) string {
	var b strings.Builder

	b.WriteString("volte a estes assuntos, sem consultar antes: ")

	for i, it := range itens {
		if i > 0 {
			b.WriteString("  ·  ")
		}

		b.WriteString(it.Tema)

		if pct, errou := aproveitamentoNoCaderno(cadernos, it); errou {
			b.WriteString(" (você foi a ")
			b.WriteString(strconv.Itoa(pct))
			b.WriteString("% aqui)")
		}
	}

	return b.String()
}

// aproveitamentoNoCaderno reports the topic's hit rate when it is in the error
// notebook, so the block can say which ones actually went wrong.
func aproveitamentoNoCaderno(
	cadernos map[string][]ItemCaderno,
	it ItemRevisao,
) (int, bool) {
	for _, c := range cadernos[it.Disciplina] {
		if c.Tema == it.Tema {
			return c.Aproveitamento(), true
		}
	}

	return 0, false
}

// maxTemasRevisao caps how many topics one review block names. Past a handful
// the block stops being a plan and becomes a list nobody works through.
const maxTemasRevisao = 4

// ordinais names the study blocks of a day; past the sixth the number is used.
var ordinais = []string{"1º bloco", "2º bloco", "3º bloco", "4º bloco", "5º bloco", "6º bloco"}

func rotuloBloco(idx int) string {
	if idx < len(ordinais) {
		return ordinais[idx]
	}

	return strconv.Itoa(idx+1) + "º bloco"
}

// repartirMinutos splits the content time equally across the day's blocks,
// rounding to 5 minutes and giving the leftover to the first block so the day
// still adds up.
//
// Equal, not weighted by reforço: the config screen calls MinutosBloco "the size
// of each activity", so a day of N blocks and a budget of N×MinutosBloco shows
// MinutosBloco on every one. Reforço still makes a heavier discipline appear on
// MORE days (see pesosDistribuicao) — it just no longer stretches a single
// block, which was the surprise of setting "30 min" and seeing 35.
func repartirMinutos(total float64, itens []ItemDia, _ Config) []int {
	n := len(itens)
	if n == 0 {
		return []int{}
	}

	out := make([]int, n)
	usado := 0

	for i := range itens {
		out[i] = m5(total / float64(n))
		usado += out[i]
	}

	if sobra := m5(total) - usado; sobra != 0 {
		maior := 0
		for i := range out {
			if out[i] > out[maior] {
				maior = i
			}
		}

		if out[maior]+sobra > 0 {
			out[maior] += sobra
		}
	}

	return out
}
