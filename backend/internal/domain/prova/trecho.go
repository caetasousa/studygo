package prova

// O trecho: o retângulo que o curador marca à mão em volta da parte que a
// extração leu mal, e o que a leitura dele troca na questão.

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Trechos são regiões que o curador marca à mão em volta de uma questão que a
// extração leu mal, para a IA ler só aquilo. O rótulo leva o número da questão,
// como o da releitura; o retângulo é o que ele desenhou, e o processador o lê
// como está — não procura a questão pelo número, porque quem apontou onde ela
// está foi o curador.
const (
	prefixoTrecho = "t"
	// Menor que isto, em pontos do PDF, é clique, não trecho.
	ladoMinimoDoTrecho = 10
	// O retângulo chega arredondado da tela; meio ponto fora da região é a
	// borda dela.
	folgaDoTrecho = 0.5
)

// eTrecho diz se a região é um trecho marcado pelo curador.
func eTrecho(o Origem) bool { return strings.HasPrefix(o.Regiao, prefixoTrecho) }

// NovoTrecho é a região que relê a questão `numero` no retângulo que o curador
// marcou. Falso se o retângulo não cabe numa região da mesma página: é dentro
// delas que a tela deixa desenhar, e fora da página o processador recusaria.
func NovoTrecho(regioes []Origem, numero int, o Origem) (Origem, bool) {
	r := o.Retangulo
	if numero < 1 || len(r) != 4 || r[2]-r[0] < ladoMinimoDoTrecho || r[3]-r[1] < ladoMinimoDoTrecho {
		return Origem{}, false
	}
	// Comparações com NaN e infinito dão falso: retângulo inválido não cabe.
	cabe := func(g Origem) bool {
		c := g.Retangulo
		return g.Pagina == o.Pagina && len(c) == 4 &&
			r[0] >= c[0]-folgaDoTrecho && r[1] >= c[1]-folgaDoTrecho &&
			r[2] <= c[2]+folgaDoTrecho && r[3] <= c[3]+folgaDoTrecho
	}
	k := slices.IndexFunc(regioes, cabe)
	if k < 0 {
		return Origem{}, false
	}
	// A folga fica na região: o centésimo que o arredondamento da tela passa da
	// borda, levado ao processador, era "retângulo fora da página".
	c := regioes[k].Retangulo
	dentro := []float64{max(r[0], c[0]), max(r[1], c[1]), min(r[2], c[2]), min(r[3], c[3])}

	return Origem{Pagina: o.Pagina, Retangulo: dentro, Regiao: fmt.Sprintf("%s%d", prefixoTrecho, numero)}, true
}

// RelerTrecho põe a leitura do trecho na fila. Ele entra no fim das regiões,
// no lugar do trecho anterior da mesma questão, e é lá que a etapa o procura.
func (i *Importacao) RelerTrecho(t Origem) {
	i.Regioes = append(slices.DeleteFunc(i.Regioes, func(o Origem) bool { return o.Regiao == t.Regiao }), t)
	i.Etapa = EtapaTrecho
	i.Estado = EstadoNaFila
	i.Erro = ""
}

// TrechoPendente é o trecho que a EtapaTrecho lê — o último das regiões — e o
// número da questão dele.
func (i Importacao) TrechoPendente() (Origem, int, bool) {
	if len(i.Regioes) == 0 {
		return Origem{}, 0, false
	}
	t := i.Regioes[len(i.Regioes)-1]
	n, err := strconv.Atoi(strings.TrimPrefix(t.Regiao, prefixoTrecho))

	return t, n, eTrecho(t) && err == nil && n > 0
}

// AplicarTrecho põe na questão `numero` o que a leitura do trecho marcado
// trouxe. O curador marca a parte que ficou errada — a questão inteira, só o
// enunciado, só as alternativas que faltaram —, e o trecho troca o enunciado,
// se veio, e cada alternativa que veio, pela letra; o que ele não cobre fica
// como estava. Diferente da releitura automática, não compara as leituras: foi
// o curador que apontou onde está o que faltava. Da questão fica também o que
// não é leitura — a matéria, a resposta do gabarito, os textos ligados —, e a
// conferência cai, porque o conteúdo é outro.
//
// Sem o número dentro do trecho, o modelo chuta outro (as alternativas da 60
// do TRT-15, marcadas sozinhas, voltaram como "questão 1"), e o trecho pode
// pegar a ponta de uma vizinha: vale a questão com o número pedido, senão a
// única lida.
func (r *Rascunho) AplicarTrecho(numero int, lido Rascunho) {
	r.Extracoes = append(r.Extracoes, lido.Extracoes...)
	k := slices.IndexFunc(lido.Questoes, func(q Questao) bool { return q.Numero == numero })
	if k < 0 && len(lido.Questoes) == 1 {
		k = 0
	}
	if k < 0 || (semConteudo(lido.Questoes[k].Blocos) && len(lido.Questoes[k].Alternativas) == 0) {
		r.Alertas = append(r.Alertas, fmt.Sprintf(
			"A leitura do trecho marcado não achou a questão %d, e ela ficou como estava. "+
				"Marque só a parte dela, sem pegar outra questão.", numero,
		))
		return
	}

	lida := lido.Questoes[k]
	referencia := larguraDaArea(lida.Origens)
	lida.Blocos = limparBlocos(lida.Blocos)
	dimensionar(lida.Blocos, referencia)
	for j := range lida.Alternativas {
		lida.Alternativas[j].Blocos = limparBlocos(lida.Alternativas[j].Blocos)
		dimensionar(lida.Alternativas[j].Blocos, referencia)
	}
	comEnunciado := !semConteudo(lida.Blocos)

	dela := func(q Questao) bool { return q.Numero == numero }
	j := slices.IndexFunc(r.Questoes, dela)
	if j < 0 {
		chave := strconv.Itoa(numero)
		r.Questoes = append(r.Questoes, Questao{
			Numero: numero, Resposta: r.Gabarito.Respostas[chave], Situacao: r.Gabarito.Situacoes[chave],
		})
		j = len(r.Questoes) - 1
	}
	q := &r.Questoes[j]
	q.Revisada, q.IgualA = false, ""
	if q.Disciplina == "" {
		q.Disciplina = lida.Disciplina
	}

	if comEnunciado && len(lida.Alternativas) > 0 {
		// O trecho cobre a questão inteira: ela é a leitura, e a leitura diz se
		// o retângulo a pegou sem corte.
		q.Blocos, q.Alternativas, q.Origens, q.Completa = lida.Blocos, lida.Alternativas, lida.Origens, lida.Completa
	} else {
		if comEnunciado {
			q.Blocos = lida.Blocos
		}
		for _, a := range lida.Alternativas {
			if i := slices.IndexFunc(q.Alternativas, func(b Alternativa) bool { return b.Letra == a.Letra }); i >= 0 {
				q.Alternativas[i] = a
			} else {
				q.Alternativas = append(q.Alternativas, a)
			}
		}
		slices.SortStableFunc(q.Alternativas, func(a, b Alternativa) int { return strings.Compare(a.Letra, b.Letra) })
		q.Origens = unirOrigens(q.Origens, lida.Origens)
		// O pedaço vem marcado como cortado — e é: juntar os pedaços era o que
		// a marca pedia, e quem os juntou foi o curador.
		q.Completa = !semConteudo(q.Blocos) && len(q.Alternativas) == 5
	}

	// Ainda sem as cinco, o trecho não cobriu onde elas estão — e sem o aviso
	// parecia que a leitura não tinha acontecido.
	if n := len(q.Alternativas); n < 5 {
		r.Alertas = append(r.Alertas, fmt.Sprintf(
			"A questão %d continua com %d de 5 alternativas depois da leitura do trecho: "+
				"marque de novo, esticando o retângulo até a alternativa (E).", numero, n,
		))
	}

	r.OrdenarQuestoes()
	// Sem matéria, a da questão anterior, como em HerdarDisciplinas.
	if j := slices.IndexFunc(r.Questoes, dela); j > 0 && r.Questoes[j].Disciplina == "" {
		r.Questoes[j].Disciplina = r.Questoes[j-1].Disciplina
	}
	r.AcertarApoios()
}

// TrechoRecusado registra que o processador recusou o trecho: a questão fica
// como estava, e o curador sabe por quê.
func (r *Rascunho) TrechoRecusado(numero int, motivo string) {
	r.Alertas = append(r.Alertas, fmt.Sprintf(
		"O trecho marcado para a questão %d não foi lido (%s), e ela ficou como estava.", numero, motivo,
	))
}
