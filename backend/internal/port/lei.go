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

	// Importar grava o pacote numa transação: a lei, a versão (se nova, vira a
	// ativa; se já existe, só volta a ser a ativa) e o plano das questões.
	// Devolve se a versão era nova.
	Importar(ctx context.Context, p lei.Pacote, plano lei.PlanoDeImportacao) (bool, error)

	TextoAtivo(ctx context.Context, leiID uuid.UUID) (lei.Texto, error)
	QuestoesAtivas(ctx context.Context, leiID, usuarioID uuid.UUID) ([]lei.QuestaoComResposta, error)
	Questao(ctx context.Context, id uuid.UUID) (lei.QuestaoPublicada, error)
	Responder(ctx context.Context, r lei.Resposta) (lei.Resposta, error)

	// Vinculos devolve, por disciplina do concurso, as leis vinculadas a ela.
	Vinculos(ctx context.Context, concursoID uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
	Vincular(ctx context.Context, disciplinaID, leiID uuid.UUID) error
	Desvincular(ctx context.Context, disciplinaID, leiID uuid.UUID) error
}
