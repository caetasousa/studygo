// Package concurso holds the exam catalogue: the disciplines, their topics, the
// official edital milestones and the programmatic content. It is the input the
// plano engine consumes — pure data, no infrastructure.
package concurso

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNotFound is returned when no concurso matches a slug or id.
	ErrNotFound = errors.New("concurso não encontrado")
	// ErrNomeObrigatorio / ErrProvaObrigatoria / ErrSemDisciplina / ErrSemPontos
	// are the concurso-registration invariants.
	ErrNomeObrigatorio   = errors.New("informe o nome do concurso")
	ErrProvaObrigatoria  = errors.New("informe a data da prova")
	ErrSemDisciplina     = errors.New("cadastre ao menos uma disciplina")
	ErrDisciplinaSemNome = errors.New("toda disciplina precisa de um nome")
	ErrBlocoInvalido     = errors.New(`bloco da disciplina deve ser "esp" ou "ger"`)
	ErrSemPontos         = errors.New("some ao menos uma questão entre as disciplinas")
)

// Bloco is the question group a discipline belongs to.
type Bloco string

const (
	BlocoEspecifico Bloco = "esp"
	BlocoGeral      Bloco = "ger"
)

// Peso is the points a single question is worth in each bloco.
var Peso = map[Bloco]int{
	BlocoEspecifico: 2,
	BlocoGeral:      1,
}

// Concurso is one exam a user registered to build a plan for.
type Concurso struct {
	ID             uuid.UUID
	OwnerID        uuid.UUID
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
	RevCiclo    []RevItem
}

// Disciplina is a subject with an ordered list of topics and study sources.
type Disciplina struct {
	ID             uuid.UUID
	Codigo         string
	Nome           string
	Bloco          Bloco
	Peso           int
	QuestoesPadrao int
	Ordem          int
	Temas          []string
	Fontes         []Fonte
}

// Fonte is a study source for a discipline — a law, a piece of jurisprudence, a
// PDF, a link. Feeds the NotebookLM hand-off dossier.
type Fonte struct {
	Ordem  int
	Titulo string
	URL    string
	Tipo   string // "lei" | "jurisprudencia" | "material" | "link"
}

// Marco is a dated milestone from the official schedule.
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

// ConteudoItem is one block of the programmatic-content page. Tipo is one of
// "ficha", "rot", "h", "p".
type ConteudoItem struct {
	Ordem int
	Tipo  string
	Texto string
}

// RevItem is one entry of the weekly review cycle used in the base phase.
type RevItem struct {
	Ordem    int
	Titulo   string
	Questoes int
}

// Validar checks the registration invariants. It is called by the service after
// mapping the wire input to this domain type.
func (c *Concurso) Validar() error {
	if strings.TrimSpace(c.Nome) == "" {
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
		if strings.TrimSpace(d.Nome) == "" {
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

// DisciplinaByCodigo returns a pointer to the discipline with the given code, or
// nil.
func (c *Concurso) DisciplinaByCodigo(codigo string) *Disciplina {
	for i := range c.Disciplinas {
		if c.Disciplinas[i].Codigo == codigo {
			return &c.Disciplinas[i]
		}
	}

	return nil
}

// MarcoByID returns a pointer to the milestone with the given id, or nil.
func (c *Concurso) MarcoByID(id uuid.UUID) *Marco {
	for i := range c.Marcos {
		if c.Marcos[i].ID == id {
			return &c.Marcos[i]
		}
	}

	return nil
}

// CorDisciplina maps a discipline's position to a palette slot 0..12, matching
// the artifact's TAGIDX.
func (c *Concurso) CorDisciplina(codigo string) int {
	for i, d := range c.Disciplinas {
		if d.Codigo == codigo {
			return i % 13
		}
	}

	return 0
}
