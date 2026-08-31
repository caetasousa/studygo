package plano

import (
	"strings"
	"time"

	"studygo/internal/domain/concurso"
)

// Config is the user's plan settings: the dates and rhythm plus the study method
// itself. The artifact kept the method fixed; here every knob — how many blocks a
// day, how long each one lasts, whether simulados matter, the spaced-review
// intervals, the per-discipline weight — lives on the one struct the engine
// consumes.
//
// The zero value is not usable — Normalizar fills every method field with a
// sensible default, and Gerar calls it. A Simulados of "" is what marks the
// method half of the struct as never filled in.
type Config struct {
	// Dates and rhythm.
	Inicio        time.Time
	Prova         time.Time
	HorasDia      float64 // daily budget the engine splits; derived from MinutosBloco when that is set
	DiasEstudo    []int   // weekdays 0=Sun..6=Sat
	DiaRevisao    int
	RetaFinalDias int
	Questoes      map[string]int // discipline codigo -> estimated questions

	// Study method (was the separate Perfil).
	BlocosPorDia       int                // how many disciplines a study day covers
	MinutosBloco       int                // length of one normal block; 0 = derive from HorasDia
	PctRevisao         float64            // slice of the day the spaced-review tail takes
	Reforcos           map[string]float64 // extra weight per discipline codigo (1 = normal)
	RevisaoPorQuestoes bool               // spaced review asks for questions before consulting
	QuestoesPorRevisao int                // questions per topic on a spaced-review block
	Intervalos         []int              // spaced-review spacing in days: 1 / 7 / 30 by default
	CicloRevisao       []concurso.RevItem // weekly-review rotation; empty = concurso's own, or RevCicloPadrao
	Simulados          Frequencia         // how often a full mock exam shows up in the reta final
	Discursiva         bool               // reserve an essay day in the reta final
	Modos              map[string]Modo    // how each discipline is studied
	PctQuestoes        float64            // slice of a study block spent on questions
	LimiarFraco        int                // % below which a battery counts as weak
}

// Frequencia is how often a full mock exam shows up in the reta final.
type Frequencia string

const (
	SimuladoNunca     Frequencia = "nunca"
	SimuladoQuinzenal Frequencia = "quinzenal"
	SimuladoSemanal   Frequencia = "semanal"
)

// Modo is how one discipline is studied.
type Modo string

const (
	ModoCompleto Modo = "completo" // theory with a summary, then questions
	ModoQuestoes Modo = "questoes" // questions only
	ModoTeoria   Modo = "teoria"   // theory only
)

// Limites of the tunable fields, shared with the service's validation.
const (
	BlocosMin       = 1
	BlocosMax       = 6
	ReforcoMin      = 0.25
	ReforcoMax      = 3
	MinutosBlocoMin = 15
	MinutosBlocoMax = 240
)

// ConfigPadrao returns the study-method defaults — the artifact's behaviour
// exactly: a full mock exam on the last day of every reta-final week, an essay
// the day before, the 24h / 7d / 30d review cycle, two blocks a day. The dates
// and question counts are left zero for the caller to fill in.
func ConfigPadrao() Config {
	return Config{
		BlocosPorDia:       2,
		PctRevisao:         0.16,
		Reforcos:           map[string]float64{},
		RevisaoPorQuestoes: true,
		QuestoesPorRevisao: 10,
		Intervalos:         []int{1, 7, 30},
		Simulados:          SimuladoSemanal,
		Discursiva:         true,
		Modos:              map[string]Modo{},
		PctQuestoes:        0.5,
		LimiarFraco:        70,
	}
}

// Normalizar clamps every method field to a usable value, so a config coming
// from the database or an API payload can never produce a broken plan. It is
// idempotent. The dates, study days and question map are validated by the
// service and left untouched here.
func (c Config) Normalizar() Config {
	d := ConfigPadrao()

	// Simulados == "" means the study method was never chosen: adopt the
	// defaults. The one exception is BlocosPorDia — a saved, in-range value is
	// the user's answer to "how many disciplines a day", and overwriting it here
	// is what silently reverted 3 blocks back to 2 on the next load, so the
	// schedule kept showing two disciplines a day.
	//
	// Booleans and percentages stay unconditional: their zero value is
	// indistinguishable from "never set", so preserving them would freeze an
	// unanswered question as an answer (Discursiva=false, for one).
	if c.Simulados == "" {
		modos, reforcos, ciclo := c.Modos, c.Reforcos, c.CicloRevisao
		blocos := c.BlocosPorDia

		c.BlocosPorDia = d.BlocosPorDia
		c.PctRevisao = d.PctRevisao
		c.RevisaoPorQuestoes = d.RevisaoPorQuestoes
		c.QuestoesPorRevisao = d.QuestoesPorRevisao
		c.Intervalos = d.Intervalos
		c.Simulados = d.Simulados
		c.Discursiva = d.Discursiva
		c.PctQuestoes = d.PctQuestoes
		c.LimiarFraco = d.LimiarFraco

		if blocos >= BlocosMin && blocos <= BlocosMax {
			c.BlocosPorDia = blocos
		}

		c.Modos = nonNilModos(modos)
		c.Reforcos = nonNilReforcos(reforcos)
		c.CicloRevisao = cicloValido(ciclo)
		c.MinutosBloco = minutosBlocoValido(c.MinutosBloco)
		c.HorasDia = horasDiaEfetiva(c)

		return c
	}

	switch c.Simulados {
	case SimuladoNunca, SimuladoQuinzenal, SimuladoSemanal:
	default:
		c.Simulados = d.Simulados
	}

	c.Intervalos = intervalosValidos(c.Intervalos)
	if len(c.Intervalos) == 0 {
		c.Intervalos = d.Intervalos
	}

	if c.PctQuestoes < 0.1 || c.PctQuestoes > 0.9 {
		c.PctQuestoes = d.PctQuestoes
	}

	if c.QuestoesPorRevisao < 1 || c.QuestoesPorRevisao > 200 {
		c.QuestoesPorRevisao = d.QuestoesPorRevisao
	}

	if c.LimiarFraco < 1 || c.LimiarFraco > 100 {
		c.LimiarFraco = d.LimiarFraco
	}

	if c.BlocosPorDia < BlocosMin || c.BlocosPorDia > BlocosMax {
		c.BlocosPorDia = d.BlocosPorDia
	}

	if c.PctRevisao < 0 || c.PctRevisao > 0.4 {
		c.PctRevisao = d.PctRevisao
	}

	c.Modos = nonNilModos(c.Modos)
	c.Reforcos = nonNilReforcos(c.Reforcos)
	c.CicloRevisao = cicloValido(c.CicloRevisao)
	c.MinutosBloco = minutosBlocoValido(c.MinutosBloco)
	c.HorasDia = horasDiaEfetiva(c)

	return c
}

// ModoDe returns how a discipline is studied, defaulting to ModoCompleto.
func (c Config) ModoDe(codigo string) Modo {
	switch c.Modos[codigo] {
	case ModoQuestoes:
		return ModoQuestoes
	case ModoTeoria:
		return ModoTeoria
	default:
		return ModoCompleto
	}
}

// ReforcoDe is a discipline's extra weight, defaulting to 1 and clamped to a
// range where the plan still makes sense. 2 makes it show up twice as often and
// take twice the minutes when it does.
func (c Config) ReforcoDe(codigo string) float64 {
	r, ok := c.Reforcos[codigo]
	if !ok || r == 0 {
		return 1
	}

	return min(max(r, ReforcoMin), ReforcoMax)
}

// horasDiaEfetiva keeps HorasDia in step with MinutosBloco: when the user set a
// block length, that plus BlocosPorDia and the review tail is what a day lasts.
// MinutosBloco == 0 means "no explicit length" and HorasDia is used as given.
func horasDiaEfetiva(c Config) float64 {
	if c.MinutosBloco <= 0 || c.BlocosPorDia <= 0 {
		return c.HorasDia
	}

	conteudo := float64(c.BlocosPorDia * c.MinutosBloco)

	total := conteudo
	if c.PctRevisao > 0 && c.PctRevisao < 1 {
		total = conteudo / (1 - c.PctRevisao)
	}

	return total / 60
}

func minutosBlocoValido(m int) int {
	switch {
	case m <= 0:
		return 0
	case m < MinutosBlocoMin:
		return MinutosBlocoMin
	case m > MinutosBlocoMax:
		return MinutosBlocoMax
	default:
		return m
	}
}

func nonNilModos(m map[string]Modo) map[string]Modo {
	if m == nil {
		return map[string]Modo{}
	}

	return m
}

func nonNilReforcos(m map[string]float64) map[string]float64 {
	if m == nil {
		return map[string]float64{}
	}

	return m
}

// cicloValido drops entries with no title, so a half-filled form cannot leave
// the weekly review with a blank headline.
func cicloValido(itens []concurso.RevItem) []concurso.RevItem {
	out := make([]concurso.RevItem, 0, len(itens))

	for _, it := range itens {
		if strings.TrimSpace(it.Titulo) == "" {
			continue
		}

		out = append(out, concurso.RevItem{
			Ordem:    len(out),
			Titulo:   strings.TrimSpace(it.Titulo),
			Questoes: max(0, it.Questoes),
		})
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// intervalosValidos keeps the positive, strictly increasing entries — a review
// cycle that goes backwards would re-queue a topic before its previous stage.
func intervalosValidos(xs []int) []int {
	out := make([]int, 0, len(xs))
	ultimo := 0

	for _, x := range xs {
		if x <= ultimo || x > 3650 {
			continue
		}

		out = append(out, x)
		ultimo = x
	}

	return out
}
