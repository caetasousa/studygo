package prova

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// Uma questão comum a vários cargos — as vinte de Conhecimentos Gerais do TJCE
// caem iguais em todos os cargos de analista — é guardada uma vez só. Para
// isso a questão se divide no que ela É, em qualquer prova (texto,
// alternativas, figuras), e no LUGAR que ocupa numa prova (número, matéria, a
// resposta do gabarito daquela prova, onde está no PDF dela). O conteúdo é
// identificado pela impressão: o mesmo conteúdo dá a mesma impressão, e o
// banco guarda uma linha só.

// ConteudoDeQuestao é o que a questão é, em qualquer prova. As figuras vêm sem
// a posição no PDF, que é de cada caderno.
type ConteudoDeQuestao struct {
	Blocos       []Bloco
	Alternativas []Alternativa
}

// FiguraNoPDF é onde uma figura está no caderno de uma prova e se o curador
// conferiu o recorte ali.
type FiguraNoPDF struct {
	Origem   *Origem
	Revisado bool
}

// LugarDaQuestao é o que a questão é numa prova.
type LugarDaQuestao struct {
	Numero             int
	Disciplina         string
	Apoios             []string
	Origens            []Origem
	Resposta, Situacao string
	Revisada, Completa bool
	// Figuras segue a ordem das figuras no conteúdo: enunciado, depois cada
	// alternativa.
	Figuras []FiguraNoPDF
	IgualA  string
}

// Separar divide a questão em conteúdo e lugar.
func (q Questao) Separar() (ConteudoDeQuestao, LugarDaQuestao) {
	var figuras []FiguraNoPDF
	c := ConteudoDeQuestao{Blocos: semPosicao(q.Blocos, &figuras)}
	for _, a := range q.Alternativas {
		c.Alternativas = append(c.Alternativas, Alternativa{Letra: a.Letra, Blocos: semPosicao(a.Blocos, &figuras)})
	}

	return c, LugarDaQuestao{
		Numero: q.Numero, Disciplina: q.Disciplina, Apoios: q.Apoios, Origens: q.Origens,
		Resposta: q.Resposta, Situacao: q.Situacao, Revisada: q.Revisada, Completa: q.Completa,
		Figuras: figuras, IgualA: q.IgualA,
	}
}

// JuntarQuestao remonta a questão. Figura a mais no conteúdo, sem lugar
// correspondente, fica sem posição no PDF.
func JuntarQuestao(c ConteudoDeQuestao, l LugarDaQuestao) Questao {
	k := 0
	q := Questao{
		Numero: l.Numero, Disciplina: l.Disciplina, Apoios: l.Apoios, Origens: l.Origens,
		Resposta: l.Resposta, Situacao: l.Situacao, Revisada: l.Revisada, Completa: l.Completa,
		IgualA: l.IgualA, Blocos: comPosicao(c.Blocos, l.Figuras, &k),
	}
	for _, a := range c.Alternativas {
		q.Alternativas = append(q.Alternativas, Alternativa{Letra: a.Letra, Blocos: comPosicao(a.Blocos, l.Figuras, &k)})
	}

	return q
}

// Impressao identifica o conteúdo. Espaços a mais ou a menos — a diferença que
// duas leituras do mesmo caderno costumam ter — não mudam a impressão.
func (c ConteudoDeQuestao) Impressao() string {
	h := sha256.New()
	escreverBlocos(h, c.Blocos)
	for _, a := range c.Alternativas {
		fmt.Fprintf(h, "alternativa %s\x1d", a.Letra)
		escreverBlocos(h, a.Blocos)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// ConteudoDeApoio é o texto (ou figura) compartilhado, em qualquer prova.
type ConteudoDeApoio struct {
	Blocos []Bloco
}

// LugarDoApoio é o que o texto é numa prova: quais questões o usam, o aviso do
// caderno e onde está no PDF.
type LugarDoApoio struct {
	ID       string
	Questoes []int
	Aviso    string
	Origens  []Origem
	Figuras  []FiguraNoPDF
	Revisado bool
	IgualA   string
}

func (a Apoio) Separar() (ConteudoDeApoio, LugarDoApoio) {
	var figuras []FiguraNoPDF
	c := ConteudoDeApoio{Blocos: semPosicao(a.Blocos, &figuras)}

	return c, LugarDoApoio{
		ID: a.ID, Questoes: a.Questoes, Aviso: a.Aviso, Origens: a.Origens,
		Figuras: figuras, Revisado: a.Revisado, IgualA: a.IgualA,
	}
}

func JuntarApoio(c ConteudoDeApoio, l LugarDoApoio) Apoio {
	k := 0

	return Apoio{
		ID: l.ID, Blocos: comPosicao(c.Blocos, l.Figuras, &k), Questoes: l.Questoes, Aviso: l.Aviso,
		Origens: l.Origens, Revisado: l.Revisado, IgualA: l.IgualA,
	}
}

func (c ConteudoDeApoio) Impressao() string {
	h := sha256.New()
	escreverBlocos(h, c.Blocos)

	return hex.EncodeToString(h.Sum(nil))
}

func semPosicao(bs []Bloco, figuras *[]FiguraNoPDF) []Bloco {
	out := make([]Bloco, len(bs))
	for i, b := range bs {
		if b.Tipo == "imagem" {
			*figuras = append(*figuras, FiguraNoPDF{Origem: b.Origem, Revisado: b.Revisado})
		}
		b.Origem, b.Revisado = nil, false
		out[i] = b
	}

	return out
}

func comPosicao(bs []Bloco, figuras []FiguraNoPDF, k *int) []Bloco {
	out := slices.Clone(bs)
	for i := range out {
		if out[i].Tipo != "imagem" {
			continue
		}
		if *k < len(figuras) {
			out[i].Origem, out[i].Revisado = figuras[*k].Origem, figuras[*k].Revisado
		}
		*k++
	}

	return out
}

func escreverBlocos(h interface{ Write([]byte) (int, error) }, bs []Bloco) {
	for _, b := range bs {
		fmt.Fprintf(h, "%s\x1f%s\x1f%s\x1f%s\x1f%d\x1f%s\x1e",
			b.Tipo, b.Formato, strings.Join(strings.Fields(b.Texto), " "), b.Arquivo, b.Largura,
			strings.Join(strings.Fields(b.Descricao), " "))
	}
}

// Rotulo é como o curador reconhece a prova: órgão, ano e o código do cargo
// ("TJCE 2026 · E05"); o nome por extenso não cabe numa etiqueta.
func (r Rascunho) Rotulo() string {
	return fmt.Sprintf("%s %d · %s", r.Orgao, r.Ano, cmp.Or(r.Cargo, r.CargoNome))
}

// DimensionarFiguras dá a cada figura sem tamanho escolhido a proporção que
// ela tem no caderno em relação à questão (ou ao texto). O recorte sai com até
// 2,8 vezes a resolução do PDF, e no tamanho do arquivo um diagrama pequeno
// ocupava a coluna inteira. O tamanho fica na figura, que é conteúdo: a mesma
// figura, reaproveitada noutra prova, vem do mesmo tamanho.
func (r *Rascunho) DimensionarFiguras() {
	for i := range r.Questoes {
		q := &r.Questoes[i]
		referencia := larguraDaArea(q.Origens)
		dimensionar(q.Blocos, referencia)
		for j := range q.Alternativas {
			dimensionar(q.Alternativas[j].Blocos, referencia)
		}
	}
	for i := range r.Apoios {
		dimensionar(r.Apoios[i].Blocos, larguraDaArea(r.Apoios[i].Origens))
	}
}

// Figura menor que isto, em % da coluna, fica ilegível na tela.
const larguraMinimaDaFigura = 20

func larguraDaArea(origens []Origem) float64 {
	for _, o := range origens {
		if len(o.Retangulo) == 4 && o.Retangulo[2] > o.Retangulo[0] {
			return o.Retangulo[2] - o.Retangulo[0]
		}
	}

	return 0
}

func dimensionar(bs []Bloco, referencia float64) {
	for i := range bs {
		b := &bs[i]
		if b.Tipo != "imagem" || b.Largura != 0 || referencia <= 0 || b.Origem == nil || len(b.Origem.Retangulo) != 4 {
			continue
		}
		proporcao := (b.Origem.Retangulo[2] - b.Origem.Retangulo[0]) / referencia * 100
		b.Largura = int(math.Round(min(100, max(larguraMinimaDaFigura, proporcao))))
	}
}

// Quanto do texto menor precisa estar no maior para ser a mesma questão — uma
// leitura pode perder uma frase que a outra pegou — ou quão parecidas as letras
// precisam ser: uma leitura troca "Sêneca" por "Såneca", "consequência" por
// "sequência".
const (
	contencaoDoEnunciado    = 0.9
	semelhancaDoEnunciado   = 0.85
	semelhancaDaAlternativa = 0.6
	contencaoDoTexto        = 0.9
	// Alternativas que batem quase letra por letra, e com texto bastante para
	// decidir ("certo." e "breve." não decidem), dizem sozinhas que é a mesma
	// questão; o enunciado só não pode ser outro.
	alternativaQuaseIgual               = 0.9
	palavrasParaAsAlternativasDecidirem = 10
	contencaoComAlternativasFortes      = 0.6
)

// Reaproveitar troca pela já cadastrada cada questão e cada texto da
// importação que são os mesmos de outra prova publicada do mesmo concurso — as
// Conhecimentos Gerais repetem em todos os cargos. A importação passa a só
// apontar para eles: o banco guarda uma linha, e a questão não volta à revisão.
//
// É a mesma questão mesmo com a leitura diferente: o OCR de um caderno pior
// troca "Sêneca" por "Sâneca" e perde uma frase, e a publicada o curador já
// revisou. Da importação fica o que é da prova: número, a resposta do gabarito
// dela, a posição no PDF.
func (r *Rascunho) Reaproveitar(irmas []Publicacao) {
	reaproveitadas := map[int]bool{}
	for i := range r.Questoes {
		q := &r.Questoes[i]
		// Já reaproveitada — editada, perderia a marca (InvalidarEdicoes): é a
		// de lá, e está conferida.
		if q.IgualA != "" {
			q.Revisada = true
			conferirFiguras(q.Blocos)
			for j := range q.Alternativas {
				conferirFiguras(q.Alternativas[j].Blocos)
			}
			reaproveitadas[q.Numero] = true
			continue
		}
		p, pub, ok := questaoIrma(*q, irmas)
		if !ok {
			continue
		}
		conteudo, _ := pub.Separar()
		_, lugar := q.Separar()
		lugar.Completa, lugar.Revisada = true, true
		lugar.Disciplina = cmp.Or(pub.Disciplina, lugar.Disciplina)
		lugar.IgualA = fmt.Sprintf("%s, questão %d", p.Conteudo.Rotulo(), pub.Numero)
		// O recorte é o de lá, já conferido.
		for k := range lugar.Figuras {
			lugar.Figuras[k].Revisado = true
		}
		*q = JuntarQuestao(conteudo, lugar)
		for j := range q.Alternativas {
			conferirFiguras(q.Alternativas[j].Blocos)
		}
		conferirFiguras(q.Blocos)
		reaproveitadas[q.Numero] = true
	}

	for i := range r.Apoios {
		a := &r.Apoios[i]
		if a.IgualA != "" {
			continue
		}
		p, pub, ok := apoioIrmao(*a, irmas)
		if !ok {
			continue
		}
		faixa := faixas(a.Questoes)
		conteudo, _ := pub.Separar()
		_, lugar := a.Separar()
		lugar.IgualA = p.Conteudo.Rotulo()
		lugar.Revisado = true
		*a = JuntarApoio(conteudo, lugar)
		// O alerta do OCR era sobre a leitura nova, que saiu.
		r.Alertas = slices.DeleteFunc(r.Alertas, func(al string) bool {
			return faixa != "" && strings.Contains(al, "texto de apoio das questões "+faixa+" ")
		})
	}

	// Os alertas da extração sobre a questão reaproveitada falavam da leitura
	// nova, que saiu.
	r.Alertas = slices.DeleteFunc(r.Alertas, func(al string) bool {
		for n := range reaproveitadas {
			if regexp.MustCompile(fmt.Sprintf(`(?i)\bquestão %d\b`, n)).MatchString(al) {
				return true
			}
		}
		return false
	})
}

func conferirFiguras(bs []Bloco) {
	for i := range bs {
		if bs[i].Tipo == "imagem" {
			bs[i].Revisado = true
		}
	}
}

// Parecida com outro número só quando as letras são quase as mesmas: duas
// questões diferentes com o mesmo enunciado de molde ("Considere a tabela")
// não se confundem.
const semelhancaEmOutroNumero = 0.95

// questaoIrma acha a mesma questão numa prova irmã: com o mesmo número, a
// comparação de mesmaQuestao; com outro — o outro nível do concurso numera
// diferente —, só a quase idêntica.
func questaoIrma(q Questao, irmas []Publicacao) (Publicacao, Questao, bool) {
	for _, p := range irmas {
		for _, pub := range p.Conteudo.Questoes {
			if pub.Numero == q.Numero && mesmaQuestao(q, pub) {
				return p, pub, true
			}
		}
	}
	texto := leituraDaQuestao(q)
	for _, p := range irmas {
		for _, pub := range p.Conteudo.Questoes {
			if mesmaQuestao(q, pub) && semelhanca(texto, leituraDaQuestao(pub)) >= semelhancaEmOutroNumero {
				return p, pub, true
			}
		}
	}

	return Publicacao{}, Questao{}, false
}

// MesmoOrgao compara as siglas sem espaço nem pontuação: a leitura da capa
// escreve "TRF 1", "TRF1" ou "TRF-1" para o mesmo tribunal.
func MesmoOrgao(a, b string) bool {
	chave := func(s string) string {
		var out strings.Builder
		for _, c := range strings.ToUpper(s) {
			if unicode.IsLetter(c) || unicode.IsDigit(c) {
				out.WriteRune(c)
			}
		}
		return out.String()
	}

	return a != "" && chave(a) == chave(b)
}

func apoioIrmao(a Apoio, irmas []Publicacao) (Publicacao, Apoio, bool) {
	este := palavras(textoDosBlocos(a.Blocos))
	for _, p := range irmas {
		for _, pub := range p.Conteudo.Apoios {
			outro := palavras(textoDosBlocos(pub.Blocos))
			if len(este) >= 20 && len(outro) >= 20 && contencao(este, outro) >= contencaoDoTexto {
				return p, pub, true
			}
		}
	}

	return Publicacao{}, Apoio{}, false
}

// mesmaQuestao: cada alternativa, na mesma letra, é a mesma (por letras, para
// tolerar o OCR); e o enunciado de uma contém o da outra, ou as letras são
// quase as mesmas. Com cinco alternativas de texto que batem quase letra por
// letra, o enunciado pode ter perdido um trecho na leitura: "Ocorre objeto
// direto pleonástico em:", sem a citação que vinha antes, é a mesma questão.
func mesmaQuestao(a, b Questao) bool {
	if len(a.Alternativas) != 5 || len(b.Alternativas) != 5 {
		return false
	}
	fortes, palavrasNasAlternativas := true, 0
	for k := range a.Alternativas {
		if a.Alternativas[k].Letra != b.Alternativas[k].Letra {
			return false
		}
		xa, xb := textoDosBlocos(a.Alternativas[k].Blocos), textoDosBlocos(b.Alternativas[k].Blocos)
		if normalizar(xa) == "" && normalizar(xb) == "" {
			fortes = false // alternativa só com figura: o enunciado decide
			continue
		}
		parecida := semelhanca(xa, xb)
		if !equivalentes(xa, xb) && parecida < semelhancaDaAlternativa {
			return false
		}
		fortes = fortes && (equivalentes(xa, xb) || parecida >= alternativaQuaseIgual)
		palavrasNasAlternativas += min(len(palavras(xa)), len(palavras(xb)))
	}
	fortes = fortes && palavrasNasAlternativas >= palavrasParaAsAlternativasDecidirem

	ta, tb := textoDosBlocos(a.Blocos), textoDosBlocos(b.Blocos)
	ea, eb := palavras(ta), palavras(tb)
	if min(len(ea), len(eb)) < 3 {
		return fortes && min(len(ea), len(eb)) > 0
	}
	contida := contencao(ea, eb)

	return contida >= contencaoDoEnunciado || semelhanca(ta, tb) >= semelhancaDoEnunciado ||
		(fortes && contida >= contencaoComAlternativasFortes)
}

// leituraDaQuestao junta enunciado e alternativas, com a letra de cada uma.
func leituraDaQuestao(q Questao) string {
	var s strings.Builder
	s.WriteString(textoDosBlocos(q.Blocos))
	for _, a := range q.Alternativas {
		s.WriteString(" (" + a.Letra + ") " + textoDosBlocos(a.Blocos))
	}

	return s.String()
}

// normalizar deixa só letras e números, em minúsculas, separados por um
// espaço: a pontuação e o espaço são o que duas leituras certas do mesmo
// caderno costumam ter de diferente. O acento fica — "Sêneca" e "Såneca" não
// são o mesmo texto.
func normalizar(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsDigit(c)
	}), " ")
}

func equivalentes(a, b string) bool { return normalizar(a) == normalizar(b) }

// semelhanca é o coeficiente de Dice dos trigramas de letras: tolera a letra
// trocada que o OCR deixa, e dois textos diferentes quase não têm trigrama em
// comum.
func semelhanca(a, b string) float64 {
	ta, tb := trigramas(normalizar(a)), trigramas(normalizar(b))
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	comuns := 0
	for t, n := range ta {
		comuns += min(n, tb[t])
	}
	total := 0
	for _, n := range ta {
		total += n
	}
	for _, n := range tb {
		total += n
	}

	return 2 * float64(comuns) / float64(total)
}

func trigramas(s string) map[string]int {
	rs := []rune(s)
	out := map[string]int{}
	for i := 0; i+3 <= len(rs); i++ {
		out[string(rs[i:i+3])]++
	}

	return out
}

func contarFiguras(q Questao) int {
	n := 0
	contar := func(bs []Bloco) {
		for _, b := range bs {
			if b.Tipo == "imagem" {
				n++
			}
		}
	}
	contar(q.Blocos)
	for _, a := range q.Alternativas {
		contar(a.Blocos)
	}

	return n
}

func textoDosBlocos(bs []Bloco) string {
	var s strings.Builder
	for _, b := range bs {
		if b.Tipo != "imagem" {
			s.WriteString(b.Texto)
			s.WriteByte(' ')
		}
	}

	return s.String()
}

// palavras do texto, em minúsculas, sem pontuação.
func palavras(s string) map[string]bool {
	out := map[string]bool{}
	for _, p := range strings.FieldsFunc(strings.ToLower(s), func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsDigit(c)
	}) {
		out[p] = true
	}

	return out
}

// contencao: quanto do conjunto menor está no maior.
func contencao(a, b map[string]bool) float64 {
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(a) == 0 {
		return 0
	}
	comuns := 0
	for p := range a {
		if b[p] {
			comuns++
		}
	}

	return float64(comuns) / float64(len(a))
}
