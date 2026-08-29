package plano

import (
	"math"
	"strconv"
)

// Bloco is one time slice of a day's routine: minutes, a label, and what to do.
type Bloco struct {
	Minutos int    `json:"minutos"`
	Titulo  string `json:"titulo"`
	Detalhe string `json:"detalhe"`
}

// Blocos returns the timed breakdown for a day, mirroring the artifact's
// blocos(): content days get two study blocks plus a spaced-review tail; the
// special days get their own fixed splits. horasDia is the daily budget.
func Blocos(d Dia, horasDia float64, nomes map[string]string) []Bloco {
	h := horasDia * 60

	if len(d.Itens) > 0 {
		rev := d.Tipo == TipoRevisaoDirigida
		porBloco := int(math.Round(float64(d.Meta) / float64(len(d.Itens))))

		out := make([]Bloco, 0, len(d.Itens)+1)

		for idx, it := range d.Itens {
			rotulo := "1º bloco"
			if idx == 1 {
				rotulo = "2º bloco"
			}

			detalhe := "teoria com resumo de própria autoria e " +
				strconv.Itoa(porBloco) + " questões do tema, corrigidas uma a uma"
			if rev {
				detalhe = "reconstrua o assunto de memória, sem consultar, e resolva " +
					strconv.Itoa(porBloco) + " questões só dele"
			}

			out = append(out, Bloco{
				Minutos: m5(h * 0.42),
				Titulo:  rotulo + " — " + nomes[it.Disciplina],
				Detalhe: detalhe,
			})
		}

		out = append(out, Bloco{
			Minutos: m5(h * 0.16),
			Titulo:  "Revisão espaçada",
			Detalhe: "retome os temas de D-1, D-7 e D-30 listados ao lado",
		})

		return out
	}

	switch d.Tipo {
	case TipoSimulado:
		return []Bloco{
			{Minutos: m5(h * 0.70), Titulo: "Simulado cronometrado", Detalhe: "25 gerais + 45 específicos, sem pausa e sem consulta"},
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
