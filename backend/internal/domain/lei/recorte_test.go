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
