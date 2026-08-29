package plano

import (
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Revisao is one scheduled review of a topic. A topic studied on day D enters
// the queue at Etapa 0 and climbs one stage each time it is recalled well,
// spacing out as it goes; a bad result pushes it back down so it returns sooner.
type Revisao struct {
	ID         uuid.UUID
	Disciplina string
	Tema       string
	OrigemData time.Time // the day the topic was studied
	Etapa      int       // index into Perfil.Intervalos
	VenceEm    time.Time
	FeitaEm    *time.Time
	Questoes   *int
	Acertos    *int
}

// Faixas of a review's hit rate, which decide where the topic goes next.
const (
	FaixaBoa   = 80 // climbs to the next interval
	FaixaFraca = 60 // below this the topic drops a stage and enters the notebook
)

// Enfileirar returns the reviews a finished day creates — one per topic studied.
// A day with no topics (simulado, véspera) creates none.
func Enfileirar(cfg Config, d Dia) []Revisao {
	perfil := cfg.Perfil.Normalizar()
	out := make([]Revisao, 0, len(d.Itens))

	for _, it := range d.Itens {
		vence, ok := agendar(cfg, DayOf(d.Data), perfil.Intervalos[0])
		if !ok {
			continue
		}

		out = append(out, Revisao{
			Disciplina: it.Disciplina,
			Tema:       it.Tema,
			OrigemData: DayOf(d.Data),
			Etapa:      0,
			VenceEm:    vence,
		})
	}

	return out
}

// Resultado records how a review went and returns the next one. The second
// value is false when the topic leaves the queue for good — either consolidated
// (it climbed past the last interval) or because the next date would fall after
// the exam.
func (r Revisao) Resultado(cfg Config, hoje time.Time, questoes, acertos int) (Revisao, bool) {
	perfil := cfg.Perfil.Normalizar()

	proxima := Revisao{
		Disciplina: r.Disciplina,
		Tema:       r.Tema,
		OrigemData: r.OrigemData,
		Etapa:      proximaEtapa(r.Etapa, Aproveitamento(questoes, acertos)),
	}

	if proxima.Etapa >= len(perfil.Intervalos) {
		return Revisao{}, false
	}

	vence, ok := agendar(cfg, DayOf(hoje), perfil.Intervalos[proxima.Etapa])
	if !ok {
		return Revisao{}, false
	}

	proxima.VenceEm = vence

	return proxima, true
}

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

// proximaEtapa applies the three Leitner bands.
func proximaEtapa(etapa, pct int) int {
	switch {
	case pct >= FaixaBoa:
		return etapa + 1
	case pct >= FaixaFraca:
		return etapa
	case etapa > 0:
		return etapa - 1
	default:
		return 0
	}
}

// agendar places a review `dias` calendar days after base, pushed forward to the
// next study day. It returns false when that lands on or after the exam — there
// is no point queueing a review the user will never reach.
func agendar(cfg Config, base time.Time, dias int) (time.Time, bool) {
	alvo := proximoDiaEstudo(AddDays(base, dias), cfg.DiasEstudo)

	if !alvo.Before(DayOf(cfg.Prova)) {
		return time.Time{}, false
	}

	return alvo, true
}

// proximoDiaEstudo advances to the next weekday the user actually studies. With
// no study days configured the date is returned untouched.
func proximoDiaEstudo(d time.Time, diasEstudo []int) time.Time {
	if len(diasEstudo) == 0 {
		return d
	}

	for i := 0; i < 7; i++ {
		if contains(diasEstudo, weekday(d)) {
			return d
		}

		d = AddDays(d, 1)
	}

	return d
}

// VencidasAte returns the reviews due on or before `data`, oldest first, so an
// overdue topic is always offered before a fresh one.
func VencidasAte(fila []Revisao, data time.Time) []Revisao {
	out := make([]Revisao, 0, len(fila))

	for _, r := range fila {
		if r.FeitaEm == nil && !r.VenceEm.After(DayOf(data)) {
			out = append(out, r)
		}
	}

	sortRevisoes(out)

	return out
}

func sortRevisoes(rs []Revisao) {
	sort.SliceStable(rs, func(i, j int) bool {
		if !rs[i].VenceEm.Equal(rs[j].VenceEm) {
			return rs[i].VenceEm.Before(rs[j].VenceEm)
		}

		return rs[i].Disciplina < rs[j].Disciplina
	})
}
