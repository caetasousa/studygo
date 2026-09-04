package port

import (
	"context"
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// PlanoRepository persiste a configuração e o progresso do plano de um usuário.
//
// O cronograma é carregado à parte (CronogramaRepository): ele é grande, e nem
// todo caso de uso precisa dele.
type PlanoRepository interface {
	// PorUsuario devolve o plano de (usuarioID, concursoID) ou
	// plano.ErrNaoEncontrado.
	PorUsuario(ctx context.Context, usuarioID, concursoID uuid.UUID) (plano.Plano, error)

	// Salvar cria ou atualiza o plano com sua configuração, devolvendo o
	// agregado gravado.
	Salvar(ctx context.Context, p plano.Plano) (plano.Plano, error)

	// MarcarMarco liga ou desliga um item do cronograma oficial.
	MarcarMarco(ctx context.Context, planoID, marcoID uuid.UUID, cumprido bool) error

	// ParaLembrete devolve todo plano com o e-mail do dono, para o worker de
	// lembretes.
	ParaLembrete(ctx context.Context) ([]PlanoDoUsuario, error)
}

// CronogramaRepository persiste o cronograma materializado e o que foi
// registrado nele.
type CronogramaRepository interface {
	// Atividades devolve o cronograma do plano, ordenado por (data, posição).
	Atividades(ctx context.Context, planoID uuid.UUID) ([]plano.Atividade, error)

	// SubstituirAtividades grava o cronograma inteiro numa transação, para que
	// um movimento que renumera vários dias nunca deixe posições duplicadas.
	//
	// Uma atividade que sai do cronograma e ainda tem registro é recusada pelo
	// banco (FK RESTRICT): apagar o que foi estudado é perda de história, não
	// replanejamento.
	SubstituirAtividades(ctx context.Context, planoID uuid.UUID, as []plano.Atividade) error

	// Registros devolve o que foi lançado, indexado pela atividade.
	Registros(ctx context.Context, planoID uuid.UUID) (plano.Registros, error)

	// SalvarRegistro grava o lançamento de uma atividade.
	SalvarRegistro(ctx context.Context, planoID uuid.UUID, r plano.RegistroAtividade) error

	// ApagarRegistros limpa todo o histórico do plano.
	ApagarRegistros(ctx context.Context, planoID uuid.UUID) error

	// RegistrosDia devolve o que pertence ao dia e não a uma atividade.
	RegistrosDia(ctx context.Context, planoID uuid.UUID) (map[time.Time]plano.RegistroDia, error)

	// SalvarRegistroDia grava a anotação do dia e o resultado da cauda de
	// revisão.
	SalvarRegistroDia(ctx context.Context, planoID uuid.UUID, r plano.RegistroDia) error
}

// CadernoRepository persiste o caderno de erros.
type CadernoRepository interface {
	Anotacoes(ctx context.Context, planoID uuid.UUID) ([]plano.Anotacao, error)
	CriarAnotacao(ctx context.Context, planoID uuid.UUID, a plano.Anotacao) (plano.Anotacao, error)
	AtualizarAnotacao(ctx context.Context, planoID uuid.UUID, a plano.Anotacao) (plano.Anotacao, error)
	RemoverAnotacao(ctx context.Context, planoID, anotacaoID uuid.UUID) error
}

// PlanoDoUsuario junta um plano gravado ao contato do dono.
type PlanoDoUsuario struct {
	Plano      plano.Plano
	ConcursoID uuid.UUID
	Email      string
	Nome       string
}
