package plano

import "time"

// Registro is a user's log for one day of the plan. Pointer fields are nil when
// never filled in; a recorded zero counts as "not filled" (matching the
// artifact's truthiness checks).
type Registro struct {
	Data      time.Time
	Horas     *float64
	Concluido bool
	Questoes  *int
	Acertos   *int
	Nota      string
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

		for _, it := range d.Itens {
			alvo := st.Disciplina[it.Disciplina]
			alvo.Blocos++

			if d.Fase == FaseReta {
				alvo.BlocosReta++
			}

			if temReg {
				if h := nzFloat(r.Horas); h != 0 {
					alvo.Horas += h * frac
				}

				if q := nzInt(r.Questoes); q != 0 {
					alvo.Questoes += float64(q) * frac
				}

				if a := nzInt(r.Acertos); a != 0 {
					alvo.Acertos += float64(a) * frac
				}

				if r.Concluido {
					alvo.Concluido += frac
				}
			}

			st.Disciplina[it.Disciplina] = alvo
		}

		if temReg {
			if h := nzFloat(r.Horas); h != 0 {
				st.HorasTotal += h
			}

			if r.Concluido {
				st.Feitos++
			}

			if q := nzInt(r.Questoes); q != 0 {
				st.QuestoesTotal += q
			}

			if a := nzInt(r.Acertos); a != 0 {
				st.AcertosTotal += a
			}
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
