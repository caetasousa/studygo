package plano

// The review queue: everything already studied, revisited in order.
//
// Review is not tied to the day's own subjects, and it does not wait for a
// fixed interval. It walks what has been covered so far, oldest first, one
// subject per day. On day 1 you study Português and Matemática; on day 2 the
// review block already comes back to Português, on day 3 to Matemática, and so
// on — the queue keeps circling, so a topic seen early is revisited many times
// before the exam while a topic seen late is still revisited at least once.
//
// The measure that follows from this is the one that matters: how many complete
// laps over everything studied fit before the reta final.

// ItemRevisao is one topic waiting in the queue.
type ItemRevisao struct {
	Disciplina string
	Tema       string
	// DiaEstudo is the plan-day number the topic was first studied on, which is
	// what orders the queue.
	DiaEstudo int
}

// FilaRevisao is what each study day should revisit.
//
// Returns, for every day of the base phase, the topics its review block covers.
// The key is the plan-day number (Dia.N).
//
// One discipline per day: a block that jumps between subjects is several
// shallow reviews instead of one real one.
func FilaRevisao(dias []Dia, porDia int) map[int][]ItemRevisao {
	if porDia <= 0 {
		return map[int][]ItemRevisao{}
	}

	out := map[int][]ItemRevisao{}

	// Everything studied so far, in the order it was studied.
	fila := []ItemRevisao{}
	// Where the next lap resumes.
	cursor := 0

	for _, d := range dias {
		// The reta final has its own guided review, so the queue stops there.
		if d.Fase == FaseReta {
			break
		}

		// Yesterday's study is available to review today: the block is planned
		// before the day happens, so it can only draw on what came before.
		if len(fila) > 0 {
			out[d.N] = proximaLevaRevisao(fila, &cursor, porDia)
		}

		for _, it := range d.Itens {
			if it.Disciplina == "" || it.Tema == "" {
				continue
			}

			fila = append(fila, ItemRevisao{
				Disciplina: it.Disciplina,
				Tema:       it.Tema,
				DiaEstudo:  d.N,
			})
		}
	}

	return out
}

// proximaLevaRevisao takes the next batch from the queue, wrapping around when
// it reaches the end — which is what makes it a cycle rather than a list that
// runs out.
//
// The batch stops early when the next topic belongs to another discipline, so a
// day reviews one subject at a time.
func proximaLevaRevisao(fila []ItemRevisao, cursor *int, porDia int) []ItemRevisao {
	if len(fila) == 0 {
		return nil
	}

	out := make([]ItemRevisao, 0, porDia)
	disciplina := ""

	for len(out) < porDia {
		if *cursor >= len(fila) {
			*cursor = 0
		}

		it := fila[*cursor]

		if disciplina == "" {
			disciplina = it.Disciplina
		} else if it.Disciplina != disciplina {
			break
		}

		out = append(out, it)
		*cursor++

		// A queue shorter than the batch must not repeat itself inside one day.
		if len(out) >= len(fila) {
			break
		}
	}

	return out
}

// RevisoesPorDisciplina counts, per discipline, how many review passes over its
// whole topic list the queue delivers before the reta final.
//
// A pass is the queue coming back to every topic of that subject once. It is
// what answers "how many times do I review Português", which the plan-wide lap
// count cannot: a subject studied early is revisited far more than one studied
// late, and the average hides that.
func RevisoesPorDisciplina(dias []Dia, porDia int) map[string]float64 {
	revisados := map[string]int{}
	temas := map[string]map[string]bool{}

	for n, itens := range FilaRevisao(dias, porDia) {
		_ = n

		for _, it := range itens {
			revisados[it.Disciplina]++
		}
	}

	// The denominator is the discipline's own topic count as the plan schedules
	// it, not the catalogue's: a subject the plan never finishes teaching has
	// fewer topics in the queue to review.
	for _, d := range dias {
		if d.Fase == FaseReta {
			break
		}

		for _, it := range d.Itens {
			if it.Disciplina == "" || it.Tema == "" {
				continue
			}

			if temas[it.Disciplina] == nil {
				temas[it.Disciplina] = map[string]bool{}
			}

			temas[it.Disciplina][it.Tema] = true
		}
	}

	out := make(map[string]float64, len(revisados))

	for cod, n := range revisados {
		t := len(temas[cod])
		if t == 0 {
			continue
		}

		out[cod] = float64(n) / float64(t)
	}

	return out
}

// VoltasRevisao is how many complete laps over everything studied the plan gets
// through before the reta final.
//
// This is the number a student actually wants: "I go over everything I have
// studied 3.4 times before the final stretch". Below 1 the plan does not even
// finish reviewing what it taught.
func VoltasRevisao(dias []Dia, porDia int) float64 {
	if porDia <= 0 {
		return 0
	}

	revisados := 0
	estudados := 0

	for _, d := range dias {
		if d.Fase == FaseReta {
			break
		}

		if estudados > 0 {
			revisados += minInt(porDia, estudados)
		}

		for _, it := range d.Itens {
			if it.Disciplina != "" && it.Tema != "" {
				estudados++
			}
		}
	}

	if estudados == 0 {
		return 0
	}

	return float64(revisados) / float64(estudados)
}
