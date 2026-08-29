package port

import (
	"context"

	"annygo/internal/domain/concurso"

	"github.com/google/uuid"
)

// ConcursoRepository persists the exam catalogue a user registers: concursos,
// disciplines, topics, sources, milestones, programmatic content, review cycle.
type ConcursoRepository interface {
	// ListByOwner returns the owner's concursos as summaries (no disciplines).
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]concurso.Concurso, error)
	// ConcursoBySlug / ByID load the full aggregate; ownership is checked by the
	// service.
	ConcursoBySlug(ctx context.Context, slug string) (concurso.Concurso, error)
	ConcursoByID(ctx context.Context, id uuid.UUID) (concurso.Concurso, error)

	CreateConcurso(ctx context.Context, c concurso.Concurso) (concurso.Concurso, error)
	UpdateConcurso(ctx context.Context, c concurso.Concurso) (concurso.Concurso, error)
	DeleteConcurso(ctx context.Context, id uuid.UUID) error
}
