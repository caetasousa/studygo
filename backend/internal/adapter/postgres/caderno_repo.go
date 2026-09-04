package postgres

import (
	"context"
	"fmt"

	"studygo/internal/domain/plano"
	"studygo/internal/port"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ port.CadernoRepository = (*CadernoRepo)(nil)

// CadernoRepo persiste o caderno de erros.
type CadernoRepo struct {
	pool *pgxpool.Pool
}

func NewCadernoRepo(pool *pgxpool.Pool) *CadernoRepo {
	return &CadernoRepo{pool: pool}
}

const colunasAnotacao = `id, data, disciplina_id, tema, texto, origem, url,
	proxima_revisao, resolvido, criado_em, atualizado_em`

func (r *CadernoRepo) Anotacoes(
	ctx context.Context,
	planoID uuid.UUID,
) ([]plano.Anotacao, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT `+colunasAnotacao+`
		   FROM anotacoes WHERE plano_id = $1 ORDER BY criado_em DESC`,
		planoID,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando anotações: %w", err)
	}
	defer rows.Close()

	out := []plano.Anotacao{}

	for rows.Next() {
		var (
			a      plano.Anotacao
			origem string
		)

		if err := rows.Scan(
			&a.ID, &a.Data, &a.DisciplinaID, &a.Tema, &a.Texto, &origem, &a.URL,
			&a.ProximaRevisao, &a.Resolvido, &a.CriadoEm, &a.AtualizadoEm,
		); err != nil {
			return nil, fmt.Errorf("lendo anotação: %w", err)
		}

		a.Origem = plano.Origem(origem)
		out = append(out, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterando anotações: %w", err)
	}

	return out, nil
}

func (r *CadernoRepo) CriarAnotacao(
	ctx context.Context,
	planoID uuid.UUID,
	a plano.Anotacao,
) (plano.Anotacao, error) {
	err := r.pool.QueryRow(
		ctx,
		`INSERT INTO anotacoes
		   (plano_id, data, disciplina_id, tema, texto, origem, url,
		    proxima_revisao, resolvido)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, criado_em, atualizado_em`,
		planoID, a.Data, a.DisciplinaID, a.Tema, a.Texto, string(a.Origem),
		a.URL, a.ProximaRevisao, a.Resolvido,
	).Scan(&a.ID, &a.CriadoEm, &a.AtualizadoEm)
	if err != nil {
		return plano.Anotacao{}, fmt.Errorf("criando anotação: %w", err)
	}

	return a, nil
}

func (r *CadernoRepo) AtualizarAnotacao(
	ctx context.Context,
	planoID uuid.UUID,
	a plano.Anotacao,
) (plano.Anotacao, error) {
	err := r.pool.QueryRow(
		ctx,
		`UPDATE anotacoes SET
		   data = $3, disciplina_id = $4, tema = $5, texto = $6, origem = $7,
		   url = $8, proxima_revisao = $9, resolvido = $10, atualizado_em = now()
		 WHERE id = $1 AND plano_id = $2
		 RETURNING id, criado_em, atualizado_em`,
		a.ID, planoID, a.Data, a.DisciplinaID, a.Tema, a.Texto, string(a.Origem),
		a.URL, a.ProximaRevisao, a.Resolvido,
	).Scan(&a.ID, &a.CriadoEm, &a.AtualizadoEm)
	if err != nil {
		return plano.Anotacao{}, fmt.Errorf("atualizando anotação: %w", err)
	}

	return a, nil
}

func (r *CadernoRepo) RemoverAnotacao(
	ctx context.Context,
	planoID, anotacaoID uuid.UUID,
) error {
	if _, err := r.pool.Exec(
		ctx,
		`DELETE FROM anotacoes WHERE id = $1 AND plano_id = $2`,
		anotacaoID, planoID,
	); err != nil {
		return fmt.Errorf("removendo anotação: %w", err)
	}

	return nil
}
