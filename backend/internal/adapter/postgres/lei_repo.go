package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"studygo/internal/domain/lei"
	"studygo/internal/port"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ port.LeiRepository = (*LeiRepo)(nil)

// LeiRepo persiste o catálogo de leis, as versões, as questões e as respostas.
type LeiRepo struct {
	pool *pgxpool.Pool
}

func NewLeiRepo(pool *pgxpool.Pool) *LeiRepo {
	return &LeiRepo{pool: pool}
}

func (r *LeiRepo) Catalogo(ctx context.Context) ([]lei.Resumo, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT l.id, l.slug, l.nome, l.curto, l.fonte, l.reconhecer, v.versao, v.importada_em,
		        (SELECT count(*) FROM leis_questoes q WHERE q.lei_id = l.id AND q.ativa)
		   FROM leis l
		   JOIN leis_versoes v ON v.lei_id = l.id AND v.ativa
		  ORDER BY l.curto`,
	)
	if err != nil {
		return nil, fmt.Errorf("listando leis: %w", err)
	}
	defer rows.Close()

	var out []lei.Resumo
	for rows.Next() {
		var s lei.Resumo
		if err := rows.Scan(
			&s.Lei.ID, &s.Lei.Slug, &s.Lei.Nome, &s.Lei.Curto, &s.Lei.Fonte, &s.Lei.Reconhecer,
			&s.Versao, &s.ImportadaEm, &s.Questoes,
		); err != nil {
			return nil, fmt.Errorf("lendo lei: %w", err)
		}
		out = append(out, s)
	}

	return out, rows.Err()
}

func (r *LeiRepo) PorSlug(ctx context.Context, slug string) (lei.Lei, error) {
	var l lei.Lei
	err := r.pool.QueryRow(
		ctx,
		`SELECT id, slug, nome, curto, fonte, reconhecer FROM leis WHERE slug = $1`, slug,
	).Scan(&l.ID, &l.Slug, &l.Nome, &l.Curto, &l.Fonte, &l.Reconhecer)
	if errors.Is(err, pgx.ErrNoRows) {
		return lei.Lei{}, lei.ErrNaoEncontrada
	}
	if err != nil {
		return lei.Lei{}, fmt.Errorf("buscando lei: %w", err)
	}

	return l, nil
}

func (r *LeiRepo) QuestoesGravadas(ctx context.Context, slug string) ([]lei.QuestaoGravada, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT q.id, q.chave, q.assinatura, q.ativa
		   FROM leis_questoes q JOIN leis l ON l.id = q.lei_id
		  WHERE l.slug = $1`, slug,
	)
	if err != nil {
		return nil, fmt.Errorf("listando questões gravadas: %w", err)
	}
	defer rows.Close()

	var out []lei.QuestaoGravada
	for rows.Next() {
		var g lei.QuestaoGravada
		if err := rows.Scan(&g.ID, &g.Chave, &g.Assinatura, &g.Ativa); err != nil {
			return nil, fmt.Errorf("lendo questão gravada: %w", err)
		}
		out = append(out, g)
	}

	return out, rows.Err()
}

func (r *LeiRepo) GravarTexto(ctx context.Context, p lei.Pacote) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	var leiID uuid.UUID
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO leis (slug, nome, curto, fonte, reconhecer) VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (slug) DO UPDATE
		   SET nome = EXCLUDED.nome, curto = EXCLUDED.curto,
		       fonte = EXCLUDED.fonte, reconhecer = EXCLUDED.reconhecer
		 RETURNING id`,
		p.Lei.Slug, p.Lei.Nome, p.Lei.Curto, p.Lei.Fonte, naoNulo(p.Lei.Reconhecer),
	).Scan(&leiID); err != nil {
		return false, fmt.Errorf("gravando lei: %w", err)
	}

	var versaoID uuid.UUID
	err = tx.QueryRow(
		ctx, `SELECT id FROM leis_versoes WHERE lei_id = $1 AND versao = $2`, leiID, p.Versao,
	).Scan(&versaoID)
	nova := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !nova {
		return false, fmt.Errorf("buscando versão: %w", err)
	}

	// Desliga a ativa ANTES de ligar a outra: o índice parcial só admite uma.
	if _, err := tx.Exec(
		ctx, `UPDATE leis_versoes SET ativa = false WHERE lei_id = $1 AND ativa AND versao <> $2`,
		leiID, p.Versao,
	); err != nil {
		return false, fmt.Errorf("desativando versão anterior: %w", err)
	}

	if nova {
		if versaoID, err = r.gravarVersao(ctx, tx, leiID, p); err != nil {
			return false, err
		}
	} else if _, err := tx.Exec(ctx, `UPDATE leis_versoes SET ativa = true WHERE id = $1`, versaoID); err != nil {
		return false, fmt.Errorf("reativando versão: %w", err)
	}

	if err := trocarUnidades(ctx, tx, versaoID, p.Unidades); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}

	return nova, nil
}

func (r *LeiRepo) GravarQuestoes(
	ctx context.Context,
	leiID uuid.UUID,
	versao string,
	unidades []lei.Unidade,
	plano lei.PlanoDeImportacao,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	var versaoID uuid.UUID
	if err := tx.QueryRow(
		ctx, `SELECT id FROM leis_versoes WHERE lei_id = $1 AND versao = $2`, leiID, versao,
	).Scan(&versaoID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return lei.ErrNaoEncontrada
		}

		return fmt.Errorf("buscando versão: %w", err)
	}

	if err := trocarUnidades(ctx, tx, versaoID, unidades); err != nil {
		return err
	}
	if err := aplicarQuestoes(ctx, tx, leiID, plano); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (r *LeiRepo) gravarVersao(ctx context.Context, tx pgx.Tx, leiID uuid.UUID, p lei.Pacote) (uuid.UUID, error) {
	var versaoID uuid.UUID
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO leis_versoes (lei_id, versao, ativa, recorte) VALUES ($1,$2,true,$3) RETURNING id`,
		leiID, p.Versao, naoNulo(p.Recorte),
	).Scan(&versaoID); err != nil {
		return uuid.Nil, fmt.Errorf("gravando versão: %w", err)
	}

	// A Constituição tem milhares de dispositivos: COPY em vez de um INSERT por
	// linha.
	linhas := make([][]any, 0, len(p.Dispositivos))
	for i, d := range p.Dispositivos {
		var pai any
		if d.Pai != "" {
			pai = d.Pai
		}
		linhas = append(linhas, []any{
			versaoID, i, d.Ref, pai, d.Tipo, d.Rotulo, d.Nome, d.Texto,
			naoNulo(d.Notas), naoNulo(d.Anteriores), d.Revogado,
		})
	}
	if _, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"leis_dispositivos"},
		[]string{"versao_id", "ordem", "ref", "pai", "tipo", "rotulo", "nome", "texto", "notas", "anteriores", "revogado"},
		pgx.CopyFromRows(linhas),
	); err != nil {
		return uuid.Nil, fmt.Errorf("gravando dispositivos: %w", err)
	}

	return versaoID, nil
}

// trocarUnidades substitui as unidades da versão pelas dadas.
func trocarUnidades(ctx context.Context, tx pgx.Tx, versaoID uuid.UUID, unidades []lei.Unidade) error {
	lote := &pgx.Batch{}
	lote.Queue(`DELETE FROM leis_unidades WHERE versao_id = $1`, versaoID)
	for i, u := range unidades {
		lote.Queue(
			`INSERT INTO leis_unidades (versao_id, ordem, ref, titulo, dispositivos, hash)
			 VALUES ($1,$2,$3,$4,$5,$6)`,
			versaoID, i, u.Ref, u.Titulo, naoNulo(u.Dispositivos), u.Hash,
		)
	}
	if err := tx.SendBatch(ctx, lote).Close(); err != nil {
		return fmt.Errorf("gravando unidades: %w", err)
	}

	return nil
}

func aplicarQuestoes(ctx context.Context, tx pgx.Tx, leiID uuid.UUID, plano lei.PlanoDeImportacao) error {
	lote := &pgx.Batch{}
	for _, q := range plano.Novas {
		lote.Queue(
			`INSERT INTO leis_questoes
			   (lei_id, chave, unidade, enunciado, alternativas, gabarito, comentario, trecho, dispositivos, assinatura)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			leiID, q.Chave, q.Unidade, q.Enunciado, q.Alternativas, q.Gabarito, q.Comentario,
			q.Trecho, q.Dispositivos, q.Assinatura(),
		)
	}
	for _, a := range plano.Atualizadas {
		q := a.Questao
		lote.Queue(
			`UPDATE leis_questoes
			    SET unidade = $2, enunciado = $3, alternativas = $4, gabarito = $5, comentario = $6,
			        trecho = $7, dispositivos = $8, assinatura = $9, ativa = true
			  WHERE id = $1`,
			a.ID, q.Unidade, q.Enunciado, q.Alternativas, q.Gabarito, q.Comentario,
			q.Trecho, q.Dispositivos, q.Assinatura(),
		)
	}
	if len(plano.Desativar) > 0 {
		lote.Queue(`UPDATE leis_questoes SET ativa = false WHERE id = ANY($1)`, plano.Desativar)
	}
	if lote.Len() == 0 {
		return nil
	}
	if err := tx.SendBatch(ctx, lote).Close(); err != nil {
		return fmt.Errorf("gravando questões: %w", err)
	}

	return nil
}

func (r *LeiRepo) TextoAtivo(ctx context.Context, leiID uuid.UUID) (lei.Texto, error) {
	var (
		t        lei.Texto
		versaoID uuid.UUID
	)
	err := r.pool.QueryRow(
		ctx, `SELECT id, versao, recorte FROM leis_versoes WHERE lei_id = $1 AND ativa`, leiID,
	).Scan(&versaoID, &t.Versao, &t.Recorte)
	if errors.Is(err, pgx.ErrNoRows) {
		return lei.Texto{}, lei.ErrNaoEncontrada
	}
	if err != nil {
		return lei.Texto{}, fmt.Errorf("buscando versão ativa: %w", err)
	}

	rows, err := r.pool.Query(
		ctx,
		`SELECT ref, coalesce(pai, ''), tipo, rotulo, nome, texto, notas, anteriores, revogado
		   FROM leis_dispositivos WHERE versao_id = $1 ORDER BY ordem`, versaoID,
	)
	if err != nil {
		return lei.Texto{}, fmt.Errorf("listando dispositivos: %w", err)
	}
	for rows.Next() {
		var d lei.Dispositivo
		if err := rows.Scan(
			&d.Ref, &d.Pai, &d.Tipo, &d.Rotulo, &d.Nome, &d.Texto, &d.Notas, &d.Anteriores, &d.Revogado,
		); err != nil {
			rows.Close()
			return lei.Texto{}, fmt.Errorf("lendo dispositivo: %w", err)
		}
		t.Dispositivos = append(t.Dispositivos, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return lei.Texto{}, err
	}

	rows, err = r.pool.Query(
		ctx,
		`SELECT ref, titulo, dispositivos, hash FROM leis_unidades WHERE versao_id = $1 ORDER BY ordem`,
		versaoID,
	)
	if err != nil {
		return lei.Texto{}, fmt.Errorf("listando unidades: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var u lei.Unidade
		if err := rows.Scan(&u.Ref, &u.Titulo, &u.Dispositivos, &u.Hash); err != nil {
			return lei.Texto{}, fmt.Errorf("lendo unidade: %w", err)
		}
		t.Unidades = append(t.Unidades, u)
	}

	return t, rows.Err()
}

const colunasQuestao = `q.id, q.lei_id, q.ativa, q.chave, q.unidade, q.enunciado, q.alternativas,
	q.gabarito, q.comentario, q.trecho, q.dispositivos`

func escanearQuestao(q *lei.QuestaoPublicada) []any {
	return []any{
		&q.ID, &q.LeiID, &q.Ativa, &q.Questao.Chave, &q.Questao.Unidade, &q.Questao.Enunciado,
		&q.Questao.Alternativas, &q.Questao.Gabarito, &q.Questao.Comentario, &q.Questao.Trecho,
		&q.Questao.Dispositivos,
	}
}

func (r *LeiRepo) QuestoesAtivas(ctx context.Context, leiID, usuarioID uuid.UUID) ([]lei.QuestaoComResposta, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT `+colunasQuestao+`, r.alternativa, r.acertou, r.respondida_em
		   FROM leis_questoes q
		   LEFT JOIN LATERAL (
		        SELECT alternativa, acertou, respondida_em FROM leis_respostas
		         WHERE usuario_id = $2 AND questao_id = q.id
		         ORDER BY respondida_em DESC LIMIT 1
		   ) r ON true
		  WHERE q.lei_id = $1 AND q.ativa
		  ORDER BY q.chave`,
		leiID, usuarioID,
	)
	if err != nil {
		return nil, fmt.Errorf("listando questões: %w", err)
	}
	defer rows.Close()

	var out []lei.QuestaoComResposta
	for rows.Next() {
		var (
			q           lei.QuestaoComResposta
			alternativa *string
			acertou     *bool
			em          *time.Time
		)
		if err := rows.Scan(append(escanearQuestao(&q.QuestaoPublicada), &alternativa, &acertou, &em)...); err != nil {
			return nil, fmt.Errorf("lendo questão: %w", err)
		}
		if alternativa != nil {
			q.Ultima = &lei.Resposta{
				UsuarioID: usuarioID, QuestaoID: q.ID, Alternativa: *alternativa, Acertou: *acertou, Em: *em,
			}
		}
		out = append(out, q)
	}

	return out, rows.Err()
}

func (r *LeiRepo) Questao(ctx context.Context, id uuid.UUID) (lei.QuestaoPublicada, error) {
	var q lei.QuestaoPublicada
	err := r.pool.QueryRow(
		ctx, `SELECT `+colunasQuestao+` FROM leis_questoes q WHERE q.id = $1`, id,
	).Scan(escanearQuestao(&q)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return lei.QuestaoPublicada{}, lei.ErrQuestaoNaoEncontrada
	}
	if err != nil {
		return lei.QuestaoPublicada{}, fmt.Errorf("buscando questão: %w", err)
	}

	return q, nil
}

func (r *LeiRepo) Responder(ctx context.Context, resp lei.Resposta) (lei.Resposta, error) {
	err := r.pool.QueryRow(
		ctx,
		`INSERT INTO leis_respostas (usuario_id, questao_id, alternativa, acertou)
		 VALUES ($1,$2,$3,$4) RETURNING respondida_em`,
		resp.UsuarioID, resp.QuestaoID, resp.Alternativa, resp.Acertou,
	).Scan(&resp.Em)
	if err != nil {
		return lei.Resposta{}, fmt.Errorf("gravando resposta: %w", err)
	}

	return resp, nil
}

func (r *LeiRepo) PorFonte(ctx context.Context, fonte string) (lei.Lei, error) {
	var l lei.Lei
	err := r.pool.QueryRow(
		ctx, `SELECT id, slug, nome, curto, fonte, reconhecer FROM leis WHERE fonte = $1 ORDER BY criada_em LIMIT 1`, fonte,
	).Scan(&l.ID, &l.Slug, &l.Nome, &l.Curto, &l.Fonte, &l.Reconhecer)
	if errors.Is(err, pgx.ErrNoRows) {
		return lei.Lei{}, lei.ErrNaoEncontrada
	}
	if err != nil {
		return lei.Lei{}, fmt.Errorf("buscando lei pela fonte: %w", err)
	}

	return l, nil
}

func (r *LeiRepo) ContarParaExcluir(ctx context.Context, leiID uuid.UUID) (int, int, error) {
	var questoes, respostas int
	if err := r.pool.QueryRow(
		ctx,
		`SELECT (SELECT count(*) FROM leis_questoes WHERE lei_id = $1),
		        (SELECT count(*) FROM leis_respostas lr JOIN leis_questoes q ON q.id = lr.questao_id WHERE q.lei_id = $1)`,
		leiID,
	).Scan(&questoes, &respostas); err != nil {
		return 0, 0, fmt.Errorf("contando o que a exclusão leva: %w", err)
	}

	return questoes, respostas, nil
}

// Excluir apaga numa transação. As respostas vão primeiro: a FK delas para a
// questão é RESTRICT de propósito (uma resposta é história), e só a exclusão
// da lei, confirmada por quem estuda, passa por cima disso.
func (r *LeiRepo) Excluir(ctx context.Context, leiID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	if _, err := tx.Exec(
		ctx,
		`DELETE FROM leis_respostas WHERE questao_id IN (SELECT id FROM leis_questoes WHERE lei_id = $1)`, leiID,
	); err != nil {
		return fmt.Errorf("apagando respostas: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM leis WHERE id = $1`, leiID)
	if err != nil {
		return fmt.Errorf("apagando a lei: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return lei.ErrNaoEncontrada
	}

	return tx.Commit(ctx)
}

func (r *LeiRepo) Estrutura(ctx context.Context, leiID uuid.UUID) ([]lei.Dispositivo, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT d.ref, coalesce(d.pai, ''), d.tipo, d.rotulo, d.nome
		   FROM leis_dispositivos d JOIN leis_versoes v ON v.id = d.versao_id
		  WHERE v.lei_id = $1 AND v.ativa
		    AND d.tipo IN ('parte', 'livro', 'titulo', 'capitulo', 'secao', 'subsecao', 'artigo', 'preambulo')
		  ORDER BY d.ordem`, leiID,
	)
	if err != nil {
		return nil, fmt.Errorf("listando a estrutura: %w", err)
	}
	defer rows.Close()

	var out []lei.Dispositivo
	for rows.Next() {
		var d lei.Dispositivo
		if err := rows.Scan(&d.Ref, &d.Pai, &d.Tipo, &d.Rotulo, &d.Nome); err != nil {
			return nil, fmt.Errorf("lendo a estrutura: %w", err)
		}
		out = append(out, d)
	}

	return out, rows.Err()
}

func (r *LeiRepo) Vinculos(ctx context.Context, concursoID uuid.UUID) (map[uuid.UUID][]lei.Vinculo, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT dl.disciplina_id, dl.lei_id, dl.recorte
		   FROM disciplinas_leis dl JOIN disciplinas d ON d.id = dl.disciplina_id
		  WHERE d.concurso_id = $1`, concursoID,
	)
	if err != nil {
		return nil, fmt.Errorf("listando vínculos: %w", err)
	}
	defer rows.Close()

	out := map[uuid.UUID][]lei.Vinculo{}
	for rows.Next() {
		var (
			disciplina uuid.UUID
			v          lei.Vinculo
		)
		if err := rows.Scan(&disciplina, &v.LeiID, &v.Recorte); err != nil {
			return nil, fmt.Errorf("lendo vínculo: %w", err)
		}
		out[disciplina] = append(out[disciplina], v)
	}

	return out, rows.Err()
}

func (r *LeiRepo) Vincular(ctx context.Context, disciplinaID, leiID uuid.UUID, recorte []string) error {
	if _, err := r.pool.Exec(
		ctx,
		`INSERT INTO disciplinas_leis (disciplina_id, lei_id, recorte) VALUES ($1,$2,$3)
		 ON CONFLICT (disciplina_id, lei_id) DO UPDATE SET recorte = EXCLUDED.recorte`,
		disciplinaID, leiID, naoNulo(recorte),
	); err != nil {
		return fmt.Errorf("vinculando lei: %w", err)
	}

	return nil
}

func (r *LeiRepo) Desvincular(ctx context.Context, disciplinaID, leiID uuid.UUID) error {
	if _, err := r.pool.Exec(
		ctx, `DELETE FROM disciplinas_leis WHERE disciplina_id = $1 AND lei_id = $2`, disciplinaID, leiID,
	); err != nil {
		return fmt.Errorf("desvinculando lei: %w", err)
	}

	return nil
}

// naoNulo troca a lista nula por vazia: as colunas são NOT NULL DEFAULT '{}', e
// um nil do Go chegaria como NULL.
func naoNulo(s []string) []string {
	if s == nil {
		return []string{}
	}

	return s
}
