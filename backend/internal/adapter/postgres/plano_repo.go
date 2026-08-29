package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"annygo/internal/domain/plano"
	"annygo/internal/port"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ port.PlanoRepository = (*PlanoRepo)(nil)

// PlanoRepo persists a user's plan config and progress.
type PlanoRepo struct {
	pool *pgxpool.Pool
}

func NewPlanoRepo(pool *pgxpool.Pool) *PlanoRepo {
	return &PlanoRepo{pool: pool}
}

func (r *PlanoRepo) PlanoByUser(ctx context.Context, userID, concursoID uuid.UUID) (plano.Salvo, error) {
	s := plano.NewSalvo()

	var diasEstudo []int32
	var horasDia float64

	err := r.pool.QueryRow(
		ctx,
		`SELECT id, user_id, concurso_id, inicio, prova, horas_dia::float8,
		        dias_estudo, dia_revisao, reta_final_dias, tema_ui, criado_em, atualizado_em
		 FROM planos WHERE user_id = $1 AND concurso_id = $2`,
		userID,
		concursoID,
	).Scan(
		&s.ID, &s.UserID, &s.ConcursoID, &s.Config.Inicio, &s.Config.Prova, &horasDia,
		&diasEstudo, &s.Config.DiaRevisao, &s.Config.RetaFinalDias, &s.TemaUI,
		&s.CriadoEm, &s.AtualizadoEm,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return plano.Salvo{}, plano.ErrNotFound
	}

	if err != nil {
		return plano.Salvo{}, fmt.Errorf("querying plano: %w", err)
	}

	s.Config.HorasDia = horasDia
	s.Config.DiasEstudo = toIntSlice(diasEstudo)
	s.Config.Questoes = map[string]int{}

	if err := r.loadQuestoes(ctx, s.ID, &s); err != nil {
		return plano.Salvo{}, err
	}

	if err := r.loadRegistros(ctx, s.ID, &s); err != nil {
		return plano.Salvo{}, err
	}

	if err := r.loadMarcos(ctx, s.ID, &s); err != nil {
		return plano.Salvo{}, err
	}

	if err := r.loadReordenacoes(ctx, s.ID, &s); err != nil {
		return plano.Salvo{}, err
	}

	return s, nil
}

func (r *PlanoRepo) loadQuestoes(ctx context.Context, planoID uuid.UUID, s *plano.Salvo) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT d.codigo, pq.questoes
		 FROM plano_questoes pq JOIN disciplinas d ON d.id = pq.disciplina_id
		 WHERE pq.plano_id = $1`,
		planoID,
	)
	if err != nil {
		return fmt.Errorf("querying plano_questoes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var codigo string
		var q int
		if err := rows.Scan(&codigo, &q); err != nil {
			return fmt.Errorf("scanning plano_questao: %w", err)
		}

		s.Config.Questoes[codigo] = q
	}

	return rows.Err()
}

func (r *PlanoRepo) loadRegistros(ctx context.Context, planoID uuid.UUID, s *plano.Salvo) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT data, horas::float8, concluido, questoes, acertos, nota
		 FROM registros_dia WHERE plano_id = $1`,
		planoID,
	)
	if err != nil {
		return fmt.Errorf("querying registros_dia: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var reg plano.Registro
		if err := rows.Scan(
			&reg.Data, &reg.Horas, &reg.Concluido, &reg.Questoes, &reg.Acertos, &reg.Nota,
		); err != nil {
			return fmt.Errorf("scanning registro_dia: %w", err)
		}

		s.Registros[reg.Data.UTC()] = reg
	}

	return rows.Err()
}

func (r *PlanoRepo) loadMarcos(ctx context.Context, planoID uuid.UUID, s *plano.Salvo) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT marco_id, cumprido FROM marco_checks WHERE plano_id = $1`,
		planoID,
	)
	if err != nil {
		return fmt.Errorf("querying marco_checks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var cumprido bool
		if err := rows.Scan(&id, &cumprido); err != nil {
			return fmt.Errorf("scanning marco_check: %w", err)
		}

		s.Marcos[id] = cumprido
	}

	return rows.Err()
}

func (r *PlanoRepo) loadReordenacoes(ctx context.Context, planoID uuid.UUID, s *plano.Salvo) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT data, tipo, itens, meta FROM reordenacoes WHERE plano_id = $1`,
		planoID,
	)
	if err != nil {
		return fmt.Errorf("querying reordenacoes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var data time.Time
		var tipo string
		var itensJSON []byte
		var meta int
		if err := rows.Scan(&data, &tipo, &itensJSON, &meta); err != nil {
			return fmt.Errorf("scanning reordenacao: %w", err)
		}

		itens := []plano.ItemDia{}
		if err := json.Unmarshal(itensJSON, &itens); err != nil {
			return fmt.Errorf("decoding reordenacao itens: %w", err)
		}

		s.Reordenacoes[data.UTC()] = plano.Reordenacao{
			Tipo:  plano.Tipo(tipo),
			Itens: itens,
			Meta:  meta,
		}
	}

	return rows.Err()
}

func (r *PlanoRepo) UpsertPlano(ctx context.Context, s plano.Salvo) (plano.Salvo, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return plano.Salvo{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(
		ctx,
		`INSERT INTO planos
		   (user_id, concurso_id, inicio, prova, horas_dia, dias_estudo, dia_revisao, reta_final_dias, tema_ui)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (user_id, concurso_id) DO UPDATE SET
		   inicio = EXCLUDED.inicio, prova = EXCLUDED.prova, horas_dia = EXCLUDED.horas_dia,
		   dias_estudo = EXCLUDED.dias_estudo, dia_revisao = EXCLUDED.dia_revisao,
		   reta_final_dias = EXCLUDED.reta_final_dias, tema_ui = EXCLUDED.tema_ui,
		   atualizado_em = now()
		 RETURNING id, criado_em, atualizado_em`,
		s.UserID, s.ConcursoID, s.Config.Inicio, s.Config.Prova, s.Config.HorasDia,
		toInt32Slice(s.Config.DiasEstudo), s.Config.DiaRevisao, s.Config.RetaFinalDias, s.TemaUI,
	).Scan(&s.ID, &s.CriadoEm, &s.AtualizadoEm)
	if err != nil {
		return plano.Salvo{}, fmt.Errorf("upserting plano: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM plano_questoes WHERE plano_id = $1`, s.ID); err != nil {
		return plano.Salvo{}, fmt.Errorf("clearing plano_questoes: %w", err)
	}

	for codigo, q := range s.Config.Questoes {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO plano_questoes (plano_id, disciplina_id, questoes)
			 SELECT $1, d.id, $3 FROM disciplinas d WHERE d.concurso_id = $2 AND d.codigo = $4`,
			s.ID, s.ConcursoID, q, codigo,
		); err != nil {
			return plano.Salvo{}, fmt.Errorf("inserting plano_questao %s: %w", codigo, err)
		}
	}

	if err := replaceReordenacoesTx(ctx, tx, s.ID, s.Reordenacoes); err != nil {
		return plano.Salvo{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return plano.Salvo{}, fmt.Errorf("commit: %w", err)
	}

	return s, nil
}

func (r *PlanoRepo) ReplaceReordenacoes(
	ctx context.Context,
	planoID uuid.UUID,
	reord map[time.Time]plano.Reordenacao,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := replaceReordenacoesTx(ctx, tx, planoID, reord); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func replaceReordenacoesTx(
	ctx context.Context,
	tx pgx.Tx,
	planoID uuid.UUID,
	reord map[time.Time]plano.Reordenacao,
) error {
	if _, err := tx.Exec(ctx, `DELETE FROM reordenacoes WHERE plano_id = $1`, planoID); err != nil {
		return fmt.Errorf("clearing reordenacoes: %w", err)
	}

	for data, ov := range reord {
		itensJSON, err := json.Marshal(ov.Itens)
		if err != nil {
			return fmt.Errorf("encoding reordenacao itens: %w", err)
		}

		if _, err := tx.Exec(
			ctx,
			`INSERT INTO reordenacoes (plano_id, data, tipo, itens, meta) VALUES ($1, $2, $3, $4, $5)`,
			planoID, data, string(ov.Tipo), itensJSON, ov.Meta,
		); err != nil {
			return fmt.Errorf("inserting reordenacao: %w", err)
		}
	}

	return nil
}

func (r *PlanoRepo) UpsertRegistro(ctx context.Context, planoID uuid.UUID, reg plano.Registro) error {
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO registros_dia (plano_id, data, horas, concluido, questoes, acertos, nota)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (plano_id, data) DO UPDATE SET
		   horas = EXCLUDED.horas, concluido = EXCLUDED.concluido, questoes = EXCLUDED.questoes,
		   acertos = EXCLUDED.acertos, nota = EXCLUDED.nota, atualizado_em = now()`,
		planoID, reg.Data, reg.Horas, reg.Concluido, reg.Questoes, reg.Acertos, reg.Nota,
	)
	if err != nil {
		return fmt.Errorf("upserting registro_dia: %w", err)
	}

	return nil
}

func (r *PlanoRepo) DeleteRegistros(ctx context.Context, planoID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM registros_dia WHERE plano_id = $1`, planoID); err != nil {
		return fmt.Errorf("deleting registros_dia: %w", err)
	}

	if _, err := r.pool.Exec(ctx, `DELETE FROM marco_checks WHERE plano_id = $1`, planoID); err != nil {
		return fmt.Errorf("deleting marco_checks: %w", err)
	}

	return nil
}

func (r *PlanoRepo) SetMarco(ctx context.Context, planoID, marcoID uuid.UUID, cumprido bool) error {
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO marco_checks (plano_id, marco_id, cumprido) VALUES ($1, $2, $3)
		 ON CONFLICT (plano_id, marco_id) DO UPDATE SET cumprido = EXCLUDED.cumprido`,
		planoID, marcoID, cumprido,
	)
	if err != nil {
		return fmt.Errorf("setting marco_check: %w", err)
	}

	return nil
}

func (r *PlanoRepo) ListAnotacoes(ctx context.Context, planoID uuid.UUID) ([]plano.Anotacao, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT id, data, disciplina_id, texto, resolvido, criado_em, atualizado_em
		 FROM anotacoes WHERE plano_id = $1 ORDER BY criado_em DESC`,
		planoID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying anotacoes: %w", err)
	}
	defer rows.Close()

	out := []plano.Anotacao{}

	for rows.Next() {
		var a plano.Anotacao
		if err := rows.Scan(
			&a.ID, &a.Data, &a.DisciplinaID, &a.Texto, &a.Resolvido, &a.CriadoEm, &a.AtualizadoEm,
		); err != nil {
			return nil, fmt.Errorf("scanning anotacao: %w", err)
		}

		out = append(out, a)
	}

	return out, rows.Err()
}

func (r *PlanoRepo) CreateAnotacao(
	ctx context.Context,
	planoID uuid.UUID,
	a plano.Anotacao,
) (plano.Anotacao, error) {
	err := r.pool.QueryRow(
		ctx,
		`INSERT INTO anotacoes (plano_id, data, disciplina_id, texto, resolvido)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, criado_em, atualizado_em`,
		planoID, a.Data, a.DisciplinaID, a.Texto, a.Resolvido,
	).Scan(&a.ID, &a.CriadoEm, &a.AtualizadoEm)
	if err != nil {
		return plano.Anotacao{}, fmt.Errorf("inserting anotacao: %w", err)
	}

	return a, nil
}

func (r *PlanoRepo) UpdateAnotacao(
	ctx context.Context,
	planoID uuid.UUID,
	a plano.Anotacao,
) (plano.Anotacao, error) {
	err := r.pool.QueryRow(
		ctx,
		`UPDATE anotacoes SET texto = $3, resolvido = $4, data = $5, disciplina_id = $6, atualizado_em = now()
		 WHERE id = $1 AND plano_id = $2
		 RETURNING id, criado_em, atualizado_em`,
		a.ID, planoID, a.Texto, a.Resolvido, a.Data, a.DisciplinaID,
	).Scan(&a.ID, &a.CriadoEm, &a.AtualizadoEm)

	if errors.Is(err, pgx.ErrNoRows) {
		return plano.Anotacao{}, plano.ErrNotFound
	}

	if err != nil {
		return plano.Anotacao{}, fmt.Errorf("updating anotacao: %w", err)
	}

	return a, nil
}

func (r *PlanoRepo) DeleteAnotacao(ctx context.Context, planoID, anotacaoID uuid.UUID) error {
	if _, err := r.pool.Exec(
		ctx,
		`DELETE FROM anotacoes WHERE id = $1 AND plano_id = $2`,
		anotacaoID, planoID,
	); err != nil {
		return fmt.Errorf("deleting anotacao: %w", err)
	}

	return nil
}

func (r *PlanoRepo) ListPlanosParaLembrete(ctx context.Context) ([]port.PlanoComEmail, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT p.id, p.concurso_id, u.email, u.nome
		 FROM planos p JOIN users u ON u.id = p.user_id`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing planos para lembrete: %w", err)
	}
	defer rows.Close()

	type ref struct {
		id         uuid.UUID
		concursoID uuid.UUID
		email      string
		nome       string
	}

	refs := []ref{}

	for rows.Next() {
		var x ref
		if err := rows.Scan(&x.id, &x.concursoID, &x.email, &x.nome); err != nil {
			return nil, fmt.Errorf("scanning plano ref: %w", err)
		}

		refs = append(refs, x)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating plano refs: %w", err)
	}

	out := make([]port.PlanoComEmail, 0, len(refs))

	for _, x := range refs {
		s, err := r.loadByID(ctx, x.id)
		if err != nil {
			return nil, err
		}

		out = append(out, port.PlanoComEmail{
			Plano:      s,
			ConcursoID: x.concursoID,
			Email:      x.email,
			Nome:       x.nome,
		})
	}

	return out, nil
}

func (r *PlanoRepo) loadByID(ctx context.Context, planoID uuid.UUID) (plano.Salvo, error) {
	var userID, concursoID uuid.UUID

	err := r.pool.QueryRow(
		ctx,
		`SELECT user_id, concurso_id FROM planos WHERE id = $1`,
		planoID,
	).Scan(&userID, &concursoID)
	if err != nil {
		return plano.Salvo{}, fmt.Errorf("resolving plano owner: %w", err)
	}

	return r.PlanoByUser(ctx, userID, concursoID)
}

func toIntSlice(xs []int32) []int {
	out := make([]int, len(xs))
	for i, x := range xs {
		out[i] = int(x)
	}

	return out
}

func toInt32Slice(xs []int) []int32 {
	out := make([]int32, len(xs))
	for i, x := range xs {
		out[i] = int32(x)
	}

	return out
}
