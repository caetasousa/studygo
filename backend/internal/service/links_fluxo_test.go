package service

import (
	"context"
	"strings"
	"testing"

	"studygo/internal/domain/concurso"
)

// Os links da matéria: o caderno de erros e o notebook do NotebookLM.
//
// São editados do CRONOGRAMA, que é a tela em que o estudante acompanha o
// estudo — mandá-lo ao cadastro do concurso para colar uma URL o tira de onde
// ele está trabalhando. Por isso existe um caso de uso próprio, que não reenvia
// o concurso inteiro.

func TestPlanoService_AtualizarLinksDisciplina(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	ce.obter(t)

	svc := NewPlanoService(ce.deps)
	codigo := ce.concursos.c.Disciplinas[0].Codigo

	caderno := "https://www.tecconcursos.com.br/questoes/caderno/123"
	notebook := "https://notebooklm.google.com/notebook/abc-123"

	if _, err := svc.AtualizarLinksDisciplina(ctx, ce.usuario, ce.slug, codigo,
		concurso.Links{Caderno: caderno, Notebook: notebook},
	); err != nil {
		t.Fatalf("AtualizarLinksDisciplina: %v", err)
	}

	d := ce.concursos.c.Disciplinas[0]

	if d.CadernoURL != caderno {
		t.Errorf("caderno = %q, quer %q", d.CadernoURL, caderno)
	}

	if d.NotebookURL != notebook {
		t.Errorf("notebook = %q, quer %q", d.NotebookURL, notebook)
	}
}

// Os dois viajam juntos e descrevem o estado final: mandar o notebook vazio
// APAGA o notebook. É o que permite ao formulário oferecer "remover link" sem
// um verbo próprio para isso.
func TestPlanoService_AtualizarLinksDisciplina_vazioApaga(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	ce.obter(t)

	svc := NewPlanoService(ce.deps)
	codigo := ce.concursos.c.Disciplinas[0].Codigo

	if _, err := svc.AtualizarLinksDisciplina(ctx, ce.usuario, ce.slug, codigo,
		concurso.Links{Caderno: "https://tec/caderno", Notebook: "https://notebooklm/x"},
	); err != nil {
		t.Fatalf("primeira gravação: %v", err)
	}

	if _, err := svc.AtualizarLinksDisciplina(ctx, ce.usuario, ce.slug, codigo,
		concurso.Links{Caderno: "https://tec/caderno"},
	); err != nil {
		t.Fatalf("segunda gravação: %v", err)
	}

	d := ce.concursos.c.Disciplinas[0]

	if d.NotebookURL != "" {
		t.Errorf("notebook = %q, quer vazio", d.NotebookURL)
	}

	if d.CadernoURL != "https://tec/caderno" {
		t.Errorf("o caderno não devia ter sido afetado: %q", d.CadernoURL)
	}
}

// Espaço em volta de uma URL colada é o caso comum, não a exceção.
func TestPlanoService_AtualizarLinksDisciplina_aparaEspacos(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	ce.obter(t)

	svc := NewPlanoService(ce.deps)
	codigo := ce.concursos.c.Disciplinas[0].Codigo

	if _, err := svc.AtualizarLinksDisciplina(ctx, ce.usuario, ce.slug, codigo,
		concurso.Links{Notebook: "  https://notebooklm.google.com/notebook/abc  "},
	); err != nil {
		t.Fatalf("AtualizarLinksDisciplina: %v", err)
	}

	if got := ce.concursos.c.Disciplinas[0].NotebookURL; got != "https://notebooklm.google.com/notebook/abc" {
		t.Errorf("notebook = %q, quer sem espaços", got)
	}
}

func TestPlanoService_AtualizarLinksDisciplina_materiaInexistente(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	ce.obter(t)

	svc := NewPlanoService(ce.deps)

	_, err := svc.AtualizarLinksDisciplina(ctx, ce.usuario, ce.slug, "NAOEXISTE",
		concurso.Links{Caderno: "https://x"},
	)

	var validacao ErrValidacao
	if !asErro(err, &validacao) {
		t.Fatalf("erro = %v, quer ErrValidacao", err)
	}
}

// O notebook viaja no CSV pelo mesmo motivo que a tag e o caderno: é trabalho
// que ninguém quer refazer numa instalação nova. Sem isto, exportar e
// reimportar apagaria o link em silêncio.
func TestPlanilha_NotebookViajaNoCSV(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	ce.obter(t)

	notebook := "https://notebooklm.google.com/notebook/xyz-789"
	ce.concursos.c.Disciplinas[0].NotebookURL = notebook

	svc := NewPlanilhaService(ce.deps)

	csv, err := svc.CSV(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("CSV: %v", err)
	}

	if !strings.Contains(string(csv), notebook) {
		t.Fatal("o link do notebook não saiu na exportação")
	}

	// Instalação nova: a matéria voltou sem link nenhum.
	ce.concursos.c.Disciplinas[0].NotebookURL = ""

	if _, err := svc.ImportarCSV(ctx, ce.usuario, ce.slug, ImportarPlanilhaCommand{
		CSV: string(csv), Confirmar: true,
	}); err != nil {
		t.Fatalf("importar: %v", err)
	}

	if got := ce.concursos.c.Disciplinas[0].NotebookURL; got != notebook {
		t.Errorf("notebook na volta = %q, quer %q", got, notebook)
	}
}

// Uma planilha exportada ANTES desta coluna não tem `materia_notebook`. Ela
// precisa continuar importável, e não pode apagar o link de quem já tem um: a
// coluna ausente chega vazia, e vazio é "não mexer".
func TestPlanilha_CSVAntigoNaoApagaONotebook(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	ce.obter(t)

	svc := NewPlanilhaService(ce.deps)

	csv, err := svc.CSV(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("CSV: %v", err)
	}

	// Simula a planilha antiga removendo a coluna do cabeçalho e o valor das
	// linhas de matéria.
	antigo := removerColunaNotebook(string(csv))

	notebook := "https://notebooklm.google.com/notebook/preservar"
	ce.concursos.c.Disciplinas[0].NotebookURL = notebook

	if _, err := svc.ImportarCSV(ctx, ce.usuario, ce.slug, ImportarPlanilhaCommand{
		CSV: antigo, Confirmar: true,
	}); err != nil {
		t.Fatalf("importando planilha antiga: %v", err)
	}

	if got := ce.concursos.c.Disciplinas[0].NotebookURL; got != notebook {
		t.Errorf("notebook = %q; uma planilha antiga não pode apagá-lo", got)
	}
}

// removerColunaNotebook desfaz a coluna nova, coluna e valor, para produzir uma
// planilha no formato anterior.
func removerColunaNotebook(csv string) string {
	linhas := strings.Split(csv, "\n")
	coluna := -1

	for i, linha := range linhas {
		campos := strings.Split(linha, ",")

		if coluna == -1 {
			for j, c := range campos {
				if strings.Trim(c, `"`) == "materia_notebook" {
					coluna = j

					break
				}
			}

			if coluna == -1 {
				continue
			}
		}

		if coluna >= len(campos) {
			continue
		}

		linhas[i] = strings.Join(append(campos[:coluna:coluna], campos[coluna+1:]...), ",")
	}

	return strings.Join(linhas, "\n")
}
