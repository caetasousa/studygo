package port

import (
	"context"
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// PlanoRepository persists a user's plan configuration and progress.
type PlanoRepository interface {
	// PlanoByUser returns the plan for (userID, concursoID) or plano.ErrNotFound.
	PlanoByUser(ctx context.Context, userID, concursoID uuid.UUID) (plano.Salvo, error)

	// UpsertPlano creates or updates the plano row plus its plano_questoes and
	// reordenacoes (full replace), returning the stored aggregate.
	UpsertPlano(ctx context.Context, s plano.Salvo) (plano.Salvo, error)

	// ReplaceReordenacoes deletes every reordenacao for the plan and inserts
	// the given set.
	ReplaceReordenacoes(ctx context.Context, planoID uuid.UUID, r map[time.Time]plano.Reordenacao) error

	// ListAtividades returns the plan's manually arranged activities, ordered by
	// date and position. Empty when the user has never moved anything.
	ListAtividades(ctx context.Context, planoID uuid.UUID) ([]plano.Atividade, error)

	// ReplaceAtividades stores the full activity layout in one transaction, so a
	// move that renumbers several days can never leave duplicate positions
	// behind. It also marks the plan as manually arranged.
	ReplaceAtividades(ctx context.Context, planoID uuid.UUID, as []plano.Atividade) error

	UpsertRegistro(ctx context.Context, planoID uuid.UUID, r plano.Registro) error
	DeleteRegistros(ctx context.Context, planoID uuid.UUID) error

	SetMarco(ctx context.Context, planoID, marcoID uuid.UUID, cumprido bool) error

	ListAnotacoes(ctx context.Context, planoID uuid.UUID) ([]plano.Anotacao, error)
	CreateAnotacao(ctx context.Context, planoID uuid.UUID, a plano.Anotacao) (plano.Anotacao, error)
	UpdateAnotacao(ctx context.Context, planoID uuid.UUID, a plano.Anotacao) (plano.Anotacao, error)
	DeleteAnotacao(ctx context.Context, planoID, anotacaoID uuid.UUID) error

	// ListPlanosParaLembrete returns every plan with its owner's email, for the
	// spaced-review reminder worker.
	ListPlanosParaLembrete(ctx context.Context) ([]PlanoComEmail, error)
}

// PlanoComEmail pairs a stored plan with the owner's contact email.
type PlanoComEmail struct {
	Plano      plano.Salvo
	ConcursoID uuid.UUID
	Email      string
	Nome       string
}
