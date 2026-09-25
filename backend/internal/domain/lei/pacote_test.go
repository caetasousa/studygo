package lei

import (
	"strings"
	"testing"
)

// Como um pacote de lei pode chegar errado — escrito antes do código. O pacote
// é montado na máquina de quem estuda (captura + questões do Claude Code) e
// importado em produção; o que passar daqui vai para a tela do estudante.
//
//	P1  sem formato, formato de outra versão, sem slug, slug fora do padrão, sem nome ou sem versão
//	P2  nenhum dispositivo
//	P3  ref vazia ou repetida — o link direto abriria o dispositivo errado
//	P4  pai que não existe, ou que vem depois do filho — a árvore não fecha
//	P5  tipo desconhecido
//	P6  unidade que cita dispositivo inexistente, ou ref de unidade repetida
//	P7  hash da unidade diferente do texto atual — questão feita sobre outra redação
//	P8  questão com chave vazia ou repetida — a próxima importação não a reconheceria
//	P9  questão sem as 5 alternativas, com alternativa vazia ou repetida
//	P10 gabarito fora de A–E
//	P11 questão que cita dispositivo inexistente, ou fora da própria unidade
//	P12 trecho que não está, literalmente, nos dispositivos citados
//	P13 questão numa unidade que não existe
//	P14 enunciado ou comentário vazio

func pacoteValido() Pacote {
	ds := []Dispositivo{
		{Ref: "cap1", Tipo: "capitulo", Rotulo: "CAPÍTULO I", Texto: "CAPÍTULO I", Nome: "DO CONTROLE"},
		{Ref: "art1", Pai: "cap1", Tipo: "artigo", Rotulo: "Art. 1º", Texto: "Art. 1º Compete ao Tribunal:"},
		{Ref: "art1.inc1", Pai: "art1", Tipo: "inciso", Rotulo: "I", Texto: "I - julgar as contas dos administradores;"},
		{Ref: "art2", Pai: "cap1", Tipo: "artigo", Rotulo: "Art. 2º", Texto: "Art. 2º O   Tribunal tem sede na Capital."},
	}
	u := Unidade{Ref: "u1", Titulo: "Arts. 1º e 2º", Dispositivos: []string{"art1", "art2"}}
	u.Hash = HashUnidade(ds, u.Dispositivos)

	return Pacote{
		Formato:      Formato,
		Lei:          Lei{Slug: "lei-teste", Nome: "Lei de Teste", Curto: "Lei Teste"},
		Versao:       "v1",
		Dispositivos: ds,
		Unidades:     []Unidade{u},
		Questoes: []Questao{{
			Chave:        "q1",
			Unidade:      "u1",
			Enunciado:    "Compete ao Tribunal:",
			Alternativas: []string{"julgar contas", "legislar", "sancionar", "vetar", "promulgar"},
			Gabarito:     "A",
			Comentario:   "Art. 1º, I.",
			Dispositivos: []string{"art1.inc1"},
			Trecho:       "julgar as contas dos administradores",
		}},
	}
}

func TestValidar_PacoteBomPassa(t *testing.T) {
	if err := pacoteValido().Validar(); err != nil {
		t.Fatalf("pacote válido recusado: %v", err)
	}
}

func TestValidar_TrechoIgnoraDiferencaDeEspacos(t *testing.T) {
	// P12: o texto da lei pode ter espaço duplo; quem escreve o trecho, não.
	p := pacoteValido()
	p.Questoes[0].Dispositivos = []string{"art2"}
	p.Questoes[0].Trecho = "O Tribunal tem sede"
	if err := p.Validar(); err != nil {
		t.Fatalf("trecho com espaços normalizados recusado: %v", err)
	}
}

func TestValidar_TrechoPodeEstarNumDescendenteDoCitado(t *testing.T) {
	p := pacoteValido()
	p.Questoes[0].Dispositivos = []string{"art1"}
	if err := p.Validar(); err != nil {
		t.Fatalf("trecho do inciso citado pelo artigo recusado: %v", err)
	}
}

func TestValidar_Recusa(t *testing.T) {
	casos := []struct {
		id     string
		mudar  func(p *Pacote)
		motivo string
	}{
		{"P1 formato", func(p *Pacote) { p.Formato = "studygo.lei/0" }, "formato"},
		{"P1 slug", func(p *Pacote) { p.Lei.Slug = "Lei Teste" }, "slug"},
		{"P1 nome", func(p *Pacote) { p.Lei.Nome = " " }, "nome"},
		{"P1 versão", func(p *Pacote) { p.Versao = "" }, "versão"},
		{"P2", func(p *Pacote) { p.Dispositivos = nil; p.Unidades = nil; p.Questoes = nil }, "nenhum dispositivo"},
		{"P3 vazia", func(p *Pacote) { p.Dispositivos[3].Ref = "" }, "ref vazia"},
		{"P3 repetida", func(p *Pacote) { p.Dispositivos[3].Ref = "art1" }, "ref repetida"},
		{"P4 inexistente", func(p *Pacote) { p.Dispositivos[2].Pai = "art9" }, "pai"},
		{"P4 depois", func(p *Pacote) { p.Dispositivos[1].Pai = "art2" }, "pai"},
		{"P5", func(p *Pacote) { p.Dispositivos[2].Tipo = "inc" }, "tipo"},
		{"P6 inexistente", func(p *Pacote) { p.Unidades[0].Dispositivos = []string{"art9"} }, "art9"},
		{"P6 repetida", func(p *Pacote) { p.Unidades = append(p.Unidades, p.Unidades[0]) }, "unidade repetida"},
		{"P7", func(p *Pacote) { p.Dispositivos[2].Texto = "I - julgar as contas;" }, "desatualizada"},
		{"P8 vazia", func(p *Pacote) { p.Questoes[0].Chave = "" }, "chave"},
		{"P8 repetida", func(p *Pacote) { p.Questoes = append(p.Questoes, p.Questoes[0]) }, "repetida"},
		{"P9 quatro", func(p *Pacote) { p.Questoes[0].Alternativas = p.Questoes[0].Alternativas[:4] }, "5 alternativas"},
		{"P9 vazia", func(p *Pacote) { p.Questoes[0].Alternativas[2] = " " }, "alternativa"},
		{"P9 repetida", func(p *Pacote) { p.Questoes[0].Alternativas[4] = "legislar" }, "alternativa"},
		{"P10", func(p *Pacote) { p.Questoes[0].Gabarito = "F" }, "gabarito"},
		{"P11 inexistente", func(p *Pacote) { p.Questoes[0].Dispositivos = []string{"art9"} }, "art9"},
		{"P11 fora da unidade", func(p *Pacote) { p.Questoes[0].Dispositivos = []string{"cap1"} }, "fora da unidade"},
		{"P11 nenhum", func(p *Pacote) { p.Questoes[0].Dispositivos = nil }, "dispositivo"},
		{"P12", func(p *Pacote) { p.Questoes[0].Trecho = "julgar as contas do Governador" }, "trecho"},
		{"P12 vazio", func(p *Pacote) { p.Questoes[0].Trecho = "" }, "trecho"},
		{"P13", func(p *Pacote) { p.Questoes[0].Unidade = "u9" }, "unidade"},
		{"P14 enunciado", func(p *Pacote) { p.Questoes[0].Enunciado = "" }, "enunciado"},
		{"P14 comentário", func(p *Pacote) { p.Questoes[0].Comentario = "" }, "comentário"},
	}

	for _, c := range casos {
		t.Run(c.id, func(t *testing.T) {
			p := pacoteValido()
			c.mudar(&p)

			err := p.Validar()
			if err == nil {
				t.Fatal("pacote inválido aceito")
			}
			if !strings.Contains(err.Error(), c.motivo) {
				t.Fatalf("erro %q não fala de %q", err, c.motivo)
			}
		})
	}
}
