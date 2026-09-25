package service

import (
	"context"
	"slices"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/lei"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// LeiService publica leis no catálogo, serve a leitura com as questões e
// guarda as respostas e os vínculos entre lei e matéria.
type LeiService struct {
	leis      port.LeiRepository
	usuarios  port.UsuarioRepository
	concursos port.ConcursoRepository
	curadoria lei.Curadoria
}

func NewLeiService(
	leis port.LeiRepository,
	usuarios port.UsuarioRepository,
	concursos port.ConcursoRepository,
	curadoria lei.Curadoria,
) *LeiService {
	return &LeiService{leis: leis, usuarios: usuarios, concursos: concursos, curadoria: curadoria}
}

// EhCurador diz se a conta importa leis — a tela usa para mostrar o botão.
func (s *LeiService) EhCurador(email string) bool {
	return s.curadoria.Pode(email)
}

// ResultadoDaImportacaoDeLei conta o que a importação fez.
type ResultadoDaImportacaoDeLei struct {
	Slug        string
	Curto       string
	Versao      string
	NovaVersao  bool
	Novas       int
	Atualizadas int
	Desativadas int
	Mantidas    int
}

func (s *LeiService) Importar(ctx context.Context, usuarioID uuid.UUID, p lei.Pacote) (ResultadoDaImportacaoDeLei, error) {
	u, err := s.usuarios.PorID(ctx, usuarioID)
	if err != nil {
		return ResultadoDaImportacaoDeLei{}, err
	}
	if !s.curadoria.Pode(u.Email) {
		return ResultadoDaImportacaoDeLei{}, lei.ErrSemPermissao
	}
	if err := p.Validar(); err != nil {
		return ResultadoDaImportacaoDeLei{}, err
	}

	gravadas, err := s.leis.QuestoesGravadas(ctx, p.Lei.Slug)
	if err != nil {
		return ResultadoDaImportacaoDeLei{}, err
	}
	plano := lei.PlanejarImportacao(gravadas, p.Questoes)

	nova, err := s.leis.Importar(ctx, p, plano)
	if err != nil {
		return ResultadoDaImportacaoDeLei{}, err
	}

	return ResultadoDaImportacaoDeLei{
		Slug:        p.Lei.Slug,
		Curto:       p.Lei.Curto,
		Versao:      p.Versao,
		NovaVersao:  nova,
		Novas:       len(plano.Novas),
		Atualizadas: len(plano.Atualizadas),
		Desativadas: len(plano.Desativar),
		Mantidas:    plano.Mantidas,
	}, nil
}

func (s *LeiService) Catalogo(ctx context.Context) ([]lei.Resumo, error) {
	return s.leis.Catalogo(ctx)
}

// Correcao é o que o estudante vê depois de responder. Antes disso o
// gabarito não sai do servidor.
type Correcao struct {
	Escolhida  string
	Acertou    bool
	Gabarito   string
	Comentario string
	Trecho     string
	Em         time.Time
}

// QuestaoParaLeitor é a questão sem gabarito — a não ser que já respondida.
type QuestaoParaLeitor struct {
	ID           uuid.UUID
	Unidade      string
	Dispositivos []string
	Enunciado    string
	Alternativas []string
	Resposta     *Correcao
}

// LeituraDaLei é a lei aberta no leitor.
type LeituraDaLei struct {
	Lei      lei.Lei
	Texto    lei.Texto
	Questoes []QuestaoParaLeitor
}

func (s *LeiService) Ler(ctx context.Context, usuarioID uuid.UUID, slug string) (LeituraDaLei, error) {
	l, err := s.leis.PorSlug(ctx, slug)
	if err != nil {
		return LeituraDaLei{}, err
	}
	texto, err := s.leis.TextoAtivo(ctx, l.ID)
	if err != nil {
		return LeituraDaLei{}, err
	}
	qs, err := s.leis.QuestoesAtivas(ctx, l.ID, usuarioID)
	if err != nil {
		return LeituraDaLei{}, err
	}

	out := LeituraDaLei{Lei: l, Texto: texto, Questoes: make([]QuestaoParaLeitor, 0, len(qs))}
	for _, q := range qs {
		p := QuestaoParaLeitor{
			ID:           q.ID,
			Unidade:      q.Questao.Unidade,
			Dispositivos: q.Questao.Dispositivos,
			Enunciado:    q.Questao.Enunciado,
			Alternativas: q.Questao.Alternativas,
		}
		if q.Ultima != nil {
			c := correcaoDe(q.Questao, *q.Ultima)
			p.Resposta = &c
		}
		out.Questoes = append(out.Questoes, p)
	}

	return out, nil
}

func correcaoDe(q lei.Questao, r lei.Resposta) Correcao {
	return Correcao{
		Escolhida:  r.Alternativa,
		Acertou:    r.Acertou,
		Gabarito:   q.Gabarito,
		Comentario: q.Comentario,
		Trecho:     q.Trecho,
		Em:         r.Em,
	}
}

func (s *LeiService) Responder(ctx context.Context, usuarioID, questaoID uuid.UUID, alternativa string) (Correcao, error) {
	q, err := s.leis.Questao(ctx, questaoID)
	if err != nil {
		return Correcao{}, err
	}
	if !q.Ativa {
		return Correcao{}, lei.ErrQuestaoNaoEncontrada
	}

	acertou, err := q.Questao.Corrigir(alternativa)
	if err != nil {
		return Correcao{}, err
	}

	r, err := s.leis.Responder(ctx, lei.Resposta{
		UsuarioID: usuarioID, QuestaoID: q.ID, Alternativa: alternativa, Acertou: acertou,
	})
	if err != nil {
		return Correcao{}, err
	}

	return correcaoDe(q.Questao, r), nil
}

// LeisDaMateria são as leis de uma disciplina: as vinculadas e as que um
// tópico dela cita e ainda não foram vinculadas.
type LeisDaMateria struct {
	DisciplinaID uuid.UUID
	Codigo       string
	Nome         string
	Vinculadas   []lei.Resumo
	Sugeridas    []lei.Resumo
}

func (s *LeiService) DoConcurso(ctx context.Context, usuarioID uuid.UUID, slug string) ([]LeisDaMateria, error) {
	c, err := s.concursoDoDono(ctx, usuarioID, slug)
	if err != nil {
		return nil, err
	}
	catalogo, err := s.leis.Catalogo(ctx)
	if err != nil {
		return nil, err
	}
	vinculos, err := s.leis.Vinculos(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	out := make([]LeisDaMateria, 0, len(c.Disciplinas))
	for _, d := range c.Disciplinas {
		m := LeisDaMateria{DisciplinaID: d.ID, Codigo: d.Codigo, Nome: d.Nome}
		for _, r := range catalogo {
			switch {
			case slices.Contains(vinculos[d.ID], r.Lei.ID):
				m.Vinculadas = append(m.Vinculadas, r)
			case r.Lei.CitadaEm(d.Temas):
				m.Sugeridas = append(m.Sugeridas, r)
			}
		}
		out = append(out, m)
	}

	return out, nil
}

// Vincular liga (ou desliga) a lei à matéria do concurso do estudante.
func (s *LeiService) Vincular(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	disciplinaID uuid.UUID,
	leiSlug string,
	ligar bool,
) error {
	c, err := s.concursoDoDono(ctx, usuarioID, slug)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(c.Disciplinas, func(d concurso.Disciplina) bool { return d.ID == disciplinaID }) {
		return concurso.ErrNaoEncontrado
	}
	l, err := s.leis.PorSlug(ctx, leiSlug)
	if err != nil {
		return err
	}

	if ligar {
		return s.leis.Vincular(ctx, disciplinaID, l.ID)
	}

	return s.leis.Desvincular(ctx, disciplinaID, l.ID)
}

func (s *LeiService) concursoDoDono(ctx context.Context, usuarioID uuid.UUID, slug string) (concurso.Concurso, error) {
	c, err := s.concursos.PorSlug(ctx, slug)
	if err != nil {
		return concurso.Concurso{}, err
	}
	if c.DonoID != usuarioID {
		return concurso.Concurso{}, concurso.ErrNaoEncontrado
	}

	return c, nil
}
