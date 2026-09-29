package port

import (
	"context"

	"studygo/internal/domain/mapa"

	"github.com/google/uuid"
)

// MapaRepository persiste os mapas mentais e o vínculo deles com as matérias.
//
// O mapa é da conta que o importou: toda consulta por slug leva o dono, e o que
// não é dele responde ErrNaoEncontrado — quem não é dono não sabe que existe.
type MapaRepository interface {
	// Catalogo lista os mapas da conta, do mais recente ao mais antigo.
	Catalogo(ctx context.Context, usuarioID uuid.UUID) ([]mapa.Resumo, error)

	// PorSlug carrega o mapa inteiro, com a árvore.
	PorSlug(ctx context.Context, usuarioID uuid.UUID, slug string) (mapa.Mapa, error)

	// ResumoPorSlug é PorSlug sem a árvore, para quem só precisa do id.
	ResumoPorSlug(ctx context.Context, usuarioID uuid.UUID, slug string) (mapa.Resumo, error)

	// Gravar cria o mapa ou, se o slug já existe na conta, troca o conteúdo
	// mantendo o id — e, com ele, os vínculos. Devolve o resumo e se o mapa era
	// novo. Tudo numa transação: o mapa nunca fica pela metade.
	Gravar(ctx context.Context, usuarioID uuid.UUID, m mapa.Mapa) (mapa.Resumo, bool, error)

	// Excluir apaga o mapa, os itens e os vínculos.
	Excluir(ctx context.Context, usuarioID uuid.UUID, slug string) error

	// Vinculos devolve, por disciplina do concurso, os mapas vinculados a ela.
	Vinculos(ctx context.Context, concursoID uuid.UUID) (map[uuid.UUID][]mapa.Resumo, error)
	// Vincular é idempotente: vincular de novo não muda nada.
	Vincular(ctx context.Context, disciplinaID, mapaID uuid.UUID) error
	Desvincular(ctx context.Context, disciplinaID, mapaID uuid.UUID) error
}
