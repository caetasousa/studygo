package service

import (
	"context"

	"annygo/internal/domain/concurso"
	"annygo/internal/port"

	"github.com/google/uuid"
)

// ConcursoService manages the concursos a user registers.
type ConcursoService struct {
	repo   port.ConcursoRepository
	edital port.EditalParser
}

func NewConcursoService(repo port.ConcursoRepository, edital port.EditalParser) *ConcursoService {
	return &ConcursoService{repo: repo, edital: edital}
}

// ImportacaoDisponivel reports whether the edital importer is wired.
func (s *ConcursoService) ImportacaoDisponivel() bool {
	return s.edital != nil && s.edital.Disponivel()
}

// ImportarEdital reads a raw edital and returns the prefilled create form plus
// any caveats. Nothing is persisted.
func (s *ConcursoService) ImportarEdital(
	ctx context.Context,
	in port.EditalEntrada,
) (ImportarEditalResposta, error) {
	extraido, err := s.edital.Parse(ctx, in)
	if err != nil {
		return ImportarEditalResposta{}, err
	}

	input, avisos := editalParaInput(extraido)

	return ImportarEditalResposta{Concurso: input, Avisos: avisos}, nil
}

// Listar returns the user's concursos as picker rows.
func (s *ConcursoService) Listar(ctx context.Context, userID uuid.UUID) ([]ConcursoResumo, error) {
	items, err := s.repo.ListByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := make([]ConcursoResumo, 0, len(items))
	for _, c := range items {
		out = append(out, resumoDe(c))
	}

	return out, nil
}

// Detalhe returns a concurso as the edit-form shape, checking ownership.
func (s *ConcursoService) Detalhe(ctx context.Context, userID uuid.UUID, slug string) (ConcursoDetalhe, error) {
	c, err := s.carregarDoDono(ctx, userID, slug)
	if err != nil {
		return ConcursoDetalhe{}, err
	}

	return detalheDe(c), nil
}

// PorSlug loads the full aggregate for a concurso the user owns. Used by
// PlanoService.
func (s *ConcursoService) PorSlug(ctx context.Context, userID uuid.UUID, slug string) (concurso.Concurso, error) {
	return s.carregarDoDono(ctx, userID, slug)
}

// Criar validates and persists a new concurso.
func (s *ConcursoService) Criar(ctx context.Context, userID uuid.UUID, in ConcursoInput) (ConcursoResumo, error) {
	c, _ := concursoFromInput(in)

	if err := c.Validar(); err != nil {
		return ConcursoResumo{}, ErrValidacao{Msg: err.Error()}
	}

	c.OwnerID = userID
	c.Slug = slugify(c.Nome)

	created, err := s.repo.CreateConcurso(ctx, c)
	if err != nil {
		return ConcursoResumo{}, err
	}

	return resumoDe(created), nil
}

// Atualizar rewrites an existing concurso the user owns.
func (s *ConcursoService) Atualizar(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	in ConcursoInput,
) (ConcursoResumo, error) {
	existente, err := s.carregarDoDono(ctx, userID, slug)
	if err != nil {
		return ConcursoResumo{}, err
	}

	c, _ := concursoFromInput(in)

	if err := c.Validar(); err != nil {
		return ConcursoResumo{}, ErrValidacao{Msg: err.Error()}
	}

	c.ID = existente.ID
	c.OwnerID = userID
	c.Slug = existente.Slug

	updated, err := s.repo.UpdateConcurso(ctx, c)
	if err != nil {
		return ConcursoResumo{}, err
	}

	return resumoDe(updated), nil
}

// Remover deletes a concurso and its plan.
func (s *ConcursoService) Remover(ctx context.Context, userID uuid.UUID, slug string) error {
	c, err := s.carregarDoDono(ctx, userID, slug)
	if err != nil {
		return err
	}

	return s.repo.DeleteConcurso(ctx, c.ID)
}

func (s *ConcursoService) carregarDoDono(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
) (concurso.Concurso, error) {
	c, err := s.repo.ConcursoBySlug(ctx, slug)
	if err != nil {
		return concurso.Concurso{}, err
	}

	if c.OwnerID != userID {
		return concurso.Concurso{}, concurso.ErrNotFound
	}

	return c, nil
}
