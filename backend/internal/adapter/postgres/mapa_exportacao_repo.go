package postgres

import (
	"context"
	"fmt"

	"studygo/internal/domain/mapa"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// VinculosDoMapa lista as matérias a que o mapa está vinculado, com o concurso
// de cada uma, pelo que se reconhece fora desta conta.
func (r *MapaRepo) VinculosDoMapa(ctx context.Context, mapaID uuid.UUID) ([]mapa.VinculoExportado, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.slug, c.nome, d.codigo, d.nome, dm.temas
		   FROM disciplinas_mapas dm
		   JOIN disciplinas d ON d.id = dm.disciplina_id
		   JOIN concursos c ON c.id = d.concurso_id
		  WHERE dm.mapa_id = $1
		  ORDER BY c.nome, c.slug, d.codigo`, mapaID)
	if err != nil {
		return nil, fmt.Errorf("listando vínculos do mapa: %w", err)
	}

	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (mapa.VinculoExportado, error) {
		var v mapa.VinculoExportado
		err := row.Scan(&v.ConcursoSlug, &v.ConcursoNome, &v.Codigo, &v.Disciplina, &v.Temas)

		return v, err
	})
	if err != nil {
		return nil, fmt.Errorf("lendo vínculos do mapa: %w", err)
	}

	return out, nil
}

// RespostasDoMapa lista as tentativas nas questões ativas do mapa, da mais
// antiga para a mais recente.
func (r *MapaRepo) RespostasDoMapa(ctx context.Context, mapaID uuid.UUID) ([]mapa.RespostaExportada, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT q.chave, r.resposta, r.acertou, r.respondida_em
		   FROM mapas_respostas r
		   JOIN mapas_questoes q ON q.id = r.questao_id
		  WHERE q.mapa_id = $1 AND q.ativa
		  ORDER BY r.respondida_em, q.chave`, mapaID)
	if err != nil {
		return nil, fmt.Errorf("listando respostas do mapa: %w", err)
	}

	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (mapa.RespostaExportada, error) {
		var x mapa.RespostaExportada
		err := row.Scan(&x.Chave, &x.Resposta, &x.Acertou, &x.Em)

		return x, err
	})
	if err != nil {
		return nil, fmt.Errorf("lendo respostas do mapa: %w", err)
	}

	return out, nil
}

// RestaurarRespostas grava as tentativas que voltam de uma exportação. A que
// já existe (mesma questão, mesma hora) fica como está: importar o mesmo .zip
// duas vezes não dobra o histórico.
func (r *MapaRepo) RestaurarRespostas(ctx context.Context, respostas []mapa.RespostaRestaurada) (int, error) {
	if len(respostas) == 0 {
		return 0, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("abrindo transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	b := &pgx.Batch{}
	for _, x := range respostas {
		b.Queue(
			`INSERT INTO mapas_respostas (questao_id, resposta, acertou, respondida_em)
			 SELECT $1, $2, $3, $4
			  WHERE NOT EXISTS (SELECT 1 FROM mapas_respostas WHERE questao_id = $1 AND respondida_em = $4)`,
			x.QuestaoID, x.Resposta.Resposta, x.Resposta.Acertou, x.Resposta.Em,
		)
	}

	res := tx.SendBatch(ctx, b)

	gravadas := 0
	for range respostas {
		tag, err := res.Exec()
		if err != nil {
			_ = res.Close()

			return 0, fmt.Errorf("restaurando resposta: %w", err)
		}

		gravadas += int(tag.RowsAffected())
	}

	if err := res.Close(); err != nil {
		return 0, fmt.Errorf("restaurando respostas: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("gravando respostas: %w", err)
	}

	return gravadas, nil
}
