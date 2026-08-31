package plano

import (
	"math"
	"sort"
)

// Faixa below which a battery counts as a miss and puts its topic in the
// notebook. It was part of the spaced-review machinery; the notebook is what
// still needs it.
const FaixaFraca = 60

// Fraca reports whether a result is bad enough to become a notebook entry.
func Fraca(questoes, acertos int) bool {
	return questoes > 0 && Aproveitamento(questoes, acertos) < FaixaFraca
}

// Aproveitamento is the hit rate as a whole percentage; 0 questions means 0.
func Aproveitamento(questoes, acertos int) int {
	if questoes <= 0 {
		return 0
	}

	return int(math.Round(float64(acertos) / float64(questoes) * 100))
}

// The error notebook, as a study method.
//
// The premise: what you got wrong is what you have to come back to. Every weak
// result (see Fraca) puts its topic in that discipline's notebook, and the
// notebook only grows — by the end of the cycle it holds the part of the edital
// that actually resisted you, which is what the daily review tail drills.
//
// This is deliberately per DISCIPLINE. A single notebook mixing every subject
// would drill Portuguese and accounting in the same breath; one per discipline
// keeps a session coherent and lets the tail follow the day's own subjects.

// ItemCaderno is one topic a discipline's notebook holds.
type ItemCaderno struct {
	Disciplina string
	Tema       string
	// Erros counts the weak results recorded for this topic. A topic missed
	// repeatedly outranks one missed once, which is the order the tail drills in.
	Erros int
	// Questoes/Acertos accumulate across every attempt, so the notebook can show
	// how the topic is actually trending rather than only its last result.
	Questoes int
	Acertos  int
	// UltimaData is the most recent day this topic was answered on. Ties on
	// Erros break by staleness: the longer since you touched it, the sooner it
	// comes back.
	UltimaData string
}

// Aproveitamento is the topic's hit rate across everything recorded for it.
func (i ItemCaderno) Aproveitamento() int {
	return Aproveitamento(i.Questoes, i.Acertos)
}

// ResultadoTema is one recorded attempt at a topic, which is all the notebook
// needs to be built. It deliberately does not depend on how the attempt was
// stored (day record, review, imported battery).
type ResultadoTema struct {
	Disciplina string
	Tema       string
	Data       string // ISO, for ordering only
	Questoes   int
	Acertos    int
}

// Caderno groups results into one notebook per discipline.
//
// A topic enters on its first weak result and never leaves: later good results
// improve its hit rate and push it down the order, but the notebook is a record
// of what was hard, not a queue that empties.
func Caderno(resultados []ResultadoTema) map[string][]ItemCaderno {
	type chave struct{ disc, tema string }

	acc := map[chave]*ItemCaderno{}
	entrou := map[chave]bool{}

	for _, r := range resultados {
		if r.Disciplina == "" || r.Tema == "" || r.Questoes <= 0 {
			continue
		}

		k := chave{r.Disciplina, r.Tema}

		it, ok := acc[k]
		if !ok {
			it = &ItemCaderno{Disciplina: r.Disciplina, Tema: r.Tema}
			acc[k] = it
		}

		it.Questoes += r.Questoes
		it.Acertos += r.Acertos

		if r.Data > it.UltimaData {
			it.UltimaData = r.Data
		}

		if Fraca(r.Questoes, r.Acertos) {
			it.Erros++
			entrou[k] = true
		}
	}

	out := map[string][]ItemCaderno{}

	for k, it := range acc {
		// Only topics that actually went wrong at some point belong here.
		if !entrou[k] {
			continue
		}

		out[k.disc] = append(out[k.disc], *it)
	}

	for disc := range out {
		ordenarCaderno(out[disc])
	}

	return out
}

// ordenarCaderno puts what needs the most work first: most missed, then worst
// hit rate, then longest untouched. The name breaks the final tie so the order
// is stable across runs.
func ordenarCaderno(itens []ItemCaderno) {
	sort.SliceStable(itens, func(i, j int) bool {
		a, b := itens[i], itens[j]

		if a.Erros != b.Erros {
			return a.Erros > b.Erros
		}

		if ap, bp := a.Aproveitamento(), b.Aproveitamento(); ap != bp {
			return ap < bp
		}

		if a.UltimaData != b.UltimaData {
			return a.UltimaData < b.UltimaData
		}

		return a.Tema < b.Tema
	})
}

// cadernoGeral flattens every discipline's notebook into one list, neediest
// first.
//
// The per-discipline notebooks are what a study session drills, because a
// session follows the day's own subjects. A whole day given over to
// reinforcement is the other case: there, how badly a topic went matters more
// than which subject it belongs to.
func cadernoGeral(cadernos map[string][]ItemCaderno) []ItemCaderno {
	out := []ItemCaderno{}
	for _, itens := range cadernos {
		out = append(out, itens...)
	}

	// Map iteration is unordered, so settle the order before the stable sort:
	// otherwise two equally needy topics swap places from one run to the next.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Disciplina != out[j].Disciplina {
			return out[i].Disciplina < out[j].Disciplina
		}

		return out[i].Tema < out[j].Tema
	})

	ordenarCaderno(out)

	return out
}

// TemasDoDia picks what the day's review block should drill.
//
// ONE discipline per day, not a mix: a block that jumps between subjects is
// three shallow reviews instead of one real one. Which discipline rotates with
// the day index, so across a week every subject of the plan gets its turn
// rather than the first one always being drilled.
//
// Within that discipline it takes the neediest topics first, capped by limite.
func TemasDoDia(
	cadernos map[string][]ItemCaderno,
	disciplinas []string,
	giro int,
	limite int,
) []ItemCaderno {
	if limite <= 0 {
		return nil
	}

	// Deduplicate while keeping the day's own order: a discipline scheduled
	// twice must not get two turns.
	vistas := make(map[string]bool, len(disciplinas))
	ordem := make([]string, 0, len(disciplinas))

	for _, d := range disciplinas {
		if d == "" || vistas[d] {
			continue
		}

		vistas[d] = true
		ordem = append(ordem, d)
	}

	// Only the disciplines that actually have something to review.
	comCaderno := make([]string, 0, len(ordem))

	for _, d := range ordem {
		if len(cadernos[d]) > 0 {
			comCaderno = append(comCaderno, d)
		}
	}

	if len(comCaderno) == 0 {
		return nil
	}

	if giro < 0 {
		giro = 0
	}

	escolhida := comCaderno[giro%len(comCaderno)]

	itens := cadernos[escolhida]
	if len(itens) > limite {
		itens = itens[:limite]
	}

	return append([]ItemCaderno(nil), itens...)
}
