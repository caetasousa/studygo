package service

import (
	"context"
	"slices"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/mapa"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// MapaService importa os mapas mentais, serve a leitura e guarda o vínculo
// entre mapa e matéria. O mapa é da conta que o importou: nada aqui vê o de
// outra.
type MapaService struct {
	mapas     port.MapaRepository
	concursos port.ConcursoRepository
}

func NewMapaService(mapas port.MapaRepository, concursos port.ConcursoRepository) *MapaService {
	return &MapaService{mapas: mapas, concursos: concursos}
}

// MateriaDoMapa é a matéria a que um mapa foi vinculado.
type MateriaDoMapa struct {
	Codigo string
	Nome   string
}

// MapaImportado é o resultado de importar um outline.
type MapaImportado struct {
	Mapa mapa.Resumo
	// Novo é falso quando o slug já existia: o conteúdo foi trocado.
	Novo bool
	// Vinculadas são as matérias do concurso aberto que o texto indica; o
	// vínculo já está gravado.
	Vinculadas []MateriaDoMapa
}

// Importar lê o outline e grava o mapa. Se o mesmo slug já existe na conta, troca
// o conteúdo e preserva os vínculos.
//
// Com o concurso aberto (`concursoSlug`), o mapa é vinculado às matérias dele
// que o texto indica — pelo nome (`materia:`) ou por um termo que algum tópico
// cita (`reconhecer:`). Diferente da lei, o vínculo é feito sem perguntar: um
// vínculo errado não estraga nada e se desfaz num clique, e o cronograma só
// oferece o mapa a quem já o vinculou.
func (s *MapaService) Importar(ctx context.Context, usuarioID uuid.UUID, texto, concursoSlug string) (MapaImportado, error) {
	m, err := mapa.Ler(texto)
	if err != nil {
		return MapaImportado{}, err
	}

	// O concurso é conferido ANTES de gravar: um slug errado não pode deixar o
	// mapa importado sem o vínculo que a pessoa esperava.
	var c concurso.Concurso

	if concursoSlug != "" {
		c, err = s.concursoDoDono(ctx, usuarioID, concursoSlug)
		if err != nil {
			return MapaImportado{}, err
		}
	}

	resumo, novo, err := s.mapas.Gravar(ctx, usuarioID, m)
	if err != nil {
		return MapaImportado{}, err
	}

	out := MapaImportado{Mapa: resumo, Novo: novo}

	for _, d := range c.Disciplinas {
		if !m.SugereMateria(d.Nome, d.Temas) {
			continue
		}

		if err := s.mapas.Vincular(ctx, d.ID, resumo.ID); err != nil {
			return MapaImportado{}, err
		}

		out.Vinculadas = append(out.Vinculadas, MateriaDoMapa{Codigo: d.Codigo, Nome: d.Nome})
	}

	return out, nil
}

// Catalogo lista os mapas da conta.
func (s *MapaService) Catalogo(ctx context.Context, usuarioID uuid.UUID) ([]mapa.Resumo, error) {
	return s.mapas.Catalogo(ctx, usuarioID)
}

// Ler devolve o mapa inteiro.
func (s *MapaService) Ler(ctx context.Context, usuarioID uuid.UUID, slug string) (mapa.Mapa, error) {
	return s.mapas.PorSlug(ctx, usuarioID, slug)
}

// Excluir apaga o mapa e os vínculos dele.
func (s *MapaService) Excluir(ctx context.Context, usuarioID uuid.UUID, slug string) error {
	return s.mapas.Excluir(ctx, usuarioID, slug)
}

// MapasDaMateria são os mapas vinculados a uma matéria do concurso.
type MapasDaMateria struct {
	DisciplinaID uuid.UUID
	Codigo       string
	Nome         string
	Mapas        []mapa.Resumo
}

// DoConcurso devolve TODAS as matérias do concurso, com os mapas de cada uma
// (vazio quando não tem): é com ela que o cronograma sabe onde oferecer o mapa
// e a tela do mapa, a que matérias vinculá-lo.
func (s *MapaService) DoConcurso(ctx context.Context, usuarioID uuid.UUID, concursoSlug string) ([]MapasDaMateria, error) {
	c, err := s.concursoDoDono(ctx, usuarioID, concursoSlug)
	if err != nil {
		return nil, err
	}

	vinculos, err := s.mapas.Vinculos(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	out := make([]MapasDaMateria, 0, len(c.Disciplinas))

	for _, d := range c.Disciplinas {
		out = append(out, MapasDaMateria{
			DisciplinaID: d.ID, Codigo: d.Codigo, Nome: d.Nome, Mapas: vinculos[d.ID],
		})
	}

	return out, nil
}

// Vincular liga (ou desliga) o mapa a uma matéria do concurso. Tanto o mapa
// quanto o concurso têm de ser da conta, e a matéria, do concurso.
func (s *MapaService) Vincular(
	ctx context.Context,
	usuarioID uuid.UUID,
	concursoSlug string,
	disciplinaID uuid.UUID,
	mapaSlug string,
	ligar bool,
) error {
	c, err := s.concursoDoDono(ctx, usuarioID, concursoSlug)
	if err != nil {
		return err
	}

	if !slices.ContainsFunc(c.Disciplinas, func(d concurso.Disciplina) bool { return d.ID == disciplinaID }) {
		return concurso.ErrNaoEncontrado
	}

	resumo, err := s.mapas.ResumoPorSlug(ctx, usuarioID, mapaSlug)
	if err != nil {
		return err
	}

	if !ligar {
		return s.mapas.Desvincular(ctx, disciplinaID, resumo.ID)
	}

	return s.mapas.Vincular(ctx, disciplinaID, resumo.ID)
}

func (s *MapaService) concursoDoDono(ctx context.Context, usuarioID uuid.UUID, slug string) (concurso.Concurso, error) {
	c, err := s.concursos.PorSlug(ctx, slug)
	if err != nil {
		return concurso.Concurso{}, err
	}

	if c.DonoID != usuarioID {
		return concurso.Concurso{}, concurso.ErrNaoEncontrado
	}

	return c, nil
}
