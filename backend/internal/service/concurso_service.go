package service

import (
	"context"

	"annygo/internal/domain/concurso"
	"annygo/internal/port"

	"github.com/google/uuid"
)

// ConcursoService manages the concursos a user registers.
type ConcursoService struct {
	repo      port.ConcursoRepository
	processor port.EditalProcessor
}

func NewConcursoService(repo port.ConcursoRepository, processor port.EditalProcessor) *ConcursoService {
	return &ConcursoService{repo: repo, processor: processor}
}

// ImportacaoDisponivel reports whether the edital importer is wired.
func (s *ConcursoService) ImportacaoDisponivel() bool {
	return s.processor != nil && s.processor.Disponivel()
}

// AnalisarEdital is wizard step 1: uploads the edital to the processor and
// returns the document handle plus the cargos. ownerRef binds the document to
// this user for the later steps.
func (s *ConcursoService) AnalisarEdital(
	ctx context.Context,
	ownerRef string,
	up port.EditalUpload,
) (AnaliseResposta, error) {
	res, err := s.processor.Analisar(ctx, ownerRef, up)
	if err != nil {
		return AnaliseResposta{}, err
	}

	return analiseParaResposta(res), nil
}

// EstruturaDoCargo is wizard step 2: the groups, disciplines, exam basics and
// schedule for the chosen cargo. Question counts the edital did not break down
// stay nil — nothing is invented.
func (s *ConcursoService) EstruturaDoCargo(
	ctx context.Context,
	ownerRef, documentoID, cargo string,
) (EstruturaResposta, error) {
	est, err := s.processor.Estrutura(ctx, ownerRef, documentoID, cargo)
	if err != nil {
		return EstruturaResposta{}, err
	}

	return estruturaParaResposta(est), nil
}

// ConteudoDoEdital is wizard step 3: the syllabus topics for the given
// disciplines. A fresh upload is accepted (documentoID empty) so the edit
// screen's "extract topics" flow keeps working without the wizard.
func (s *ConcursoService) ConteudoDoEdital(
	ctx context.Context,
	ownerRef, documentoID, cargo string,
	disciplinas []string,
	up port.EditalUpload,
) (ConteudoEditalResposta, error) {
	res, err := s.processor.Conteudo(ctx, ownerRef, documentoID, cargo, disciplinas, up)
	if err != nil {
		return ConteudoEditalResposta{}, err
	}

	out := ConteudoEditalResposta{
		Itens:   make([]ConteudoEditalDisc, 0, len(res.Itens)),
		Alertas: editalAlertasParaResposta(res.Alertas),
	}
	for _, it := range res.Itens {
		out.Itens = append(out.Itens, ConteudoEditalDisc{Nome: it.Nome, Temas: it.Temas})
	}

	return out, nil
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
