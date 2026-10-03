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

	// TrocarItens grava a árvore editada do mapa e desativa as questões
	// listadas, numa transação. `itensAntes` é quantos itens o mapa tinha
	// quando foi lido: se outro pedido o mudou nesse meio-tempo, nada é gravado
	// e o erro é mapa.ErrItemMudou.
	TrocarItens(ctx context.Context, mapaID uuid.UUID, itensAntes int, ramos []mapa.Item, desativar []uuid.UUID) error

	// Excluir apaga o mapa, os itens e os vínculos.
	Excluir(ctx context.Context, usuarioID uuid.UUID, slug string) error

	// Vinculos devolve, por disciplina do concurso, os mapas vinculados a ela.
	Vinculos(ctx context.Context, concursoID uuid.UUID) (map[uuid.UUID][]mapa.Resumo, error)
	// Vincular é idempotente: vincular de novo não muda nada.
	Vincular(ctx context.Context, disciplinaID, mapaID uuid.UUID) error
	Desvincular(ctx context.Context, disciplinaID, mapaID uuid.UUID) error

	// QuestoesGravadas devolve todas as questões do mapa, ativas ou não: é
	// com elas que a importação decide o que é novo, o que mudou e o que saiu.
	QuestoesGravadas(ctx context.Context, mapaID uuid.UUID) ([]mapa.QuestaoGravada, error)
	// GravarQuestoes aplica o plano numa transação: o arquivo entra inteiro
	// ou não entra.
	GravarQuestoes(ctx context.Context, mapaID uuid.UUID, plano mapa.PlanoDeQuestoes) error
	// Questoes lista as questões ativas do mapa, na ordem do arquivo, com a
	// última resposta de cada uma.
	Questoes(ctx context.Context, mapaID uuid.UUID) ([]mapa.QuestaoComResposta, error)
	// QuestaoDoDono carrega a questão ativa de um mapa da conta; a de outra
	// conta, ou retirada, responde ErrQuestaoNaoEncontrada.
	QuestaoDoDono(ctx context.Context, usuarioID, questaoID uuid.UUID) (mapa.QuestaoComResposta, error)
	// Responder grava uma tentativa; as anteriores ficam.
	Responder(ctx context.Context, questaoID uuid.UUID, r mapa.Resposta) (mapa.Resposta, error)
}
