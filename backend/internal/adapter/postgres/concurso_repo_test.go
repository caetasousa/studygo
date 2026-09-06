//go:build integration

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

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

// A tag da disciplina é rótulo, não chave: trocá-la ("BANDA" -> "BD") não pode
// mexer no cronograma, no histórico nem nos ajustes por matéria — todos ligados
// pelo id. É o que sustenta deixar o usuário escolher a própria tag.
func TestConcursoRepo_TrocarATagNaoDesligaOPlano(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "tag@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	// Ajustes que o usuário fez para a matéria, gravados sob a tag antiga.
	p.Config.Modos = map[string]plano.Modo{"BANDA": plano.ModoQuestoes}
	p.Config.Reforcos = map[string]float64{"BANDA": 2}

	if _, err := r.planos.Salvar(t.Context(), p); err != nil {
		t.Fatalf("salvando ajustes: %v", err)
	}

	bd := c.Disciplinas[1]
	lidas := r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(bd, dia(2026, time.September, 1), 0, "SQL"),
	})

	horas := 1.5
	if err := r.cronograma.SalvarRegistro(t.Context(), p.ID, plano.RegistroAtividade{
		AtividadeID: lidas[0].ID, Horas: &horas, Concluido: true,
	}); err != nil {
		t.Fatalf("SalvarRegistro: %v", err)
	}

	c.Disciplinas[1].Codigo = "BD"
	if _, err := r.concursos.Atualizar(t.Context(), c); err != nil {
		t.Fatalf("Atualizar: %v", err)
	}

	// A atividade continua sendo a mesma, apontando para a mesma matéria.
	atividades, err := r.cronograma.Atividades(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("Atividades: %v", err)
	}

	if len(atividades) != 1 || atividades[0].ID != lidas[0].ID {
		t.Fatalf("o cronograma mudou depois da troca de tag: %+v", atividades)
	}

	if atividades[0].DisciplinaID == nil || *atividades[0].DisciplinaID != bd.ID {
		t.Error("a atividade se desligou da matéria")
	}

	registros, err := r.cronograma.Registros(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("Registros: %v", err)
	}

	if !registros.Concluida(lidas[0].ID) {
		t.Error("o registro de estudo sumiu")
	}

	// E os ajustes voltam sob a tag NOVA: a coluna é o id, e o join traduz.
	recarregado, err := r.planos.PorUsuario(t.Context(), u.ID, c.ID)
	if err != nil {
		t.Fatalf("PorUsuario: %v", err)
	}

	if got := recarregado.Config.Modos["BD"]; got != plano.ModoQuestoes {
		t.Errorf("modo depois da troca = %q, quer %q", got, plano.ModoQuestoes)
	}

	if got := recarregado.Config.Reforcos["BD"]; got != 2 {
		t.Errorf("reforço depois da troca = %v, quer 2", got)
	}

	if _, ainda := recarregado.Config.Modos["BANDA"]; ainda {
		t.Error("a tag antiga continuou no plano")
	}
}

// Excluir o concurso apaga o progresso junto — é o que a tela avisa antes de
// perguntar. A FK RESTRICT de registros_atividade existe contra o
// replanejamento que apagaria história sem querer, não contra a exclusão
// pedida; sem limpar os registros primeiro, todo concurso já estudado ficava
// impossível de excluir, com "erro interno" na tela.
func TestConcursoRepo_RemoverConcursoJaEstudado(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "excluir@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	lidas := r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(c.Disciplinas[0], dia(2026, time.September, 1), 0, "Crase"),
	})

	horas := 1.0
	if err := r.cronograma.SalvarRegistro(t.Context(), p.ID, plano.RegistroAtividade{
		AtividadeID: lidas[0].ID, Horas: &horas, Concluido: true,
	}); err != nil {
		t.Fatalf("SalvarRegistro: %v", err)
	}

	if err := r.concursos.Remover(t.Context(), c.ID); err != nil {
		t.Fatalf("Remover: %v", err)
	}

	if _, err := r.concursos.PorID(t.Context(), c.ID); err == nil {
		t.Error("o concurso continuou lá depois da exclusão")
	}

	var sobraram int
	if err := r.pool.QueryRow(
		t.Context(), `SELECT count(*) FROM registros_atividade`,
	).Scan(&sobraram); err != nil {
		t.Fatalf("contando registros: %v", err)
	}

	if sobraram != 0 {
		t.Errorf("sobraram %d registros órfãos", sobraram)
	}
}
