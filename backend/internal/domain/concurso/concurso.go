// Package concurso guarda o catálogo da prova: as disciplinas, seus temas, os
// marcos do edital e o conteúdo programático. É a entrada que o motor do plano
// consome — dado puro, sem infraestrutura.
package concurso

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNaoEncontrado é devolvido quando nenhum concurso bate com o slug ou id.
	ErrNaoEncontrado = errors.New("concurso não encontrado")
	// As invariantes do cadastro de um concurso.
	ErrNomeObrigatorio   = errors.New("informe o nome do concurso")
	ErrProvaObrigatoria  = errors.New("informe a data da prova")
	ErrSemDisciplina     = errors.New("cadastre ao menos uma disciplina")
	ErrDisciplinaSemNome = errors.New("toda disciplina precisa de um nome")
	ErrBlocoInvalido     = errors.New(`bloco da disciplina deve ser "esp" ou "ger"`)
	ErrSemPontos         = errors.New("some ao menos uma questão entre as disciplinas")
)

// RetaPadraoDiasPadrao é quanto dura a reta final quando o cadastro não diz.
// Abaixo de RetaPadraoDiasMinimo a reta não comporta nem uma semana de revisão
// dirigida, então o valor é corrigido para o padrão.
const (
	RetaPadraoDiasPadrao  = 28
	RetaPadraoDiasMinimo  = 7
	TotalCoresDisciplinas = 13
)

// Bloco é o grupo de questões a que uma disciplina pertence.
type Bloco string

const (
	BlocoEspecifico Bloco = "esp"
	BlocoGeral      Bloco = "ger"
)

// Peso é quanto vale uma questão em cada bloco. É a regra que faz o cronograma
// dar mais dias às específicas, e mora só aqui.
var Peso = map[Bloco]int{
	BlocoEspecifico: 2,
	BlocoGeral:      1,
}

// BlocoValido converte um texto em Bloco, caindo em BlocoEspecifico quando o
// valor não é reconhecido.
func BlocoValido(s string) Bloco {
	if b := Bloco(strings.ToLower(strings.TrimSpace(s))); b == BlocoGeral {
		return BlocoGeral
	}

	return BlocoEspecifico
}

// PesoDe é o peso efetivo de uma disciplina: o informado, quando positivo, ou o
// do bloco a que ela pertence.
func PesoDe(bloco Bloco, informado int) int {
	if informado > 0 {
		return informado
	}

	return Peso[bloco]
}

// TiposConteudo são as formas que um item do conteúdo programático assume na
// tela: ficha, rótulo, cabeçalho e parágrafo.
var TiposConteudo = map[string]bool{"ficha": true, "rot": true, "h": true, "p": true}

// TipoConteudoValido normaliza o tipo de um item, caindo em parágrafo.
func TipoConteudoValido(s string) string {
	if t := strings.TrimSpace(s); TiposConteudo[t] {
		return t
	}

	return "p"
}

// TiposFonte são as origens de estudo que uma disciplina pode listar. "questoes"
// é o banco de questões da disciplina, que o cronograma usa para abrir o
// treino do dia.
var TiposFonte = map[string]bool{
	"lei": true, "jurisprudencia": true, "material": true, "link": true, "questoes": true,
}

// TipoFonteValido normaliza o tipo de uma fonte, caindo em "lei".
func TipoFonteValido(s string) string {
	if t := strings.ToLower(strings.TrimSpace(s)); TiposFonte[t] {
		return t
	}

	return "lei"
}

// Concurso é uma prova que o usuário cadastrou para montar um plano.
type Concurso struct {
	ID             uuid.UUID
	DonoID         uuid.UUID
	Slug           string
	Nome           string
	Banca          string
	Cargo          string
	Emoji          string
	ProvaPadrao    time.Time
	RetaPadraoDias int
	Resumo         string

	Disciplinas []Disciplina
	Marcos      []Marco
	Conteudo    []ConteudoItem
	// RevCiclo é a rotação de revisão semanal que o próprio edital sugere.
	// O plano pode sobrepô-la; sem nenhuma das duas, vale a rotação padrão.
	RevCiclo []ItemRevisao
}

// Disciplina é uma matéria com sua lista ordenada de temas e fontes de estudo.
//
// A identidade é o ID. O Codigo é o mnemônico que aparece em cada chip do
// cronograma ("DIRAD"), único dentro do concurso — mas quem edita o concurso
// não pode trocar a identidade da matéria, ou o histórico de estudo dela ficaria
// apontando para o nada.
type Disciplina struct {
	ID             uuid.UUID
	Codigo         string
	Nome           string
	Bloco          Bloco
	Peso           int
	QuestoesPadrao int
	Ordem          int
	// CadernoURL é um link opcional para onde o estudante guarda os erros desta
	// matéria (um caderno do TEC/Qconcursos, um documento). O bloco de revisão
	// do cronograma leva direto para lá.
	CadernoURL string
	Temas      []string
	Fontes     []Fonte
}

// Fonte é uma origem de estudo de uma disciplina — uma lei, uma
// jurisprudência, um PDF, um link. Alimenta o dossiê para o NotebookLM.
type Fonte struct {
	Ordem  int
	Titulo string
	URL    string
	Tipo   string
}

// Marco é uma data do cronograma oficial do edital.
type Marco struct {
	ID         uuid.UUID
	Ordem      int
	Rotulo     int
	DataInicio time.Time
	DataFim    *time.Time
	Titulo     string
	ExigeAcao  bool
	EProva     bool
}

// ConteudoItem é um bloco da página de conteúdo programático.
type ConteudoItem struct {
	Ordem int
	Tipo  string
	Texto string
}

// ItemRevisao é uma entrada da rotação de revisão semanal.
type ItemRevisao struct {
	Ordem    int
	Titulo   string
	Questoes int
}

// Normalizar aplica os padrões do cadastro e limpa o que veio pela borda: uma
// reta final curta demais volta ao padrão, o marco que cai na data da prova é
// reconhecido como tal, e tipos desconhecidos caem no valor seguro.
//
// É idempotente, e roda antes de Validar.
func (c *Concurso) Normalizar() {
	c.Nome = strings.TrimSpace(c.Nome)
	c.Banca = strings.TrimSpace(c.Banca)
	c.Cargo = strings.TrimSpace(c.Cargo)
	c.Resumo = strings.TrimSpace(c.Resumo)

	if c.RetaPadraoDias < RetaPadraoDiasMinimo {
		c.RetaPadraoDias = RetaPadraoDiasPadrao
	}

	for i := range c.Disciplinas {
		d := &c.Disciplinas[i]
		d.Nome = strings.TrimSpace(d.Nome)
		d.Bloco = BlocoValido(string(d.Bloco))
		d.Peso = PesoDe(d.Bloco, d.Peso)
		d.CadernoURL = strings.TrimSpace(d.CadernoURL)
		d.Ordem = i

		if d.QuestoesPadrao < 0 {
			d.QuestoesPadrao = 0
		}

		d.Temas = linhasLimpas(d.Temas)

		fontes := make([]Fonte, 0, len(d.Fontes))
		for _, f := range d.Fontes {
			f.Titulo = strings.TrimSpace(f.Titulo)
			f.URL = strings.TrimSpace(f.URL)

			if f.Titulo == "" && f.URL == "" {
				continue
			}

			f.Tipo = TipoFonteValido(f.Tipo)
			f.Ordem = len(fontes)
			fontes = append(fontes, f)
		}

		d.Fontes = fontes
	}

	marcos := make([]Marco, 0, len(c.Marcos))
	for _, m := range c.Marcos {
		if m.DataInicio.IsZero() {
			continue
		}

		m.Titulo = strings.TrimSpace(m.Titulo)
		// O marco da própria prova é o que cai na data dela — não é um campo do
		// formulário.
		m.EProva = !c.ProvaPadrao.IsZero() && m.DataInicio.Equal(c.ProvaPadrao)

		if m.DataFim != nil && m.DataFim.Before(m.DataInicio) {
			m.DataFim = nil
		}

		m.Ordem = len(marcos)
		marcos = append(marcos, m)
	}

	c.Marcos = marcos

	conteudo := make([]ConteudoItem, 0, len(c.Conteudo))
	for _, it := range c.Conteudo {
		it.Texto = strings.TrimSpace(it.Texto)
		if it.Texto == "" {
			continue
		}

		it.Tipo = TipoConteudoValido(it.Tipo)
		it.Ordem = len(conteudo)
		conteudo = append(conteudo, it)
	}

	c.Conteudo = conteudo

	c.atribuirCodigos()
}

// atribuirCodigos garante que toda disciplina tenha um mnemônico único dentro
// do concurso, PRESERVANDO o que as disciplinas já cadastradas têm.
//
// Regerar todos os códigos a cada edição é o que desligava atividades e
// registros da matéria: eles referenciam a disciplina, e um código novo em
// disciplina já existente equivale a trocar a matéria por outra. Só quem chega
// sem código ganha um.
func (c *Concurso) atribuirCodigos() {
	usados := make(map[string]bool, len(c.Disciplinas))

	for i := range c.Disciplinas {
		if cod := c.Disciplinas[i].Codigo; cod != "" {
			usados[cod] = true
		}
	}

	for i := range c.Disciplinas {
		d := &c.Disciplinas[i]
		if d.Codigo != "" {
			continue
		}

		d.Codigo = CodigoUnico(d.Nome, i, usados)
		usados[d.Codigo] = true
	}
}

// Validar confere as invariantes do cadastro. Roda depois de Normalizar.
func (c *Concurso) Validar() error {
	if c.Nome == "" {
		return ErrNomeObrigatorio
	}

	if c.ProvaPadrao.IsZero() {
		return ErrProvaObrigatoria
	}

	if len(c.Disciplinas) == 0 {
		return ErrSemDisciplina
	}

	pontos := 0

	for _, d := range c.Disciplinas {
		if d.Nome == "" {
			return ErrDisciplinaSemNome
		}

		if d.Bloco != BlocoEspecifico && d.Bloco != BlocoGeral {
			return ErrBlocoInvalido
		}

		pontos += d.QuestoesPadrao * Peso[d.Bloco]
	}

	if pontos == 0 {
		return ErrSemPontos
	}

	return nil
}

// DisciplinaPorCodigo devolve um ponteiro para a disciplina com o código dado,
// ou nil.
func (c *Concurso) DisciplinaPorCodigo(codigo string) *Disciplina {
	for i := range c.Disciplinas {
		if c.Disciplinas[i].Codigo == codigo {
			return &c.Disciplinas[i]
		}
	}

	return nil
}

// DisciplinaPorID devolve um ponteiro para a disciplina com o id dado, ou nil.
func (c *Concurso) DisciplinaPorID(id uuid.UUID) *Disciplina {
	for i := range c.Disciplinas {
		if c.Disciplinas[i].ID == id {
			return &c.Disciplinas[i]
		}
	}

	return nil
}

// MarcoPorID devolve um ponteiro para o marco com o id dado, ou nil.
func (c *Concurso) MarcoPorID(id uuid.UUID) *Marco {
	for i := range c.Marcos {
		if c.Marcos[i].ID == id {
			return &c.Marcos[i]
		}
	}

	return nil
}

// CorDisciplina mapeia a posição de uma disciplina para uma casa da paleta.
func (c *Concurso) CorDisciplina(codigo string) int {
	for i, d := range c.Disciplinas {
		if d.Codigo == codigo {
			return i % TotalCoresDisciplinas
		}
	}

	return 0
}

func linhasLimpas(xs []string) []string {
	out := make([]string, 0, len(xs))

	for _, x := range xs {
		if s := strings.TrimSpace(x); s != "" {
			out = append(out, s)
		}
	}

	return out
}
