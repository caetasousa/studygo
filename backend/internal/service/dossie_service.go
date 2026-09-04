package service

import (
	"context"
	"fmt"
	"strings"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// DossieService monta o documento de estudo de uma disciplina para colar como
// fonte no NotebookLM: a ementa, as leis e materiais cadastrados, e o caderno de
// erros do estudante naquela matéria.
type DossieService struct {
	carregador

	repo port.CadernoRepository
}

func NewDossieService(deps Dependencias) *DossieService {
	return &DossieService{carregador: deps.carregador(), repo: deps.Caderno}
}

// Dossie é o documento pronto, mais os links soltos para quem preferir
// adicioná-los um a um.
type Dossie struct {
	Disciplina string
	Markdown   string
	Fontes     []FonteDoDossie
}

type FonteDoDossie struct {
	Titulo string
	URL    string
}

func (s *DossieService) Dossie(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug, codigo string,
) (Dossie, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return Dossie{}, err
	}

	d := c.Concurso.DisciplinaPorCodigo(codigo)
	if d == nil {
		return Dossie{}, concurso.ErrNaoEncontrado
	}

	anotacoes, err := s.repo.Anotacoes(ctx, c.Plano.ID)
	if err != nil {
		return Dossie{}, err
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# %s — %s\n\n", d.Nome, c.Concurso.Nome)
	b.WriteString("Documento de estudo para colar como fonte no NotebookLM.\n\n")

	escreverEmenta(&b, *d)
	fontes := escreverFontes(&b, *d)
	escreverCaderno(&b, c.Concurso, anotacoes, codigo)

	b.WriteString("\n---\n")
	b.WriteString("No NotebookLM: crie um notebook, cole este texto e os links acima como fontes, ")
	b.WriteString("e peça um Guia de Estudos e um Áudio Overview em português.\n")

	return Dossie{Disciplina: d.Nome, Markdown: b.String(), Fontes: fontes}, nil
}

func escreverEmenta(b *strings.Builder, d concurso.Disciplina) {
	b.WriteString("## Ementa\n\n")

	if len(d.Temas) == 0 {
		b.WriteString("_Nenhum tema cadastrado para esta disciplina._\n\n")

		return
	}

	for _, t := range d.Temas {
		fmt.Fprintf(b, "- %s\n", t)
	}

	b.WriteString("\n")
}

func escreverFontes(b *strings.Builder, d concurso.Disciplina) []FonteDoDossie {
	b.WriteString("## Fontes\n\n")

	if len(d.Fontes) == 0 {
		b.WriteString("_Nenhuma lei/material cadastrado._\n\n")

		return []FonteDoDossie{}
	}

	fontes := make([]FonteDoDossie, 0, len(d.Fontes))

	for _, f := range d.Fontes {
		fontes = append(fontes, FonteDoDossie{Titulo: f.Titulo, URL: f.URL})

		if f.URL != "" {
			fmt.Fprintf(b, "- [%s](%s)\n", f.Titulo, f.URL)
		} else {
			fmt.Fprintf(b, "- %s\n", f.Titulo)
		}
	}

	b.WriteString("\n")

	return fontes
}

func escreverCaderno(
	b *strings.Builder,
	cur concurso.Concurso,
	anotacoes []plano.Anotacao,
	codigo string,
) {
	b.WriteString("## Meu caderno de erros\n\n")

	d := cur.DisciplinaPorCodigo(codigo)
	if d == nil {
		return
	}

	escreveu := false

	for _, a := range anotacoes {
		if a.DisciplinaID == nil || *a.DisciplinaID != d.ID {
			continue
		}

		marca := "-"
		if a.Resolvido {
			marca = "- [x]"
		}

		prefixo := ""
		if a.Tema != "" {
			prefixo = "**" + a.Tema + "** — "
		}

		sufixo := ""
		if a.Origem != "" && a.Origem != plano.OrigemManual {
			sufixo = " _(" + string(a.Origem) + ")_"
		}

		fmt.Fprintf(b, "%s %s%s%s\n", marca, prefixo, a.Texto, sufixo)

		escreveu = true
	}

	if !escreveu {
		b.WriteString("_Ainda sem anotações para esta disciplina._\n")
	}
}
