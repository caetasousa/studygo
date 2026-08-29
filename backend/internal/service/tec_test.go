package service_test

import (
	"strings"
	"testing"

	"annygo/internal/domain/concurso"
	"annygo/internal/service"
)

func concursoTEC() concurso.Concurso {
	return concurso.Concurso{
		Disciplinas: []concurso.Disciplina{
			{
				Codigo: "D01",
				Nome:   "Língua Portuguesa",
				Bloco:  concurso.BlocoGeral,
				Temas:  []string{"Crase", "Concordância verbal"},
			},
			{
				Codigo: "D02",
				Nome:   "Controle Externo",
				Bloco:  concurso.BlocoEspecifico,
				Temas:  []string{"Tomada de contas especial"},
			},
		},
	}
}

func TestLerPlanilhaTEC(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome     string
		csv      string
		quer     int
		querErro bool
	}{
		{
			nome: "separador ponto e vírgula com BOM",
			csv:  "\ufeffAssunto;Questões;Acertos\nCrase;25;18\nConcordância verbal;15;14\n",
			quer: 2,
		},
		{
			nome: "separador vírgula",
			csv:  "Assunto,Questoes,Acertos\nCrase,25,18\n",
			quer: 1,
		},
		{
			nome: "cabeçalhos alternativos",
			csv:  "Matéria;Resolvidas;Certas\nCrase;10;7\n",
			quer: 1,
		},
		{
			nome: "milhar com separador",
			csv:  "Assunto;Questões;Acertos\nCrase;1.250;900\n",
			quer: 1,
		},
		{
			nome: "linha sem questões é ignorada",
			csv:  "Assunto;Questões;Acertos\nCrase;25;18\nVazio;0;0\n",
			quer: 1,
		},
		{
			nome:     "sem a coluna de acertos",
			csv:      "Assunto;Questões\nCrase;25\n",
			querErro: true,
		},
		{
			nome:     "planilha vazia",
			csv:      "Assunto;Questões;Acertos\n",
			querErro: true,
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			got, err := service.LerPlanilhaTEC(strings.NewReader(c.csv))

			if c.querErro {
				if err == nil {
					t.Fatal("queria erro, veio nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}

			if len(got) != c.quer {
				t.Fatalf("leu %d linhas, queria %d", len(got), c.quer)
			}
		})
	}
}

func TestLerPlanilhaTEC_clampAcertos(t *testing.T) {
	t.Parallel()

	got, err := service.LerPlanilhaTEC(strings.NewReader("Assunto;Questões;Acertos\nCrase;10;99\n"))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if got[0].Acertos != 10 {
		t.Errorf("acertos = %d, queria 10 — nunca acima do total", got[0].Acertos)
	}
}

func TestCasarTEC(t *testing.T) {
	t.Parallel()

	linhas := []service.LinhaTEC{
		{Assunto: "crase", Questoes: 25, Acertos: 18},              // tema, sem acento
		{Assunto: "Concordancia Verbal", Questoes: 10, Acertos: 4}, // tema, sem acento
		{Assunto: "Controle Externo", Questoes: 30, Acertos: 30},   // nome da disciplina
		{Assunto: "Astronomia", Questoes: 5, Acertos: 1},           // nada a ver
	}

	got := service.CasarTEC(concursoTEC(), linhas)

	if len(got.Casados) != 3 {
		t.Fatalf("casaram %d, queria 3", len(got.Casados))
	}

	if len(got.SemCorrespon) != 1 || got.SemCorrespon[0].Assunto != "Astronomia" {
		t.Fatalf("sem correspondência = %+v, queria só Astronomia", got.SemCorrespon)
	}

	// Os piores primeiro, para o caderno de erros começar por eles.
	if got.Casados[0].Assunto != "Concordancia Verbal" {
		t.Errorf("primeiro = %q, queria o pior aproveitamento", got.Casados[0].Assunto)
	}

	if got.Questoes != 65 || got.Acertos != 52 {
		t.Errorf("totais = %d/%d, queria 65/52 (só os casados)", got.Questoes, got.Acertos)
	}

	porAssunto := map[string]service.CasamentoTEC{}
	for _, c := range got.Casados {
		porAssunto[c.Assunto] = c
	}

	if c := porAssunto["crase"]; c.Disciplina != "D01" || c.Tema != "Crase" || c.Erros != 7 || c.Pct != 72 {
		t.Errorf("crase = %+v, queria D01/Crase com 7 erros e 72%%", c)
	}

	if c := porAssunto["Controle Externo"]; c.Disciplina != "D02" || c.Tema != "" {
		t.Errorf("controle externo = %+v, queria D02 casado pelo nome da disciplina", c)
	}
}
