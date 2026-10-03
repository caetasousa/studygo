//go:build integration

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"studygo/internal/adapter/postgres"
	"studygo/internal/domain/concurso"
	"studygo/internal/platform/pgtest"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// A edição do concurso vista de fora: o que o formulário manda e o que sobra
// gravado. O que importa aqui é o par (id, tag) — o id é identidade e não muda;
// a tag é rótulo e é do usuário.

type edicao struct {
	svc  *ConcursoService
	repo port.ConcursoRepository
	id   uuid.UUID
	dono uuid.UUID
	ctx  context.Context //nolint:containedctx // é um cenário de teste, não uma struct de produção
}

func novaEdicao(t *testing.T) *edicao {
	t.Helper()

	pool := pgtest.Novo(t)
	dono := novoDono(t, pool)

	c := concurso.Concurso{
		DonoID: dono, Slug: "tce-go", Nome: "TCE-GO",
		ProvaPadrao:    time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC),
		RetaPadraoDias: 30,
		Disciplinas: []concurso.Disciplina{
			{
				Codigo: "MATRA", Nome: "Matemática e Raciocínio Lógico",
				Bloco: concurso.BlocoGeral, Peso: 1, QuestoesPadrao: 10, Ordem: 0,
			},
			{
				Codigo: "LINPO", Nome: "Língua Portuguesa",
				Bloco: concurso.BlocoGeral, Peso: 1, QuestoesPadrao: 15, Ordem: 1,
			},
		},
	}

	repo := postgres.NewConcursoRepo(pool)

	c, err := repo.Criar(t.Context(), c)
	if err != nil {
		t.Fatalf("criando concurso: %v", err)
	}

	return &edicao{
		svc: NewConcursoService(repo, nil), repo: repo,
		id: c.ID, dono: dono, ctx: context.Background(),
	}
}

// gravado é o concurso como ficou no banco.
func (e *edicao) gravado(t *testing.T) concurso.Concurso {
	t.Helper()

	c, err := e.repo.PorID(e.ctx, e.id)
	if err != nil {
		t.Fatalf("lendo o concurso: %v", err)
	}

	return c
}

// comando devolve o formulário como a tela o preenche: as matérias que já
// existem voltam com id e tag.
func (e *edicao) comando(t *testing.T) ConcursoCommand {
	t.Helper()

	c := e.gravado(t)
	cmd := ConcursoCommand{
		Nome:          c.Nome,
		Prova:         c.ProvaPadrao.Format("2006-01-02"),
		RetaFinalDias: c.RetaPadraoDias,
	}

	for _, d := range c.Disciplinas {
		cmd.Disciplinas = append(cmd.Disciplinas, DisciplinaCommand{
			ID:       d.ID.String(),
			Codigo:   d.Codigo,
			Nome:     d.Nome,
			Bloco:    string(d.Bloco),
			Questoes: d.QuestoesPadrao,
		})
	}

	return cmd
}

func (e *edicao) salvar(t *testing.T, cmd ConcursoCommand) {
	t.Helper()

	if _, _, err := e.svc.Atualizar(e.ctx, e.dono, "tce-go", cmd); err != nil {
		t.Fatalf("Atualizar: %v", err)
	}
}

// A tag é do usuário: "MATRA" vira "RLM" porque foi isso que ele escreveu, e a
// matéria continua sendo a mesma — mesmo id.
func TestConcurso_TagEscolhidaPeloUsuarioVale(t *testing.T) {
	t.Parallel()

	e := novaEdicao(t)
	antes := e.gravado(t).Disciplinas[0].ID

	cmd := e.comando(t)
	cmd.Disciplinas[0].Codigo = "rlm"
	e.salvar(t, cmd)

	d := e.gravado(t).Disciplinas[0]
	if d.Codigo != "RLM" {
		t.Errorf("tag = %q, quer RLM", d.Codigo)
	}

	if d.ID != antes {
		t.Errorf("a matéria trocou de id (%v -> %v): o histórico dela se desligaria", antes, d.ID)
	}
}

// Um cliente que não conhece o campo manda a matéria sem tag. Regenerar o
// mnemônico aí trocaria o chip de todo o cronograma sem ninguém ter pedido.
func TestConcurso_SemTagAMateriaMantemAQueTinha(t *testing.T) {
	t.Parallel()

	e := novaEdicao(t)

	cmd := e.comando(t)
	for i := range cmd.Disciplinas {
		cmd.Disciplinas[i].Codigo = ""
	}

	cmd.Disciplinas[0].Nome = "Raciocínio Lógico-Matemático"
	e.salvar(t, cmd)

	if got := e.gravado(t).Disciplinas[0].Codigo; got != "MATRA" {
		t.Errorf("tag depois de renomear = %q, quer MATRA", got)
	}
}

// Duas matérias com a mesma tag mostrariam o mesmo chip: o cadastro é recusado
// dizendo qual tag repetiu.
func TestConcurso_RecusaDuasMateriasComAMesmaTag(t *testing.T) {
	t.Parallel()

	e := novaEdicao(t)

	cmd := e.comando(t)
	cmd.Disciplinas[0].Codigo = "RLM"
	cmd.Disciplinas[1].Codigo = "RLM"

	_, _, err := e.svc.Atualizar(e.ctx, e.dono, "tce-go", cmd)

	var repetida concurso.ErrCodigoRepetido
	if !errors.As(err, &repetida) {
		t.Fatalf("Atualizar = %v, quer ErrCodigoRepetido", err)
	}

	if repetida.Codigo != "RLM" {
		t.Errorf("tag no erro = %q, quer RLM", repetida.Codigo)
	}
}

// Uma matéria nova estreia com a tag que o usuário escreveu, sem passar pela
// derivação do nome.
func TestConcurso_MateriaNovaEstreiaComATagEscolhida(t *testing.T) {
	t.Parallel()

	e := novaEdicao(t)

	cmd := e.comando(t)
	cmd.Disciplinas = append(cmd.Disciplinas, DisciplinaCommand{
		Codigo: "SEG", Nome: "Segurança da Informação",
		Bloco: string(concurso.BlocoEspecifico), Questoes: 10,
	})
	e.salvar(t, cmd)

	nova := e.gravado(t).Disciplinas[2]
	if nova.Codigo != "SEG" {
		t.Errorf("tag da matéria nova = %q, quer SEG", nova.Codigo)
	}

	if nova.ID == uuid.Nil {
		t.Error("a matéria nova ficou sem id")
	}
}
