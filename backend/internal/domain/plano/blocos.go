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
}

// BlocoCtx carries everything the breakdown needs beyond the day itself.
type BlocoCtx struct {
	HorasDia float64           // the daily budget
	Nomes    map[string]string // discipline codigo -> display name
	Simulado Composicao        // question split of a full mock exam
	Perfil   Perfil            // the user's study method
}

// Composicao is how many questions of each bloco a full exam has.
type Composicao struct {
	Gerais      int
	Especificas int
}

// Blocos returns the timed breakdown for a day, mirroring the artifact's
// blocos(): content days get two study blocks plus a spaced-review tail; the
// special days get their own fixed splits.
func Blocos(d Dia, ctx BlocoCtx) []Bloco {
	h := ctx.HorasDia * 60

	if len(d.Itens) > 0 {
		rev := d.Tipo == TipoRevisaoDirigida
		porBloco := int(math.Round(float64(d.Meta) / float64(len(d.Itens))))

		out := make([]Bloco, 0, len(d.Itens)+1)

		perfil := ctx.Perfil.Normalizar()

		for idx, it := range d.Itens {
			rotulo := "1º bloco"
			if idx == 1 {
				rotulo = "2º bloco"
			}

			out = append(out, Bloco{
				Minutos: m5(h * 0.42),
				Titulo:  rotulo + " — " + ctx.Nomes[it.Disciplina],
				Detalhe: detalheDoBloco(perfil.ModoDe(it.Disciplina), rev, porBloco),
			})
		}

		out = append(out, Bloco{
			Minutos: m5(h * 0.16),
			Titulo:  "Revisão espaçada",
			Detalhe: revisaoDetalhe(perfil),
		})

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
	default: // weekly review
		return []Bloco{
			{Minutos: m5(h * 0.35), Titulo: "Revisão ativa", Detalhe: "releia os resumos das duas matérias de cada dia da semana"},
			{Minutos: m5(h * 0.50), Titulo: "Bateria de questões", Detalhe: "no peso da prova: mais específicos que gerais"},
			{Minutos: m5(h * 0.15), Titulo: "Caderno de erros", Detalhe: "anote o porquê de cada erro, não só a resposta"},
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

// revisaoDetalhe names the user's own review intervals instead of a fixed
// D-1 / D-7 / D-30.
func revisaoDetalhe(perfil Perfil) string {
	partes := make([]string, 0, len(perfil.Intervalos))
	for _, d := range perfil.Intervalos {
		partes = append(partes, "D-"+strconv.Itoa(d))
	}

	como := "retome"
	if perfil.RevisaoPorQuestoes {
		como = "resolva questões dos temas de"
	}

	return como + " " + strings.Join(partes, ", ") + " listados ao lado"
}
