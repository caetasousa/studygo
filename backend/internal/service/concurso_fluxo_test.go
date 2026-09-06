package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"studygo/internal/domain/concurso"

	"github.com/google/uuid"
)

// A edição do concurso vista de fora: o que o formulário manda e o que sobra
// gravado. O que importa aqui é o par (id, tag) — o id é identidade e não muda;
// a tag é rótulo e é do usuário.

type edicao struct {
	svc  *ConcursoService
	repo *fakeConcursos
	dono uuid.UUID
	ctx  context.Context //nolint:containedctx // é um cenário de teste, não uma struct de produção
}

func novaEdicao(t *testing.T) *edicao {
	t.Helper()

	dono := uuid.New()
	c := concurso.Concurso{
		ID: uuid.New(), DonoID: dono, Slug: "tce-go", Nome: "TCE-GO",
		ProvaPadrao:    time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC),
		RetaPadraoDias: 30,
		Disciplinas: []concurso.Disciplina{
			{
				ID: uuid.New(), Codigo: "MATRA", Nome: "Matemática e Raciocínio Lógico",
				Bloco: concurso.BlocoGeral, Peso: 1, QuestoesPadrao: 10, Ordem: 0,
			},
			{
				ID: uuid.New(), Codigo: "LINPO", Nome: "Língua Portuguesa",
				Bloco: concurso.BlocoGeral, Peso: 1, QuestoesPadrao: 15, Ordem: 1,
			},
		},
	}

	repo := &fakeConcursos{c: c}

	return &edicao{
		svc: NewConcursoService(repo, nil), repo: repo,
		dono: dono, ctx: context.Background(),
	}
}

// comando devolve o formulário como a tela o preenche: as matérias que já
// existem voltam com id e tag.
func (e *edicao) comando() ConcursoCommand {
	cmd := ConcursoCommand{
		Nome:          e.repo.c.Nome,
		Prova:         e.repo.c.ProvaPadrao.Format("2006-01-02"),
		RetaFinalDias: e.repo.c.RetaPadraoDias,
	}

	for _, d := range e.repo.c.Disciplinas {
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
	antes := e.repo.c.Disciplinas[0].ID

	cmd := e.comando()
	cmd.Disciplinas[0].Codigo = "rlm"
	e.salvar(t, cmd)

	d := e.repo.c.Disciplinas[0]
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

	cmd := e.comando()
	for i := range cmd.Disciplinas {
		cmd.Disciplinas[i].Codigo = ""
	}

	cmd.Disciplinas[0].Nome = "Raciocínio Lógico-Matemático"
	e.salvar(t, cmd)

	if got := e.repo.c.Disciplinas[0].Codigo; got != "MATRA" {
		t.Errorf("tag depois de renomear = %q, quer MATRA", got)
	}
}

// Duas matérias com a mesma tag mostrariam o mesmo chip: o cadastro é recusado
// dizendo qual tag repetiu.
func TestConcurso_RecusaDuasMateriasComAMesmaTag(t *testing.T) {
	t.Parallel()

	e := novaEdicao(t)

	cmd := e.comando()
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

	cmd := e.comando()
	cmd.Disciplinas = append(cmd.Disciplinas, DisciplinaCommand{
		Codigo: "SEG", Nome: "Segurança da Informação",
		Bloco: string(concurso.BlocoEspecifico), Questoes: 10,
	})
	e.salvar(t, cmd)

	nova := e.repo.c.Disciplinas[2]
	if nova.Codigo != "SEG" {
		t.Errorf("tag da matéria nova = %q, quer SEG", nova.Codigo)
	}

	if nova.ID == uuid.Nil {
		t.Error("a matéria nova ficou sem id")
	}
}
