// Package plano is the plan engine: given a user's Config and the exam
// catalogue, it builds the day-by-day study schedule — weighting each discipline
// by its points, splitting days into two blocks, assigning weekly-review and
// reta-final phases. It is a direct port of the artifact's construir() and is
// pure: no time.Now, no I/O.
package plano

import (
	"time"

	"annygo/internal/domain/concurso"
)

// Tipo is the kind of a plan day.
type Tipo string

const (
	TipoEstudo          Tipo = "est"     // two content blocks
	TipoRevisaoDirigida Tipo = "revd"    // reta-final guided review
	TipoSimulado        Tipo = "sim"     // full mock exam
	TipoDiscursiva      Tipo = "disc"    // essay practice
	TipoVespera         Tipo = "vespera" // eve of the exam
	TipoRevisaoSemanal  Tipo = "rev"     // base-phase weekly review
)

// Fase is base (first pass over the syllabus) or reta (final stretch).
type Fase string

const (
	FaseBase Fase = "base"
	FaseReta Fase = "reta"
)

// ItemDia is one study block: a discipline and the specific topic for it.
type ItemDia struct {
	Disciplina string `json:"disciplina"` // discipline codigo
	Tema       string `json:"tema"`
	Passada    int    `json:"passada"` // 1 = first pass, 2 = second pass
}

// Dia is a single day of the plan.
type Dia struct {
	N      int       `json:"n"`
	Data   time.Time `json:"data"`
	Semana int       `json:"semana"`
	Fase   Fase      `json:"fase"`
	Tipo   Tipo      `json:"tipo"`
	Itens  []ItemDia `json:"itens"`
	Tema   string    `json:"tema"` // headline for non-content days
	Meta   int       `json:"meta"` // target number of questions
}

// Config is the user's plan settings.
type Config struct {
	Inicio        time.Time
	Prova         time.Time
	HorasDia      float64
	DiasEstudo    []int // weekdays 0=Sun..6=Sat
	DiaRevisao    int
	RetaFinalDias int
	Questoes      map[string]int // discipline codigo -> estimated questions
}

// Resultado is everything the engine produces: the days plus the intermediate
// weightings the balanceamento view needs.
type Resultado struct {
	Dias       []Dia
	Slots      map[string]int // content blocks per discipline
	SlotsReta  map[string]int // reta-final blocks per discipline
	Pontos     map[string]int // points per discipline (questoes * peso)
	SomaPontos int
}

// Gerar builds the plan. It never returns nil slices/maps.
func Gerar(cfg Config, c *concurso.Concurso) Resultado {
	codes := make([]string, 0, len(c.Disciplinas))
	temas := map[string][]string{}
	pesos := map[string]int{}

	for _, d := range c.Disciplinas {
		codes = append(codes, d.Codigo)
		pesos[d.Codigo] = d.Peso

		// A discipline with no registered topics still gets study days — the
		// day just headlines the discipline name instead of a specific topic.
		if len(d.Temas) == 0 {
			temas[d.Codigo] = []string{d.Nome}
		} else {
			temas[d.Codigo] = d.Temas
		}
	}

	pontos := map[string]int{}
	soma := 0

	for _, k := range codes {
		pontos[k] = cfg.Questoes[k] * pesos[k]
		soma += pontos[k]
	}

	res := Resultado{
		Dias:       []Dia{},
		Slots:      map[string]int{},
		SlotsReta:  map[string]int{},
		Pontos:     pontos,
		SomaPontos: soma,
	}

	dias := construir(cfg, c, codes, temas, pontos, soma, &res)
	res.Dias = dias

	return res
}
