package postgres

import (
	"context"
	"fmt"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/port"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ port.CronogramaRepository = (*CronogramaRepo)(nil)

// CronogramaRepo persiste o cronograma materializado e o que foi registrado
// nele.
type CronogramaRepo struct {
	pool *pgxpool.Pool
}

func NewCronogramaRepo(pool *pgxpool.Pool) *CronogramaRepo {
	return &CronogramaRepo{pool: pool}
}

// itemCiclo é a linha de plano_ciclo. Existe para dar nome ao que as duas
// pontas trocam; o domínio não conhece a tabela.
type itemCiclo struct {
	Ordem    int
	Titulo   string
	Questoes int
}

func (i itemCiclo) paraDominio() concurso.ItemRevisao {
	return concurso.ItemRevisao{Ordem: i.Ordem, Titulo: i.Titulo, Questoes: i.Questoes}
}

// Atividades devolve o cronograma do plano. O código da disciplina vem por
// join: o domínio raciocina em códigos, o banco guarda a identidade.
func (r *CronogramaRepo) Atividades(
	ctx context.Context,
	planoID uuid.UUID,
) ([]plano.Atividade, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT a.id, a.data, a.posicao, a.disciplina_id, COALESCE(d.codigo, ''),
		        a.tema, a.passada, a.tipo, a.duracao_min, a.movida
		   FROM atividades a
		   LEFT JOIN disciplinas d ON d.id = a.disciplina_id
		  WHERE a.plano_id = $1
		  ORDER BY a.data, a.posicao`,
		planoID,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando atividades: %w", err)
	}
	defer rows.Close()

	out := []plano.Atividade{}

	for rows.Next() {
		var (
			a    plano.Atividade
			tipo string
		)

		if err := rows.Scan(
			&a.ID, &a.Data, &a.Posicao, &a.DisciplinaID, &a.Disciplina,
			&a.Tema, &a.Passada, &tipo, &a.DuracaoMin, &a.Movida,
		); err != nil {
			return nil, fmt.Errorf("lendo atividade: %w", err)
		}

		a.Data = a.Data.UTC()
		a.Tipo = plano.TipoAtividade(tipo)
		out = append(out, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterando atividades: %w", err)
	}

	return out, nil
}

// SubstituirAtividades grava o cronograma inteiro numa transação.
//
// As linhas que continuam no cronograma são ATUALIZADAS pelo id, nunca apagadas
// e reinseridas: registros_atividade aponta para elas com FK RESTRICT, e um
// DELETE geral seria recusado pelo banco — que é exatamente a proteção
// desejada, já que apagar atividade estudada é perder história. Só some quem de
// fato saiu do cronograma, e só quando nada foi registrado nela.
//
// A UNIQUE (plano_id, data, posicao) é DEFERRABLE, então os estados
// intermediários por que um remanejamento passa não colidem antes do commit.
func (r *CronogramaRepo) SubstituirAtividades(
	ctx context.Context,
	planoID uuid.UUID,
	as []plano.Atividade,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	manter := make([]uuid.UUID, 0, len(as))
	for _, a := range as {
		if a.ID != uuid.Nil {
			manter = append(manter, a.ID)
		}
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM atividades WHERE plano_id = $1 AND NOT (id = ANY($2))`,
		planoID, manter,
	); err != nil {
		return fmt.Errorf("removendo atividades que saíram do cronograma: %w", err)
	}

	const gravar = `
		INSERT INTO atividades
			(id, plano_id, data, posicao, disciplina_id, tema, passada, tipo,
			 duracao_min, movida)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			data = EXCLUDED.data, posicao = EXCLUDED.posicao,
			disciplina_id = EXCLUDED.disciplina_id, tema = EXCLUDED.tema,
			passada = EXCLUDED.passada, tipo = EXCLUDED.tipo,
			duracao_min = EXCLUDED.duracao_min, movida = EXCLUDED.movida,
			atualizado_em = now()`

	if len(as) > 0 {
		lote := &pgx.Batch{}

		for _, a := range as {
			id := a.ID
			if id == uuid.Nil {
				id = uuid.New()
			}

			lote.Queue(gravar,
				id, planoID, a.Data, a.Posicao, a.DisciplinaID, a.Tema, a.Passada,
				string(a.Tipo), a.DuracaoMin, a.Movida,
			)
		}

		if err := tx.SendBatch(ctx, lote).Close(); err != nil {
			return fmt.Errorf("gravando atividades: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (r *CronogramaRepo) Registros(
	ctx context.Context,
	planoID uuid.UUID,
) (plano.Registros, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT ra.atividade_id, ra.horas::float8, ra.questoes, ra.acertos,
		        ra.nota, ra.concluido
		   FROM registros_atividade ra
		   JOIN atividades a ON a.id = ra.atividade_id
		  WHERE a.plano_id = $1`,
		planoID,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando registros: %w", err)
	}
	defer rows.Close()

	out := plano.Registros{}

	for rows.Next() {
		var reg plano.RegistroAtividade

		if err := rows.Scan(
			&reg.AtividadeID, &reg.Horas, &reg.Questoes, &reg.Acertos,
			&reg.Nota, &reg.Concluido,
		); err != nil {
			return nil, fmt.Errorf("lendo registro: %w", err)
		}

		out[reg.AtividadeID] = reg
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterando registros: %w", err)
	}

	return out, nil
}

func (r *CronogramaRepo) SalvarRegistro(
	ctx context.Context,
	planoID uuid.UUID,
	reg plano.RegistroAtividade,
) error {
	// A atividade precisa ser deste plano: sem essa checagem, um id de outro
	// plano gravaria um registro que a leitura nunca devolveria.
	ct, err := r.pool.Exec(
		ctx,
		`INSERT INTO registros_atividade
		   (atividade_id, horas, questoes, acertos, nota, concluido)
		 SELECT a.id, $3, $4, $5, $6, $7 FROM atividades a
		  WHERE a.id = $2 AND a.plano_id = $1
		 ON CONFLICT (atividade_id) DO UPDATE SET
		   horas = EXCLUDED.horas, questoes = EXCLUDED.questoes,
		   acertos = EXCLUDED.acertos, nota = EXCLUDED.nota,
		   concluido = EXCLUDED.concluido, atualizado_em = now()`,
		planoID, reg.AtividadeID, reg.Horas, reg.Questoes, reg.Acertos,
		reg.Nota, reg.Concluido,
	)
	if err != nil {
		return fmt.Errorf("gravando registro: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return plano.ErrAtividadeNaoEncontrada
	}

	return nil
}

func (r *CronogramaRepo) ApagarRegistros(ctx context.Context, planoID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	if _, err := tx.Exec(ctx,
		`DELETE FROM registros_atividade ra
		  USING atividades a
		  WHERE a.id = ra.atividade_id AND a.plano_id = $1`,
		planoID,
	); err != nil {
		return fmt.Errorf("apagando registros: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM registros_dia WHERE plano_id = $1`, planoID,
	); err != nil {
		return fmt.Errorf("apagando registros do dia: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM marco_checks WHERE plano_id = $1`, planoID,
	); err != nil {
		return fmt.Errorf("apagando marcos: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (r *CronogramaRepo) RegistrosDia(
	ctx context.Context,
	planoID uuid.UUID,
) (map[time.Time]plano.RegistroDia, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT data, nota, revisao_questoes, revisao_acertos
		   FROM registros_dia WHERE plano_id = $1`,
		planoID,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando registros_dia: %w", err)
	}
	defer rows.Close()

	out := map[time.Time]plano.RegistroDia{}

	for rows.Next() {
		var reg plano.RegistroDia

		if err := rows.Scan(
			&reg.Data, &reg.Nota, &reg.RevisaoQuestoes, &reg.RevisaoAcertos,
		); err != nil {
			return nil, fmt.Errorf("lendo registro do dia: %w", err)
		}

		reg.Data = reg.Data.UTC()
		out[reg.Data] = reg
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterando registros_dia: %w", err)
	}

	return out, nil
}

func (r *CronogramaRepo) SalvarRegistroDia(
	ctx context.Context,
	planoID uuid.UUID,
	reg plano.RegistroDia,
) error {
	if _, err := r.pool.Exec(
		ctx,
		`INSERT INTO registros_dia
		   (plano_id, data, nota, revisao_questoes, revisao_acertos)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (plano_id, data) DO UPDATE SET
		   nota = EXCLUDED.nota,
		   revisao_questoes = EXCLUDED.revisao_questoes,
		   revisao_acertos = EXCLUDED.revisao_acertos,
		   atualizado_em = now()`,
		planoID, reg.Data, reg.Nota, reg.RevisaoQuestoes, reg.RevisaoAcertos,
	); err != nil {
		return fmt.Errorf("gravando registro do dia: %w", err)
	}

	return nil
}
