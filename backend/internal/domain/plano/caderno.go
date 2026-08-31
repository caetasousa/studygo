package plano

import "sort"

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

// TemasDoDia picks what the day's review tail should drill.
//
// It draws from the notebooks of the disciplines the day itself studies, so the
// tail stays on the same ground as the blocks above it, taking the neediest
// topics first and spreading across the day's disciplines rather than emptying
// one notebook before touching the next.
func TemasDoDia(cadernos map[string][]ItemCaderno, disciplinas []string, limite int) []ItemCaderno {
	if limite <= 0 {
		return nil
	}

	// Deduplicate while keeping the day's own order: a discipline scheduled
	// twice must not get two turns at the notebook.
	vistas := make(map[string]bool, len(disciplinas))
	ordem := make([]string, 0, len(disciplinas))

	for _, d := range disciplinas {
		if d == "" || vistas[d] {
			continue
		}

		vistas[d] = true
		ordem = append(ordem, d)
	}

	out := make([]ItemCaderno, 0, limite)
	pos := map[string]int{}

	// Round-robin: one topic per discipline per pass, so a day covering two
	// subjects reviews both.
	for len(out) < limite {
		avancou := false

		for _, d := range ordem {
			if len(out) >= limite {
				break
			}

			i := pos[d]
			if i >= len(cadernos[d]) {
				continue
			}

			out = append(out, cadernos[d][i])
			pos[d] = i + 1
			avancou = true
		}

		if !avancou {
			break
		}
	}

	return out
}
