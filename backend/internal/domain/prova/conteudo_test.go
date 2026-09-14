package prova

import (
	"reflect"
	"strings"
	"testing"
)

func comFiguraEm(q Questao, pagina int, ret ...float64) Questao {
	q.Blocos = append(q.Blocos, Bloco{Tipo: "imagem", Arquivo: "fig", Origem: &Origem{Pagina: pagina, Retangulo: ret}, Revisado: true})
	return q
}

func TestSepararEJuntar_NaoPerdeNada(t *testing.T) {
	t.Parallel()

	q := comFiguraEm(questao(3, true, "Considere o gráfico."), 2, 100, 50, 300, 200)
	q.Alternativas[1].Blocos = []Bloco{{Tipo: "imagem", Arquivo: "alt", Origem: &Origem{Pagina: 2, Retangulo: []float64{1, 2, 3, 4}}}}
	q.Resposta, q.Situacao, q.Disciplina, q.Apoios = "C", "Gabarito sem alteração", "Estatística", []string{"t1"}
	q.Origens = []Origem{{Pagina: 2, Regiao: "1", Retangulo: []float64{0, 0, 500, 400}}}

	c, l := q.Separar()
	for _, b := range append(c.Blocos, c.Alternativas[1].Blocos...) {
		if b.Origem != nil || b.Revisado {
			t.Fatalf("o conteúdo levou a posição do PDF: %+v", b)
		}
	}
	if got := JuntarQuestao(c, l); !reflect.DeepEqual(got, q) {
		t.Fatalf("remontada = %+v\nquer %+v", got, q)
	}

	a := Apoio{ID: "t1", Blocos: []Bloco{{Tipo: "texto", Texto: "A vida"}}, Questoes: []int{1, 2}, Aviso: "Considere", Revisado: true}
	ca, la := a.Separar()
	if got := JuntarApoio(ca, la); !reflect.DeepEqual(got, a) {
		t.Fatalf("apoio remontado = %+v", got)
	}
}

// A mesma questão, de outro caderno — outra posição, outro espaço entre as
// palavras — é o mesmo conteúdo. Mudar o texto muda a impressão.
func TestImpressao(t *testing.T) {
	t.Parallel()

	a, _ := comFiguraEm(questao(3, true, "No texto, Sêneca caracteriza o presente como"), 2, 1, 2, 3, 4).Separar()
	b, _ := comFiguraEm(questao(7, true, "No  texto,\nSêneca caracteriza o presente como "), 9, 50, 60, 70, 80).Separar()
	if a.Impressao() != b.Impressao() {
		t.Fatal("posição e espaço mudaram a impressão")
	}
	c, _ := questao(3, true, "No texto, Sêneca caracteriza o passado como").Separar()
	if a.Impressao() == c.Impressao() {
		t.Fatal("texto diferente, mesma impressão")
	}
}

func TestDimensionarFiguras(t *testing.T) {
	t.Parallel()

	// A questão 11 do TJCE: a conta de quadradinhos ocupa 23% da questão.
	q := comFiguraEm(questao(11, true, "Utilize os algarismos"), 1, 251.3, 1400, 334.6, 1501)
	q.Origens = []Origem{{Pagina: 1, Retangulo: []float64{107.8, 1380, 467.4, 1531}}}
	pequena := comFiguraEm(questao(12, true, "x"), 1, 0, 0, 10, 10)
	pequena.Origens = q.Origens
	escolhida := comFiguraEm(questao(13, true, "y"), 1, 0, 0, 300, 10)
	escolhida.Origens, escolhida.Blocos[1].Largura = q.Origens, 60
	r := Rascunho{Questoes: []Questao{q, pequena, escolhida}}

	r.DimensionarFiguras()

	if got := []int{r.Questoes[0].Blocos[1].Largura, r.Questoes[1].Blocos[1].Largura, r.Questoes[2].Blocos[1].Largura}; !reflect.DeepEqual(got, []int{23, 20, 60}) {
		t.Fatalf("larguras = %v, quer [23 20 60]", got)
	}
}

// irma é a prova E05 do mesmo concurso, já publicada e revisada.
func irma(qs ...Questao) Publicacao {
	return Publicacao{ID: "e05", Conteudo: Rascunho{Orgao: "TJCE", Ano: 2026, Cargo: "E05", Questoes: qs}}
}

func TestReaproveitar_QuestaoIgualDeOutroCargo(t *testing.T) {
	t.Parallel()

	// A publicada foi corrigida pelo curador; a leitura nova tem um erro de OCR
	// só no espaço, e a figura noutra posição (outro PDF).
	publicada := comFiguraEm(questao(3, true, "No texto, Sêneca caracteriza o presente como"), 1, 10, 20, 30, 40)
	publicada.Blocos[1].Largura, publicada.Disciplina, publicada.Resposta = 40, "Língua Portuguesa", "C"
	lida := comFiguraEm(questao(3, false, "No  texto, Sêneca caracteriza o presente como"), 2, 500, 600, 700, 800)
	lida.Blocos[1].Arquivo, lida.Blocos[1].Revisado = "outro-recorte", false
	lida.Resposta, lida.Disciplina, lida.Revisada = "C", "CONHECIMENTOS GERAIS", false
	r := Rascunho{Questoes: []Questao{lida}}

	r.Reaproveitar([]Publicacao{irma(publicada)})

	q := r.Questoes[0]
	if q.IgualA != "TJCE 2026 · E05, questão 3" || q.Blocos[0].Texto != publicada.Blocos[0].Texto || !q.Completa {
		t.Fatalf("não reaproveitou: %+v", q)
	}
	if f := q.Blocos[1]; f.Arquivo != "fig" || f.Largura != 40 || f.Origem.Pagina != 2 || !f.Revisado {
		t.Fatalf("figura = %+v; quer o recorte publicado, já conferido, na posição deste PDF", f)
	}
	// Referência à já cadastrada: nada a conferir.
	if q.Disciplina != "Língua Portuguesa" || q.Resposta != "C" || !q.Revisada {
		t.Fatalf("lugar = %+v", q)
	}
	if len(r.Alertas) != 0 {
		t.Fatalf("leitura idêntica virou alerta: %v", r.Alertas)
	}
	// E o conteúdo é o mesmo da publicada: o banco guarda uma linha só.
	ca, _ := q.Separar()
	cb, _ := publicada.Separar()
	if ca.Impressao() != cb.Impressao() {
		t.Fatal("reaproveitada com impressão diferente da publicada")
	}
}

// O caderno pior perdeu uma frase e trocou letras: é a mesma questão, e fica
// a publicada, que o curador revisou — conferida, sem alerta, e os alertas da
// extração sobre ela saem junto com a leitura.
func TestReaproveitar_LeituraPiorFicaComAPublicada(t *testing.T) {
	t.Parallel()

	publicada := questao(11, true, "Utilize os algarismos 0, 1, 2, 3, 4, 5 e 6, cada um apenas uma vez. "+
		"O algarismo das unidades da soma, representado por ?, é")
	publicada.Alternativas[2].Blocos[0].Texto = "consequência."
	lida := questao(11, true, "Utilize os algarismos 0, 1, 2, 3, 4, 5 e 6, cada um apenas uma vez")
	lida.Alternativas[2].Blocos[0].Texto = "sequência."
	lida.Revisada = false
	r := Rascunho{
		Questoes: []Questao{lida},
		Alertas: []string{
			"Questão 11 foi lida duas vezes com conteúdo diferente (regiões 1 e 2).",
			"Questão 12 foi lida duas vezes com conteúdo diferente (regiões 2 e 3).",
		},
	}

	r.Reaproveitar([]Publicacao{irma(publicada)})

	q := r.Questoes[0]
	if q.IgualA != "TJCE 2026 · E05, questão 11" || !strings.Contains(q.Blocos[0].Texto, "unidades da soma") ||
		q.Alternativas[2].Blocos[0].Texto != "consequência." || !q.Revisada {
		t.Fatalf("questão = %+v; quer a publicada, conferida", q)
	}
	if len(r.Alertas) != 1 || !strings.Contains(r.Alertas[0], "Questão 12") {
		t.Fatalf("alertas = %v; quer só o da 12", r.Alertas)
	}
}

// Outro nível do concurso numera diferente: a quase idêntica vale com outro
// número; a de enunciado de molde, não.
func TestReaproveitar_OutroNumero(t *testing.T) {
	t.Parallel()

	longo := "Em um Tribunal Regional Federal, a equipe de infraestrutura precisa garantir alta disponibilidade"
	publicada := questao(31, true, longo)
	lida := questao(7, true, strings.Replace(longo, "Federal", "Fedcral", 1))
	molde := questao(8, true, "Considere a tabela abaixo.")
	r := Rascunho{Questoes: []Questao{lida, molde}}

	r.Reaproveitar([]Publicacao{irma(publicada, questao(40, true, "Considere a tabela abaixo sobre vendas."))})

	if r.Questoes[0].IgualA != "TJCE 2026 · E05, questão 31" || r.Questoes[0].Numero != 7 {
		t.Fatalf("questão 7 = %+v", r.Questoes[0])
	}
	if r.Questoes[1].IgualA != "" {
		t.Fatalf("enunciado de molde foi reaproveitado: %+v", r.Questoes[1])
	}
}

func TestMesmoOrgao(t *testing.T) {
	t.Parallel()

	for _, par := range [][2]string{{"TRF 1", "TRF1"}, {"trf-1", "TRF1"}, {"TJCE", "TJ CE"}} {
		if !MesmoOrgao(par[0], par[1]) {
			t.Errorf("%q e %q deviam ser o mesmo órgão", par[0], par[1])
		}
	}
	if MesmoOrgao("TRF1", "TRF2") || MesmoOrgao("", "") {
		t.Error("órgãos diferentes, ou vazios, deram o mesmo")
	}
}

func TestReaproveitar_QuestaoDiferenteFica(t *testing.T) {
	t.Parallel()

	publicada := questao(21, true, "Considere o banco de dados SQL Server de um Tribunal")
	lida := questao(21, true, "Considere o banco de dados SQL Server de um Tribunal")
	for i := range lida.Alternativas {
		lida.Alternativas[i].Blocos[0].Texto = "outra resposta " + lida.Alternativas[i].Letra + " totalmente diferente"
	}
	r := Rascunho{Questoes: []Questao{lida}}

	r.Reaproveitar([]Publicacao{irma(publicada)})

	if r.Questoes[0].IgualA != "" || len(r.Alertas) != 0 {
		t.Fatalf("alternativas diferentes, e reaproveitou ou alertou: %+v %v", r.Questoes[0], r.Alertas)
	}
}

func TestReaproveitar_TextoDeApoio(t *testing.T) {
	t.Parallel()

	sêneca := "A vida divide-se em três períodos: o que se foi, o que está sendo e o que há de vir. Desses, o que " +
		"estamos atravessando é breve, o que havemos de atravessar é duvidoso, o que já atravessamos é certo. "
	publicado := Apoio{ID: "r1-t1", Blocos: []Bloco{{Tipo: "texto", Texto: sêneca + "(Adaptado de: Sêneca.)"}}, Questoes: []int{1, 2}, Revisado: true}
	lido := Apoio{ID: "r1-ocr1", Blocos: []Bloco{{Tipo: "texto", Texto: sêneca + "(Adaptado de: Sêneca)"}}, Questoes: []int{1, 2}}
	r := Rascunho{
		Apoios:  []Apoio{lido},
		Alertas: []string{"O texto de apoio das questões 1-2 foi transcrito por OCR, porque a IA se recusou; confira."},
	}
	p := irma()
	p.Conteudo.Apoios = []Apoio{publicado}

	r.Reaproveitar([]Publicacao{p})

	a := r.Apoios[0]
	if a.ID != "r1-ocr1" || a.Blocos[0].Texto != publicado.Blocos[0].Texto || a.IgualA == "" || !a.Revisado {
		t.Fatalf("texto = %+v", a)
	}
	if len(r.Alertas) != 0 {
		t.Fatalf("o alerta do OCR ficou: %v", r.Alertas)
	}
}

// O caderno pior perdeu a citação do enunciado da 8 — sobrou "Ocorre objeto
// direto pleonástico em:" —, mas as cinco alternativas batem: é a mesma questão.
func TestReaproveitar_EnunciadoMalLidoComAlternativasIguais(t *testing.T) {
	t.Parallel()

	alternativas := []string{
		"quem submete todos os seus atos à própria censura", "isso os ocupados não têm tempo de fazer",
		"não lhes sobra tempo para examinar o passado", "o que havemos de atravessar é duvidoso",
		"Esse período os ocupados o perdem",
	}
	montar := func(enunciado string) Questao {
		q := questao(8, true, enunciado)
		for k := range q.Alternativas {
			q.Alternativas[k].Blocos[0].Texto = alternativas[k]
		}
		return q
	}
	publicada := montar("Quando se quer chamar a atenção para o objeto direto que precede o verbo, costuma-se " +
		"repeti-lo. É o que se chama objeto direto pleonástico. Ocorre objeto direto pleonástico em:")
	lida := montar("Ocorre objeto direto pleonástico em:")
	lida.Alternativas[1].Blocos[0].Texto = "isso os ocupados não tém tempo de fazer"
	r := Rascunho{Questoes: []Questao{lida}}

	r.Reaproveitar([]Publicacao{irma(publicada)})

	if r.Questoes[0].IgualA == "" || !strings.Contains(r.Questoes[0].Blocos[0].Texto, "chamar a atenção") {
		t.Fatalf("questão 8 = %+v", r.Questoes[0])
	}
}
