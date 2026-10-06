package mapa

import (
	"errors"
	"slices"
	"strings"
)

// ErrTemaForaDaMateria: o tópico escolhido não é um tópico da matéria.
var ErrTemaForaDaMateria = errors.New("o tópico escolhido não é desta matéria — recarregue a página e escolha de novo")

// Vinculo é um mapa ligado a uma matéria e os tópicos dela que ele cobre.
//
// Os tópicos vão pelo texto, como o cronograma os grava em cada atividade: a
// ementa não tem identidade estável (editar o concurso regrava os tópicos), e
// é pelo texto que a atividade diz de que assunto é. Sem tópico nenhum, o mapa
// vale para a matéria inteira — é o que eram os vínculos de antes da escolha.
type Vinculo struct {
	Mapa  Resumo
	Temas []string
}

// TemasCitados são os tópicos da matéria que citam um termo de `reconhecer`,
// na ordem da ementa: o que a importação marca sem perguntar. A palavra tem de
// aparecer inteira, sem caixa nem acento: "ITIL" não casa com "utilizar".
func (m Mapa) TemasCitados(temas []string) []string {
	var out []string

	for _, t := range temas {
		tema := " " + dobrar(t) + " "

		if slices.ContainsFunc(m.Reconhecer, func(termo string) bool {
			agulha := dobrar(termo)
			return agulha != "" && strings.Contains(tema, " "+agulha+" ")
		}) {
			out = append(out, t)
		}
	}

	return out
}

// EscolherTemas confere a escolha da tela contra a ementa da matéria e a
// devolve na ordem dela, sem repetição. Nenhum tópico é a matéria inteira.
func EscolherTemas(escolhidos, daMateria []string) ([]string, error) {
	quer := map[string]bool{}

	for _, e := range escolhidos {
		e = strings.TrimSpace(e)
		if !slices.Contains(daMateria, e) {
			return nil, ErrTemaForaDaMateria
		}

		quer[e] = true
	}

	out := []string{}

	for _, t := range daMateria {
		if quer[t] {
			out = append(out, t)
			delete(quer, t)
		}
	}

	return out, nil
}
