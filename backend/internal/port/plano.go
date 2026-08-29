package port

import (
	"context"
	"time"

	"annygo/internal/domain/plano"

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

	UpsertRegistro(ctx context.Context, planoID uuid.UUID, r plano.Registro) error
	DeleteRegistros(ctx context.Context, planoID uuid.UUID) error

	SetMarco(ctx context.Context, planoID, marcoID uuid.UUID, cumprido bool) error

	// ListRevisoes returns the plan's open review queue (feita_em IS NULL).
	ListRevisoes(ctx context.Context, planoID uuid.UUID) ([]plano.Revisao, error)

	// EnfileirarRevisoes adds reviews, ignoring any topic already queued at the
	// same stage — re-completing a day must not duplicate its queue entries.
	EnfileirarRevisoes(ctx context.Context, planoID uuid.UUID, rs []plano.Revisao) error

	// ConcluirRevisao marks one review done with its result and enqueues the
	// next stage, if there is one.
	ConcluirRevisao(ctx context.Context, planoID uuid.UUID, feita plano.Revisao, proxima *plano.Revisao) error

	RevisaoByID(ctx context.Context, planoID, revisaoID uuid.UUID) (plano.Revisao, error)

	DeleteRevisoes(ctx context.Context, planoID uuid.UUID) error

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
