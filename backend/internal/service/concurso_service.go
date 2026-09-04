package service

import (
	"context"
	"strings"

	"studygo/internal/domain/concurso"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// ConcursoService cadastra e edita o catálogo da prova, e conduz o assistente
// de importação do edital.
type ConcursoService struct {
	repo      port.ConcursoRepository
	processor port.EditalProcessor
}

func NewConcursoService(
	repo port.ConcursoRepository,
	processor port.EditalProcessor,
) *ConcursoService {
	return &ConcursoService{repo: repo, processor: processor}
}

// ImportacaoDisponivel diz se o processador de editais está configurado. Sem
// ele o cadastro manual continua funcionando normalmente.
func (s *ConcursoService) ImportacaoDisponivel() bool {
	return s.processor != nil && s.processor.Disponivel()
}

func (s *ConcursoService) AnalisarEdital(
	ctx context.Context,
	dono string,
	up port.EditalUpload,
) (AnaliseDoEdital, error) {
	a, err := s.processor.Analisar(ctx, dono, up)
	if err != nil {
		return AnaliseDoEdital{}, err
	}

	return analiseDoEdital(a), nil
}

func (s *ConcursoService) EstruturaDoCargo(
	ctx context.Context,
	dono, documentoID, cargo string,
) (EstruturaDoEdital, error) {
	e, err := s.processor.Estrutura(ctx, dono, documentoID, cargo)
	if err != nil {
		return EstruturaDoEdital{}, err
	}

	return estruturaDoEdital(e), nil
}

func (s *ConcursoService) ConteudoDoEdital(
	ctx context.Context,
	dono, documentoID, cargo string,
	disciplinas []string,
	up port.EditalUpload,
) (ConteudoDoEdital, error) {
	c, err := s.processor.Conteudo(ctx, dono, documentoID, cargo, disciplinas, up)
	if err != nil {
		return ConteudoDoEdital{}, err
	}

	itens := make([]DisciplinaComTemas, 0, len(c.Itens))
	for _, it := range c.Itens {
		itens = append(itens, DisciplinaComTemas{Nome: it.Nome, Temas: it.Temas})
	}

	return ConteudoDoEdital{Itens: itens, Alertas: alertasDoEdital(c.Alertas)}, nil
}

func (s *ConcursoService) Listar(
	ctx context.Context,
	usuarioID uuid.UUID,
) ([]ConcursoResumo, error) {
	cs, err := s.repo.ListarPorDono(ctx, usuarioID)
	if err != nil {
		return nil, err
	}

	out := make([]ConcursoResumo, 0, len(cs))
	for _, c := range cs {
		out = append(out, resumoDe(c))
	}

	return out, nil
}

func (s *ConcursoService) Detalhe(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) (ConcursoDetalhe, error) {
	c, err := s.doDono(ctx, usuarioID, slug)
	if err != nil {
		return ConcursoDetalhe{}, err
	}

	return detalheDe(c), nil
}

func (s *ConcursoService) PorSlug(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) (concurso.Concurso, error) {
	return s.doDono(ctx, usuarioID, slug)
}

func (s *ConcursoService) Criar(
	ctx context.Context,
	usuarioID uuid.UUID,
	cmd ConcursoCommand,
) (ConcursoResumo, []string, error) {
	c, avisos := concursoDoComando(cmd)
	c.DonoID = usuarioID
	c.Slug = concurso.Slug(c.Nome)

	c.Normalizar()

	if err := c.Validar(); err != nil {
		return ConcursoResumo{}, nil, err
	}

	criado, err := s.repo.Criar(ctx, c)
	if err != nil {
		return ConcursoResumo{}, nil, err
	}

	return resumoDe(criado), avisos, nil
}

func (s *ConcursoService) Atualizar(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	cmd ConcursoCommand,
) (ConcursoResumo, []string, error) {
	atual, err := s.doDono(ctx, usuarioID, slug)
	if err != nil {
		return ConcursoResumo{}, nil, err
	}

	c, avisos := concursoDoComando(cmd)
	c.ID = atual.ID
	c.DonoID = atual.DonoID
	c.Slug = atual.Slug

	// As disciplinas que o formulário devolveu com id são as que já existem, e
	// elas guardam o código atual: assim uma matéria renomeada continua sendo a
	// mesma matéria, e o cronograma e o histórico dela permanecem ligados.
	preservarIdentidade(&c, atual)

	c.Normalizar()

	if err := c.Validar(); err != nil {
		return ConcursoResumo{}, nil, err
	}

	atualizado, err := s.repo.Atualizar(ctx, c)
	if err != nil {
		return ConcursoResumo{}, nil, err
	}

	return resumoDe(atualizado), avisos, nil
}

// preservarIdentidade casa as disciplinas que chegaram com as já gravadas,
// mantendo id e código. Uma matéria sem id é nova e recebe os dois.
func preservarIdentidade(novo *concurso.Concurso, atual concurso.Concurso) {
	porID := make(map[uuid.UUID]concurso.Disciplina, len(atual.Disciplinas))
	for _, d := range atual.Disciplinas {
		porID[d.ID] = d
	}

	for i := range novo.Disciplinas {
		d := &novo.Disciplinas[i]

		if anterior, ok := porID[d.ID]; ok {
			d.Codigo = anterior.Codigo
		} else {
			// Não é uma disciplina conhecida deste concurso: trate como nova.
			d.ID = uuid.Nil
			d.Codigo = ""
		}
	}
}

func (s *ConcursoService) Remover(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) error {
	c, err := s.doDono(ctx, usuarioID, slug)
	if err != nil {
		return err
	}

	return s.repo.Remover(ctx, c.ID)
}

func (s *ConcursoService) doDono(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) (concurso.Concurso, error) {
	c, err := s.repo.PorSlug(ctx, slug)
	if err != nil {
		return concurso.Concurso{}, err
	}

	if c.DonoID != usuarioID {
		return concurso.Concurso{}, concurso.ErrNaoEncontrado
	}

	return c, nil
}

// concursoDoComando traduz o formulário para o domínio. A normalização e a
// validação são do domínio; aqui só se converte o formato, e se acumulam os
// avisos sobre o que o usuário deixou pela metade.
func concursoDoComando(cmd ConcursoCommand) (concurso.Concurso, []string) {
	avisos := []string{}

	c := concurso.Concurso{
		Nome:           cmd.Nome,
		Banca:          cmd.Banca,
		Cargo:          cmd.Cargo,
		Emoji:          concurso.PrimeiroEmoji(cmd.Emoji),
		Resumo:         cmd.Resumo,
		RetaPadraoDias: cmd.RetaFinalDias,
		Disciplinas:    make([]concurso.Disciplina, 0, len(cmd.Disciplinas)),
		Marcos:         make([]concurso.Marco, 0, len(cmd.Marcos)),
		Conteudo:       make([]concurso.ConteudoItem, 0, len(cmd.Conteudo)),
	}

	if prova, err := dataISO(cmd.Prova); err == nil {
		c.ProvaPadrao = prova
	} else if cmd.Prova != "" {
		avisos = append(avisos, "não entendi a data da prova ("+cmd.Prova+") — confira")
	}

	for _, dc := range cmd.Disciplinas {
		d := concurso.Disciplina{
			Nome:           dc.Nome,
			Bloco:          concurso.BlocoValido(dc.Bloco),
			Peso:           dc.Peso,
			QuestoesPadrao: dc.Questoes,
			CadernoURL:     dc.CadernoURL,
			Temas:          dc.Temas,
		}

		if id, err := uuid.Parse(dc.ID); err == nil {
			d.ID = id
		}

		for _, fc := range dc.Fontes {
			d.Fontes = append(d.Fontes, concurso.Fonte{
				Titulo: fc.Titulo, URL: fc.URL, Tipo: fc.Tipo,
			})
		}

		if d.QuestoesPadrao == 0 && strings.TrimSpace(d.Nome) != "" {
			avisos = append(avisos,
				`a disciplina "`+d.Nome+`" está sem número de questões — estime um valor`)
		}

		c.Disciplinas = append(c.Disciplinas, d)
	}

	for _, mc := range cmd.Marcos {
		data, err := dataISO(mc.Data)
		if err != nil {
			continue
		}

		m := concurso.Marco{
			DataInicio: data,
			Titulo:     mc.Titulo,
			ExigeAcao:  mc.ExigeAcao,
		}

		if fim, err := dataISO(mc.DataFim); err == nil {
			m.DataFim = &fim
		}

		c.Marcos = append(c.Marcos, m)
	}

	for _, cc := range cmd.Conteudo {
		c.Conteudo = append(c.Conteudo, concurso.ConteudoItem{
			Tipo: cc.Tipo, Texto: cc.Texto,
		})
	}

	return c, avisos
}
