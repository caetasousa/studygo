package postgres

import (
	"context"
	"errors"
	"fmt"

	"studygo/internal/domain/mapa"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *MapaRepo) NomesDasImagens(ctx context.Context, mapaID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT nome FROM mapas_imagens WHERE mapa_id = $1 ORDER BY nome`, mapaID)
	if err != nil {
		return nil, fmt.Errorf("listando imagens do mapa: %w", err)
	}

	nomes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("lendo imagens do mapa: %w", err)
	}

	return nomes, nil
}

func (r *MapaRepo) GravarImagens(ctx context.Context, mapaID uuid.UUID, imagens []mapa.Imagem, teto int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("abrindo transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	// A trava no mapa faz dois envios ao mesmo tempo contarem um depois do
	// outro: sem ela, cada um veria o teto livre e os dois juntos o passariam.
	if _, err := tx.Exec(ctx, `SELECT 1 FROM mapas WHERE id = $1 FOR UPDATE`, mapaID); err != nil {
		return fmt.Errorf("travando o mapa: %w", err)
	}

	batch := &pgx.Batch{}
	for _, img := range imagens {
		batch.Queue(
			`INSERT INTO mapas_imagens (mapa_id, nome, tipo, dados) VALUES ($1, $2, $3, $4)
			 ON CONFLICT (mapa_id, nome) DO UPDATE
			    SET tipo = EXCLUDED.tipo, dados = EXCLUDED.dados, enviada_em = now()`,
			mapaID, img.Nome, img.Tipo, img.Dados,
		)
	}

	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("gravando imagens do mapa: %w", err)
	}

	var total int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM mapas_imagens WHERE mapa_id = $1`, mapaID).Scan(&total); err != nil {
		return fmt.Errorf("contando imagens do mapa: %w", err)
	}

	if total > teto {
		return mapa.ErrImagensInvalidas{Problemas: []string{
			fmt.Sprintf("o mapa passaria de %d imagens (ficaria com %d)", teto, total),
		}}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmando as imagens: %w", err)
	}

	return nil
}

func (r *MapaRepo) Imagem(ctx context.Context, mapaID uuid.UUID, nome string) (mapa.Imagem, error) {
	img := mapa.Imagem{Nome: nome}

	err := r.pool.QueryRow(ctx,
		`SELECT tipo, dados FROM mapas_imagens WHERE mapa_id = $1 AND nome = $2`, mapaID, nome,
	).Scan(&img.Tipo, &img.Dados)
	if errors.Is(err, pgx.ErrNoRows) {
		return mapa.Imagem{}, mapa.ErrImagemNaoEncontrada
	}

	if err != nil {
		return mapa.Imagem{}, fmt.Errorf("lendo imagem do mapa: %w", err)
	}

	return img, nil
}
