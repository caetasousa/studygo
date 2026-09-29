package postgres

import (
	"context"
	"errors"
	"fmt"

	"studygo/internal/domain/mapa"
	"studygo/internal/port"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ port.MapaRepository = (*MapaRepo)(nil)

// MapaRepo persiste os mapas mentais.
type MapaRepo struct {
	pool *pgxpool.Pool
}

func NewMapaRepo(pool *pgxpool.Pool) *MapaRepo {
	return &MapaRepo{pool: pool}
}

// colunasDoResumo é o resumo de um mapa m: os ramos são os itens sem pai.
const colunasDoResumo = `m.id, m.slug, m.titulo, m.fonte, m.materia, m.importado_em,
	(SELECT count(*) FROM mapas_itens i WHERE i.mapa_id = m.id AND i.pai IS NULL),
	(SELECT count(*) FROM mapas_itens i WHERE i.mapa_id = m.id)`

func lerResumo(row pgx.Row) (mapa.Resumo, error) {
	var r mapa.Resumo

	err := row.Scan(&r.ID, &r.Slug, &r.Titulo, &r.Fonte, &r.Materia, &r.ImportadoEm, &r.Ramos, &r.Itens)

	return r, err
}

func (r *MapaRepo) Catalogo(ctx context.Context, usuarioID uuid.UUID) ([]mapa.Resumo, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT `+colunasDoResumo+`
		   FROM mapas m
		  WHERE m.usuario_id = $1
		  ORDER BY m.importado_em DESC, m.titulo`,
		usuarioID,
	)
	if err != nil {
		return nil, fmt.Errorf("listando mapas: %w", err)
	}
	defer rows.Close()

	var out []mapa.Resumo

	for rows.Next() {
		resumo, err := lerResumo(rows)
		if err != nil {
			return nil, fmt.Errorf("lendo mapa: %w", err)
		}

		out = append(out, resumo)
	}

	return out, rows.Err()
}

func (r *MapaRepo) ResumoPorSlug(ctx context.Context, usuarioID uuid.UUID, slug string) (mapa.Resumo, error) {
	resumo, err := lerResumo(r.pool.QueryRow(
		ctx,
		`SELECT `+colunasDoResumo+` FROM mapas m WHERE m.usuario_id = $1 AND m.slug = $2`,
		usuarioID, slug,
	))

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return mapa.Resumo{}, mapa.ErrNaoEncontrado
	case err != nil:
		return mapa.Resumo{}, fmt.Errorf("lendo mapa: %w", err)
	}

	return resumo, nil
}

func (r *MapaRepo) PorSlug(ctx context.Context, usuarioID uuid.UUID, slug string) (mapa.Mapa, error) {
	resumo, err := r.ResumoPorSlug(ctx, usuarioID, slug)
	if err != nil {
		return mapa.Mapa{}, err
	}

	rows, err := r.pool.Query(
		ctx,
		`SELECT ordem, pai, texto, marca FROM mapas_itens WHERE mapa_id = $1 ORDER BY ordem`,
		resumo.ID,
	)
	if err != nil {
		return mapa.Mapa{}, fmt.Errorf("lendo itens do mapa: %w", err)
	}
	defer rows.Close()

	// As linhas vêm em pré-ordem e `ordem` é o índice delas: o pai de cada uma já
	// foi lido quando ela chega.
	type no struct {
		item   mapa.Item
		filhos []int
	}

	var (
		nos    []no
		raizes []int
	)

	for rows.Next() {
		var (
			ordem int
			pai   *int
			texto string
			marca string
		)

		if err := rows.Scan(&ordem, &pai, &texto, &marca); err != nil {
			return mapa.Mapa{}, fmt.Errorf("lendo item do mapa: %w", err)
		}

		if ordem != len(nos) || (pai != nil && *pai >= ordem) {
			return mapa.Mapa{}, fmt.Errorf("itens do mapa %s fora de ordem na linha %d", slug, ordem)
		}

		nos = append(nos, no{item: mapa.Item{Texto: texto, Marca: mapa.Marca(marca)}})

		if pai == nil {
			raizes = append(raizes, ordem)
		} else {
			nos[*pai].filhos = append(nos[*pai].filhos, ordem)
		}
	}

	if err := rows.Err(); err != nil {
		return mapa.Mapa{}, fmt.Errorf("lendo itens do mapa: %w", err)
	}

	var montar func(indices []int) []mapa.Item

	montar = func(indices []int) []mapa.Item {
		out := make([]mapa.Item, 0, len(indices))

		for _, i := range indices {
			item := nos[i].item
			item.Filhos = montar(nos[i].filhos)
			out = append(out, item)
		}

		return out
	}

	return mapa.Mapa{
		ID: resumo.ID, Slug: resumo.Slug, Titulo: resumo.Titulo, Fonte: resumo.Fonte,
		Materia: resumo.Materia, Ramos: montar(raizes),
	}, nil
}

func (r *MapaRepo) Gravar(ctx context.Context, usuarioID uuid.UUID, m mapa.Mapa) (mapa.Resumo, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapa.Resumo{}, false, fmt.Errorf("abrindo transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	// Criar primeiro e, se o slug já existe, trocar: duas importações ao mesmo
	// tempo do mesmo slug não disputam um "existe?" seguido de INSERT.
	var (
		id   uuid.UUID
		novo = true
	)

	err = tx.QueryRow(
		ctx,
		`INSERT INTO mapas (usuario_id, slug, titulo, fonte, materia) VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (usuario_id, slug) DO NOTHING
		 RETURNING id`,
		usuarioID, m.Slug, m.Titulo, m.Fonte, m.Materia,
	).Scan(&id)

	if errors.Is(err, pgx.ErrNoRows) {
		novo = false

		err = tx.QueryRow(
			ctx,
			`UPDATE mapas SET titulo = $3, fonte = $4, materia = $5, importado_em = now()
			  WHERE usuario_id = $1 AND slug = $2
			  RETURNING id`,
			usuarioID, m.Slug, m.Titulo, m.Fonte, m.Materia,
		).Scan(&id)
		if err == nil {
			_, err = tx.Exec(ctx, `DELETE FROM mapas_itens WHERE mapa_id = $1`, id)
		}
	}

	if err != nil {
		return mapa.Resumo{}, false, fmt.Errorf("gravando mapa: %w", err)
	}

	// Uma única ida ao banco para os itens: um mapa passa de mil linhas.
	if _, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"mapas_itens"},
		[]string{"mapa_id", "ordem", "pai", "texto", "marca"},
		pgx.CopyFromRows(linhasDoMapa(id, m.Ramos)),
	); err != nil {
		return mapa.Resumo{}, false, fmt.Errorf("gravando itens do mapa: %w", err)
	}

	resumo, err := lerResumo(tx.QueryRow(ctx, `SELECT `+colunasDoResumo+` FROM mapas m WHERE m.id = $1`, id))
	if err != nil {
		return mapa.Resumo{}, false, fmt.Errorf("lendo o mapa gravado: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return mapa.Resumo{}, false, fmt.Errorf("confirmando o mapa: %w", err)
	}

	return resumo, novo, nil
}

// linhasDoMapa achata a árvore em pré-ordem: cada item leva a `ordem` do pai, e
// os ramos, nulo. É o formato que a tabela guarda.
func linhasDoMapa(id uuid.UUID, ramos []mapa.Item) [][]any {
	var (
		linhas [][]any
		descer func(itens []mapa.Item, pai any)
	)

	descer = func(itens []mapa.Item, pai any) {
		for _, it := range itens {
			ordem := len(linhas)
			linhas = append(linhas, []any{id, ordem, pai, it.Texto, string(it.Marca)})

			descer(it.Filhos, ordem)
		}
	}

	descer(ramos, nil)

	return linhas
}

func (r *MapaRepo) Excluir(ctx context.Context, usuarioID uuid.UUID, slug string) error {
	res, err := r.pool.Exec(ctx, `DELETE FROM mapas WHERE usuario_id = $1 AND slug = $2`, usuarioID, slug)
	if err != nil {
		return fmt.Errorf("excluindo mapa: %w", err)
	}

	if res.RowsAffected() == 0 {
		return mapa.ErrNaoEncontrado
	}

	return nil
}

func (r *MapaRepo) Vinculos(ctx context.Context, concursoID uuid.UUID) (map[uuid.UUID][]mapa.Resumo, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT dm.disciplina_id, `+colunasDoResumo+`
		   FROM disciplinas_mapas dm
		   JOIN disciplinas d ON d.id = dm.disciplina_id
		   JOIN mapas m ON m.id = dm.mapa_id
		  WHERE d.concurso_id = $1
		  ORDER BY m.titulo`,
		concursoID,
	)
	if err != nil {
		return nil, fmt.Errorf("listando vínculos dos mapas: %w", err)
	}
	defer rows.Close()

	out := map[uuid.UUID][]mapa.Resumo{}

	for rows.Next() {
		var (
			disciplina uuid.UUID
			resumo     mapa.Resumo
		)

		if err := rows.Scan(
			&disciplina, &resumo.ID, &resumo.Slug, &resumo.Titulo, &resumo.Fonte, &resumo.Materia,
			&resumo.ImportadoEm, &resumo.Ramos, &resumo.Itens,
		); err != nil {
			return nil, fmt.Errorf("lendo vínculo do mapa: %w", err)
		}

		out[disciplina] = append(out[disciplina], resumo)
	}

	return out, rows.Err()
}

func (r *MapaRepo) Vincular(ctx context.Context, disciplinaID, mapaID uuid.UUID) error {
	if _, err := r.pool.Exec(
		ctx,
		`INSERT INTO disciplinas_mapas (disciplina_id, mapa_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		disciplinaID, mapaID,
	); err != nil {
		return fmt.Errorf("vinculando mapa: %w", err)
	}

	return nil
}

func (r *MapaRepo) Desvincular(ctx context.Context, disciplinaID, mapaID uuid.UUID) error {
	if _, err := r.pool.Exec(
		ctx, `DELETE FROM disciplinas_mapas WHERE disciplina_id = $1 AND mapa_id = $2`, disciplinaID, mapaID,
	); err != nil {
		return fmt.Errorf("desvinculando mapa: %w", err)
	}

	return nil
}
