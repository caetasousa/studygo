package mapa

import (
	"slices"
	"testing"
)

// Como a separação do comentário por alternativa pode falhar — escrito antes
// do código. Na dúvida, ela não separa: a tela mostra o comentário inteiro,
// como sempre mostrou, em vez de pôr numa alternativa o texto de outra.
//
//  1. comentário em prosa ganha uma separação inventada;
//  2. questão de Certo/Errado é separada por um "a)" que aparece no texto;
//  3. falta a explicação de uma alternativa e as outras se deslocam (o texto
//     da C vai para a B);
//  4. as letras vêm fora de ordem e a explicação cai na alternativa errada;
//  5. o comentário fala de uma letra que a questão não tem ("e)" numa de 4);
//  6. um "a)" no meio da frase ("o item a) do art. 5º") conta como marcador;
//  7. "A)" maiúsculo ou "(a)" entre parênteses não é reconhecido;
//  8. o texto antes do "a)" se perde, ou vai para a alternativa A;
//  9. o marcador ("a)") fica no começo da explicação, repetindo a letra;
//  10. a quebra de linha do Windows (\r\n) deixa sobra no fim do trecho;
//  11. uma letra sem texto nenhum vira uma explicação vazia;
//  12. a explicação de várias linhas é cortada na primeira.
func TestExplicacoes(t *testing.T) {
	quatro := []string{"um", "dois", "três", "quatro"}

	casos := []struct {
		nome         string
		alternativas []string
		comentario   string
		geral        string
		porAlt       []string
	}{
		{
			nome:         "prosa fica inteira",
			alternativas: quatro,
			comentario:   "O orvalho se forma na superfície; não cai das nuvens.",
			geral:        "O orvalho se forma na superfície; não cai das nuvens.",
		},
		{
			nome:       "de julgar não separa",
			comentario: "a) isto\nb) aquilo",
			geral:      "a) isto\nb) aquilo",
		},
		{
			nome:         "falta uma letra",
			alternativas: quatro,
			comentario:   "a) Errada. x\nb) Errada. y\nd) Correta. z",
			geral:        "a) Errada. x\nb) Errada. y\nd) Correta. z",
		},
		{
			nome:         "fora de ordem",
			alternativas: quatro,
			comentario:   "b) Errada. y\na) Errada. x\nc) Errada. w\nd) Correta. z",
			geral:        "b) Errada. y\na) Errada. x\nc) Errada. w\nd) Correta. z",
		},
		{
			nome:         "letra além das alternativas",
			alternativas: quatro,
			comentario:   "a) x\nb) y\nc) w\nd) z\ne) v",
			geral:        "a) x\nb) y\nc) w\nd) z\ne) v",
		},
		{
			nome:         "marcador no meio da frase não conta",
			alternativas: []string{"um", "dois"},
			comentario:   "Veja o item a) do art. 5º e o b) do 6º.",
			geral:        "Veja o item a) do art. 5º e o b) do 6º.",
		},
		{
			nome:         "maiúsculo e entre parênteses",
			alternativas: []string{"um", "dois"},
			comentario:   "A) Errada. x\n(b) Correta. y",
			porAlt:       []string{"Errada. x", "Correta. y"},
		},
		{
			nome:         "introdução fica à parte",
			alternativas: []string{"um", "dois"},
			comentario:   "Questão sobre o ciclo.\nVejamos:\na) Errada. x\nb) Correta. y",
			geral:        "Questão sobre o ciclo.\nVejamos:",
			porAlt:       []string{"Errada. x", "Correta. y"},
		},
		{
			nome:         "quebra de linha do Windows",
			alternativas: []string{"um", "dois"},
			comentario:   "a) Errada. x\r\nb) Correta. y\r\n",
			porAlt:       []string{"Errada. x", "Correta. y"},
		},
		{
			nome:         "letra sem texto",
			alternativas: []string{"um", "dois"},
			comentario:   "a)\nb) Correta. y",
			geral:        "a)\nb) Correta. y",
		},
		{
			nome:         "explicação de várias linhas",
			alternativas: []string{"um", "dois"},
			comentario:   "  a) Errada. x\ncontinua x\n\n b) Correta. y\nGabarito: B",
			porAlt:       []string{"Errada. x\ncontinua x", "Correta. y\nGabarito: B"},
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			q := Questao{Alternativas: c.alternativas, Comentario: c.comentario}

			geral, porAlt := q.Explicacoes()
			if geral != c.geral {
				t.Errorf("geral = %q, quero %q", geral, c.geral)
			}

			if !slices.Equal(porAlt, c.porAlt) {
				t.Errorf("por alternativa = %q, quero %q", porAlt, c.porAlt)
			}
		})
	}
}
