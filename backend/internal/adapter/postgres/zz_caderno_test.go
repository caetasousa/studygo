//go:build integration

package postgres_test

import (
	"testing"

	"studygo/internal/domain/concurso"
)

// O caminho que grava o link do caderno de erros da matéria — o que a tela de
// registro chama quando o estudante cola a URL do TEC.
func TestReproDefinirCadernoURL(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "cad@b.c")
	c := r.criarConcurso(t, u, "tce-go")

	url := "https://www.tecconcursos.com.br/questoes/caderno/123"

	if err := r.concursos.DefinirLinks(
		t.Context(), c.ID, c.Disciplinas[0].Codigo, concurso.Links{Caderno: url},
	); err != nil {
		t.Fatalf("DefinirLinks: %v", err)
	}

	lido, err := r.concursos.PorID(t.Context(), c.ID)
	if err != nil {
		t.Fatalf("PorID: %v", err)
	}

	t.Logf("depois de gravar: %q", lido.Disciplinas[0].CadernoURL)

	// E agora um salvamento do concurso vindo do formulário, como o de editar.
	salvo := lido
	salvo.Disciplinas[0].Nome = "Língua Portuguesa (editada)"

	if _, err := r.concursos.Atualizar(t.Context(), salvo); err != nil {
		t.Fatalf("Atualizar: %v", err)
	}

	depois, err := r.concursos.PorID(t.Context(), c.ID)
	if err != nil {
		t.Fatalf("PorID: %v", err)
	}

	t.Logf("depois de editar o concurso: %q", depois.Disciplinas[0].CadernoURL)

	if depois.Disciplinas[0].CadernoURL != url {
		t.Error("o link do caderno se perdeu ao salvar o concurso")
	}
}
