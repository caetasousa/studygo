package service

import (
	"context"

	"annygo/internal/domain/concurso"
	"annygo/internal/platform/pdftext"
	"annygo/internal/port"

	"github.com/google/uuid"
)

// ConcursoService manages the concursos a user registers.
type ConcursoService struct {
	repo   port.ConcursoRepository
	edital port.EditalAnalisador
}

func NewConcursoService(repo port.ConcursoRepository, edital port.EditalAnalisador) *ConcursoService {
	return &ConcursoService{repo: repo, edital: edital}
}

// ImportacaoDisponivel reports whether the edital importer is wired.
func (s *ConcursoService) ImportacaoDisponivel() bool {
	return s.edital != nil && s.edital.Disponivel()
}

// prepararEntrada extracts a text layer from the PDF when there is one — that
// keeps the later steps cheap and lets the client stop re-uploading the file.
// Scanned PDFs have no text layer, so the raw bytes travel to the model instead.
func prepararEntrada(in port.EditalEntrada) port.EditalEntrada {
	if len(in.PDF) == 0 || in.Texto != "" {
		return in
	}

	if txt, err := pdftext.Extrair(in.PDF); err == nil {
		in.Texto = txt
		in.PDF = nil
	}

	return in
}

// AnalisarEdital is wizard step 1: lists the cargos, returning the extracted
// text when the PDF had one (the client then reuses it instead of re-uploading).
func (s *ConcursoService) AnalisarEdital(ctx context.Context, in port.EditalEntrada) (CargosResposta, error) {
	res, err := s.edital.Cargos(ctx, prepararEntrada(in))
	if err != nil {
		return CargosResposta{}, err
	}

	return cargosParaResposta(res), nil
}

// EstruturaDoCargo is wizard step 2: the disciplines, exam date and schedule for
// the chosen cargo, with block totals already spread across the disciplines. The
// disciplines and the (edital-wide) schedule are fetched concurrently.
func (s *ConcursoService) EstruturaDoCargo(
	ctx context.Context,
	in port.EditalEntrada,
	cargo string,
) (EstruturaResposta, error) {
	in = prepararEntrada(in)

	var (
		est    port.EditalEstrutura
		marcos []port.EditalMarco
		errEst error
		errCr  error
	)

	done := make(chan struct{}, 2)

	go func() {
		est, errEst = s.edital.Estrutura(ctx, in, cargo)
		done <- struct{}{}
	}()
	go func() {
		marcos, errCr = s.edital.Cronograma(ctx, in)
		done <- struct{}{}
	}()

	<-done
	<-done

	if errEst != nil {
		return EstruturaResposta{}, errEst
	}

	// The schedule is a nice-to-have; a failure there just drops the marcos.
	if errCr == nil {
		est.Marcos = marcos
	}

	return estruturaParaResposta(est), nil
}

// ConteudoDoEdital is wizard step 3: the syllabus topics for the given
// disciplines.
func (s *ConcursoService) ConteudoDoEdital(
	ctx context.Context,
	in port.EditalEntrada,
	disciplinas []string,
) (ConteudoEditalResposta, error) {
	itens, err := s.edital.Conteudo(ctx, prepararEntrada(in), disciplinas)
	if err != nil {
		return ConteudoEditalResposta{}, err
	}

	out := ConteudoEditalResposta{Itens: make([]ConteudoEditalDisc, 0, len(itens))}
	for _, it := range itens {
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
