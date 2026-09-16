package prova

// A releitura automática: quando as regiões do caderno cortam uma questão,
// ela ganha um recorte só dela, e da segunda leitura entra só o que faltava.

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Releituras são regiões a mais, uma por questão que as regiões do caderno
// deixaram cortada. A sobreposição entre regiões não basta para questão alta
// ou que começa rente à borda: uma região a vê sem o fim, e a seguinte, que a
// vê sem o número, pula a questão — sobravam questões com uma ou duas
// alternativas.
const (
	prefixoReleitura = "q"
	maxReleituras    = 8
	// bordaDoCorte: pedaço lido a menos disto da borda da região foi cortado por ela.
	bordaDoCorte = 0.15
)

// EReleitura diz se a região é a releitura de uma questão.
func EReleitura(o Origem) bool { return strings.HasPrefix(o.Regiao, prefixoReleitura) }

// QuestaoDaReleitura é o número da questão que a região relê: com ele, o
// processador acha a questão pelo número impresso no caderno, que não erra — o
// retângulo que o modelo dá erra por centenas de pontos.
func QuestaoDaReleitura(o Origem) (int, bool) {
	if !EReleitura(o) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(o.Regiao, prefixoReleitura))

	return n, err == nil && n > 0
}

// inteira é a questão com as cinco alternativas, lida sem corte.
func inteira(q Questao) bool { return q.Completa && len(q.Alternativas) == 5 }

// vazia é o número sem nada: o modelo listando o que um texto cita.
func vazia(q Questao) bool { return len(q.Blocos) == 0 && len(q.Alternativas) == 0 }

// Releituras devolve as regiões que releem as questões incompletas (sem as
// cinco alternativas, ou marcadas como cortadas) e as que faltam entre duas
// lidas. Vazio se as regiões já têm releituras: elas acontecem uma vez só.
//
// Cada releitura é uma região do tamanho das outras da mesma página, centrada
// onde a questão foi cortada — a borda da região que a leu pela metade. O
// retângulo que o modelo dá para a questão erra por dezenas de pontos, e
// recortar só por ele pegava as vizinhas e perdia a própria questão; a borda é
// exata.
//
// A medida é da página, nunca do documento: um PDF escaneado pode ter a capa
// com o dobro do tamanho das outras páginas, e a releitura com a largura dela
// saía da página.
func (r Rascunho) Releituras(regioes []Origem) []Origem {
	if len(regioes) == 0 || slices.ContainsFunc(regioes, EReleitura) {
		return nil
	}

	paginas := map[int]areaLida{}
	porRotulo := map[string]Origem{}
	for _, o := range regioes {
		if len(o.Retangulo) != 4 {
			continue
		}
		paginas[o.Pagina] = paginas[o.Pagina].com(o.Retangulo)
		porRotulo[o.Regiao] = o
	}
	if len(paginas) == 0 {
		return nil
	}

	var out []Origem
	centros := map[int][]float64{}
	reler := func(numero, pagina int, centro float64) {
		a, ok := paginas[pagina]
		// Página que nenhuma região cobriu não tem onde reler.
		if !ok || len(out) >= maxReleituras {
			return
		}
		// O mesmo recorte já relê esta questão: duas cortadas na mesma borda,
		// ou duas que faltaram entre as mesmas vizinhas.
		for _, c := range centros[pagina] {
			if math.Abs(c-centro) < a.altura/4 {
				return
			}
		}
		centros[pagina] = append(centros[pagina], centro)
		fim := min(a.y1, max(a.y0, centro-a.altura/2)+a.altura)
		out = append(out, Origem{
			Pagina: pagina, Retangulo: []float64{a.x0, max(a.y0, fim-a.altura), a.x1, fim},
			Regiao: fmt.Sprintf("%s%d", prefixoReleitura, numero),
		})
	}

	lidas := map[int]Questao{}
	ultima := r.Total
	for _, q := range r.Questoes {
		lidas[q.Numero] = q
		ultima = max(ultima, q.Numero)
	}

	// A área que o modelo deu para a questão. A que veio vazia, com a região
	// inteira como origem, não diz onde a questão está.
	propria := func(n int) (Origem, bool) {
		o, ok := ondeEsta(lidas[n])
		if !ok || semAreaPropria(o, porRotulo) {
			return Origem{}, false
		}
		return o, true
	}
	// Sem área própria, a questão está entre as vizinhas lidas mais próximas.
	entreVizinhas := func(n int) (int, float64, bool) {
		var anterior, proxima Origem
		a, p := false, false
		for m := n - 1; m >= 1 && !a; m-- {
			anterior, a = propria(m)
		}
		for m := n + 1; m <= ultima && !p; m++ {
			proxima, p = propria(m)
		}
		// Passar do pé da página, ou do topo, é mudar de página: com uma região
		// por página, a questão que faltou antes da primeira lida da página 3
		// está no pé da 2 — relê-la na 3 lia de novo o que já estava lido.
		switch {
		case a && p && anterior.Pagina == proxima.Pagina:
			return anterior.Pagina, (anterior.Retangulo[3] + proxima.Retangulo[1]) / 2, true
		case a:
			pg := paginas[anterior.Pagina]
			centro := anterior.Retangulo[3] + 0.4*pg.altura
			if seguinte, ok := paginas[anterior.Pagina+1]; ok && centro > pg.y1 {
				return anterior.Pagina + 1, seguinte.y0, true
			}
			return anterior.Pagina, centro, true
		case p:
			pg := paginas[proxima.Pagina]
			centro := proxima.Retangulo[1] - 0.4*pg.altura
			if antes, ok := paginas[proxima.Pagina-1]; ok && centro < pg.y0 {
				return proxima.Pagina - 1, antes.y1, true
			}
			return proxima.Pagina, centro, true
		}
		return 0, 0, false
	}

	for n := 1; n <= ultima; n++ {
		q, lida := lidas[n]
		if (lida && inteira(q)) || (!lida && r.Total > 0 && n > r.Total) {
			continue
		}
		var (
			pagina int
			centro float64
			ok     bool
		)
		if _, temArea := propria(n); temArea {
			pagina, centro, ok = ondeFoiCortada(q, porRotulo)
		} else {
			pagina, centro, ok = entreVizinhas(n)
		}
		if ok {
			reler(n, pagina, centro)
		}
	}

	return out
}

// areaLida é o que as regiões cobrem de uma página, e a altura da maior delas.
type areaLida struct {
	x0, y0, x1, y1, altura float64
	vista                  bool
}

func (a areaLida) com(ret []float64) areaLida {
	if !a.vista {
		return areaLida{x0: ret[0], y0: ret[1], x1: ret[2], y1: ret[3], altura: ret[3] - ret[1], vista: true}
	}

	return areaLida{
		x0: min(a.x0, ret[0]), y0: min(a.y0, ret[1]), x1: max(a.x1, ret[2]), y1: max(a.y1, ret[3]),
		altura: max(a.altura, ret[3]-ret[1]), vista: true,
	}
}

// semAreaPropria: a origem cobre a região inteira — o modelo não disse onde a
// questão está.
func semAreaPropria(o Origem, regioes map[string]Origem) bool {
	reg, ok := regioes[o.Regiao]
	if !ok || len(reg.Retangulo) != 4 {
		return false
	}

	return o.Retangulo[3]-o.Retangulo[1] >= 0.9*(reg.Retangulo[3]-reg.Retangulo[1])
}

// ondeFoiCortada devolve a borda da região em que a questão foi lida pela
// metade — é por ali que ela atravessa para a região vizinha. Sem borda por
// perto, o meio do pedaço lido.
func ondeFoiCortada(q Questao, regioes map[string]Origem) (int, float64, bool) {
	o, ok := ondeEsta(q)
	if !ok {
		return 0, 0, false
	}
	y0, y1 := o.Retangulo[1], o.Retangulo[3]
	if reg, achou := regioes[o.Regiao]; achou && len(reg.Retangulo) == 4 {
		altura := reg.Retangulo[3] - reg.Retangulo[1]
		if reg.Retangulo[3]-y1 < bordaDoCorte*altura {
			return o.Pagina, reg.Retangulo[3], true
		}
		// A borda de cima da primeira região é o topo da página: nada a cortar.
		if reg.Retangulo[1] > 0 && y0-reg.Retangulo[1] < bordaDoCorte*altura {
			return o.Pagina, reg.Retangulo[1], true
		}
	}

	return o.Pagina, (y0 + y1) / 2, true
}

// ondeEsta é a primeira origem da questão com retângulo.
func ondeEsta(q Questao) (Origem, bool) {
	for _, o := range q.Origens {
		if len(o.Retangulo) == 4 {
			return o, true
		}
	}

	return Origem{}, false
}

// ReleituraRecusada registra que a releitura não aconteceu: ela é um reforço,
// e a questão fica como as regiões a leram, com o curador avisado.
func (r *Rascunho) ReleituraRecusada(o Origem, motivo string) {
	r.Alertas = append(r.Alertas, fmt.Sprintf(
		"A releitura da questão %s não foi feita (%s); ela ficou como as regiões a leram — confira no original.",
		strings.TrimPrefix(o.Regiao, prefixoReleitura), motivo,
	))
}

// AplicarReleitura aproveita da releitura só o que estava incompleto ou
// faltando, e só se ficou melhor: completa, ou com mais alternativas. A
// releitura cobre uma região inteira em volta do corte; mesclar as vizinhas
// geraria alertas de leitura dupla, e juntar dois pedaços da mesma questão
// repetiria alternativas. Texto de apoio do recorte já veio na região dele.
func (r *Rascunho) AplicarReleitura(n Rascunho) {
	r.Extracoes = append(r.Extracoes, n.Extracoes...)
	for _, q := range n.Questoes {
		j := slices.IndexFunc(r.Questoes, func(o Questao) bool { return o.Numero == q.Numero })
		if j < 0 {
			r.Questoes = append(r.Questoes, q)
			continue
		}
		atual := &r.Questoes[j]
		if atual.Completa && len(atual.Alternativas) == 5 {
			continue
		}
		if (q.Completa && !atual.Completa) || len(q.Alternativas) > len(atual.Alternativas) {
			*atual = q
		}
	}
	// O texto de apoio que a releitura leu não entrou: a questão fica com os
	// textos do rascunho, pela lista de cada um.
	r.AcertarApoios()
}
