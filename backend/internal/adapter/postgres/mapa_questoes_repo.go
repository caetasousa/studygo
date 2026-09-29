package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"studygo/internal/domain/mapa"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *MapaRepo) QuestoesGravadas(ctx context.Context, mapaID uuid.UUID) ([]mapa.QuestaoGravada, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, chave, assinatura, ativa FROM mapas_questoes WHERE mapa_id = $1`, mapaID)
	if err != nil {
		return nil, fmt.Errorf("listando questões gravadas: %w", err)
	}

	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (mapa.QuestaoGravada, error) {
		var g mapa.QuestaoGravada
		err := row.Scan(&g.ID, &g.Chave, &g.Assinatura, &g.Ativa)

		return g, err
	})
	if err != nil {
		return nil, fmt.Errorf("lendo questões gravadas: %w", err)
	}

	return out, nil
}

func (r *MapaRepo) GravarQuestoes(ctx context.Context, mapaID uuid.UUID, plano mapa.PlanoDeQuestoes) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("abrindo transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	// A ordem de cada questão é a posição no arquivo; a nova entra já nela, e
	// as outras são reposicionadas no fim.
	posicao := make(map[string]int, len(plano.Ordem))
	for i, chave := range plano.Ordem {
		posicao[chave] = i
	}

	b := &pgx.Batch{}

	for _, q := range plano.Novas {
		b.Queue(
			`INSERT INTO mapas_questoes
			   (mapa_id, chave, ordem, ramo, origem, enunciado, alternativas, gabarito, comentario, assinatura)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			mapaID, q.Chave, posicao[q.Chave], q.Ramo, q.Origem, q.Enunciado, alternativasOuVazio(q.Alternativas),
			q.Gabarito, q.Comentario, q.Assinatura(),
		)
	}

	for _, a := range plano.Atualizadas {
		q := a.Questao
		b.Queue(
			`UPDATE mapas_questoes
			    SET ramo = $2, origem = $3, enunciado = $4, alternativas = $5, gabarito = $6,
			        comentario = $7, assinatura = $8, ativa = true
			  WHERE id = $1`,
			a.ID, q.Ramo, q.Origem, q.Enunciado, alternativasOuVazio(q.Alternativas), q.Gabarito, q.Comentario, q.Assinatura(),
		)
	}

	for _, id := range plano.Desativar {
		b.Queue(`UPDATE mapas_questoes SET ativa = false WHERE id = $1`, id)
	}

	for chave, ordem := range posicao {
		b.Queue(`UPDATE mapas_questoes SET ordem = $3 WHERE mapa_id = $1 AND chave = $2`, mapaID, chave, ordem)
	}

	if err := tx.SendBatch(ctx, b).Close(); err != nil {
		return fmt.Errorf("gravando questões do mapa: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmando questões do mapa: %w", err)
	}

	return nil
}

// alternativasOuVazio garante o array vazio (e não NULL) da questão de julgar.
func alternativasOuVazio(as []string) []string {
	if as == nil {
		return []string{}
	}

	return as
}

// colunasDaQuestao traz a questão e a última resposta a ela, se houver.
const colunasDaQuestao = `q.id, q.chave, q.ramo, q.origem, q.enunciado, q.alternativas, q.gabarito, q.comentario,
	u.resposta, u.acertou, u.respondida_em`

const ultimaResposta = `LEFT JOIN LATERAL (
	SELECT resposta, acertou, respondida_em FROM mapas_respostas
	 WHERE questao_id = q.id ORDER BY respondida_em DESC, id DESC LIMIT 1
) u ON true`

func lerQuestao(row pgx.Row) (mapa.QuestaoComResposta, error) {
	var (
		q        mapa.QuestaoComResposta
		resposta *string
		acertou  *bool
		em       *time.Time
	)

	err := row.Scan(&q.ID, &q.Questao.Chave, &q.Questao.Ramo, &q.Questao.Origem, &q.Questao.Enunciado,
		&q.Questao.Alternativas, &q.Questao.Gabarito, &q.Questao.Comentario, &resposta, &acertou, &em)
	if err != nil {
		return q, err
	}

	if len(q.Questao.Alternativas) == 0 {
		q.Questao.Alternativas = nil
	}

	if resposta != nil {
		q.Ultima = &mapa.Resposta{Resposta: *resposta, Acertou: *acertou, Em: *em}
	}

	return q, nil
}

func (r *MapaRepo) Questoes(ctx context.Context, mapaID uuid.UUID) ([]mapa.QuestaoComResposta, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+colunasDaQuestao+` FROM mapas_questoes q `+ultimaResposta+`
		  WHERE q.mapa_id = $1 AND q.ativa
		  ORDER BY q.ordem, q.chave`, mapaID)
	if err != nil {
		return nil, fmt.Errorf("listando questões do mapa: %w", err)
	}

	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (mapa.QuestaoComResposta, error) {
		return lerQuestao(row)
	})
	if err != nil {
		return nil, fmt.Errorf("lendo questões do mapa: %w", err)
	}

	return out, nil
}

func (r *MapaRepo) QuestaoDoDono(ctx context.Context, usuarioID, questaoID uuid.UUID) (mapa.QuestaoComResposta, error) {
	q, err := lerQuestao(r.pool.QueryRow(ctx,
		`SELECT `+colunasDaQuestao+` FROM mapas_questoes q
		   JOIN mapas m ON m.id = q.mapa_id `+ultimaResposta+`
		  WHERE q.id = $1 AND m.usuario_id = $2 AND q.ativa`, questaoID, usuarioID))

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return mapa.QuestaoComResposta{}, mapa.ErrQuestaoNaoEncontrada
	case err != nil:
		return mapa.QuestaoComResposta{}, fmt.Errorf("lendo questão do mapa: %w", err)
	}

	return q, nil
}

func (r *MapaRepo) Responder(ctx context.Context, questaoID uuid.UUID, resp mapa.Resposta) (mapa.Resposta, error) {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO mapas_respostas (questao_id, resposta, acertou) VALUES ($1,$2,$3) RETURNING respondida_em`,
		questaoID, resp.Resposta, resp.Acertou,
	).Scan(&resp.Em)
	if err != nil {
		return mapa.Resposta{}, fmt.Errorf("gravando resposta: %w", err)
	}

	return resp, nil
}
