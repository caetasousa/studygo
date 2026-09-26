package port

import (
	"context"

	"studygo/internal/domain/lei"

	"github.com/google/uuid"
)

// LeiRepository persiste o catálogo de leis, as versões do texto, as questões
// e as respostas.
type LeiRepository interface {
	Catalogo(ctx context.Context) ([]lei.Resumo, error)
	PorSlug(ctx context.Context, slug string) (lei.Lei, error)

	// QuestoesGravadas devolve as questões da lei (ativas ou não), vazio se a
	// lei ainda não existe.
	QuestoesGravadas(ctx context.Context, slug string) ([]lei.QuestaoGravada, error)

	// GravarTexto grava a lei e a versão numa transação: se nova, vira a
	// ativa; se já existe, só volta a ser a ativa. As unidades do pacote
	// substituem as da versão; as questões não são tocadas. Devolve se a
	// versão era nova.
	GravarTexto(ctx context.Context, p lei.Pacote) (bool, error)

	// GravarQuestoes troca as unidades da versão e aplica o plano das
	// questões, numa transação.
	GravarQuestoes(ctx context.Context, leiID uuid.UUID, versao string, unidades []lei.Unidade, plano lei.PlanoDeImportacao) error

	TextoAtivo(ctx context.Context, leiID uuid.UUID) (lei.Texto, error)
	QuestoesAtivas(ctx context.Context, leiID, usuarioID uuid.UUID) ([]lei.QuestaoComResposta, error)
	Questao(ctx context.Context, id uuid.UUID) (lei.QuestaoPublicada, error)
	Responder(ctx context.Context, r lei.Resposta) (lei.Resposta, error)

	// Estrutura é a versão ativa só com as divisões e os artigos: o que o
	// recorte do edital precisa, sem carregar a lei inteira.
	Estrutura(ctx context.Context, leiID uuid.UUID) ([]lei.Dispositivo, error)

	// Vinculos devolve, por disciplina do concurso, as leis vinculadas a ela.
	Vinculos(ctx context.Context, concursoID uuid.UUID) (map[uuid.UUID][]lei.Vinculo, error)
	// Vincular grava o vínculo, ou troca o recorte do que já existe.
	Vincular(ctx context.Context, disciplinaID, leiID uuid.UUID, recorte []string) error
	Desvincular(ctx context.Context, disciplinaID, leiID uuid.UUID) error
}

// CapturadorDeLeis baixa a lei da fonte oficial e a organiza em dispositivos,
// em segundo plano: a Constituição leva mais tempo do que uma requisição pode
// ficar aberta. O dono é quem pediu, e só ele consulta.
type CapturadorDeLeis interface {
	IniciarCaptura(ctx context.Context, dono, link string) (string, error)
	Captura(ctx context.Context, dono, id string) (lei.Captura, error)
}
