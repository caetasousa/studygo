package plano

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// normalizarTema é a comparação de tópico: sem caixa e sem espaço sobrando. O
// mesmo tópico chega com grafias diferentes — o edital editado, uma ementa
// dividida — e comparar o texto exato fazia o estudado voltar ao cronograma.
func normalizarTema(tema string) string {
	return strings.Join(strings.Fields(strings.ToLower(tema)), " ")
}

// ArrumarEstudado põe o cronograma de acordo com o que o estudante já
// estudou, e diz se mudou alguma coisa.
//
//   - O que foi concluído com data adiante (marcado como estudado antes da
//     hora) vai para hoje, ou para o último dia de estudo antes de hoje quando
//     hoje não é um, marcado como Antecipada: sai do cronograma sem deixar de
//     existir. Concluído no futuro era "feito lá no final", e o cronograma
//     continuava cobrando o dia de onde ele não saiu.
//   - A 1ª passada pendente de um tópico já estudado sai do cronograma; no
//     bloco que junta vários tópicos, sai só o estudado. A atividade que já
//     tem algo lançado fica: o lançamento é história.
//
// A 2ª passada e as revisões ficam: são repetição por desenho. Encostar o que
// sobrou é o passo seguinte (CompactarAtividades), a partir de hoje.
func ArrumarEstudado(
	atividades []Atividade,
	dias []Dia,
	hoje time.Time,
	concluida func(uuid.UUID) bool,
	lancada func(uuid.UUID) bool,
) ([]Atividade, bool) {
	hoje = day(hoje)
	destino := DiaDoEstudoFeito(dias, hoje)
	mudou := false

	feito := map[chaveDeConteudo]bool{}

	for _, a := range atividades {
		if coberta(a) && concluida(a.ID) {
			for _, p := range PartesDoTema(a.Tema) {
				feito[chaveDeConteudo{disciplina: a.Disciplina, tema: normalizarTema(p)}] = true
			}
		}
	}

	saida := make([]Atividade, 0, len(atividades))
	adiante := []Atividade{}

	for _, a := range atividades {
		switch {
		case concluida(a.ID) && a.Disciplina != "" && day(a.Data).After(hoje) && !destino.IsZero():
			adiante = append(adiante, a)

			continue
		case coberta(a) && !concluida(a.ID) && !day(a.Data).Before(hoje):
			partes := PartesDoTema(a.Tema)
			resto := make([]string, 0, len(partes))

			for _, p := range partes {
				if !feito[chaveDeConteudo{disciplina: a.Disciplina, tema: normalizarTema(p)}] {
					resto = append(resto, p)
				}
			}

			if len(resto) == 0 && !lancada(a.ID) {
				mudou = true

				continue
			}

			if len(resto) > 0 && len(resto) < len(partes) {
				a.Tema = strings.Join(resto, separadorDeTemas)
				mudou = true
			}
		}

		saida = append(saida, a)
	}

	if len(adiante) > 0 {
		sort.SliceStable(adiante, func(i, j int) bool {
			if !sameDay(adiante[i].Data, adiante[j].Data) {
				return adiante[i].Data.Before(adiante[j].Data)
			}

			return adiante[i].Posicao < adiante[j].Posicao
		})

		posicao := len(doDia(saida, destino))
		origens := map[time.Time]bool{destino: true}

		for _, a := range adiante {
			origens[day(a.Data)] = true
			a.Antecipada = true
			a.Data = destino
			a.Posicao = posicao
			posicao++
			saida = append(saida, a)
		}

		renumerar(saida, origens)

		mudou = true
	} else if mudou {
		renumerar(saida, diasDe(saida))
	}

	return saida, mudou
}

// DiaDoEstudoFeito é onde fica o que foi estudado antes da hora: hoje, se
// hoje recebe conteúdo; senão o último dia que recebe antes de hoje; e, com o
// plano ainda por começar, o primeiro que recebe. Zero quando não há nenhum.
func DiaDoEstudoFeito(dias []Dia, hoje time.Time) time.Time {
	var antes, depois time.Time

	for _, d := range dias {
		dt := day(d.Data)
		if !DestinoValido(dias, dt) {
			continue
		}

		switch {
		case dt.Equal(hoje):
			return dt
		case dt.Before(hoje) && dt.After(antes):
			antes = dt
		case dt.After(hoje) && (depois.IsZero() || dt.Before(depois)):
			depois = dt
		}
	}

	if !antes.IsZero() {
		return antes
	}

	return depois
}

func diasDe(atividades []Atividade) map[time.Time]bool {
	out := map[time.Time]bool{}
	for _, a := range atividades {
		out[day(a.Data)] = true
	}

	return out
}

// TopicosEstudados conta quantos tópicos da matéria já foram estudados: os que
// têm uma 1ª passada concluída, contados uma vez cada, na grafia que for, e um
// por um dentro dos blocos que juntam vários. É o avanço que o balanceamento
// mostra — a 2ª passada é repetição, não cobertura.
func TopicosEstudados(disciplina string, temas []string, atividades []Atividade, concluida func(uuid.UUID) bool) int {
	feitos := map[string]bool{}

	for _, a := range atividades {
		if a.Disciplina != disciplina || !coberta(a) || !concluida(a.ID) {
			continue
		}

		for _, p := range PartesDoTema(a.Tema) {
			feitos[normalizarTema(p)] = true
		}
	}

	n := 0
	contados := map[string]bool{}

	for _, t := range temas {
		k := normalizarTema(t)
		if feitos[k] && !contados[k] {
			contados[k] = true
			n++
		}
	}

	return n
}

// Cobertura é o que o cronograma gravado faz com os tópicos de uma matéria na
// fase de aprender: quantos deles aparecem (estudados ou por estudar) e
// quantas vezes, somando as aparições.
type Cobertura struct {
	Cobertos  int
	Aparicoes int
}

// CoberturaDaMateria lê o cronograma como ele está — com o que foi
// antecipado, reorganizado ou removido — em vez dos blocos que o motor
// calcularia. Cada tópico conta dentro dos blocos que juntam vários: uma
// matéria com mais tópicos que vagas vê todos eles, só que agrupados, e contar
// blocos a fazia parecer incompleta. Reforço, revisão dirigida e a reta final
// não são aprender.
func CoberturaDaMateria(dias []Dia, disciplina string, temas []string) Cobertura {
	daMateria := map[string]bool{}
	for _, t := range temas {
		daMateria[normalizarTema(t)] = true
	}

	vistos := map[string]bool{}
	c := Cobertura{}

	for _, d := range dias {
		if d.Fase == FaseReta {
			continue
		}

		for _, it := range d.Itens {
			if it.Disciplina != disciplina || temaBase(it.Tema) != it.Tema {
				continue
			}

			for _, p := range PartesDoTema(it.Tema) {
				k := normalizarTema(p)
				if !daMateria[k] {
					continue
				}

				c.Aparicoes++
				vistos[k] = true
			}
		}
	}

	c.Cobertos = len(vistos)

	return c
}
