//go:build integration

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"studygo/internal/domain/concurso"

	"github.com/google/uuid"
)

// O catálogo: round-trip do agregado, isolamento entre donos e a invariante que
// motivou a mudança de modelo — editar um concurso não pode trocar a identidade
// das disciplinas.

func TestConcursoRepo_RoundTripDoAgregado(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	criado := r.criarConcurso(t, u, "tce-go")

	lido, err := r.concursos.PorSlug(t.Context(), "tce-go")
	if err != nil {
		t.Fatalf("PorSlug: %v", err)
	}

	if lido.ID != criado.ID || lido.Nome != "TCE-GO" || lido.Banca != "FGV" {
		t.Errorf("campos do concurso não voltaram: %+v", lido)
	}

	// A data vem como `date`; o domínio a trata em UTC à meia-noite.
	if !lido.ProvaPadrao.Equal(dia(2026, time.December, 15)) {
		t.Errorf("prova = %v, quer 2026-12-15 UTC", lido.ProvaPadrao)
	}

	if len(lido.Disciplinas) != 2 {
		t.Fatalf("disciplinas = %d, quer 2", len(lido.Disciplinas))
	}

	// ORDER BY ordem: a primeira disciplina é a de ordem 0.
	if lido.Disciplinas[0].Codigo != "LINPO" || lido.Disciplinas[1].Codigo != "BANDA" {
		t.Errorf("ordem das disciplinas trocada: %s, %s",
			lido.Disciplinas[0].Codigo, lido.Disciplinas[1].Codigo)
	}

	// Temas e fontes vêm por consulta em lote e precisam cair na matéria certa.
	if got := lido.Disciplinas[0].Temas; len(got) != 2 || got[0] != "Crase" {
		t.Errorf("temas de LINPO = %v, quer [Crase Regência]", got)
	}

	if got := lido.Disciplinas[1].Temas; len(got) != 1 || got[0] != "SQL" {
		t.Errorf("temas de BANDA = %v, quer [SQL]", got)
	}

	if len(lido.Disciplinas[0].Fontes) != 1 {
		t.Errorf("fontes de LINPO = %d, quer 1", len(lido.Disciplinas[0].Fontes))
	}

	if len(lido.Marcos) != 1 || len(lido.Conteudo) != 1 {
		t.Errorf("marcos = %d, conteúdo = %d, quer 1 e 1",
			len(lido.Marcos), len(lido.Conteudo))
	}
}

// ListarPorDono só devolve o que é do dono. É a base do isolamento entre
// usuários: o service confia nisso para decidir 404.
func TestConcursoRepo_ListarPorDonoIsolaUsuarios(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)

	ana := r.criarUsuario(t, "ana@x.com")
	bruno := r.criarUsuario(t, "bruno@x.com")

	r.criarConcurso(t, ana, "da-ana")
	r.criarConcurso(t, bruno, "do-bruno")

	daAna, err := r.concursos.ListarPorDono(t.Context(), ana.ID)
	if err != nil {
		t.Fatalf("ListarPorDono: %v", err)
	}

	if len(daAna) != 1 || daAna[0].Slug != "da-ana" {
		t.Fatalf("Ana enxergou %d concursos: %+v", len(daAna), daAna)
	}

	doBruno, err := r.concursos.ListarPorDono(t.Context(), bruno.ID)
	if err != nil {
		t.Fatalf("ListarPorDono: %v", err)
	}

	if len(doBruno) != 1 || doBruno[0].Slug != "do-bruno" {
		t.Fatalf("Bruno enxergou %d concursos: %+v", len(doBruno), doBruno)
	}
}

func TestConcursoRepo_NaoEncontrado(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)

	if _, err := r.concursos.PorSlug(t.Context(), "nao-existe"); !errors.Is(
		err, concurso.ErrNaoEncontrado,
	) {
		t.Errorf("PorSlug inexistente = %v, quer ErrNaoEncontrado", err)
	}

	if _, err := r.concursos.PorID(t.Context(), uuid.New()); !errors.Is(
		err, concurso.ErrNaoEncontrado,
	) {
		t.Errorf("PorID inexistente = %v, quer ErrNaoEncontrado", err)
	}
}

// A regressão que motivou o modelo novo: renomear uma matéria não pode trocar a
// identidade dela, ou o cronograma e o histórico se desligam.
func TestConcursoRepo_AtualizarPreservaIdentidadeDasDisciplinas(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")

	antes := c.Disciplinas[0]

	c.Disciplinas[0].Nome = "Português e Redação"

	if _, err := r.concursos.Atualizar(t.Context(), c); err != nil {
		t.Fatalf("Atualizar: %v", err)
	}

	depois, err := r.concursos.PorSlug(t.Context(), "tce-go")
	if err != nil {
		t.Fatalf("PorSlug: %v", err)
	}

	if depois.Disciplinas[0].ID != antes.ID {
		t.Errorf("o id da disciplina mudou: %s -> %s", antes.ID, depois.Disciplinas[0].ID)
	}

	if depois.Disciplinas[0].Codigo != antes.Codigo {
		t.Errorf("o código mudou: %s -> %s", antes.Codigo, depois.Disciplinas[0].Codigo)
	}

	if depois.Disciplinas[0].Nome != "Português e Redação" {
		t.Errorf("o nome novo não foi gravado: %q", depois.Disciplinas[0].Nome)
	}
}

// Uma disciplina removida da lista sai do banco — e leva junto o que dependia
// dela. Só ela: as outras continuam intactas.
func TestConcursoRepo_AtualizarRemoveApenasAsQueSairam(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")

	sobrevivente := c.Disciplinas[0].ID
	c.Disciplinas = c.Disciplinas[:1]

	if _, err := r.concursos.Atualizar(t.Context(), c); err != nil {
		t.Fatalf("Atualizar: %v", err)
	}

	depois, err := r.concursos.PorSlug(t.Context(), "tce-go")
	if err != nil {
		t.Fatalf("PorSlug: %v", err)
	}

	if len(depois.Disciplinas) != 1 {
		t.Fatalf("disciplinas = %d, quer 1", len(depois.Disciplinas))
	}

	if depois.Disciplinas[0].ID != sobrevivente {
		t.Error("sobrou a disciplina errada")
	}

	// Os temas da que ficou não podem ter sido levados junto.
	if len(depois.Disciplinas[0].Temas) != 2 {
		t.Errorf("temas da sobrevivente = %d, quer 2", len(depois.Disciplinas[0].Temas))
	}
}

// O slug identifica o concurso na URL, então é UNIQUE no banco.
func TestConcursoRepo_SlugDuplicadoFalha(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	r.criarConcurso(t, u, "tce-go")

	_, err := r.concursos.Criar(t.Context(), concurso.Concurso{
		DonoID: u.ID, Slug: "tce-go", Nome: "Outro",
		ProvaPadrao: dia(2026, time.December, 15), RetaPadraoDias: 30,
		Disciplinas: []concurso.Disciplina{{
			Codigo: "X", Nome: "X", Bloco: concurso.BlocoGeral,
			Peso: 1, QuestoesPadrao: 10, Ordem: 0,
		}},
	})

	if err == nil {
		t.Fatal("dois concursos com o mesmo slug deviam ser recusados")
	}
}

func TestConcursoRepo_DefinirCadernoURL(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")

	if err := r.concursos.DefinirCadernoURL(
		t.Context(), c.ID, "LINPO", "https://tec/caderno",
	); err != nil {
		t.Fatalf("DefinirCadernoURL: %v", err)
	}

	lido, err := r.concursos.PorSlug(t.Context(), "tce-go")
	if err != nil {
		t.Fatalf("PorSlug: %v", err)
	}

	if got := lido.Disciplinas[0].CadernoURL; got != "https://tec/caderno" {
		t.Errorf("cadernoUrl = %q", got)
	}

	// Um código que não existe naquele concurso não é silenciosamente ignorado.
	if err := r.concursos.DefinirCadernoURL(
		t.Context(), c.ID, "NAOEXISTE", "https://x",
	); !errors.Is(err, concurso.ErrNaoEncontrado) {
		t.Errorf("código inexistente = %v, quer ErrNaoEncontrado", err)
	}
}

// Apagar o concurso leva o catálogo inteiro junto (FK CASCADE).
func TestConcursoRepo_RemoverLevaOCatalogo(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")

	if err := r.concursos.Remover(t.Context(), c.ID); err != nil {
		t.Fatalf("Remover: %v", err)
	}

	for _, tabela := range []string{"disciplinas", "temas", "fontes", "marcos"} {
		var restantes int
		if err := r.pool.QueryRow(
			t.Context(), `SELECT count(*) FROM `+tabela,
		).Scan(&restantes); err != nil {
			t.Fatalf("contando %s: %v", tabela, err)
		}

		if restantes != 0 {
			t.Errorf("sobraram %d linhas em %s depois de apagar o concurso", restantes, tabela)
		}
	}
}
