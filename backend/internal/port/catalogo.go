package port

import (
	"context"

	"studygo/internal/domain/concurso"

	"github.com/google/uuid"
)

// ConcursoRepository persiste o catálogo que o usuário cadastra: concursos,
// disciplinas, temas, fontes, marcos e conteúdo programático.
type ConcursoRepository interface {
	// ListarPorDono devolve os concursos do dono como resumos (sem disciplinas).
	ListarPorDono(ctx context.Context, donoID uuid.UUID) ([]concurso.Concurso, error)

	// PorSlug e PorID carregam o agregado completo; quem confere a propriedade
	// é a aplicação.
	PorSlug(ctx context.Context, slug string) (concurso.Concurso, error)
	PorID(ctx context.Context, id uuid.UUID) (concurso.Concurso, error)

	Criar(ctx context.Context, c concurso.Concurso) (concurso.Concurso, error)

	// Atualizar grava o concurso PRESERVANDO a identidade das disciplinas que já
	// existem: elas são referenciadas pelo cronograma e pelo histórico, e
	// recriá-las desligaria o que o estudante já fez.
	Atualizar(ctx context.Context, c concurso.Concurso) (concurso.Concurso, error)

	Remover(ctx context.Context, id uuid.UUID) error

	// DefinirCadernoURL atualiza só o link do caderno de uma disciplina, para que
	// o cronograma possa editá-lo sem reenviar o concurso inteiro.
	DefinirCadernoURL(ctx context.Context, concursoID uuid.UUID, codigo, url string) error
}
