package service_test

import (
	"context"
	"errors"
	"testing"

	"studygo/internal/domain/concurso"
	"studygo/internal/port"
	"studygo/internal/service"

	"github.com/google/uuid"
)

// A colisão de slug é rara e por isso ninguém a vê acontecer — o que torna o
// teste o único lugar onde o comportamento fica visível.
//
// O slug é o nome normalizado mais dois bytes de acaso, e o UNIQUE é do banco
// inteiro: dois estudantes cadastrando o mesmo concurso disputam o mesmo espaço
// de sufixos. Antes, a colisão virava 500 "erro interno" numa operação que só
// precisava sortear de novo.

// repoQueColide recusa as primeiras `recusas` tentativas com ErrSlugEmUso e
// aceita a seguinte, guardando os slugs que foram tentados.
type repoQueColide struct {
	port.ConcursoRepository

	recusas   int
	tentativa int
	tentados  []string
}

func (r *repoQueColide) Criar(_ context.Context, c concurso.Concurso) (concurso.Concurso, error) {
	r.tentativa++
	r.tentados = append(r.tentados, c.Slug)

	if r.tentativa <= r.recusas {
		return concurso.Concurso{}, concurso.ErrSlugEmUso
	}

	c.ID = uuid.New()

	return c, nil
}

func comandoValido() service.ConcursoCommand {
	return service.ConcursoCommand{
		Nome:  "Polícia Federal",
		Prova: "2026-12-15",
		Disciplinas: []service.DisciplinaCommand{
			{Nome: "Língua Portuguesa", Bloco: "ger", Questoes: 20},
		},
	}
}

func TestConcursoService_Criar_sorteiaOutroSlugNaColisao(t *testing.T) {
	t.Parallel()

	repo := &repoQueColide{recusas: 2}
	svc := service.NewConcursoService(repo, nil)

	resumo, _, err := svc.Criar(t.Context(), uuid.New(), comandoValido())
	if err != nil {
		t.Fatalf("Criar depois de duas colisões: %v", err)
	}

	if repo.tentativa != 3 {
		t.Errorf("tentativas = %d, quer 3", repo.tentativa)
	}

	// Repetir o mesmo slug colidiria de novo: o sorteio precisa acontecer DENTRO
	// do laço, e é isso que este teste protege.
	vistos := map[string]bool{}
	for _, s := range repo.tentados {
		if vistos[s] {
			t.Fatalf("o slug %q foi tentado duas vezes: %v", s, repo.tentados)
		}

		vistos[s] = true
	}

	// A base continua derivada do nome; só o sufixo muda.
	base := concurso.BaseSlug("Polícia Federal")
	for _, s := range repo.tentados {
		if len(s) <= len(base) || s[:len(base)] != base {
			t.Errorf("slug %q não deriva de %q", s, base)
		}
	}

	if resumo.Slug != repo.tentados[len(repo.tentados)-1] {
		t.Errorf("resumo.Slug = %q, quer o slug que finalmente entrou", resumo.Slug)
	}
}

// Colidir sempre não pode virar laço infinito nem 500: depois das tentativas o
// erro que sobe é o de conflito, que o adapter traduz em 409.
func TestConcursoService_Criar_desisteDepoisDasTentativas(t *testing.T) {
	t.Parallel()

	repo := &repoQueColide{recusas: 99}
	svc := service.NewConcursoService(repo, nil)

	_, _, err := svc.Criar(t.Context(), uuid.New(), comandoValido())

	if !errors.Is(err, concurso.ErrSlugEmUso) {
		t.Fatalf("erro = %v, quer ErrSlugEmUso", err)
	}

	if repo.tentativa != 3 {
		t.Errorf("tentativas = %d, quer parar em 3", repo.tentativa)
	}
}

// Erro que não é colisão sobe na primeira: repetir uma falha de infraestrutura
// só multiplicaria o dano.
func TestConcursoService_Criar_naoRepeteOutroErro(t *testing.T) {
	t.Parallel()

	falha := errors.New("conexão caiu")
	repo := &repoQueFalha{err: falha}
	svc := service.NewConcursoService(repo, nil)

	_, _, err := svc.Criar(t.Context(), uuid.New(), comandoValido())

	if !errors.Is(err, falha) {
		t.Fatalf("erro = %v, quer o erro original", err)
	}

	if repo.tentativa != 1 {
		t.Errorf("tentativas = %d, quer 1", repo.tentativa)
	}
}

type repoQueFalha struct {
	port.ConcursoRepository

	err       error
	tentativa int
}

func (r *repoQueFalha) Criar(context.Context, concurso.Concurso) (concurso.Concurso, error) {
	r.tentativa++

	return concurso.Concurso{}, r.err
}
