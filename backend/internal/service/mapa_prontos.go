package service

import (
	"context"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/mapa"

	"github.com/google/uuid"
)

// MapaProntoResumo é o que a lista dos prontos mostra.
type MapaProntoResumo struct {
	Slug     string
	Titulo   string
	Ramos    int
	Questoes int
	// NaConta diz se a conta já tem um mapa com esse slug: adicionar de novo
	// atualiza o que ela tem.
	NaConta bool
	// Materias são os códigos das matérias do concurso aberto em que o mapa
	// entra ao ser adicionado — a mesma sugestão pelo texto que a importação
	// usa. É onde a tela de Mapas mentais o mostra antes de estar na conta.
	Materias []string
}

// Prontos lista os mapas que vêm com o app, diz quais a conta já tem e, com o
// concurso aberto, em que matérias dele cada um entraria.
func (s *MapaService) Prontos(ctx context.Context, usuarioID uuid.UUID, concursoAberto string) ([]MapaProntoResumo, error) {
	daConta, err := s.mapas.Catalogo(ctx, usuarioID)
	if err != nil {
		return nil, err
	}

	var c concurso.Concurso

	if concursoAberto != "" {
		c, err = s.concursoDoDono(ctx, usuarioID, concursoAberto)
		if err != nil {
			return nil, err
		}
	}

	tem := make(map[string]bool, len(daConta))
	for _, r := range daConta {
		tem[r.Slug] = true
	}

	out := make([]MapaProntoResumo, 0, len(s.prontos))

	for _, p := range s.prontos {
		m, err := mapa.Ler(p.Texto)
		if err != nil {
			return nil, err
		}

		materias := []string{}

		for _, d := range c.Disciplinas {
			if m.SugereMateria(d.Nome, d.Temas) {
				materias = append(materias, d.Codigo)
			}
		}

		ramos, _ := m.Contar()
		out = append(out, MapaProntoResumo{
			Slug: m.Slug, Titulo: m.Titulo, Ramos: ramos,
			Questoes: len(p.Questoes.Questoes), NaConta: tem[m.Slug], Materias: materias,
		})
	}

	return out, nil
}

// AdicionarPronto põe na conta o mapa pronto e as questões dele, como a
// importação de um .zip sem estudo: com o concurso aberto, o mapa vai para a
// matéria que o texto indica. Adicionar de novo troca o conteúdo e mantém o
// vínculo e as respostas, que casam pela chave da questão.
func (s *MapaService) AdicionarPronto(
	ctx context.Context, usuarioID uuid.UUID, slug, concursoAberto string,
) (PacoteImportado, error) {
	for _, p := range s.prontos {
		m, err := mapa.Ler(p.Texto)
		if err != nil {
			return PacoteImportado{}, err
		}

		if m.Slug != slug {
			continue
		}

		questoes := p.Questoes

		return s.ImportarPacote(ctx, usuarioID, concursoAberto, PacoteDeMapa{Texto: p.Texto, Questoes: &questoes})
	}

	return PacoteImportado{}, mapa.ErrNaoEncontrado
}
