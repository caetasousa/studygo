package postgres

import (
	"context"
	"errors"
	"fmt"

	"studygo/internal/domain/concurso"
	"studygo/internal/port"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ port.ConcursoRepository = (*ConcursoRepo)(nil)

// ConcursoRepo persiste o catálogo da prova.
type ConcursoRepo struct {
	pool *pgxpool.Pool
}

func NewConcursoRepo(pool *pgxpool.Pool) *ConcursoRepo {
	return &ConcursoRepo{pool: pool}
}

const colunasConcurso = `id, dono_id, slug, nome, banca, cargo, emoji,
	prova_padrao, reta_padrao_dias, resumo`

func (r *ConcursoRepo) ListarPorDono(
	ctx context.Context,
	donoID uuid.UUID,
) ([]concurso.Concurso, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT `+colunasConcurso+` FROM concursos
		  WHERE dono_id = $1 ORDER BY prova_padrao`,
		donoID,
	)
	if err != nil {
		return nil, fmt.Errorf("listando concursos: %w", err)
	}
	defer rows.Close()

	out := []concurso.Concurso{}

	for rows.Next() {
		c, err := escanearConcurso(rows)
		if err != nil {
			return nil, fmt.Errorf("lendo concurso: %w", err)
		}

		out = append(out, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterando concursos: %w", err)
	}

	return out, nil
}

func (r *ConcursoRepo) PorSlug(ctx context.Context, slug string) (concurso.Concurso, error) {
	return r.carregar(ctx, `WHERE slug = $1`, slug)
}

func (r *ConcursoRepo) PorID(ctx context.Context, id uuid.UUID) (concurso.Concurso, error) {
	return r.carregar(ctx, `WHERE id = $1`, id)
}

func escanearConcurso(linha pgx.Row) (concurso.Concurso, error) {
	var c concurso.Concurso

	err := linha.Scan(
		&c.ID, &c.DonoID, &c.Slug, &c.Nome, &c.Banca, &c.Cargo, &c.Emoji,
		&c.ProvaPadrao, &c.RetaPadraoDias, &c.Resumo,
	)

	return c, err
}

func (r *ConcursoRepo) carregar(
	ctx context.Context,
	onde string,
	arg any,
) (concurso.Concurso, error) {
	c, err := escanearConcurso(r.pool.QueryRow(
		ctx, `SELECT `+colunasConcurso+` FROM concursos `+onde, arg,
	))

	if errors.Is(err, pgx.ErrNoRows) {
		return concurso.Concurso{}, concurso.ErrNaoEncontrado
	}

	if err != nil {
		return concurso.Concurso{}, fmt.Errorf("consultando concurso: %w", err)
	}

	if c.Disciplinas, err = r.disciplinas(ctx, c.ID); err != nil {
		return concurso.Concurso{}, err
	}

	if c.Marcos, err = r.marcos(ctx, c.ID); err != nil {
		return concurso.Concurso{}, err
	}

	if c.Conteudo, err = r.conteudo(ctx, c.ID); err != nil {
		return concurso.Concurso{}, err
	}

	return c, nil
}

func (r *ConcursoRepo) Criar(
	ctx context.Context,
	c concurso.Concurso,
) (concurso.Concurso, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return concurso.Concurso{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	err = tx.QueryRow(
		ctx,
		`INSERT INTO concursos
		   (dono_id, slug, nome, banca, cargo, emoji, prova_padrao,
		    reta_padrao_dias, resumo)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id`,
		c.DonoID, c.Slug, c.Nome, c.Banca, c.Cargo, c.Emoji,
		c.ProvaPadrao, c.RetaPadraoDias, c.Resumo,
	).Scan(&c.ID)
	if err != nil {
		if violaUnique(err) {
			return concurso.Concurso{}, fmt.Errorf("slug %q já existe", c.Slug)
		}

		return concurso.Concurso{}, fmt.Errorf("inserindo concurso: %w", err)
	}

	if err := gravarFilhos(ctx, tx, &c); err != nil {
		return concurso.Concurso{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return concurso.Concurso{}, fmt.Errorf("commit: %w", err)
	}

	return c, nil
}

// Atualizar grava o concurso preservando a IDENTIDADE das disciplinas.
//
// Antes isto apagava todas as disciplinas e as reinseria com ids novos. Como o
// cronograma e o histórico apontam para a disciplina, editar qualquer campo do
// concurso desligava tudo que o estudante já tinha feito. Agora cada disciplina
// que chega com id é atualizada no lugar; só as que sumiram da lista são
// removidas, e aí o cascade é o comportamento correto.
func (r *ConcursoRepo) Atualizar(
	ctx context.Context,
	c concurso.Concurso,
) (concurso.Concurso, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return concurso.Concurso{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	ct, err := tx.Exec(
		ctx,
		`UPDATE concursos SET
		   nome = $2, banca = $3, cargo = $4, emoji = $5, prova_padrao = $6,
		   reta_padrao_dias = $7, resumo = $8, atualizado_em = now()
		 WHERE id = $1 AND dono_id = $9`,
		c.ID, c.Nome, c.Banca, c.Cargo, c.Emoji, c.ProvaPadrao,
		c.RetaPadraoDias, c.Resumo, c.DonoID,
	)
	if err != nil {
		return concurso.Concurso{}, fmt.Errorf("atualizando concurso: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return concurso.Concurso{}, concurso.ErrNaoEncontrado
	}

	if _, err := tx.Exec(
		ctx, `DELETE FROM conteudo_programatico WHERE concurso_id = $1`, c.ID,
	); err != nil {
		return concurso.Concurso{}, fmt.Errorf("limpando conteúdo: %w", err)
	}

	if err := gravarFilhos(ctx, tx, &c); err != nil {
		return concurso.Concurso{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return concurso.Concurso{}, fmt.Errorf("commit: %w", err)
	}

	return c, nil
}

func (r *ConcursoRepo) Remover(ctx context.Context, id uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM concursos WHERE id = $1`, id); err != nil {
		return fmt.Errorf("removendo concurso: %w", err)
	}

	return nil
}

func (r *ConcursoRepo) DefinirCadernoURL(
	ctx context.Context,
	concursoID uuid.UUID,
	codigo, url string,
) error {
	ct, err := r.pool.Exec(
		ctx,
		`UPDATE disciplinas SET caderno_url = $3
		  WHERE concurso_id = $1 AND codigo = $2`,
		concursoID, codigo, url,
	)
	if err != nil {
		return fmt.Errorf("atualizando caderno_url: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return concurso.ErrNaoEncontrado
	}

	return nil
}

// gravarFilhos grava disciplinas (com temas e fontes), marcos e conteúdo.
// Disciplinas e marcos que já existem são atualizados pelo id; os que saíram da
// lista são removidos.
func gravarFilhos(ctx context.Context, tx pgx.Tx, c *concurso.Concurso) error {
	if err := gravarDisciplinas(ctx, tx, c); err != nil {
		return err
	}

	if err := gravarMarcos(ctx, tx, c); err != nil {
		return err
	}

	return gravarConteudo(ctx, tx, c)
}

func gravarDisciplinas(ctx context.Context, tx pgx.Tx, c *concurso.Concurso) error {
	sobrevivem := make([]uuid.UUID, 0, len(c.Disciplinas))

	for i := range c.Disciplinas {
		d := &c.Disciplinas[i]

		// Disciplina nova: o banco gera o id. Disciplina existente: o id é a
		// identidade e não muda, então o UPDATE encontra a linha e o cronograma
		// continua apontando para a matéria certa.
		if d.ID == uuid.Nil {
			if err := tx.QueryRow(
				ctx,
				`INSERT INTO disciplinas
				   (concurso_id, codigo, nome, bloco, peso, questoes_padrao, ordem, caderno_url)
				 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
				 RETURNING id`,
				c.ID, d.Codigo, d.Nome, string(d.Bloco), d.Peso,
				d.QuestoesPadrao, d.Ordem, d.CadernoURL,
			).Scan(&d.ID); err != nil {
				return fmt.Errorf("inserindo disciplina %s: %w", d.Nome, err)
			}
		} else if _, err := tx.Exec(
			ctx,
			`UPDATE disciplinas SET
			   codigo = $3, nome = $4, bloco = $5, peso = $6,
			   questoes_padrao = $7, ordem = $8, caderno_url = $9
			 WHERE id = $1 AND concurso_id = $2`,
			d.ID, c.ID, d.Codigo, d.Nome, string(d.Bloco), d.Peso,
			d.QuestoesPadrao, d.Ordem, d.CadernoURL,
		); err != nil {
			return fmt.Errorf("atualizando disciplina %s: %w", d.Nome, err)
		}

		sobrevivem = append(sobrevivem, d.ID)
	}

	// Só as disciplinas que o usuário de fato removeu saem — e aí levar junto o
	// que estava agendado nelas é o comportamento certo.
	if _, err := tx.Exec(
		ctx,
		`DELETE FROM disciplinas WHERE concurso_id = $1 AND NOT (id = ANY($2))`,
		c.ID, sobrevivem,
	); err != nil {
		return fmt.Errorf("removendo disciplinas que saíram: %w", err)
	}

	// Temas e fontes são listas pequenas, completas e sem identidade própria:
	// recriá-las é mais simples e mais correto que diferenciar.
	if _, err := tx.Exec(
		ctx,
		`DELETE FROM temas WHERE disciplina_id = ANY($1)`, sobrevivem,
	); err != nil {
		return fmt.Errorf("limpando temas: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`DELETE FROM fontes WHERE disciplina_id = ANY($1)`, sobrevivem,
	); err != nil {
		return fmt.Errorf("limpando fontes: %w", err)
	}

	// Um edital real traz dezenas de temas por matéria, então tudo vai num lote
	// só em vez de uma ida ao banco por linha.
	lote := &pgx.Batch{}

	for i := range c.Disciplinas {
		d := &c.Disciplinas[i]

		for j, tema := range d.Temas {
			lote.Queue(
				`INSERT INTO temas (disciplina_id, ordem, texto) VALUES ($1,$2,$3)`,
				d.ID, j, tema,
			)
		}

		for j := range d.Fontes {
			f := d.Fontes[j]
			lote.Queue(
				`INSERT INTO fontes (disciplina_id, ordem, titulo, url, tipo)
				 VALUES ($1,$2,$3,$4,$5)`,
				d.ID, j, f.Titulo, f.URL, f.Tipo,
			)
		}
	}

	if lote.Len() > 0 {
		if err := tx.SendBatch(ctx, lote).Close(); err != nil {
			return fmt.Errorf("gravando temas e fontes: %w", err)
		}
	}

	return nil
}

// gravarMarcos atualiza os marcos no lugar, para que os "cumprido" de
// marco_checks sobrevivam à edição do concurso.
func gravarMarcos(ctx context.Context, tx pgx.Tx, c *concurso.Concurso) error {
	for i := range c.Marcos {
		m := &c.Marcos[i]
		m.Ordem = i
		m.Rotulo = i + 1

		if err := tx.QueryRow(
			ctx,
			`INSERT INTO marcos
			   (concurso_id, ordem, rotulo, data_inicio, data_fim, titulo,
			    exige_acao, e_prova)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			 ON CONFLICT (concurso_id, ordem) DO UPDATE SET
			   rotulo = EXCLUDED.rotulo, data_inicio = EXCLUDED.data_inicio,
			   data_fim = EXCLUDED.data_fim, titulo = EXCLUDED.titulo,
			   exige_acao = EXCLUDED.exige_acao, e_prova = EXCLUDED.e_prova
			 RETURNING id`,
			c.ID, m.Ordem, m.Rotulo, m.DataInicio, m.DataFim, m.Titulo,
			m.ExigeAcao, m.EProva,
		).Scan(&m.ID); err != nil {
			return fmt.Errorf("gravando marco: %w", err)
		}
	}

	if _, err := tx.Exec(
		ctx,
		`DELETE FROM marcos WHERE concurso_id = $1 AND ordem >= $2`,
		c.ID, len(c.Marcos),
	); err != nil {
		return fmt.Errorf("removendo marcos que saíram: %w", err)
	}

	return nil
}

func gravarConteudo(ctx context.Context, tx pgx.Tx, c *concurso.Concurso) error {
	if len(c.Conteudo) == 0 {
		return nil
	}

	lote := &pgx.Batch{}

	for i, item := range c.Conteudo {
		lote.Queue(
			`INSERT INTO conteudo_programatico (concurso_id, ordem, tipo, texto)
			 VALUES ($1,$2,$3,$4)`,
			c.ID, i, item.Tipo, item.Texto,
		)
	}

	if err := tx.SendBatch(ctx, lote).Close(); err != nil {
		return fmt.Errorf("gravando conteúdo: %w", err)
	}

	return nil
}

func (r *ConcursoRepo) disciplinas(
	ctx context.Context,
	concursoID uuid.UUID,
) ([]concurso.Disciplina, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT id, codigo, nome, bloco, peso, questoes_padrao, ordem, caderno_url
		   FROM disciplinas WHERE concurso_id = $1 ORDER BY ordem`,
		concursoID,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando disciplinas: %w", err)
	}
	defer rows.Close()

	out := []concurso.Disciplina{}
	ids := []uuid.UUID{}

	for rows.Next() {
		var (
			d     concurso.Disciplina
			bloco string
		)

		if err := rows.Scan(
			&d.ID, &d.Codigo, &d.Nome, &bloco, &d.Peso,
			&d.QuestoesPadrao, &d.Ordem, &d.CadernoURL,
		); err != nil {
			return nil, fmt.Errorf("lendo disciplina: %w", err)
		}

		d.Bloco = concurso.Bloco(bloco)
		out = append(out, d)
		ids = append(ids, d.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterando disciplinas: %w", err)
	}

	if len(out) == 0 {
		return out, nil
	}

	temas, err := r.temas(ctx, ids)
	if err != nil {
		return nil, err
	}

	fontes, err := r.fontes(ctx, ids)
	if err != nil {
		return nil, err
	}

	for i := range out {
		out[i].Temas = temas[out[i].ID]
		out[i].Fontes = fontes[out[i].ID]
	}

	return out, nil
}

// temas e fontes carregam TODAS as disciplinas de uma vez (WHERE ... = ANY),
// em vez de uma consulta por matéria.
func (r *ConcursoRepo) temas(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID][]string, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT disciplina_id, texto FROM temas
		  WHERE disciplina_id = ANY($1) ORDER BY disciplina_id, ordem`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando temas: %w", err)
	}
	defer rows.Close()

	out := map[uuid.UUID][]string{}

	for rows.Next() {
		var (
			id    uuid.UUID
			texto string
		)

		if err := rows.Scan(&id, &texto); err != nil {
			return nil, fmt.Errorf("lendo tema: %w", err)
		}

		out[id] = append(out[id], texto)
	}

	return out, rows.Err()
}

func (r *ConcursoRepo) fontes(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID][]concurso.Fonte, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT disciplina_id, ordem, titulo, url, tipo FROM fontes
		  WHERE disciplina_id = ANY($1) ORDER BY disciplina_id, ordem`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando fontes: %w", err)
	}
	defer rows.Close()

	out := map[uuid.UUID][]concurso.Fonte{}

	for rows.Next() {
		var (
			id uuid.UUID
			f  concurso.Fonte
		)

		if err := rows.Scan(&id, &f.Ordem, &f.Titulo, &f.URL, &f.Tipo); err != nil {
			return nil, fmt.Errorf("lendo fonte: %w", err)
		}

		out[id] = append(out[id], f)
	}

	return out, rows.Err()
}

func (r *ConcursoRepo) marcos(
	ctx context.Context,
	concursoID uuid.UUID,
) ([]concurso.Marco, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT id, ordem, rotulo, data_inicio, data_fim, titulo, exige_acao, e_prova
		   FROM marcos WHERE concurso_id = $1 ORDER BY ordem`,
		concursoID,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando marcos: %w", err)
	}
	defer rows.Close()

	out := []concurso.Marco{}

	for rows.Next() {
		var m concurso.Marco

		if err := rows.Scan(
			&m.ID, &m.Ordem, &m.Rotulo, &m.DataInicio, &m.DataFim,
			&m.Titulo, &m.ExigeAcao, &m.EProva,
		); err != nil {
			return nil, fmt.Errorf("lendo marco: %w", err)
		}

		out = append(out, m)
	}

	return out, rows.Err()
}

func (r *ConcursoRepo) conteudo(
	ctx context.Context,
	concursoID uuid.UUID,
) ([]concurso.ConteudoItem, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT ordem, tipo, texto FROM conteudo_programatico
		  WHERE concurso_id = $1 ORDER BY ordem`,
		concursoID,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando conteúdo: %w", err)
	}
	defer rows.Close()

	out := []concurso.ConteudoItem{}

	for rows.Next() {
		var it concurso.ConteudoItem

		if err := rows.Scan(&it.Ordem, &it.Tipo, &it.Texto); err != nil {
			return nil, fmt.Errorf("lendo item de conteúdo: %w", err)
		}

		out = append(out, it)
	}

	return out, rows.Err()
}

func violaUnique(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
