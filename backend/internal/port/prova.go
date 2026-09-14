package port

import (
	"context"
	"errors"
	"time"

	"studygo/internal/domain/prova"
)

var (
	// ErrProcessamentoTransitorio é falha do processador ou do Gemini que pode
	// passar sozinha: sobrecarga, timeout, rede. O worker tenta de novo.
	ErrProcessamentoTransitorio = errors.New("processador de provas indisponível no momento")

	// ErrDocumentoRecusado é o processador lendo o pedido e recusando — PDF
	// inválido, região fora da página. Repetir não resolve.
	ErrDocumentoRecusado = errors.New("o processador recusou o documento")
)

// FiltroCatalogo filtra as provas publicadas. Campo vazio não filtra.
type FiltroCatalogo struct {
	Ano, Orgao, Cargo, Disciplina string
	Offset, Limite                int
}

// ProvaRepository persiste importações, arquivos e publicações de provas.
//
// Todas as escritas numa importação levam a versão lida (ou a tentativa, no
// caso do worker) e devolvem prova.ErrConflito quando ela já não vale: é assim
// que um resultado atrasado não sobrescreve a revisão de um humano.
type ProvaRepository interface {
	// Criar grava a importação e registra seus arquivos. Mesmo hash de uma
	// importação ativa devolve a existente; acima de limite pendentes por
	// criador, prova.ErrLimite.
	Criar(ctx context.Context, i prova.Importacao, limite int) (prova.Importacao, error)
	Obter(ctx context.Context, id string) (prova.Importacao, error)
	// ListarResumos traz as importações mais recentes com o rascunho SEM
	// questões e apoios: a lista mostra identificação e andamento, e o
	// conteúdo de cem provas não cabe numa tela de lista.
	ListarResumos(ctx context.Context) ([]prova.Importacao, error)
	Salvar(ctx context.Context, i prova.Importacao, versao int) error

	// Reservar entrega a próxima etapa pendente, ou prova.ErrNaoEncontrada se
	// não houver nada ou se outra importação estiver em andamento.
	Reservar(ctx context.Context) (prova.Importacao, error)
	Renovar(ctx context.Context, id, tentativa string) (bool, error)
	// ConcluirEtapa grava o avanço da importação e o resultado isolado da
	// etapa executada, se a tentativa ainda detiver a reserva.
	ConcluirEtapa(ctx context.Context, i prova.Importacao, etapa int, resultado any, duracao time.Duration) error
	// Excluir apaga a importação se ela continua como foi carregada — mesma
	// versão e mesmo estado; senão, prova.ErrConflito.
	Excluir(ctx context.Context, i prova.Importacao) error
	// Falhar registra a falha da etapa: ela volta à fila depois da espera ou,
	// com espera zero, a importação fica como falha.
	Falhar(ctx context.Context, i prova.Importacao, msg string, espera time.Duration) error

	RegistrarArquivo(ctx context.Context, importacao, id, extensao string) error
	// Arquivo devolve a extensão de um arquivo que quem pede pode ver: curador
	// vê tudo, os demais só o que pertence a uma prova visível.
	Arquivo(ctx context.Context, id string, curador bool) (string, error)
	ArquivosDaImportacao(ctx context.Context, id string) ([]string, error)

	Publicar(ctx context.Context, i prova.Importacao, usuario string) (string, error)
	Catalogo(ctx context.Context, f FiltroCatalogo) ([]prova.Publicacao, error)
	Publicacao(ctx context.Context, id string) (prova.Publicacao, error)
	Questoes(ctx context.Context, provaID string, numero int, disciplina string) ([]prova.Questao, error)
	// QuestoesAvulsas lista as questões das provas visíveis, sem o conteúdo,
	// uma vez por conteúdo: a questão comum a dois cargos vem só da prova que
	// estreou primeiro no catálogo. Ordem: ano mais recente, prova, número.
	QuestoesAvulsas(ctx context.Context) ([]prova.QuestaoAvulsa, error)
	// ProvasDoAno devolve a identificação, sem as questões, das provas visíveis
	// da mesma banca e ano, menos `exceto`: as do mesmo órgão são as candidatas
	// a ter as mesmas questões de uma importação (quem decide o órgão é o
	// domínio, que lê "TRF 1" e "TRF1" como o mesmo).
	ProvasDoAno(ctx context.Context, banca string, ano int, exceto string) ([]prova.Publicacao, error)
	// Anotacoes são as do usuário nas questões de uma prova, por número.
	Anotacoes(ctx context.Context, usuario, provaID string) ([]prova.Anotacao, error)
	// SalvarAnotacao grava a anotação da questão, criando ou substituindo.
	SalvarAnotacao(ctx context.Context, usuario, provaID string, a prova.Anotacao) (prova.Anotacao, error)
	ExcluirAnotacao(ctx context.Context, usuario, provaID string, numero int) error
	// ImportacaoDaPublicacao é a importação que gerou a revisão em vigor — a
	// base de uma nova revisão.
	ImportacaoDaPublicacao(ctx context.Context, provaID string) (prova.Importacao, error)
	Retirar(ctx context.Context, id string) error

	// Expirar cancela rascunhos parados desde antes de `antes`.
	Expirar(ctx context.Context, antes time.Time) error
	// LimparReferencias solta os arquivos de importações canceladas antes de
	// `antes` e devolve os que ficaram sem dono, como "<id>.<extensão>".
	LimparReferencias(ctx context.Context, antes time.Time) ([]string, error)
}

// ProvaProcessor é o pipeline de extração. Recebe identificadores de arquivos
// já gravados no volume compartilhado — nunca caminhos.
type ProvaProcessor interface {
	Preparar(ctx context.Context, documento string) ([]prova.Origem, error)
	Metadados(ctx context.Context, documento string, capa prova.Origem) (prova.Metadados, error)
	Extrair(ctx context.Context, documento string, regiao prova.Origem) (prova.Rascunho, error)
	Gabarito(ctx context.Context, arquivo string) (prova.Gabarito, error)
	// Classificar sugere a matéria de cada questão, pelo número. Só lê texto:
	// não recebe o PDF.
	Classificar(ctx context.Context, questoes []prova.ResumoDeQuestao) (map[int]string, error)
	Recortar(ctx context.Context, documento string, o prova.Origem) (string, error)
}

// ProvaArquivos é o volume durável dos PDFs e recortes.
type ProvaArquivos interface {
	Guardar(id string, conteudo []byte) error
	Remover(nome string) error
	Existe(id, extensao string) bool
	Caminho(id, extensao string) (string, error)
}
