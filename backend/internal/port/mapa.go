package port

import (
	"context"
	"time"

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

	// Vinculos devolve, por disciplina do concurso, os mapas vinculados a ela
	// e os tópicos que cada um cobre, como gravados.
	Vinculos(ctx context.Context, concursoID uuid.UUID) (map[uuid.UUID][]mapa.Vinculo, error)
	// Vincular grava o vínculo com estes tópicos; se já existe, troca os tópicos.
	Vincular(ctx context.Context, disciplinaID, mapaID uuid.UUID, temas []string) error
	// SugerirVinculo grava o vínculo só se ele ainda não existe: importar o
	// mapa de novo não desfaz os tópicos escolhidos na tela.
	SugerirVinculo(ctx context.Context, disciplinaID, mapaID uuid.UUID, temas []string) error
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
	// NomesDasImagens lista as imagens que o mapa guarda, em ordem de nome.
	NomesDasImagens(ctx context.Context, mapaID uuid.UUID) ([]string, error)
	// GravarImagens grava as imagens numa transação, trocando a de mesmo nome.
	// Se com elas o mapa passaria de `teto` imagens, nada é gravado e o
	// retorno é mapa.ErrImagensInvalidas.
	GravarImagens(ctx context.Context, mapaID uuid.UUID, imagens []mapa.Imagem, teto int) error
	// Imagem carrega uma imagem do mapa, ou mapa.ErrImagemNaoEncontrada.
	Imagem(ctx context.Context, mapaID uuid.UUID, nome string) (mapa.Imagem, error)

	// Responder grava uma tentativa; as anteriores ficam.
	Responder(ctx context.Context, questaoID uuid.UUID, r mapa.Resposta) (mapa.Resposta, error)

	// VinculosDoMapa e RespostasDoMapa são o que a exportação leva do estudo
	// da conta; RestaurarRespostas os traz de volta, sem repetir a tentativa
	// que já está gravada (mesma questão, mesma hora). Devolve quantas gravou.
	VinculosDoMapa(ctx context.Context, mapaID uuid.UUID) ([]mapa.VinculoExportado, error)
	RespostasDoMapa(ctx context.Context, mapaID uuid.UUID) ([]mapa.RespostaExportada, error)
	RestaurarRespostas(ctx context.Context, respostas []mapa.RespostaRestaurada) (int, error)

	// Os pedidos de mapa: a fila de PDFs da conta. Todo acesso leva o dono, e o
	// pedido de outra conta responde mapa.ErrPedidoNaoEncontrado.
	CriarPedido(ctx context.Context, usuarioID uuid.UUID, disciplinaID uuid.NullUUID, arquivo string, pdf []byte) (mapa.Pedido, error)
	// Pedidos lista os pedidos da conta, do mais recente ao mais antigo.
	Pedidos(ctx context.Context, usuarioID uuid.UUID) ([]mapa.Pedido, error)
	Pedido(ctx context.Context, usuarioID, id uuid.UUID) (mapa.Pedido, error)
	// PedidoPorID carrega o pedido e a conta dele, para a rota interna por onde
	// o processador devolve o resultado — que não tem conta logada.
	PedidoPorID(ctx context.Context, id uuid.UUID) (mapa.Pedido, uuid.UUID, error)
	// PedidosProcessando lista os pedidos "processando" de todas as contas, com
	// a conta de cada um: o processador que reiniciou os pede de volta.
	PedidosProcessando(ctx context.Context) ([]PedidoDaConta, error)
	// GravarMapaDoPedido anota o slug do mapa que o pedido já importou: numa
	// nova tentativa, reimportar o mesmo slug não conta como mapa alheio.
	GravarMapaDoPedido(ctx context.Context, id uuid.UUID, slug string) error
	// PDFDoPedido devolve os bytes do PDF, enquanto ele existe.
	PDFDoPedido(ctx context.Context, usuarioID, id uuid.UUID) ([]byte, error)
	// MudarPedido leva o pedido de uma das situações `de` para `para`, com o
	// mapa e o relatório dados, e descarta o PDF se pedido. Se o pedido não
	// está em nenhuma de `de`, nada muda e o erro é mapa.ErrPedidoForaDeHora.
	MudarPedido(ctx context.Context, usuarioID, id uuid.UUID, de []mapa.Situacao, para mapa.Situacao, mapaSlug, relatorio string, descartarPDF bool) error
	ExcluirPedido(ctx context.Context, usuarioID, id uuid.UUID) error
}

// PedidoDaConta é um pedido com a conta dele.
type PedidoDaConta struct {
	UsuarioID uuid.UUID
	Pedido    mapa.Pedido
}

// TrabalhoDeMapa é o que o backend entrega ao processador para virar mapa: o
// PDF e o contexto que o mapa precisa (a matéria e os tópicos dela, os slugs
// que a conta já usa) e o token do Claude da conta, se ela guardou um.
type TrabalhoDeMapa struct {
	Pedido          uuid.UUID
	Dono            string
	Arquivo         string
	PDF             []byte
	Materia         string
	Temas           []string
	SlugsExistentes []string
	TokenClaude     string
}

// ProcessadorDeMapas faz o mapa a partir do PDF, em segundo plano: aceita o
// trabalho e, ao terminar, devolve o resultado pela rota interna do backend.
// Erro aqui é só "não aceitou" (fora do ar, ocupado).
type ProcessadorDeMapas interface {
	Processar(ctx context.Context, t TrabalhoDeMapa) error

	// A conexão do Claude de cada conta (dono), feita pela tela: o link de
	// autorização, o código que a página do Claude mostra, a situação e a saída.
	ConectarClaude(ctx context.Context, dono string) (string, error)
	ConcluirConexaoDoClaude(ctx context.Context, dono, codigo string) (ConexaoDoClaude, error)
	ConexaoDoClaude(ctx context.Context, dono string) (ConexaoDoClaude, error)
	DesconectarClaude(ctx context.Context, dono string) error
}

// ConexaoDoClaude é a conta do Claude conectada ao processador, se houver.
type ConexaoDoClaude struct {
	Conectado bool
	Email     string
	Plano     string
}

// FaxinaDosPedidos é a limpeza periódica da fila de PDFs, separada de
// MapaRepository pelo mesmo motivo de SessaoManutencao: só o worker varre.
type FaxinaDosPedidos interface {
	// DescartarPDFsDeFalhas apaga o PDF dos pedidos que falharam antes de
	// `antesDe` e devolve quantos perderam o PDF. O pedido fica, com o motivo.
	DescartarPDFsDeFalhas(ctx context.Context, antesDe time.Time) (int64, error)
}
