package prova

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func questao(numero int, completa bool, texto string) Questao {
	q := Questao{
		Numero: numero, Completa: completa, Revisada: true,
		Blocos:  []Bloco{{Tipo: "texto", Texto: texto}},
		Origens: []Origem{{Pagina: 1, Regiao: "0"}},
	}
	for _, l := range []string{"A", "B", "C", "D", "E"} {
		q.Alternativas = append(q.Alternativas, Alternativa{Letra: l, Blocos: []Bloco{{Tipo: "texto", Texto: l}}})
	}

	return q
}

// valida é um rascunho publicável: identificação completa, uma questão
// conferida e o gabarito do mesmo cargo e caderno.
func valida() Rascunho {
	q := questao(1, true, "Enunciado")
	q.Resposta = "E"

	return Rascunho{
		Banca: "FCC", Orgao: "TJCE", Ano: 2026, Cargo: "E05", Caderno: "004", Total: 1,
		Questoes: []Questao{q},
		Gabarito: Gabarito{Cargo: "E05", Caderno: "4", Tipo: "preliminar", Respostas: map[string]string{"1": "E"}},
	}
}

func contem(pendencias []string, trecho string) bool {
	return strings.Contains(strings.Join(pendencias, "\n"), trecho)
}

var publicar = Criterios{Conferencia: true, Gabarito: true}

func motivos(fora []DeFora) string {
	var sb strings.Builder
	for _, f := range fora {
		fmt.Fprintf(&sb, "%d: %s\n", f.Numero, f.Motivo)
	}

	return sb.String()
}

func TestPendencias_RascunhoValidoPublica(t *testing.T) {
	t.Parallel()

	r := valida()
	pub, fora := r.ParaPublicar(publicar)
	if p := r.Pendencias(publicar); len(p) > 0 || len(pub.Questoes) != 1 || len(fora) > 0 || len(r.Avisos()) > 0 {
		t.Fatalf("rascunho válido: pendências %v, %d publicáveis, de fora %v, avisos %v", p, len(pub.Questoes), fora, r.Avisos())
	}
}

// O caso do MPEAL: 56 questões conferidas e quatro que a extração não achou.
// As prontas vão ao catálogo; as outras ficam de fora, sem travar nada, e
// entram nas excluídas da revisão publicada.
func TestParaPublicar_PublicaAsProntasEDeixaAsOutrasDeFora(t *testing.T) {
	t.Parallel()

	r := valida()
	r.Total = 4
	dois, tres := questao(2, true, "Segunda"), questao(3, true, "Terceira")
	tres.Revisada = false
	r.Questoes = append(r.Questoes, dois, tres)
	r.Gabarito.Respostas = map[string]string{"1": "E", "2": "B", "3": "C", "4": "D"}
	r.Apoios = []Apoio{
		{ID: "t1", Blocos: []Bloco{{Tipo: "texto", Texto: "A vida"}}, Questoes: []int{2, 3}, Revisado: true},
		{ID: "t2", Blocos: []Bloco{{Tipo: "texto", Texto: "Só da 3"}}, Questoes: []int{3}, Revisado: true},
	}
	r.AcertarApoios()

	pub, fora := r.ParaPublicar(publicar)

	if p := r.Pendencias(publicar); len(p) > 0 {
		t.Fatalf("pendências = %v; quer publicar as prontas", p)
	}
	numeros := []int{}
	for _, q := range pub.Questoes {
		numeros = append(numeros, q.Numero)
	}
	if !slices.Equal(numeros, []int{1, 2}) || pub.Questoes[1].Resposta != "B" {
		t.Fatalf("publicadas = %v (resposta da 2: %q)", numeros, pub.Questoes[1].Resposta)
	}
	if got := motivos(fora); got != "3: não foi conferida\n4: não está no rascunho\n" {
		t.Fatalf("de fora:\n%s", got)
	}
	if !slices.Equal(pub.Excluidas, []int{3, 4}) || pub.QuestoesNaProva() != 2 {
		t.Fatalf("excluídas %v, %d na prova", pub.Excluidas, pub.QuestoesNaProva())
	}
	if len(pub.Apoios) != 1 || !slices.Equal(pub.Apoios[0].Questoes, []int{2}) {
		t.Fatalf("textos publicados = %+v; quer só o t1, da 2", pub.Apoios)
	}
	// O rascunho não muda: a revisão continua com a 3 para conferir.
	if len(r.Questoes) != 3 || len(r.Apoios[0].Questoes) != 2 {
		t.Fatal("ParaPublicar mexeu no rascunho")
	}
}

// Cada caso deixa a única questão de fora, com o motivo dito.
func TestParaPublicar_Motivos(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome   string
		mudar  func(*Rascunho)
		motivo string
	}{
		{"não conferida", func(r *Rascunho) { r.Questoes[0].Revisada = false }, "não foi conferida"},
		{"sem resposta no gabarito", func(r *Rascunho) { r.Gabarito.Respostas = map[string]string{"2": "A"} },
			"não tem resposta no gabarito"},
		{"letra que o banco recusa", func(r *Rascunho) { r.Gabarito.Respostas["1"] = "F" }, "não tem resposta no gabarito"},
		{"alternativa faltando", func(r *Rascunho) {
			r.Questoes[0].Alternativas = r.Questoes[0].Alternativas[:4]
		}, "está incompleta"},
		{"fragmento não confirmado", func(r *Rascunho) { r.Questoes[0].Completa = false }, "está incompleta"},
		{"imagem sem recorte", func(r *Rascunho) {
			r.Questoes[0].Blocos = append(r.Questoes[0].Blocos, Bloco{Tipo: "imagem"})
		}, "o enunciado tem figura sem recorte"},
		{"recorte não conferido", func(r *Rascunho) {
			r.Questoes[0].Blocos = append(r.Questoes[0].Blocos,
				Bloco{Tipo: "imagem", Arquivo: "x", Origem: &Origem{Pagina: 1}})
		}, "o enunciado tem recorte não conferido"},
		{"figura maior que a coluna", func(r *Rascunho) {
			r.Questoes[0].Blocos = append(r.Questoes[0].Blocos,
				Bloco{Tipo: "imagem", Arquivo: "x", Origem: &Origem{Pagina: 1}, Revisado: true, Largura: 140})
		}, "tamanho fora de 0 a 100%"},
		{"apoio inexistente", func(r *Rascunho) { r.Questoes[0].Apoios = []string{"t1"} }, "não existe mais"},
		{"texto não conferido", func(r *Rascunho) {
			r.Apoios = []Apoio{{ID: "t1", Blocos: []Bloco{{Tipo: "texto", Texto: "A vida"}}, Questoes: []int{1}}}
			r.Questoes[0].Apoios = []string{"t1"}
		}, "usa o texto de apoio das questões 1, que não foi conferido"},
		{"número fora do caderno", func(r *Rascunho) { r.Questoes[0].Numero = 7 }, "fora da numeração do caderno (1 a 1)"},
		{"número repetido", func(r *Rascunho) { r.Questoes = append(r.Questoes, r.Questoes[0]) }, "repetido"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			r := valida()
			c.mudar(&r)
			pub, fora := r.ParaPublicar(publicar)
			if len(pub.Questoes) != 0 || !strings.Contains(motivos(fora), c.motivo) {
				t.Fatalf("%d publicadas, de fora:\n%squer o motivo %q", len(pub.Questoes), motivos(fora), c.motivo)
			}
			if p := r.Pendencias(publicar); len(p) != 1 {
				t.Fatalf("sem nenhuma publicável, a pendência é uma: %v", p)
			}
		})
	}
}

func TestPendencias_SemQuestaoOuSemGabarito(t *testing.T) {
	t.Parallel()

	if p := (Rascunho{}).Pendencias(publicar); !contem(p, "não tem questões") {
		t.Fatalf("rascunho vazio: %v", p)
	}
	r := valida()
	r.Gabarito = Gabarito{}
	if p := r.Pendencias(publicar); !contem(p, "Sem gabarito, nenhuma questão tem resposta") {
		t.Fatalf("sem gabarito: %v", p)
	}
	// A prova de pacote não pede gabarito.
	if p := r.Pendencias(Criterios{}); len(p) > 0 {
		t.Fatalf("pacote sem gabarito: %v", p)
	}
}

// Sem a exigência de conferência, a questão não conferida vai, mas a figura
// sem recorte continua de fora.
func TestParaPublicar_SemExigirConferencia(t *testing.T) {
	t.Parallel()

	semConferir := Criterios{Gabarito: true}
	r := valida()
	r.Questoes[0].Revisada = false
	r.Questoes[0].Blocos = append(r.Questoes[0].Blocos,
		Bloco{Tipo: "imagem", Arquivo: "x", Origem: &Origem{Pagina: 1}})
	if pub, fora := r.ParaPublicar(semConferir); len(pub.Questoes) != 1 {
		t.Fatalf("conferência deixou de fora sem ser exigida: %v", fora)
	}

	r.Questoes[0].Blocos = append(r.Questoes[0].Blocos, Bloco{Tipo: "imagem"})
	if _, fora := r.ParaPublicar(semConferir); !strings.Contains(motivos(fora), "figura sem recorte") {
		t.Fatalf("figura sem recorte publicou: %v", fora)
	}
}

// O gabarito em tabela escreve "01": é a questão 1.
func TestParaPublicar_GabaritoComZeroAEsquerda(t *testing.T) {
	t.Parallel()

	r := valida()
	r.Gabarito.Respostas = map[string]string{"01": "E"}
	r.Gabarito.Situacoes = map[string]string{"01": "Gabarito sem alteração"}
	pub, fora := r.ParaPublicar(publicar)
	if len(pub.Questoes) != 1 || pub.Gabarito.Respostas["1"] != "E" || pub.Gabarito.Situacoes["1"] == "" {
		t.Fatalf("publicadas %d, gabarito %+v, de fora %v", len(pub.Questoes), pub.Gabarito, fora)
	}
}

// O que não impede publicar, mas vale conferir.
func TestAvisos(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome   string
		mudar  func(*Rascunho)
		trecho string
	}{
		{"caderno do gabarito é outro", func(r *Rascunho) { r.Gabarito.Caderno = "3" }, "tipo de gabarito"},
		{"caderno com prefixo é outro", func(r *Rascunho) { r.Caderno = "TIPO-003" }, "tipo de gabarito"},
		{"cargo do gabarito é outro", func(r *Rascunho) { r.Gabarito.Cargo = "E04" },
			`O gabarito é do cargo E04 e a prova está com o cargo "E05"`},
		// A leitura da capa às vezes traz o nome no lugar do código: o aviso
		// diz onde achar o código.
		{"nome no lugar do código", func(r *Rascunho) { r.Cargo = "Analista Judiciário – Sistemas da Informação" },
			`Se a capa diz "Caderno de Prova 'E05'", use esse código na etapa Dados`},
		{"prova sem código", func(r *Rascunho) { r.Cargo = "" }, "a prova está sem o código do cargo"},
		{"banca de outra", func(r *Rascunho) { r.Banca = "Cebraspe" }, "banca FCC"},
		{"gabarito maior que o caderno", func(r *Rascunho) { r.Gabarito.Respostas["2"] = "A" },
			"O gabarito tem 2 respostas e o caderno, 1 questões"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			r := valida()
			c.mudar(&r)
			if a := r.Avisos(); !contem(a, c.trecho) {
				t.Fatalf("avisos %v não mencionam %q", a, c.trecho)
			}
			if p := r.Pendencias(publicar); len(p) > 0 {
				t.Fatalf("aviso virou pendência: %v", p)
			}
		})
	}
}

// O gabarito cita só o código; a prova pode tê-lo junto do nome.
func TestAvisos_CodigoDoCargoJuntoDoNome(t *testing.T) {
	t.Parallel()

	for _, cargo := range []string{"e05", "E05 - Analista Judiciário", "Analista Judiciário (E05)"} {
		r := valida()
		r.Cargo = cargo
		if a := r.Avisos(); len(a) > 0 {
			t.Errorf("cargo %q: %v", cargo, a)
		}
	}
	r := valida()
	r.Cargo = "E050"
	if a := r.Avisos(); len(a) != 1 {
		t.Errorf("E050 não é E05, e passou: %v", a)
	}
}

// A capa escreve o caderno de um jeito e o gabarito de outro.
func TestAvisos_CadernoPorDigitos(t *testing.T) {
	t.Parallel()

	for _, caderno := range []string{"004", "TIPO-004", "Tipo 4"} {
		r := valida()
		r.Caderno = caderno
		if a := r.Avisos(); len(a) > 0 {
			t.Errorf("caderno %q contra gabarito 4: %v", caderno, a)
		}
	}
}

func TestInvalidarEdicoes_ConferirSemAlterarVale(t *testing.T) {
	t.Parallel()

	antigo := valida()
	antigo.Questoes[0].Revisada = false
	novo := valida() // mesma questão, agora marcada

	InvalidarEdicoes(antigo, &novo)

	if !novo.Questoes[0].Revisada {
		t.Fatal("conferência de conteúdo já salvo foi desfeita")
	}
}

func TestInvalidarEdicoes_AlterarDesfazConferencia(t *testing.T) {
	t.Parallel()

	novo := valida()
	novo.Questoes[0].Disciplina = "Português"

	InvalidarEdicoes(valida(), &novo)

	if novo.Questoes[0].Revisada {
		t.Fatal("questão alterada e conferida na mesma gravação")
	}
}

// O navegador devolve [] onde o banco guardou null. Isso não é edição, e
// tratá-lo como edição impediria o curador de conferir a questão para sempre.
func TestInvalidarEdicoes_ListaVaziaNaoEEdicao(t *testing.T) {
	t.Parallel()

	antigo := valida()
	antigo.Questoes[0].Apoios = nil
	novo := valida()
	novo.Questoes[0].Apoios = []string{}

	InvalidarEdicoes(antigo, &novo)

	if !novo.Questoes[0].Revisada {
		t.Fatal("[] contra null contou como edição")
	}
}

// Marcar o recorte como conferido não é editar a questão: as duas marcas podem
// ir na mesma gravação.
func TestInvalidarEdicoes_MarcarRecorteNaoDesfazQuestao(t *testing.T) {
	t.Parallel()

	antigo := valida()
	antigo.Questoes[0].Blocos = append(antigo.Questoes[0].Blocos,
		Bloco{Tipo: "imagem", Arquivo: "fig", Origem: &Origem{Pagina: 1}})
	novo := valida()
	novo.Questoes[0].Blocos = append(novo.Questoes[0].Blocos,
		Bloco{Tipo: "imagem", Arquivo: "fig", Origem: &Origem{Pagina: 1}, Revisado: true})

	InvalidarEdicoes(antigo, &novo)

	if !novo.Questoes[0].Revisada || !novo.Questoes[0].Blocos[1].Revisado {
		t.Fatal("conferir o recorte desfez a conferência")
	}
}

func TestInvalidarEdicoes_RecorteNovoChegaSemConferencia(t *testing.T) {
	t.Parallel()

	novo := valida()
	novo.Questoes[0].Blocos = append(novo.Questoes[0].Blocos,
		Bloco{Tipo: "imagem", Arquivo: "novo", Origem: &Origem{Pagina: 1}, Revisado: true})

	InvalidarEdicoes(valida(), &novo)

	if novo.Questoes[0].Blocos[1].Revisado {
		t.Fatal("recorte novo aceito como conferido na mesma gravação")
	}
}

func TestMesclar_FragmentosSeJuntam(t *testing.T) {
	t.Parallel()

	r := Rascunho{Questoes: []Questao{questao(1, false, "início")}}
	fim := questao(1, false, "fim")
	fim.Origens = []Origem{{Pagina: 1, Regiao: "1"}}

	r.Mesclar(Rascunho{Questoes: []Questao{fim}})

	q := r.Questoes[0]
	if len(r.Questoes) != 1 || len(q.Blocos) != 2 || q.Completa {
		t.Fatalf("fragmentos mal mesclados: %+v", r.Questoes)
	}
	if len(q.Origens) != 2 {
		t.Fatalf("origens = %v, quer as duas regiões", q.Origens)
	}
}

func TestMesclar_CompletaVenceFragmento(t *testing.T) {
	t.Parallel()

	r := Rascunho{Questoes: []Questao{questao(1, false, "pedaço")}}
	inteira := questao(1, true, "inteira")
	inteira.Origens = []Origem{{Pagina: 1, Regiao: "1"}}

	r.Mesclar(Rascunho{Questoes: []Questao{inteira}})

	q := r.Questoes[0]
	if !q.Completa || q.Blocos[0].Texto != "inteira" {
		t.Fatalf("questão = %+v", q)
	}
	// A leitura inteira já mostra a questão toda; o pedaço não soma origem.
	if len(q.Origens) != 1 || q.Origens[0].Regiao != "1" {
		t.Fatalf("origens = %v, quer só a da leitura inteira", q.Origens)
	}
}

// A sobreposição entrega a mesma questão duas vezes. Leituras que só diferem
// em espaço e pontuação são a mesma; conteúdo diferente vira alerta.
func TestMesclar_DuasLeiturasCompletas(t *testing.T) {
	t.Parallel()

	r := Rascunho{Questoes: []Questao{questao(15, true, "Qual é o valor de x?")}}
	r.Mesclar(Rascunho{Questoes: []Questao{questao(15, true, "Qual é o valor de x ?")}})
	if len(r.Alertas) != 0 {
		t.Fatalf("mesma leitura virou alerta: %v", r.Alertas)
	}

	outra := questao(15, true, "Redija um texto dissertativo")
	outra.Origens = []Origem{{Pagina: 1, Regiao: "11"}}
	r.Mesclar(Rascunho{Questoes: []Questao{outra}})

	if len(r.Alertas) != 1 || !strings.Contains(r.Alertas[0], "Questão 15") {
		t.Fatalf("divergência não sinalizada: %v", r.Alertas)
	}
	if r.Questoes[0].Blocos[0].Texto != "Qual é o valor de x?" {
		t.Fatal("a primeira leitura não foi mantida")
	}
}

// A região recusada devolve a questão pelo OCR; a vizinha, que a leu pela IA,
// vence em qualquer ordem — e o pedaço da IA não troca a questão inteira do
// OCR, nem se soma a ela.
func TestMesclar_LeituraDaIAVenceADoOCR(t *testing.T) {
	t.Parallel()

	doOCR := questao(7, true, "texto do OCR")
	doOCR.LidaPorOCR = true
	daIA := questao(7, true, "texto da IA")

	for nome, ordem := range map[string][2]Questao{"OCR antes": {doOCR, daIA}, "IA antes": {daIA, doOCR}} {
		r := Rascunho{Questoes: []Questao{ordem[0]}}
		r.Mesclar(Rascunho{Questoes: []Questao{ordem[1]}})
		if q := r.Questoes[0]; q.LidaPorOCR || q.Blocos[0].Texto != "texto da IA" || len(r.Alertas) != 0 {
			t.Errorf("%s: questão = %+v, alertas %v", nome, q, r.Alertas)
		}
	}

	pedaco := questao(7, false, "pedaço da IA")
	pedaco.Alternativas = pedaco.Alternativas[:2]
	r := Rascunho{Questoes: []Questao{doOCR}}
	r.Mesclar(Rascunho{Questoes: []Questao{pedaco}})
	if q := r.Questoes[0]; !q.LidaPorOCR || len(q.Alternativas) != 5 || len(q.Blocos) != 1 {
		t.Fatalf("pedaço da IA sobre o OCR inteiro: %+v", q)
	}
}

// Questão do OCR não é dada como conferida: só o curador, olhando o original.
func TestConfirmarSemProblema_NaoConfereAQuestaoDoOCR(t *testing.T) {
	t.Parallel()

	q := questao(1, true, "Enunciado")
	q.Revisada, q.LidaPorOCR, q.Resposta = false, true, "E"
	r := Rascunho{Questoes: []Questao{q}, Gabarito: Gabarito{Respostas: map[string]string{"1": "E"}}}

	r.ConfirmarSemProblema()

	if r.Questoes[0].Revisada {
		t.Fatal("questão do OCR conferida sem o curador")
	}
}

func TestMesclar_LigaApoioAsQuestoesDeOutraRegiao(t *testing.T) {
	t.Parallel()

	r := Rascunho{
		Apoios:   []Apoio{{ID: "r0-t1", Questoes: []int{1, 2, 3}}},
		Questoes: []Questao{questao(1, true, "a")},
	}
	r.Mesclar(Rascunho{Questoes: []Questao{questao(2, true, "b"), questao(4, true, "d")}})

	if got := r.Questoes[1].Apoios; len(got) != 1 || got[0] != "r0-t1" {
		t.Fatalf("questão 2 sem o texto de apoio: %v", got)
	}
	if got := r.Questoes[2].Apoios; len(got) != 0 {
		t.Fatalf("questão 4 ligada a apoio que não a cita: %v", got)
	}
}

func TestAplicarGabarito_RespostaSoDoOficial(t *testing.T) {
	t.Parallel()

	r := Rascunho{
		Questoes: []Questao{questao(1, true, "a"), questao(2, true, "b"), questao(3, true, "c")},
		Gabarito: Gabarito{
			Respostas: map[string]string{"1": "C", "3": "D"},
			Situacoes: map[string]string{"1": "Gabarito sem alteração"},
		},
	}
	r.Questoes[1].Resposta = "A" // inventada: não está no gabarito
	r.Questoes[2].Resposta = "B"

	r.AplicarGabarito()

	if r.Questoes[0].Resposta != "C" || r.Questoes[1].Resposta != "" || r.Questoes[2].Resposta != "D" {
		t.Fatalf("respostas = %q, %q, %q", r.Questoes[0].Resposta, r.Questoes[1].Resposta, r.Questoes[2].Resposta)
	}
	// A letra que mudou pede conferência; a que chegou onde não havia, não.
	if !r.Questoes[0].Revisada || r.Questoes[1].Revisada || r.Questoes[2].Revisada {
		t.Fatalf("conferidas = %v, %v, %v; quer só a 1", r.Questoes[0].Revisada, r.Questoes[1].Revisada, r.Questoes[2].Revisada)
	}
}

func TestAplicarMetadados_NaoApagaOQueACapaNaoTrouxe(t *testing.T) {
	t.Parallel()

	r := Rascunho{Banca: "FCC", Orgao: "TJCE", Ano: 2026}
	r.AplicarMetadados(Metadados{Cargo: "E05", CargoNome: "Analista Judiciário", Caderno: "004", Total: 60})

	if r.Orgao != "TJCE" || r.Ano != 2026 || r.Cargo != "E05" || r.CargoNome != "Analista Judiciário" || r.Total != 60 {
		t.Fatalf("rascunho = %+v", r)
	}
}

func TestHerdarDisciplinas_TituloValeAteOProximo(t *testing.T) {
	t.Parallel()

	r := Rascunho{Questoes: []Questao{
		{Numero: 1, Disciplina: "Língua Portuguesa"},
		{Numero: 2},
		{Numero: 21, Disciplina: "Conhecimentos Específicos"},
		{Numero: 22},
	}}
	r.HerdarDisciplinas()

	quer := []string{"Língua Portuguesa", "Língua Portuguesa", "Conhecimentos Específicos", "Conhecimentos Específicos"}
	for i, q := range r.Questoes {
		if q.Disciplina != quer[i] {
			t.Errorf("questão %d: disciplina = %q, quer %q", q.Numero, q.Disciplina, quer[i])
		}
	}
}

func TestInvalidarEdicoes_RedimensionarFiguraNaoEEdicao(t *testing.T) {
	t.Parallel()

	antigo := valida()
	antigo.Questoes[0].Blocos = append(antigo.Questoes[0].Blocos,
		Bloco{Tipo: "imagem", Arquivo: "fig", Origem: &Origem{Pagina: 1}})
	novo := valida()
	novo.Questoes[0].Blocos = append(novo.Questoes[0].Blocos,
		Bloco{Tipo: "imagem", Arquivo: "fig", Origem: &Origem{Pagina: 1}, Largura: 60})

	InvalidarEdicoes(antigo, &novo)

	if !novo.Questoes[0].Revisada {
		t.Fatal("mudar o tamanho da figura desfez a conferência")
	}
}

func TestMaterias_ResumoEAplicacao(t *testing.T) {
	t.Parallel()

	r := Rascunho{Questoes: []Questao{
		questao(1, true, "Em relação à oração"),
		{Numero: 21, Disciplina: "CONHECIMENTOS ESPECÍFICOS", Blocos: []Bloco{
			{Tipo: "texto", Texto: "Considere a VLAN"}, {Tipo: "imagem", Arquivo: "x"},
		}},
	}}
	r.Questoes[0].Disciplina = "Língua Portuguesa"

	resumo := r.ParaClassificar()
	if resumo[1].Secao != "CONHECIMENTOS ESPECÍFICOS" || resumo[1].Texto != "Considere a VLAN" {
		t.Fatalf("resumo = %+v", resumo[1])
	}
	if !strings.Contains(resumo[0].Texto, "(A) A") {
		t.Fatalf("resumo sem as alternativas: %q", resumo[0].Texto)
	}

	// A seção que já é matéria manda: a IA juntar "Língua Portuguesa" em outra
	// coisa não pode desfazer o título do caderno.
	r.AplicarMaterias(map[int]string{21: "Redes de Computadores", 1: "Linguagens"})
	if r.Questoes[1].Disciplina != "Redes de Computadores" || r.Questoes[0].Disciplina != "Língua Portuguesa" {
		t.Fatalf("disciplinas = %q, %q", r.Questoes[0].Disciplina, r.Questoes[1].Disciplina)
	}
}

func TestProximaEtapa(t *testing.T) {
	t.Parallel()

	const regioes = 13
	casos := []struct {
		executada, etapa int
		estado           string
	}{
		{EtapaPreparar, EtapaMetadados, EstadoNaFila},
		{EtapaGabarito, EtapaPrimeiraRegiao, EstadoNaFila},
		{EtapaPrimeiraRegiao + regioes - 1, EtapaPrimeiraRegiao + regioes, EstadoNaFila},
		{EtapaPrimeiraRegiao + regioes, TotalEtapas(regioes), EstadoEmRevisao},
		{EtapaSoGabarito, TotalEtapas(regioes), EstadoEmRevisao},
		{EtapaTrecho, TotalEtapas(regioes), EstadoEmRevisao},
	}
	for _, c := range casos {
		etapa, estado := ProximaEtapa(c.executada, regioes)
		if etapa != c.etapa || estado != c.estado {
			t.Errorf("ProximaEtapa(%d) = %d %s, quer %d %s", c.executada, etapa, estado, c.etapa, c.estado)
		}
	}
}

// Seis tentativas em pouco mais de sete minutos: uma sobrecarga do Gemini de
// alguns minutos não derruba mais a importação.
func TestEsperaParaRepetir(t *testing.T) {
	t.Parallel()

	var total time.Duration
	for falhas, quer := range []time.Duration{
		15 * time.Second, 30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 0,
	} {
		if got := EsperaParaRepetir(falhas); got != quer {
			t.Errorf("EsperaParaRepetir(%d) = %v, quer %v", falhas, got, quer)
		}
		total += quer
	}

	if total != 7*time.Minute+45*time.Second {
		t.Errorf("espera total = %v, quer 7m45s", total)
	}
}

// Os números são os da prova do TJCE: a questão 22 começa no fim da região 3,
// e a região 4, que a via inteira, não a transcreveu.
func TestReleituras_QuestaoCortadaEntreRegioes(t *testing.T) {
	t.Parallel()

	regioes := []Origem{
		{Pagina: 1, Regiao: "3", Retangulo: []float64{0, 2080, 595.44, 2925.5}},
		{Pagina: 1, Regiao: "4", Retangulo: []float64{0, 2773.3, 595.44, 3618.8}},
	}
	na := func(q Questao, y0, y1 float64) Questao {
		q.Origens = []Origem{{Pagina: 1, Regiao: "3", Retangulo: []float64{123, y0, 458, y1}}}
		return q
	}
	// As coordenadas das questões 21 a 23; numeradas 1 a 3 para o rascunho
	// não ter questões faltando, que também seriam relidas.
	cortada := na(questao(2, false, "Considere o banco"), 2779.2, 2925.5)
	cortada.Alternativas = cortada.Alternativas[:1]
	r := Rascunho{Total: 3, Questoes: []Questao{
		na(questao(1, true, "Uma equipe"), 2597.5, 2802.1),
		cortada,
		na(questao(3, true, "Uma administradora"), 2912.8, 3073.5),
	}}

	got := r.Releituras(regioes)

	if len(got) != 1 {
		t.Fatalf("releituras = %+v, quer uma, da questão 2", got)
	}
	q22 := got[0]
	if q22.Regiao != "q2" || q22.Pagina != 1 || !EReleitura(q22) {
		t.Fatalf("releitura = %+v", q22)
	}
	// Uma região inteira, centrada na borda que cortou a 22 — o fim da região 3.
	if ret := q22.Retangulo; ret[0] != 0 || ret[1] != 2925.5-845.5/2 || ret[2] != 595.44 || ret[3] != 2925.5+845.5/2 {
		t.Fatalf("retângulo = %v", ret)
	}

	// Lidas as releituras, não há outra rodada.
	if mais := r.Releituras(append(regioes, got...)); mais != nil {
		t.Fatalf("segunda rodada de releituras: %+v", mais)
	}
}

// PDF escaneado com a capa maior que as outras páginas: a releitura de uma
// questão da página pequena não pode ter a largura da capa — o processador
// recusava o retângulo e a importação inteira falhava.
func TestReleituras_MedidaDaPropriaPagina(t *testing.T) {
	t.Parallel()

	regioes := []Origem{
		{Pagina: 1, Regiao: "0", Retangulo: []float64{0, 0, 2133.3, 3017.3}},
		{Pagina: 2, Regiao: "1", Retangulo: []float64{0, 0, 1066.7, 1508}},
		{Pagina: 3, Regiao: "2", Retangulo: []float64{0, 0, 1066.7, 1508}},
	}
	na := func(q Questao, pagina int, regiao string, y0, y1 float64) Questao {
		q.Origens = []Origem{{Pagina: pagina, Regiao: regiao, Retangulo: []float64{80, y0, 1000, y1}}}
		return q
	}
	cortada := na(questao(2, false, "cortada no pé da página 2"), 2, "1", 1300, 1500)
	cortada.Alternativas = cortada.Alternativas[:2]
	r := Rascunho{Total: 3, Questoes: []Questao{
		na(questao(1, true, "a"), 2, "1", 100, 900),
		cortada,
		na(questao(3, true, "c"), 3, "2", 100, 600),
	}}

	got := r.Releituras(regioes)

	if len(got) != 1 || got[0].Pagina != 2 {
		t.Fatalf("releituras = %+v, quer uma na página 2", got)
	}
	if ret := got[0].Retangulo; ret[0] != 0 || ret[1] != 0 || ret[2] != 1066.7 || ret[3] != 1508 {
		t.Fatalf("retângulo = %v, quer dentro da página 2 (1066.7 × 1508)", ret)
	}
}

// Uma região por página, e a página 2 inteira recusada: as questões 1 a 5
// faltaram, e a primeira lida é a 6, no alto da página 3. A releitura vai ao
// pé da página 2, não à 3, que já foi lida.
func TestReleituras_QuestaoQueFaltouNaPaginaAnterior(t *testing.T) {
	t.Parallel()

	regioes := []Origem{
		{Pagina: 2, Regiao: "1", Retangulo: []float64{0, 0, 2133.3, 3017.3}},
		{Pagina: 3, Regiao: "2", Retangulo: []float64{0, 0, 2133.3, 3017.3}},
	}
	q6 := questao(6, true, "sexta")
	q6.Origens = []Origem{{Pagina: 3, Regiao: "2", Retangulo: []float64{80, 150, 2000, 700}}}
	r := Rascunho{Total: 6, Questoes: []Questao{q6}}

	got := r.Releituras(regioes)

	if len(got) != 1 || got[0].Regiao != "q1" || got[0].Pagina != 2 {
		t.Fatalf("releituras = %+v, quer uma, da questão 1, na página 2", got)
	}
	if ret := got[0].Retangulo; ret[1] != 0 || ret[3] != 3017.3 {
		t.Fatalf("retângulo = %v, quer a página 2", ret)
	}
}

// A questão que nenhuma região leu fica entre as vizinhas.
func TestReleituras_QuestaoQueFaltouEntreDuasLidas(t *testing.T) {
	t.Parallel()

	regioes := []Origem{{Pagina: 1, Regiao: "0", Retangulo: []float64{0, 0, 595, 845}}}
	em := func(n int, y0, y1 float64) Questao {
		q := questao(n, true, "x")
		q.Origens = []Origem{{Pagina: 1, Regiao: "0", Retangulo: []float64{100, y0, 480, y1}}}
		return q
	}
	r := Rascunho{Total: 4, Questoes: []Questao{em(1, 50, 200), em(2, 210, 380), em(4, 520, 700)}}

	got := r.Releituras(regioes)

	if len(got) != 1 || got[0].Regiao != "q3" {
		t.Fatalf("releituras = %+v, quer a da questão 3", got)
	}
	// Centrada entre a 2 e a 4, e sem passar do fim da página.
	if ret := got[0].Retangulo; ret[1] != 0 || ret[3] != 845 {
		t.Fatalf("retângulo da 3 = %v, quer a página de 0 a 845", ret)
	}
}

func TestReleituras_TemTeto(t *testing.T) {
	t.Parallel()

	var regioes []Origem
	for k := range 30 {
		y := float64(k) * 700
		regioes = append(regioes, Origem{Pagina: 1, Regiao: strconv.Itoa(k), Retangulo: []float64{0, y, 595, y + 845}})
	}
	// Longe uma da outra, cada cortada pede o próprio recorte.
	var qs []Questao
	for n := 1; n <= 20; n++ {
		q := questao(n, false, "cortada")
		y := float64(n) * 1000
		q.Origens = []Origem{{Pagina: 1, Regiao: "fora", Retangulo: []float64{100, y, 480, y + 100}}}
		qs = append(qs, q)
	}

	if got := (Rascunho{Total: 20, Questoes: qs}).Releituras(regioes); len(got) != maxReleituras {
		t.Fatalf("releituras = %d, quer o teto de %d", len(got), maxReleituras)
	}
}

// Da releitura só entra o que estava incompleto, e só se ficou melhor: as
// vizinhas relidas não geram alerta, e dois pedaços não repetem alternativas.
func TestAplicarReleitura_SoCompletaOQueFaltava(t *testing.T) {
	t.Parallel()

	cortada := questao(2, false, "começo")
	cortada.Alternativas = cortada.Alternativas[:2]
	r := Rascunho{Questoes: []Questao{questao(1, true, "a primeira"), cortada, questao(3, false, "outra cortada")}}
	pior := questao(3, false, "de novo cortada")
	pior.Alternativas = nil

	r.AplicarReleitura(Rascunho{
		Questoes: []Questao{questao(1, true, "relida diferente"), questao(2, true, "inteira"), pior},
		Apoios:   []Apoio{{ID: "rq2-t1"}},
		Alertas:  []string{"alerta do recorte"},
	})

	if r.Questoes[0].Blocos[0].Texto != "a primeira" || len(r.Alertas) != 0 {
		t.Fatalf("a vizinha completa mudou ou gerou alerta: %+v %v", r.Questoes[0], r.Alertas)
	}
	if q := r.Questoes[1]; !q.Completa || len(q.Alternativas) != 5 || q.Blocos[0].Texto != "inteira" {
		t.Fatalf("questão 2 = %+v, quer a releitura inteira", q)
	}
	if q := r.Questoes[2]; len(q.Alternativas) != 5 || q.Blocos[0].Texto != "outra cortada" {
		t.Fatalf("questão 3 trocada por leitura pior: %+v", q)
	}
	if len(r.Apoios) != 0 {
		t.Fatalf("texto de apoio da releitura entrou: %+v", r.Apoios)
	}
}

// A questão que veio do OCR é relida sozinha pela IA, mesmo inteira; e a
// releitura da IA toma o lugar dela, mas a que voltou do OCR de novo não troca
// a leitura que já existe.
func TestReleituras_QuestaoDoOCRERelida(t *testing.T) {
	t.Parallel()

	regioes := []Origem{{Pagina: 1, Regiao: "5", Retangulo: []float64{0, 0, 595, 842}}}
	doOCR := questao(1, true, "do OCR")
	doOCR.LidaPorOCR = true
	doOCR.Origens = []Origem{{Pagina: 1, Regiao: "5", Retangulo: []float64{0, 100, 595, 300}}}
	r := Rascunho{Total: 1, Questoes: []Questao{doOCR}}

	if got := r.Releituras(regioes); len(got) != 1 || got[0].Regiao != "q1" {
		t.Fatalf("releituras = %+v, quer a da questão 1", got)
	}

	outraDoOCR := questao(1, true, "OCR de novo")
	outraDoOCR.LidaPorOCR = true
	r.AplicarReleitura(Rascunho{Questoes: []Questao{outraDoOCR}})
	if r.Questoes[0].Blocos[0].Texto != "do OCR" {
		t.Fatalf("releitura do OCR trocou a leitura: %+v", r.Questoes[0])
	}
	r.AplicarReleitura(Rascunho{Questoes: []Questao{questao(1, true, "da IA")}})
	if q := r.Questoes[0]; q.LidaPorOCR || q.Blocos[0].Texto != "da IA" {
		t.Fatalf("releitura da IA não entrou: %+v", q)
	}
}

// O caso da prova do TJCE: na região do texto de apoio, o modelo devolveu as
// questões 9 e 10 vazias, com a região inteira como origem. Sem área própria,
// elas são relidas entre as vizinhas lidas — a 8 e a 11 —, num recorte só.
func TestReleituras_QuestoesVaziasEntreVizinhas(t *testing.T) {
	t.Parallel()

	regioes := []Origem{
		{Pagina: 1, Regiao: "0", Retangulo: []float64{0, 0, 595.44, 845.5}},
		{Pagina: 1, Regiao: "1", Retangulo: []float64{0, 693.3, 595.44, 1538.9}},
		{Pagina: 1, Regiao: "2", Retangulo: []float64{0, 1386.7, 595.44, 2232.2}},
	}
	em := func(n int, y0, y1 float64) Questao {
		q := questao(n, true, "lida")
		q.Origens = []Origem{{Pagina: 1, Regiao: "1", Retangulo: []float64{108, y0, 467, y1}}}
		return q
	}
	vazia := func(n int) Questao {
		return Questao{Numero: n, Completa: true, Origens: []Origem{regioes[0]}}
	}
	r := Rascunho{Total: 11, Questoes: []Questao{
		em(8, 1161.8, 1265.8), vazia(9), vazia(10), em(11, 1375.7, 1521.9),
	}}
	for n := 1; n <= 7; n++ {
		r.Questoes = append(r.Questoes, em(n, float64(n)*100, float64(n)*100+90))
	}

	got := r.Releituras(regioes)

	if len(got) != 1 || got[0].Regiao != "q9" {
		t.Fatalf("releituras = %+v, quer uma só, da 9 (que relê a 10 junto)", got)
	}
	centro := (got[0].Retangulo[1] + got[0].Retangulo[3]) / 2
	if centro != (1265.8+1375.7)/2 {
		t.Fatalf("centro do recorte = %v, quer entre a 8 e a 11", centro)
	}
}

// A entrada vazia que o modelo marca como completa não vence a leitura de
// verdade, nem gera alerta de leitura dupla.
func TestMesclar_LeituraDeVerdadeVenceAVazia(t *testing.T) {
	t.Parallel()

	r := Rascunho{Questoes: []Questao{{Numero: 9, Completa: true}}}
	r.Mesclar(Rascunho{Questoes: []Questao{questao(9, true, "Enunciado da 9")}})

	if q := r.Questoes[0]; len(q.Alternativas) != 5 || q.Blocos[0].Texto != "Enunciado da 9" || len(r.Alertas) != 0 {
		t.Fatalf("questão 9 = %+v, alertas = %v", q, r.Alertas)
	}
}

// O caso da prova do TJCE: a releitura da questão 4 leu de novo o texto de
// Sêneca, com outro id, e as questões 4 a 8 ficavam apontando para ele — um
// texto descartado. A ligação vem da lista de questões de cada texto.
func TestAplicarReleitura_QuestaoFicaComOTextoDoRascunho(t *testing.T) {
	t.Parallel()

	cortada := questao(4, false, "começo")
	cortada.Alternativas = cortada.Alternativas[:1]
	cortada.Apoios = []string{"r0-t1"}
	r := Rascunho{
		Questoes: []Questao{cortada},
		Apoios:   []Apoio{{ID: "r0-t1", Questoes: []int{1, 2, 3, 4, 5}}},
	}
	relida := questao(4, true, "inteira")
	relida.Apoios = []string{"rq4-t1"}

	r.AplicarReleitura(Rascunho{Questoes: []Questao{relida}, Apoios: []Apoio{{ID: "rq4-t1"}}})

	if got := r.Questoes[0].Apoios; len(got) != 1 || got[0] != "r0-t1" {
		t.Fatalf("textos da questão 4 = %v, quer só r0-t1", got)
	}
	if _, fora := r.ParaPublicar(Criterios{}); strings.Contains(motivos(fora), "não existe mais") {
		t.Fatalf("de fora por texto que não existe: %v", fora)
	}
}

func TestAcertarApoios_ALigacaoVemDoTexto(t *testing.T) {
	t.Parallel()

	solta := questao(1, true, "a")
	solta.Apoios = []string{"sumiu"}
	r := Rascunho{
		Questoes: []Questao{solta, questao(2, true, "b")},
		Apoios:   []Apoio{{ID: "t1", Questoes: []int{1, 2}}},
	}

	r.AcertarApoios()

	for _, q := range r.Questoes {
		if len(q.Apoios) != 1 || q.Apoios[0] != "t1" {
			t.Fatalf("questão %d com textos %v, quer [t1]", q.Numero, q.Apoios)
		}
	}
}

// Questão sem nada para ver já vem conferida; cada defeito, e qualquer figura,
// deixa a questão para o curador.
func TestConfirmarSemProblema(t *testing.T) {
	t.Parallel()

	semConferir := func(r Rascunho) Rascunho {
		for i := range r.Questoes {
			r.Questoes[i].Revisada = false
		}
		return r
	}
	comFigura := func(q Questao, arquivo string) Questao {
		q.Blocos = append(q.Blocos, Bloco{Tipo: "imagem", Arquivo: arquivo, Origem: &Origem{Pagina: 1}})
		return q
	}

	r := semConferir(valida())
	r.ConfirmarSemProblema()
	if q := r.Questoes[0]; !q.Revisada {
		t.Fatalf("questão sem problema não foi conferida: %+v", q)
	}

	casos := []struct {
		nome  string
		mudar func(*Rascunho)
	}{
		{"cortada", func(r *Rascunho) { r.Questoes[0].Completa = false }},
		{"quatro alternativas", func(r *Rascunho) { r.Questoes[0].Alternativas = r.Questoes[0].Alternativas[:4] }},
		{"alternativa vazia", func(r *Rascunho) { r.Questoes[0].Alternativas[2].Blocos[0].Texto = " " }},
		// O recorte só se confere olhando o original: figura é sempre do curador.
		{"figura recortada", func(r *Rascunho) { r.Questoes[0] = comFigura(r.Questoes[0], "fig") }},
		{"figura numa alternativa", func(r *Rascunho) {
			r.Questoes[0].Alternativas[3].Blocos = []Bloco{{Tipo: "imagem", Arquivo: "fig"}}
		}},
		{"figura sem recorte", func(r *Rascunho) { r.Questoes[0] = comFigura(r.Questoes[0], "") }},
		{"resposta fora do gabarito", func(r *Rascunho) { r.Questoes[0].Resposta = "A" }},
		{"cita texto sem texto ligado", func(r *Rascunho) {
			r.Questoes[0].Blocos[0].Texto = "De acordo com o texto, a fortuna"
		}},
		{"texto ligado que não existe", func(r *Rascunho) { r.Questoes[0].Apoios = []string{"t9"} }},
		{"alerta da extração sobre ela", func(r *Rascunho) {
			r.Alertas = []string{"Questão 1 foi lida duas vezes com conteúdo diferente (regiões 6 e 7)."}
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			r := semConferir(valida())
			c.mudar(&r)
			r.ConfirmarSemProblema()
			if r.Questoes[0].Revisada {
				t.Fatal("questão com problema foi conferida sozinha")
			}
		})
	}

	// O alerta de outra questão não conta: "questão 12" não é a 1.
	r = semConferir(valida())
	r.Alertas = []string{"Questão 12 foi lida duas vezes com conteúdo diferente."}
	r.ConfirmarSemProblema()
	if !r.Questoes[0].Revisada {
		t.Fatal("o alerta da questão 12 segurou a 1")
	}
}

// Os blocos que travaram a TRT 1: vazio depois do "gateway" em itálico
// (questão 33) e o espaço entre dois trechos em itálico (questão 55). O vazio
// sai; o espaço entre trechos fica — tirá-lo grudava as palavras.
func TestLimparBlocos(t *testing.T) {
	t.Parallel()

	texto := func(s, formato string) Bloco { return Bloco{Tipo: "texto", Texto: s, Formato: formato} }
	figura := Bloco{Tipo: "imagem", Arquivo: "f", Origem: &Origem{Pagina: 1}}
	r := Rascunho{Questoes: []Questao{questao(33, true, "x"), questao(55, true, "y")}}
	r.Questoes[0].Blocos = []Bloco{texto("o ", ""), texto("gateway", "italico"), texto("", "")}
	r.Questoes[1].Alternativas[3].Blocos = []Bloco{texto("Keycloak", "italico"), texto(" ", ""), texto("OAuth", "italico")}
	r.Questoes[1].Blocos = []Bloco{texto(" ", ""), texto("Considere:", ""), texto(" ", ""), figura, {Tipo: "codigo", Texto: "  \n"}}

	r.LimparBlocos()

	if got := r.Questoes[0].Blocos; len(got) != 2 {
		t.Fatalf("questão 33 = %+v; o vazio do fim fica", got)
	}
	if got := r.Questoes[1].Alternativas[3].Blocos; len(got) != 3 || got[1].Texto != " " {
		t.Fatalf("alternativa D = %+v; o espaço entre os itálicos sumiu", got)
	}
	if got := r.Questoes[1].Blocos; len(got) != 2 || got[0].Texto != "Considere:" || got[1].Tipo != "imagem" {
		t.Fatalf("enunciado da 55 = %+v; quer só o texto e a figura", got)
	}
	if _, fora := r.ParaPublicar(Criterios{}); strings.Contains(motivos(fora), "vazio") {
		t.Fatalf("de fora por bloco vazio depois de limpar: %v", fora)
	}
}

// Conferidos os recortes, a figura era o que faltava: a questão está conferida.
func TestConfirmarSemProblema_ComOsRecortesConferidos(t *testing.T) {
	t.Parallel()

	r := valida()
	r.Questoes[0].Revisada = false
	r.Questoes[0].Blocos = append(r.Questoes[0].Blocos,
		Bloco{Tipo: "imagem", Arquivo: "f", Origem: &Origem{Pagina: 1}, Revisado: true})

	r.ConfirmarSemProblema()

	if !r.Questoes[0].Revisada {
		t.Fatal("recortes conferidos e a questão ficou pendente")
	}
}

// O curador confere o último recorte e salva: a questão passa a conferida. Se
// depois ele a desmarca de propósito, a gravação seguinte não a remarca.
func TestConfirmarPelosRecortes(t *testing.T) {
	t.Parallel()

	comFigura := func(conferida bool) Rascunho {
		r := valida()
		r.Questoes[0].Revisada = false
		r.Questoes[0].Blocos = append(r.Questoes[0].Blocos,
			Bloco{Tipo: "imagem", Arquivo: "f", Origem: &Origem{Pagina: 1}, Revisado: conferida})
		return r
	}

	antigo, novo := comFigura(false), comFigura(true)
	ConfirmarPelosRecortes(antigo, &novo)
	if !novo.Questoes[0].Revisada {
		t.Fatal("conferiu o recorte e a questão ficou pendente")
	}

	antigo, novo = comFigura(true), comFigura(true)
	ConfirmarPelosRecortes(antigo, &novo)
	if novo.Questoes[0].Revisada {
		t.Fatal("remarcou a questão que o curador desmarcou")
	}
}

func TestQuestaoDaReleitura(t *testing.T) {
	t.Parallel()

	if n, ok := QuestaoDaReleitura(Origem{Regiao: "q42"}); !ok || n != 42 {
		t.Fatalf("q42 = %d, %v", n, ok)
	}
	for _, rotulo := range []string{"4", "q", "qx", "", "t42"} {
		if _, ok := QuestaoDaReleitura(Origem{Regiao: rotulo}); ok {
			t.Errorf("%q passou por releitura", rotulo)
		}
	}
}

// O gabarito vai para o banco em linhas, uma por questão, em ordem; e volta
// igual. Situação sem resposta também é linha.
func TestGabaritoEmLinhas(t *testing.T) {
	t.Parallel()

	g := Gabarito{
		Cargo: "E05", Caderno: "5", Tipo: "preliminar",
		Respostas: map[string]string{"10": "C", "2": "", "1": "B"},
		Situacoes: map[string]string{"2": "Anulada", "3": "Gabarito sem alteração"},
	}

	linhas := g.Linhas()

	quer := []RespostaDoGabarito{
		{Numero: 1, Resposta: "B"}, {Numero: 2, Situacao: "Anulada"},
		{Numero: 3, Situacao: "Gabarito sem alteração"}, {Numero: 10, Resposta: "C"},
	}
	if !reflect.DeepEqual(linhas, quer) {
		t.Fatalf("linhas = %+v\nquer %+v", linhas, quer)
	}
	volta := GabaritoDasLinhas(g.Cargo, g.Caderno, g.Tipo, linhas)
	if volta.Respostas["10"] != "C" || volta.Situacoes["2"] != "Anulada" || volta.Tipo != "preliminar" {
		t.Fatalf("remontado = %+v", volta)
	}
	if g.Vazio() || !(Gabarito{}).Vazio() {
		t.Fatal("Vazio errado")
	}
}

// A capa não lida deixava total 0, e cada questão virava uma "Numeração
// inválida": agora é aviso, e a prova publica com o total do que se conhece.
func TestAvisos_CapaNaoLida(t *testing.T) {
	t.Parallel()

	r := valida()
	r.Ano, r.Cargo, r.Total = 0, "", 0
	r.Questoes = append(r.Questoes, questao(2, true, "Segunda"), questao(3, true, "Terceira"))
	r.Gabarito = Gabarito{Tipo: "preliminar", Respostas: map[string]string{"1": "E", "2": "", "3": "", "4": "A"}}

	quer := []string{
		"Confira na etapa Dados: ano, código do cargo.",
		"A capa não disse quantas questões a prova tem: preencha o Total de questões na etapa Dados.",
	}
	if a := r.Avisos(); !slices.Equal(a, quer) {
		t.Fatalf("avisos:\n%s\nquer:\n%s", strings.Join(a, "\n"), strings.Join(quer, "\n"))
	}
	pub, fora := r.ParaPublicar(publicar)
	if p := r.Pendencias(publicar); len(p) > 0 || len(pub.Questoes) != 3 || pub.Total != 4 {
		t.Fatalf("pendências %v, %d publicadas, total %d; quer 3 de 4", p, len(pub.Questoes), pub.Total)
	}
	if got := motivos(fora); got != "4: não está no rascunho\n" {
		t.Fatalf("de fora:\n%s", got)
	}
}

func TestTotalPeloGabarito(t *testing.T) {
	t.Parallel()

	r := Rascunho{Gabarito: Gabarito{Respostas: map[string]string{"1": "A", "60": "", "x": "B"}}}
	r.TotalPeloGabarito()
	if r.Total != 60 || len(r.Alertas) != 1 {
		t.Fatalf("total %d, alertas %v; quer 60, avisado", r.Total, r.Alertas)
	}

	// A capa manda; sem gabarito, nada a completar.
	lida := Rascunho{Total: 50, Gabarito: r.Gabarito}
	lida.TotalPeloGabarito()
	vazio := Rascunho{}
	vazio.TotalPeloGabarito()
	if lida.Total != 50 || len(lida.Alertas) != 0 || vazio.Total != 0 || len(vazio.Alertas) != 0 {
		t.Fatalf("lida %d %v, vazio %d %v", lida.Total, lida.Alertas, vazio.Total, vazio.Alertas)
	}
}

// A excluída não vai ao catálogo nem é listada de fora: foi decisão do
// curador, anulada ou não.
func TestParaPublicar_Excluida(t *testing.T) {
	t.Parallel()

	r := valida()
	r.Total = 3
	tres := questao(3, true, "Terceira")
	r.Questoes = append(r.Questoes, questao(2, true, "Segunda"), tres)
	r.Gabarito.Respostas = map[string]string{"1": "E", "2": "C", "3": "A"}
	r.Excluidas = []int{2, 2, 9}

	pub, fora := r.ParaPublicar(publicar)

	if len(pub.Questoes) != 2 || len(fora) != 0 || !slices.Equal(pub.Excluidas, []int{2}) || pub.QuestoesNaProva() != 2 {
		t.Fatalf("%d publicadas, de fora %v, excluídas %v", len(pub.Questoes), fora, pub.Excluidas)
	}
}

// A prova pode ser publicada antes de o gabarito sair, mas a questão sem
// resposta não chega ao aluno: ele responderia sem ter como conferir.
func TestSoComGabarito(t *testing.T) {
	t.Parallel()

	p := Publicacao{Conteudo: Rascunho{
		Total:     4,
		Questoes:  []Questao{questao(1, true, "com resposta"), questao(2, true, "anulada"), questao(3, true, "sem gabarito")},
		Excluidas: []int{4},
		Gabarito:  Gabarito{Respostas: map[string]string{"1": "C", "2": ""}},
	}}

	vista := p.SoComGabarito()

	numeros := []int{}
	for _, q := range vista.Conteudo.Questoes {
		numeros = append(numeros, q.Numero)
	}
	// A anulada tem linha no gabarito: é questão da prova, sem letra certa.
	if !slices.Equal(numeros, []int{1, 2}) {
		t.Fatalf("questões = %v, quer a com resposta e a anulada", numeros)
	}
	if !slices.Equal(vista.Conteudo.Excluidas, []int{3, 4}) || vista.Conteudo.QuestoesNaProva() != 2 {
		t.Fatalf("excluídas %v, %d na prova", vista.Conteudo.Excluidas, vista.Conteudo.QuestoesNaProva())
	}
	// O original não muda: a revisão do curador continua com as três.
	if len(p.Conteudo.Questoes) != 3 {
		t.Fatal("SoComGabarito mexeu na publicação original")
	}

	// Sem gabarito nenhum, nenhuma questão aparece.
	sem := Publicacao{Conteudo: Rascunho{Total: 1, Questoes: []Questao{questao(1, true, "a")}}}
	if vista := sem.SoComGabarito(); len(vista.Conteudo.Questoes) != 0 || vista.Conteudo.QuestoesNaProva() != 0 {
		t.Fatalf("prova sem gabarito = %+v", vista.Conteudo)
	}
}

// A folha da banca muda só o que cita: o resto do gabarito preliminar fica.
func TestAplicarAlteracoes(t *testing.T) {
	t.Parallel()

	r := valida()
	r.Total = 3
	dois, tres := questao(2, true, "Segunda"), questao(3, true, "Terceira")
	dois.Resposta, tres.Resposta = "A", "B"
	r.Questoes = append(r.Questoes, dois, tres)
	r.Gabarito = Gabarito{
		Cargo: "E05", Caderno: "4", Tipo: "preliminar",
		Respostas: map[string]string{"1": "E", "2": "A", "3": "B"},
		Situacoes: map[string]string{"1": "Gabarito sem alteração"},
	}

	alteradas, atribuidas := r.AplicarAlteracoes(Gabarito{
		Respostas: map[string]string{"2": "C", "3": "", "9": "X"},
		Situacoes: map[string]string{"2": "Gabarito alterado", "3": "Questão atribuída a todos"},
	})

	if !slices.Equal(alteradas, []int{2}) || !slices.Equal(atribuidas, []int{3}) {
		t.Fatalf("alteradas %v, atribuídas %v", alteradas, atribuidas)
	}
	// A 1 não é citada e fica como estava; a letra inválida não entra.
	if g := r.Gabarito; g.Respostas["1"] != "E" || g.Respostas["2"] != "C" || g.Respostas["3"] != "" ||
		g.Respostas["9"] != "" || g.Tipo != "definitivo" {
		t.Fatalf("gabarito = %+v", g)
	}
	if r.Questoes[1].Resposta != "C" || r.Questoes[2].Resposta != "" {
		t.Fatalf("respostas das questões = %q, %q", r.Questoes[1].Resposta, r.Questoes[2].Resposta)
	}
	// Quem mudou de letra volta a pedir conferência; a 1 continua conferida.
	if !r.Questoes[0].Revisada || r.Questoes[1].Revisada || r.Questoes[2].Revisada {
		t.Fatalf("conferidas = %v", []bool{r.Questoes[0].Revisada, r.Questoes[1].Revisada, r.Questoes[2].Revisada})
	}
	if p := r.Pendencias(publicar); len(p) > 0 {
		t.Fatalf("pendências = %v", p)
	}
}

func TestNomeDoCargo(t *testing.T) {
	t.Parallel()

	for digitado, quer := range map[string]string{
		"  Técnico Judiciário   -  Tecnologia da Informação ": "Técnico Judiciário - Tecnologia da Informação",
		"Analista\tJudiciário\n":                              "Analista Judiciário",
	} {
		if got, ok := NomeDoCargo(digitado); !ok || got != quer {
			t.Errorf("NomeDoCargo(%q) = %q, %v; quer %q", digitado, got, ok, quer)
		}
	}
	for _, invalido := range []string{"", "   ", strings.Repeat("a", 201)} {
		if _, ok := NomeDoCargo(invalido); ok {
			t.Errorf("NomeDoCargo(%.20q) aceitou", invalido)
		}
	}
}

func TestNomeDoArquivo(t *testing.T) {
	t.Parallel()

	longo := strings.Repeat("á", 300) + ".pdf"
	for enviado, quer := range map[string]string{
		"fcc-2025-trt-15-regiao-sp-tecnico-judiciario-prova.pdf": "fcc-2025-trt-15-regiao-sp-tecnico-judiciario-prova.pdf",
		`C:\fakepath\prova.pdf`:                                  "prova.pdf",
		"/home/curador/Downloads/gabarito.pdf":                   "gabarito.pdf",
		"  com espaço.pdf  ":                                     "com espaço.pdf",
		"quebrado\xff.pdf":                                       "quebrado.pdf",
		"":                                                       "",
		longo:                                                    strings.Repeat("á", 255),
	} {
		if got := NomeDoArquivo(enviado); got != quer {
			t.Errorf("NomeDoArquivo(%.40q) = %.40q, quer %.40q", enviado, got, quer)
		}
	}
}

// O trecho é o retângulo que o curador desenhou dentro de uma região da
// página — nunca fora dela, onde o processador recusaria.
func TestNovoTrecho(t *testing.T) {
	t.Parallel()

	regioes := []Origem{
		{Pagina: 1, Regiao: "0", Retangulo: []float64{0, 0, 595, 842}},
		{Pagina: 2, Regiao: "1", Retangulo: []float64{0, 0, 595, 842}},
		{Pagina: 2, Regiao: "q7", Retangulo: []float64{0, 300, 595, 700}},
	}

	got, ok := NovoTrecho(regioes, 7, Origem{Pagina: 2, Retangulo: []float64{40, 310.5, 560, 700}, Regiao: "1"})
	if !ok || got.Regiao != "t7" || got.Pagina != 2 || !slices.Equal(got.Retangulo, []float64{40, 310.5, 560, 700}) {
		t.Fatalf("trecho = %+v, %v; quer t7 na página 2 com o retângulo marcado", got, ok)
	}
	// O pé arredondado na tela passa da borda por um centésimo: fica na borda,
	// que é o que o processador aceita.
	if got, ok := NovoTrecho(regioes, 7, Origem{Pagina: 2, Retangulo: []float64{40, 310.5, 560, 842.4}}); !ok || got.Retangulo[3] != 842 {
		t.Fatalf("trecho no pé da página = %+v, %v; quer a base em 842", got, ok)
	}

	recusados := map[string]struct {
		numero int
		o      Origem
	}{
		"fora da página":      {7, Origem{Pagina: 2, Retangulo: []float64{40, 300, 560, 900}}},
		"página sem região":   {7, Origem{Pagina: 3, Retangulo: []float64{40, 300, 560, 700}}},
		"clique, não trecho":  {7, Origem{Pagina: 1, Retangulo: []float64{40, 300, 45, 700}}},
		"sem retângulo":       {7, Origem{Pagina: 1}},
		"questão sem número":  {0, Origem{Pagina: 1, Retangulo: []float64{40, 300, 560, 700}}},
		"coordenada inválida": {7, Origem{Pagina: 1, Retangulo: []float64{math.NaN(), 300, 560, 700}}},
		"infinito":            {7, Origem{Pagina: 1, Retangulo: []float64{math.Inf(-1), 300, 560, 700}}},
	}
	for nome, c := range recusados {
		if got, ok := NovoTrecho(regioes, c.numero, c.o); ok {
			t.Errorf("%s: aceitou %+v", nome, got)
		}
	}
}

// O trecho entra no fim das regiões, no lugar do anterior da mesma questão: é
// lá que a etapa dele o procura.
func TestRelerTrecho_UltimaRegiaoEOTrecho(t *testing.T) {
	t.Parallel()

	i := Importacao{
		Estado: EstadoEmRevisao, Erro: "velho",
		Regioes: []Origem{{Regiao: "0"}, {Regiao: "t7"}, {Regiao: "q9"}},
	}
	i.RelerTrecho(Origem{Pagina: 1, Regiao: "t7", Retangulo: []float64{1, 2, 300, 400}})

	if got := regioesDe(i.Regioes); strings.Join(got, ",") != "0,q9,t7" {
		t.Fatalf("regiões = %v, quer 0,q9,t7", got)
	}
	if i.Etapa != EtapaTrecho || i.Estado != EstadoNaFila || i.Erro != "" {
		t.Fatalf("etapa %d, estado %s, erro %q", i.Etapa, i.Estado, i.Erro)
	}
	if o, n, ok := i.TrechoPendente(); !ok || n != 7 || o.Retangulo[2] != 300 {
		t.Fatalf("TrechoPendente = %+v, %d, %v", o, n, ok)
	}
	if _, _, ok := (Importacao{Regioes: []Origem{{Regiao: "q9"}}}).TrechoPendente(); ok {
		t.Fatal("releitura passou por trecho")
	}
}

// A leitura do trecho troca a questão sem comparar com a antiga — foi o
// curador que pediu —, mas o que não é leitura fica: matéria, resposta e
// textos de apoio. A conferência cai.
func TestNovoTrechoDeApoio(t *testing.T) {
	t.Parallel()

	regioes := []Origem{{Pagina: 4, Regiao: "3", Retangulo: []float64{0, 0, 595, 842}}}
	marcado := Origem{Pagina: 4, Retangulo: []float64{30, 500, 560, 842.3}, Regiao: "3"}

	got, ok := NovoTrechoDeApoio(regioes, "r3-t1", marcado)
	if !ok || got.Regiao != "ta:r3-t1" || got.Retangulo[3] != 842 || !ETrechoDeApoio(got) {
		t.Fatalf("trecho = %+v, %v; quer ta:r3-t1 na borda da página", got, ok)
	}
	i := Importacao{Regioes: append(slices.Clone(regioes), got)}
	if _, id, ok := i.TrechoDeApoioPendente(); !ok || id != "r3-t1" {
		t.Fatalf("TrechoDeApoioPendente = %q, %v", id, ok)
	}
	// O da questão não confunde com o do texto, e vice-versa.
	if _, _, ok := i.TrechoPendente(); ok {
		t.Fatal("o trecho do texto passou por trecho de questão")
	}
	if _, ok := NovoTrechoDeApoio(regioes, " ", marcado); ok {
		t.Fatal("aceitou texto sem id")
	}
	if _, ok := NovoTrechoDeApoio(regioes, "r3-t1", Origem{Pagina: 5, Retangulo: []float64{30, 500, 560, 800}}); ok {
		t.Fatal("aceitou trecho fora das páginas do caderno")
	}
}

// O texto das questões 14 a 17 do TRT-18 veio só com o primeiro parágrafo.
func TestAplicarTrechoDeApoio(t *testing.T) {
	t.Parallel()

	rascunho := func() Rascunho {
		r := valida()
		r.Questoes[0].Numero = 14
		r.Apoios = []Apoio{{
			ID: "r3-t1", Questoes: []int{14}, Revisado: true, IgualA: "TRT 18 2023 · L12",
			Aviso:   "Para responder às questões de números 14 a 17",
			Blocos:  []Bloco{{Tipo: "texto", Texto: "No voo da caneta. Numa das cartas…"}},
			Origens: []Origem{{Pagina: 4, Regiao: "3"}},
		}}
		r.AcertarApoios()
		return r
	}
	trecho := Origem{Pagina: 4, Regiao: "ta:r3-t1", Retangulo: []float64{30, 500, 560, 842}}
	lido := Rascunho{
		Alertas: []string{"O texto de apoio do trecho marcado veio do texto do PDF"},
		Apoios: []Apoio{{
			ID: "rta:r3-t1-t1", Questoes: []int{14, 15, 16, 17}, Origens: []Origem{trecho},
			Blocos: []Bloco{{Tipo: "texto", Texto: "No voo da caneta. Numa das cartas… (quatro parágrafos) (Adaptado de …)"}},
		}},
	}

	r := rascunho()
	r.AplicarTrechoDeApoio("r3-t1", lido)
	a := r.Apoios[0]
	if !strings.Contains(a.Blocos[0].Texto, "quatro parágrafos") || a.Revisado || a.IgualA != "" {
		t.Fatalf("texto = %+v; quer a leitura do trecho, sem conferência nem reaproveitamento", a)
	}
	if !slices.Equal(a.Questoes, []int{14}) || !slices.Equal(r.Questoes[0].Apoios, []string{"r3-t1"}) {
		t.Fatalf("ligação = %v / %v; as questões ligadas são do curador", a.Questoes, r.Questoes[0].Apoios)
	}
	if a.Aviso != "Para responder às questões de números 14 a 17" || !reflect.DeepEqual(a.Origens, []Origem{trecho}) {
		t.Fatalf("aviso %q, origens %+v", a.Aviso, a.Origens)
	}
	if !contem(r.Alertas, "veio do texto do PDF") {
		t.Fatalf("o aviso da leitura sumiu: %v", r.Alertas)
	}

	// Sem nenhuma questão ligada, ganha as que o aviso lido cita.
	r = rascunho()
	r.Apoios[0].Questoes = nil
	r.AplicarTrechoDeApoio("r3-t1", lido)
	if !slices.Equal(r.Apoios[0].Questoes, []int{14, 15, 16, 17}) || !slices.Equal(r.Questoes[0].Apoios, []string{"r3-t1"}) {
		t.Fatalf("texto sem questões: %v / %v", r.Apoios[0].Questoes, r.Questoes[0].Apoios)
	}

	// Leitura vazia, ou texto que não existe mais: fica como estava, e avisa.
	r = rascunho()
	antes := r.Apoios[0]
	r.AplicarTrechoDeApoio("r3-t1", Rascunho{Apoios: []Apoio{{Blocos: []Bloco{{Tipo: "texto", Texto: ""}}}}})
	if !reflect.DeepEqual(r.Apoios[0], antes) || !contem(r.Alertas, "não achou texto para o texto de apoio das questões 14") {
		t.Fatalf("leitura vazia: %+v, %v", r.Apoios[0], r.Alertas)
	}
	r.AplicarTrechoDeApoio("sumiu", lido)
	if !contem(r.Alertas, "não está mais no rascunho") {
		t.Fatalf("texto removido: %v", r.Alertas)
	}
	r.TrechoDeApoioRecusado("r3-t1", "a IA se recusou")
	if !contem(r.Alertas, "O trecho marcado para o texto de apoio das questões 14 não foi lido (a IA se recusou)") {
		t.Fatalf("recusa: %v", r.Alertas)
	}
}

func TestAplicarTrecho_TrocaAQuestao(t *testing.T) {
	t.Parallel()

	antiga := questao(2, true, "lida sem as alternativas")
	antiga.Alternativas = antiga.Alternativas[:2]
	antiga.Disciplina, antiga.Resposta, antiga.Situacao = "Redes", "C", "Gabarito sem alteração"
	antiga.IgualA = "TJCE 2026 · E05, questão 2"
	r := Rascunho{
		Questoes: []Questao{questao(1, true, "a primeira"), antiga},
		Apoios:   []Apoio{{ID: "r0-t1", Questoes: []int{1, 2}}},
		Gabarito: Gabarito{Respostas: map[string]string{"2": "D"}},
	}
	relida := questao(2, true, "inteira")
	relida.Blocos = append(relida.Blocos, Bloco{Tipo: "texto"}) // vazio: sai
	relida.Apoios = []string{"rt2-t1"}
	vizinha := questao(3, false, "a ponta da vizinha")

	r.AplicarTrecho(2, Rascunho{
		Extracoes: []Extracao{{Regiao: "t2"}},
		Questoes:  []Questao{vizinha, relida},
		Apoios:    []Apoio{{ID: "rt2-t1"}},
	})

	q := r.Questoes[1]
	if len(r.Questoes) != 2 || q.Blocos[0].Texto != "inteira" || len(q.Blocos) != 1 || len(q.Alternativas) != 5 {
		t.Fatalf("questões = %+v, quer a 2 trocada pela leitura e a vizinha fora", r.Questoes)
	}
	if q.Revisada || q.IgualA != "" || q.Disciplina != "Redes" || q.Resposta != "C" || q.Situacao == "" {
		t.Fatalf("questão 2 = %+v, quer sem conferência, própria, com matéria e resposta da antiga", q)
	}
	if len(q.Apoios) != 1 || q.Apoios[0] != "r0-t1" || len(r.Apoios) != 1 {
		t.Fatalf("textos = %v / %+v, quer só o r0-t1 do rascunho", q.Apoios, r.Apoios)
	}
	if len(r.Extracoes) != 1 || len(r.Alertas) != 0 {
		t.Fatalf("extrações = %v, alertas = %v", r.Extracoes, r.Alertas)
	}
}

// Sem o número dentro do trecho, o modelo pode chutar outro: a única questão
// lida é a pedida. Questão que faltou entra na ordem, com a resposta do
// gabarito e a matéria da anterior.
// Relida pelo trecho que o curador marcou, a questão é a leitura da IA.
func TestAplicarTrecho_TiraAMarcaDoOCR(t *testing.T) {
	t.Parallel()

	doOCR := questao(4, true, "do OCR")
	doOCR.LidaPorOCR = true
	r := Rascunho{Questoes: []Questao{doOCR}}

	r.AplicarTrecho(4, Rascunho{Questoes: []Questao{questao(4, true, "da IA")}})

	if q := r.Questoes[0]; q.LidaPorOCR || q.Blocos[0].Texto != "da IA" {
		t.Fatalf("questão = %+v", q)
	}
}

func TestAplicarTrecho_QuestaoQueFaltou(t *testing.T) {
	t.Parallel()

	primeira, terceira := questao(1, true, "a"), questao(3, true, "c")
	primeira.Disciplina = "Português"
	r := Rascunho{
		Questoes: []Questao{primeira, terceira},
		Gabarito: Gabarito{Respostas: map[string]string{"2": "B"}},
	}

	r.AplicarTrecho(2, Rascunho{Questoes: []Questao{questao(12, true, "a que faltou")}})

	if len(r.Questoes) != 3 || r.Questoes[1].Numero != 2 || r.Questoes[1].Blocos[0].Texto != "a que faltou" {
		t.Fatalf("questões = %+v, quer a 2 entre a 1 e a 3", r.Questoes)
	}
	if q := r.Questoes[1]; q.Resposta != "B" || q.Disciplina != "Português" {
		t.Fatalf("questão 2 = %+v, quer resposta B e a matéria da 1", q)
	}
}

// Marcar o trecho da questão excluída é querê-la de volta: sem isso, ela
// estaria na prova e entre as excluídas, e a publicação travaria.
func TestAplicarTrecho_ExcluidaVoltaAProva(t *testing.T) {
	t.Parallel()

	r := Rascunho{Questoes: []Questao{questao(1, true, "a")}, Excluidas: []int{2, 3}}

	r.AplicarTrecho(2, Rascunho{Questoes: []Questao{questao(2, true, "a excluída")}})

	if len(r.Questoes) != 2 || !slices.Equal(r.Excluidas, []int{3}) {
		t.Fatalf("questões %d, excluídas %v; quer a 2 de volta e só a 3 excluída", len(r.Questoes), r.Excluidas)
	}
}

// O caso da questão 60 do TRT-15: ela veio sem alternativas, e o curador marca
// só elas. Sem o número no trecho, o modelo devolveu "questão 1", sem
// enunciado e cortada — o enunciado que existe fica, as alternativas entram
// pela letra, e a questão está inteira.
func TestAplicarTrecho_SoAsAlternativasQueFaltaram(t *testing.T) {
	t.Parallel()

	sem := questao(60, false, "O padrão de projeto mais adequado é o")
	sem.Alternativas = sem.Alternativas[:1]
	sem.Alternativas[0].Blocos[0].Texto = "Singleton lido errado"
	sem.Origens = []Origem{{Pagina: 15, Regiao: "14", Retangulo: []float64{58, 650, 572, 702}}}
	r := Rascunho{Questoes: []Questao{sem}}
	pedaco := questao(1, false, "")
	pedaco.Blocos = nil
	pedaco.Alternativas[0].Blocos[0].Texto = "Singleton."
	pedaco.Origens = []Origem{{Pagina: 15, Regiao: "t60", Retangulo: []float64{40, 704, 580, 788}}}

	r.AplicarTrecho(60, Rascunho{Questoes: []Questao{pedaco}})

	q := r.Questoes[0]
	if q.Numero != 60 || q.Blocos[0].Texto != "O padrão de projeto mais adequado é o" {
		t.Fatalf("questão = %+v, quer a 60 com o enunciado que tinha", q)
	}
	if len(q.Alternativas) != 5 || q.Alternativas[0].Blocos[0].Texto != "Singleton." || !q.Completa || q.Revisada {
		t.Fatalf("alternativas = %+v, completa %v; quer as cinco do trecho, inteira e por conferir", q.Alternativas, q.Completa)
	}
	if got := regioesDe(q.Origens); strings.Join(got, ",") != "14,t60" {
		t.Fatalf("origens = %v, quer o enunciado e o trecho", got)
	}
	if len(r.Alertas) != 0 {
		t.Fatalf("alertas = %v", r.Alertas)
	}
}

// Só o enunciado marcado: as alternativas que estavam certas ficam.
func TestAplicarTrecho_SoOEnunciado(t *testing.T) {
	t.Parallel()

	r := Rascunho{Questoes: []Questao{questao(7, true, "enunciado truncado")}}
	lida := questao(7, false, "enunciado inteiro")
	lida.Alternativas = nil

	r.AplicarTrecho(7, Rascunho{Questoes: []Questao{lida}})

	if q := r.Questoes[0]; q.Blocos[0].Texto != "enunciado inteiro" || len(q.Alternativas) != 5 || !q.Completa {
		t.Fatalf("questão = %+v, quer o enunciado novo com as alternativas de antes", q)
	}
	if len(r.Alertas) != 0 {
		t.Fatalf("alertas = %v; com as cinco alternativas, nada a avisar", r.Alertas)
	}
}

// O caso da 60 do TRT-15 em staging: o trecho ficou do tamanho do enunciado,
// onde a questão tinha sido lida, e não chegou às alternativas. A leitura não
// trouxe nenhuma, e sem aviso parecia que ela não tinha acontecido.
func TestAplicarTrecho_QueNaoChegouAsAlternativasAvisa(t *testing.T) {
	t.Parallel()

	semAlternativas := questao(60, false, "O padrão de projeto mais adequado é o")
	semAlternativas.Alternativas = nil
	r := Rascunho{Questoes: []Questao{semAlternativas}}
	soEnunciado := questao(60, false, "O padrão de projeto mais adequado é o")
	soEnunciado.Alternativas = nil

	r.AplicarTrecho(60, Rascunho{Questoes: []Questao{soEnunciado}})

	if !contem(r.Alertas, "questão 60 continua com 0 de 5 alternativas") || r.Questoes[0].Completa {
		t.Fatalf("alertas = %v, completa = %v; quer o aviso para marcar até a (E)", r.Alertas, r.Questoes[0].Completa)
	}
}

// Trecho que não achou a questão — pegou outras, ou nada — não mexe nela.
func TestAplicarTrecho_NaoAchouAQuestao(t *testing.T) {
	t.Parallel()

	for nome, lido := range map[string]Rascunho{
		"outras questões": {Questoes: []Questao{questao(1, true, "a"), questao(3, true, "c")}},
		"nada lido":       {},
		"questão vazia":   {Questoes: []Questao{{Numero: 2, Completa: true}}},
	} {
		r := Rascunho{Questoes: []Questao{questao(2, false, "enunciado")}}
		antes := r.Questoes[0]

		r.AplicarTrecho(2, lido)

		if !reflect.DeepEqual(r.Questoes[0], antes) || !contem(r.Alertas, "não achou a questão 2") {
			t.Errorf("%s: questão = %+v, alertas = %v", nome, r.Questoes[0], r.Alertas)
		}
	}
}
