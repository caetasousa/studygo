package prova

// O gabarito oficial: a transcrição do documento da banca, que é de onde
// sai a resposta de cada questão — nunca da IA.

import (
	"maps"
	"slices"
	"strconv"
)

// Gabarito é a transcrição do gabarito oficial. Tipo diz se é preliminar ou
// definitivo — resposta de gabarito preliminar ainda pode mudar.
//
// É um documento da banca à parte das questões: publicado, vai para as
// tabelas do gabarito, e a questão publicada não guarda a resposta.
type Gabarito struct {
	Cargo, Caderno, Tipo string
	Respostas, Situacoes map[string]string
}

// As letras que uma resposta do gabarito pode ter; vazia é questão anulada.
var letrasDoGabarito = []string{"", "A", "B", "C", "D", "E"}

// RespostaDoGabarito é uma linha do gabarito: a questão, a letra — vazia na
// anulada — e a situação que a banca escreveu.
type RespostaDoGabarito struct {
	Numero             int
	Resposta, Situacao string
}

// Linhas são as respostas do gabarito em ordem de questão. Número que não é
// número não tem linha — as pendências já recusam o gabarito com ele.
func (g Gabarito) Linhas() []RespostaDoGabarito {
	numeros := map[int]bool{}
	for _, m := range []map[string]string{g.Respostas, g.Situacoes} {
		for chave := range m {
			if n, err := strconv.Atoi(chave); err == nil && n > 0 {
				numeros[n] = true
			}
		}
	}
	out := make([]RespostaDoGabarito, 0, len(numeros))
	for _, n := range slices.Sorted(maps.Keys(numeros)) {
		chave := strconv.Itoa(n)
		out = append(out, RespostaDoGabarito{Numero: n, Resposta: g.Respostas[chave], Situacao: g.Situacoes[chave]})
	}

	return out
}

// Vazio diz se não há gabarito: nem identificação nem resposta.
func (g Gabarito) Vazio() bool {
	return g.Cargo == "" && g.Caderno == "" && g.Tipo == "" && len(g.Linhas()) == 0
}

// GabaritoDasLinhas remonta o gabarito gravado em linhas.
func GabaritoDasLinhas(cargo, caderno, tipo string, linhas []RespostaDoGabarito) Gabarito {
	g := Gabarito{Cargo: cargo, Caderno: caderno, Tipo: tipo, Respostas: map[string]string{}, Situacoes: map[string]string{}}
	for _, l := range linhas {
		chave := strconv.Itoa(l.Numero)
		g.Respostas[chave], g.Situacoes[chave] = l.Resposta, l.Situacao
	}

	return g
}

// AplicarGabarito copia as respostas oficiais para as questões. A resposta de
// uma questão nunca vem da IA: sem gabarito, ela fica vazia.
//
// Só a questão cuja resposta mudou volta a pedir conferência. Trocar o
// preliminar pelo definitivo, ou reenviar o mesmo arquivo, desmarcava a prova
// inteira — sessenta questões para conferir de novo por causa de uma ou duas. A
// situação não conta: é o texto da banca ao lado da letra, e o definitivo a
// escreve em todas ("Gabarito sem alteração") onde o preliminar não escrevia; a
// anulação muda a letra, e essa conta.
//
// Ganhar a resposta que não tinha também não desconfere: o curador conferiu o
// texto, e a letra vem do documento oficial. O gabarito lido errado da SCGE-PE
// deixava metade das questões sem resposta; o certo, enviado depois,
// desconferiria as que ele confirmou.
func (r *Rascunho) AplicarGabarito() {
	for j := range r.Questoes {
		q := &r.Questoes[j]
		chave := strconv.Itoa(q.Numero)
		if resposta := r.Gabarito.Respostas[chave]; q.Resposta != resposta {
			q.Revisada = q.Revisada && q.Resposta == ""
			q.Resposta = resposta
		}
		q.Situacao = r.Gabarito.Situacoes[chave]
	}
}

// AplicarAlteracoes junta ao gabarito o que a folha de alterações da banca
// mudou: a questão que trocou de letra e a atribuída a todos, que fica sem
// letra como qualquer anulada. O resto do gabarito continua como está — a
// folha só traz o que mudou —, e o gabarito passa a ser o definitivo.
//
// Devolve os números que mudaram de letra e os atribuídos a todos, em ordem.
func (r *Rascunho) AplicarAlteracoes(a Gabarito) (alteradas, atribuidas []int) {
	if r.Gabarito.Respostas == nil {
		r.Gabarito.Respostas = map[string]string{}
	}
	if r.Gabarito.Situacoes == nil {
		r.Gabarito.Situacoes = map[string]string{}
	}
	for _, chave := range slices.Sorted(maps.Keys(a.Respostas)) {
		n, err := strconv.Atoi(chave)
		if err != nil || n < 1 {
			continue
		}
		letra := a.Respostas[chave]
		if !slices.Contains(letrasDoGabarito, letra) {
			continue
		}
		r.Gabarito.Respostas[chave] = letra
		r.Gabarito.Situacoes[chave] = a.Situacoes[chave]
		if letra == "" {
			atribuidas = append(atribuidas, n)
		} else {
			alteradas = append(alteradas, n)
		}
	}
	if len(alteradas) > 0 || len(atribuidas) > 0 {
		r.Gabarito.Tipo = "definitivo"
		r.AplicarGabarito()
	}

	return alteradas, atribuidas
}
