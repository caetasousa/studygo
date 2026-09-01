package plano

import "time"

// Registro is a user's log for one day of the plan. Pointer fields are nil when
// never filled in; a recorded zero counts as "not filled" (matching the
// artifact's truthiness checks).
//
// Blocos is the per-discipline breakdown of the same day. When it is present it
// is the source of truth and the day-level totals are derived from it; when it
// is empty the day-level numbers are split evenly across the day's blocks, as
// the artifact did.
type Registro struct {
	Data      time.Time
	Horas     *float64
	Concluido bool
	Questoes  *int
	Acertos   *int
	Nota      string
	Blocos    []RegistroBloco
}

// RegistroBloco is the user's log for one discipline inside a day. Unlike the
// day-level fields, a recorded zero here is real data — "resolvi 10 e acertei 0"
// has to be distinguishable from "não lancei nada".
type RegistroBloco struct {
	Disciplina string
	Horas      *float64
	Questoes   *int
	Acertos    *int
	Nota       string
	// Concluido marks this discipline done on its own, independently of the
	// day's own flag: a day can be half finished.
	Concluido bool
	// AtividadeID ties the record to one scheduled activity, which is the real
	// unit of work: a day may schedule the same discipline twice, and each
	// occurrence records and completes on its own. Empty on records made before
	// activities were materialised, which still key on (data, disciplina).
	AtividadeID string
}

// RegistroRevisao is the user's log for one day's review tail (see
// caudaRevisao) — the spaced-review block that closes a content day from its
// second day onward. It is not an Atividade: the review tail is a rolling
// queue the engine derives fresh every time (FilaRevisao), never a stored,
// individually addressable unit like a subject block — so it is keyed by
// date, the one thing that is stable about it.
type RegistroRevisao struct {
	Data     time.Time
	Questoes *int
	Acertos  *int
}

// BlocoDeAtividade returns the record for one scheduled activity, or nil.
func (r Registro) BlocoDeAtividade(id string) *RegistroBloco {
	if id == "" {
		return nil
	}

	for i := range r.Blocos {
		if r.Blocos[i].AtividadeID == id {
			return &r.Blocos[i]
		}
	}

	return nil
}

// BlocoDe returns the record for one discipline of the day, or nil.
func (r Registro) BlocoDe(codigo string) *RegistroBloco {
	for i := range r.Blocos {
		if r.Blocos[i].Disciplina == codigo {
			return &r.Blocos[i]
		}
	}

	return nil
}

// Totais collapses the per-discipline blocks into day-level sums. It only
// reports a value when at least one block recorded it, so an untouched field
// stays nil instead of becoming a zero.
func (r Registro) Totais() (horas *float64, questoes, acertos *int) {
	for _, b := range r.Blocos {
		if b.Horas != nil {
			h := nzFloat(horas) + *b.Horas
			horas = &h
		}

		if b.Questoes != nil {
			q := nzInt(questoes) + *b.Questoes
			questoes = &q
		}

		if b.Acertos != nil {
			a := nzInt(acertos) + *b.Acertos
			acertos = &a
		}
	}

	return horas, questoes, acertos
}

// StatDisciplina holds the fractional totals attributed to one discipline.
// Each content day splits its record evenly across its blocks.
type StatDisciplina struct {
	Horas      float64
	Concluido  float64
	Questoes   float64
	Acertos    float64
	Blocos     int
	BlocosReta int
}

// StatExtras aggregates the special (non-content) days.
type StatExtras struct {
	Dias      int
	Horas     float64
	Concluido int
	Questoes  int
	Acertos   int
}

// Stats is the full aggregate over a plan and its records.
type Stats struct {
	Disciplina    map[string]StatDisciplina
	Extras        StatExtras
	HorasTotal    float64
	Feitos        int
	QuestoesTotal int
	AcertosTotal  int
}

// CalcularStats is the port of the artifact's stats(). registros is keyed by the
// day's date (normalized to midnight UTC).
func CalcularStats(dias []Dia, codes []string, registros map[time.Time]Registro) Stats {
	st := Stats{
		Disciplina: map[string]StatDisciplina{},
	}

	for _, k := range codes {
		st.Disciplina[k] = StatDisciplina{}
	}

	for _, d := range dias {
		r, temReg := registros[day(d.Data)]

		if len(d.Itens) == 0 {
			st.Extras.Dias++

			if temReg {
				if h := nzFloat(r.Horas); h != 0 {
					st.Extras.Horas += h
					st.HorasTotal += h
				}

				if r.Concluido {
					st.Extras.Concluido++
					st.Feitos++
				}

				if q := nzInt(r.Questoes); q != 0 {
					st.Extras.Questoes += q
					st.QuestoesTotal += q
				}

				if a := nzInt(r.Acertos); a != 0 {
					st.Extras.Acertos += a
					st.AcertosTotal += a
				}
			}

			continue
		}

		frac := 1.0 / float64(len(d.Itens))
		porBloco := temReg && len(r.Blocos) > 0

		for _, it := range d.Itens {
			alvo := st.Disciplina[it.Disciplina]
			alvo.Blocos++

			if d.Fase == FaseReta {
				alvo.BlocosReta++
			}

			if temReg {
				if r.Concluido {
					alvo.Concluido += frac
				}

				// Sem lançamento por disciplina, o dia é rateado entre seus blocos,
				// como no artefato.
				if !porBloco {
					if h := nzFloat(r.Horas); h != 0 {
						alvo.Horas += h * frac
					}

					if q := nzInt(r.Questoes); q != 0 {
						alvo.Questoes += float64(q) * frac
					}

					if a := nzInt(r.Acertos); a != 0 {
						alvo.Acertos += float64(a) * frac
					}
				}
			}

			st.Disciplina[it.Disciplina] = alvo
		}

		// Lançado por disciplina: cada bloco conta uma vez, para a sua disciplina —
		// independente de quantos slots do dia ela ocupa, e mesmo que ocupe nenhum.
		// Um zero aqui é dado real, então não passa pelo filtro de "não preenchido".
		if porBloco {
			for _, b := range r.Blocos {
				alvo, ok := st.Disciplina[b.Disciplina]
				if !ok {
					continue
				}

				alvo.Horas += nzFloat(b.Horas)
				alvo.Questoes += float64(nzInt(b.Questoes))
				alvo.Acertos += float64(nzInt(b.Acertos))
				st.Disciplina[b.Disciplina] = alvo
			}
		}

		if !temReg {
			continue
		}

		horas, questoes, acertos := r.Horas, r.Questoes, r.Acertos
		if porBloco {
			horas, questoes, acertos = r.Totais()
		}

		st.HorasTotal += nzFloat(horas)
		st.QuestoesTotal += nzInt(questoes)
		st.AcertosTotal += nzInt(acertos)

		if r.Concluido {
			st.Feitos++
		}
	}

	return st
}

func nzFloat(p *float64) float64 {
	if p == nil {
		return 0
	}

	return *p
}

func nzInt(p *int) int {
	if p == nil {
		return 0
	}

	return *p
}
