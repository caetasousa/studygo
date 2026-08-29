package plano

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

// Perfil is the user's study method. Whether simulados matter, how long the
// spaced-review intervals are and how much of a block is spent on questions are
// personal calls, so they live here instead of being baked into the engine.
//
// The zero value is not usable — build one with PerfilPadrao and override it.
type Perfil struct {
	Simulados          Frequencia
	Discursiva         bool
	Intervalos         []int // days after study: 24h / 7d / 30d by default
	PctQuestoes        float64
	RevisaoPorQuestoes bool
	QuestoesPorRevisao int
	LimiarFraco        int // % below which a battery counts as weak
	Modos              map[string]Modo
}

// PerfilPadrao reproduces the artifact's behaviour exactly: a full mock exam on
// the last day of every reta-final week, an essay on the day before it, and the
// 24h / 7d / 30d review cycle. Gerar with this profile is byte-identical to the
// engine before the profile existed.
func PerfilPadrao() Perfil {
	return Perfil{
		Simulados:          SimuladoSemanal,
		Discursiva:         true,
		Intervalos:         []int{1, 7, 30},
		PctQuestoes:        0.5,
		RevisaoPorQuestoes: true,
		QuestoesPorRevisao: 10,
		LimiarFraco:        70,
		Modos:              map[string]Modo{},
	}
}

// ModoDe returns how a discipline is studied, defaulting to ModoCompleto.
func (p Perfil) ModoDe(codigo string) Modo {
	switch p.Modos[codigo] {
	case ModoQuestoes:
		return ModoQuestoes
	case ModoTeoria:
		return ModoTeoria
	default:
		return ModoCompleto
	}
}

// Normalizar clamps every field to a usable value, so a profile coming from the
// database or an API payload can never produce a broken plan.
//
// The zero Perfil means "no method chosen" and normalizes to PerfilPadrao — the
// booleans have no third state to say "unset", so Simulados being empty is what
// marks the whole struct as never filled in.
func (p Perfil) Normalizar() Perfil {
	padrao := PerfilPadrao()

	if p.Simulados == "" {
		if p.Modos != nil {
			padrao.Modos = p.Modos
		}

		return padrao
	}

	switch p.Simulados {
	case SimuladoNunca, SimuladoQuinzenal, SimuladoSemanal:
	default:
		p.Simulados = padrao.Simulados
	}

	p.Intervalos = intervalosValidos(p.Intervalos)
	if len(p.Intervalos) == 0 {
		p.Intervalos = padrao.Intervalos
	}

	if p.PctQuestoes < 0.1 || p.PctQuestoes > 0.9 {
		p.PctQuestoes = padrao.PctQuestoes
	}

	if p.QuestoesPorRevisao < 1 || p.QuestoesPorRevisao > 200 {
		p.QuestoesPorRevisao = padrao.QuestoesPorRevisao
	}

	if p.LimiarFraco < 1 || p.LimiarFraco > 100 {
		p.LimiarFraco = padrao.LimiarFraco
	}

	if p.Modos == nil {
		p.Modos = map[string]Modo{}
	}

	return p
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
