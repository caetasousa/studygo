package lei

import (
	"slices"
	"testing"
)

// Como a leitura do edital pode errar o recorte — escrito antes do código. O
// edital raramente cita artigo: ele nomeia o assunto ("Administração Pública;
// fiscalização contábil…"), e o assunto é o título de uma divisão da lei.
//
//	R1  tópico que só nomeia a lei, sem delimitar, vira um recorte — e esconde a lei que cai inteira
//	R2  o assunto não acha a divisão cujo título ele repete ("Administração Pública" → Capítulo VII)
//	R3  título genérico ("DISPOSIÇÕES GERAIS", repetido em cada capítulo) casa com qualquer tópico
//	R4  título que o tópico só cita em parte ("Da Proteção aos Registros, aos Dados Pessoais…" × "proteção de dados") entra
//	R5  artigo citado ("arts. 37 a 43", "art. 5º", "artigos 1º ao 17", "arts. 70, 71 e 74") não entra
//	R6  artigo citado que a lei não tem trava a leitura ou vira ref fantasma
//	R7  divisão dentro de outra já no recorte aparece duas vezes
//	R8  acento e caixa impedem o casamento
//	R9  tópico de outra lei (a matéria cita várias) entra no recorte desta

func leiDeRecorte() (Lei, []Dispositivo) {
	l := Lei{Slug: "cf", Nome: "Constituição da República Federativa do Brasil de 1988", Reconhecer: []string{"Constituição da República Federativa do Brasil"}}
	ds := []Dispositivo{
		{Ref: "tit3", Tipo: "titulo", Rotulo: "TÍTULO III", Nome: "DA ORGANIZAÇÃO DO ESTADO"},
		{Ref: "tit3.cap7", Pai: "tit3", Tipo: "capitulo", Rotulo: "CAPÍTULO VII", Nome: "DA ADMINISTRAÇÃO PÚBLICA"},
		{Ref: "tit3.cap7.sec1", Pai: "tit3.cap7", Tipo: "secao", Rotulo: "Seção I", Nome: "DISPOSIÇÕES GERAIS"},
		{Ref: "art37", Pai: "tit3.cap7.sec1", Tipo: "artigo", Rotulo: "Art. 37"},
		{Ref: "art38", Pai: "tit3.cap7.sec1", Tipo: "artigo", Rotulo: "Art. 38"},
		{Ref: "tit4", Tipo: "titulo", Rotulo: "TÍTULO IV", Nome: "DA ORGANIZAÇÃO DOS PODERES"},
		{Ref: "tit4.cap1", Pai: "tit4", Tipo: "capitulo", Rotulo: "CAPÍTULO I", Nome: "DO PODER LEGISLATIVO"},
		{Ref: "tit4.cap1.sec1", Pai: "tit4.cap1", Tipo: "secao", Rotulo: "Seção I", Nome: "DISPOSIÇÕES GERAIS"},
		{Ref: "art44", Pai: "tit4.cap1.sec1", Tipo: "artigo", Rotulo: "Art. 44"},
		{Ref: "tit4.cap1.sec9", Pai: "tit4.cap1", Tipo: "secao", Rotulo: "Seção IX", Nome: "DA FISCALIZAÇÃO CONTÁBIL, FINANCEIRA E ORÇAMENTÁRIA"},
		{Ref: "art70", Pai: "tit4.cap1.sec9", Tipo: "artigo", Rotulo: "Art. 70"},
		{Ref: "art71", Pai: "tit4.cap1.sec9", Tipo: "artigo", Rotulo: "Art. 71"},
		{Ref: "art71.inc1", Pai: "art71", Tipo: "inciso", Rotulo: "I"},
		{Ref: "art74", Pai: "tit4.cap1.sec9", Tipo: "artigo", Rotulo: "Art. 74"},
		{Ref: "tit4.cap1.sec10", Pai: "tit4.cap1", Tipo: "secao", Rotulo: "Seção X", Nome: "DA PROTEÇÃO AOS REGISTROS, AOS DADOS PESSOAIS E ÀS COMUNICAÇÕES PRIVADAS"},
		{Ref: "art75", Pai: "tit4.cap1.sec10", Tipo: "artigo", Rotulo: "Art. 75"},
	}

	return l, ds
}

func TestRecorte(t *testing.T) {
	l, ds := leiDeRecorte()
	casos := []struct {
		id    string
		temas []string
		quer  []string
	}{
		{"R1", []string{"Constituição da República Federativa do Brasil de 1988"}, nil},
		{"R1 (com parênteses)", []string{"Lei Orgânica (Constituição da República Federativa do Brasil)"}, nil},
		{"R2", []string{"Constituição da República Federativa do Brasil de 1988: Administração Pública; fiscalização contábil, financeira, orçamentária, operacional e patrimonial; controle interno e controle externo"},
			[]string{"tit3.cap7", "tit4.cap1.sec9"}},
		{"R3", []string{"Constituição da República Federativa do Brasil: disposições gerais"}, nil},
		{"R4", []string{"Constituição da República Federativa do Brasil: proteção de dados"}, nil},
		{"R5 faixa", []string{"Constituição da República Federativa do Brasil, arts. 37 a 38"}, []string{"art37", "art38"}},
		{"R5 lista", []string{"Constituição da República Federativa do Brasil (arts. 70, 71 e 74)"}, []string{"art70", "art71", "art74"}},
		{"R5 ordinal", []string{"Constituição da República Federativa do Brasil: art. 44º"}, []string{"art44"}},
		{"R5 artigos ao", []string{"Constituição da República Federativa do Brasil, artigos 70 ao 71"}, []string{"art70", "art71"}},
		{"R6", []string{"Constituição da República Federativa do Brasil: arts. 999 e 70"}, []string{"art70"}},
		{"R7", []string{"Constituição da República Federativa do Brasil: Administração Pública; art. 37"}, []string{"tit3.cap7"}},
		{"R8", []string{"CONSTITUICAO DA REPUBLICA FEDERATIVA DO BRASIL: administracao publica"}, []string{"tit3.cap7"}},
		{"R9", []string{"Lei nº 8.666/1993: Administração Pública", "Constituição da República Federativa do Brasil: fiscalização contábil, financeira e orçamentária"},
			[]string{"tit4.cap1.sec9"}},
		{"R9 (nenhum tópico cita)", []string{"Lei nº 8.666/1993: Administração Pública"}, nil},
	}
	for _, c := range casos {
		if got := RecorteDoEdital(l, c.temas, ds); !slices.Equal(got, c.quer) {
			t.Errorf("%s: recorte %v, queria %v", c.id, got, c.quer)
		}
	}
}

func TestRecorte_UmTopicoQueNaoDelimitaLiberaALeiInteira(t *testing.T) {
	l, ds := leiDeRecorte()
	// Dois tópicos citam a lei; um deles a pede inteira: a matéria cobra tudo.
	temas := []string{
		"Constituição da República Federativa do Brasil: Administração Pública",
		"Constituição da República Federativa do Brasil de 1988",
	}
	if got := RecorteDoEdital(l, temas, ds); got != nil {
		t.Fatalf("recorte %v, queria a lei inteira", got)
	}
}

func TestDescreverRecorte(t *testing.T) {
	_, ds := leiDeRecorte()
	got := DescreverRecorte(ds, []string{"tit4.cap1.sec9", "art37"})
	quer := []TrechoDoRecorte{
		{Ref: "art37", Rotulo: "Art. 37", Artigos: "art. 37"},
		{Ref: "tit4.cap1.sec9", Rotulo: "Seção IX", Nome: "DA FISCALIZAÇÃO CONTÁBIL, FINANCEIRA E ORÇAMENTÁRIA", Artigos: "arts. 70 a 74"},
	}
	if !slices.Equal(got, quer) {
		t.Fatalf("descrição %+v, queria %+v", got, quer)
	}
}

// Como a leitura do tópico, assunto por assunto, pode errar — escrito antes do
// código. É o que a pesquisa mostra antes de importar.
//
//	T1  o pedaço que só nomeia a lei aparece como assunto sem divisão (ruído)
//	T2  assunto que não casa some da lista, e a pessoa não vê que ficou de fora
//	T3  artigo citado no pedaço que nomeia a lei ("Constituição Federal, arts. 37 a 43") é perdido
//	T4  o recorte final repete divisões ou traz uma dentro da outra
func TestLerTema(t *testing.T) {
	_, ds := leiDeRecorte()
	ds = append([]Dispositivo{{Ref: "preambulo1", Tipo: "preambulo", Texto: "CONSTITUIÇÃO DA REPÚBLICA FEDERATIVA DO BRASIL DE 1988"}}, ds...)
	tema := "Constituição da República Federativa do Brasil de 1988: Administração Pública; " +
		"fiscalização contábil, financeira, orçamentária, operacional e patrimonial; controle interno e controle externo"

	assuntos, recorte := LerTema(tema, ds)
	quer := []Assunto{
		{Texto: "Administração Pública", Refs: []string{"tit3.cap7"}},
		{Texto: "fiscalização contábil, financeira, orçamentária, operacional e patrimonial", Refs: []string{"tit4.cap1.sec9"}},
		{Texto: "controle interno e controle externo"}, // T2: fica, sem divisão
	}
	if len(assuntos) != len(quer) {
		t.Fatalf("assuntos %+v, queria %+v", assuntos, quer)
	}
	for i := range quer {
		if assuntos[i].Texto != quer[i].Texto || !slices.Equal(assuntos[i].Refs, quer[i].Refs) {
			t.Errorf("assunto %d: %+v, queria %+v", i, assuntos[i], quer[i])
		}
	}
	if !slices.Equal(recorte, []string{"tit3.cap7", "tit4.cap1.sec9"}) {
		t.Errorf("recorte %v", recorte)
	}

	// T3 e T4: artigos no pedaço que nomeia a lei, e um deles dentro de divisão já pedida.
	assuntos, recorte = LerTema("Constituição da República Federativa do Brasil de 1988, arts. 37 e 70: Administração Pública", ds)
	if len(assuntos) != 2 || !slices.Equal(assuntos[0].Refs, []string{"art37", "art70"}) {
		t.Errorf("artigos do pedaço que nomeia a lei: %+v", assuntos)
	}
	if !slices.Equal(recorte, []string{"tit3.cap7", "art70"}) {
		t.Errorf("recorte sem repetir o que está dentro: %v", recorte)
	}

	// T1: tópico que só nomeia a lei não tem assunto — é a lei inteira.
	if assuntos, recorte = LerTema("Constituição da República Federativa do Brasil de 1988", ds); len(assuntos) != 0 || recorte != nil {
		t.Errorf("só o nome da lei: %+v %v", assuntos, recorte)
	}
}

func TestCurtoDe(t *testing.T) {
	for epigrafe, quer := range map[string]string{
		"CONSTITUIÇÃO DA REPÚBLICA FEDERATIVA DO BRASIL DE 1988": "Constituição Federal",
		"LEI Nº 13.709, DE 14 DE AGOSTO DE 2018":                 "Lei nº 13.709/2018",
		"LEI COMPLEMENTAR Nº 205, DE 19 DE MAIO DE 2025":         "LC nº 205/2025",
		"RESOLUÇÃO Nº 22":                                        "RESOLUÇÃO Nº 22",
	} {
		if got := CurtoDe(epigrafe); got != quer {
			t.Errorf("CurtoDe(%q) = %q, queria %q", epigrafe, got, quer)
		}
	}
}

func TestArtigosCitados(t *testing.T) {
	for texto, quer := range map[string][]string{
		"arts. 74 e 75":       {"art74", "art75"},
		"37 a 39":             {"art37", "art38", "art39"},
		"art. 5º, 7º":         {"art5", "art7"},
		"":                    nil,
		"nada de artigo aqui": nil,
	} {
		if got := ArtigosCitados(texto); !slices.Equal(got, quer) {
			t.Errorf("ArtigosCitados(%q) = %v, queria %v", texto, got, quer)
		}
	}
}
