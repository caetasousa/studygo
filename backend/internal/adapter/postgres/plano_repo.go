package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/port"

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

	var (
		diasEstudo []int32
		horasDia   float64
		pctQuest   float64
		simulados  string
	)

	err := r.pool.QueryRow(
		ctx,
		`SELECT id, user_id, concurso_id, inicio, prova, horas_dia::float8,
		        dias_estudo, dia_revisao, reta_final_dias, tema_ui, criado_em, atualizado_em,
		        simulados, discursiva, pct_questoes::float8,
		        limiar_fraco, blocos_por_dia, minutos_revisao, minutos_bloco, revisao_semanal
		 FROM planos WHERE user_id = $1 AND concurso_id = $2`,
		userID,
		concursoID,
	).Scan(
		&s.ID, &s.UserID, &s.ConcursoID, &s.Config.Inicio, &s.Config.Prova, &horasDia,
		&diasEstudo, &s.Config.DiaRevisao, &s.Config.RetaFinalDias, &s.TemaUI,
		&s.CriadoEm, &s.AtualizadoEm,
		&simulados, &s.Config.Discursiva, &pctQuest,
		&s.Config.LimiarFraco, &s.Config.BlocosPorDia, &s.Config.MinutosRevisao,
		&s.Config.MinutosBloco, &s.Config.RevisaoSemanal,
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

	s.Config.Simulados = plano.Frequencia(simulados)
	s.Config.PctQuestoes = pctQuest
	s.Config.Modos = map[string]plano.Modo{}
	s.Config.Reforcos = map[string]float64{}

	if err := r.loadDisciplinas(ctx, s.ID, &s); err != nil {
		return plano.Salvo{}, err
	}

	if err := r.loadCiclo(ctx, s.ID, &s); err != nil {
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

func (r *PlanoRepo) loadDisciplinas(ctx context.Context, planoID uuid.UUID, s *plano.Salvo) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT disciplina, questoes, modo, reforco::float8
		 FROM plano_disciplinas WHERE plano_id = $1`,
		planoID,
	)
	if err != nil {
		return fmt.Errorf("querying plano_disciplinas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			codigo  string
			q       int
			modo    string
			reforco float64
		)

		if err := rows.Scan(&codigo, &q, &modo, &reforco); err != nil {
			return fmt.Errorf("scanning plano_disciplina: %w", err)
		}

		s.Config.Questoes[codigo] = q
		s.Config.Modos[codigo] = plano.Modo(modo)
		s.Config.Reforcos[codigo] = reforco
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

	if err := rows.Err(); err != nil {
		return err
	}

	return r.loadRegistrosBloco(ctx, planoID, s)
}

// loadRegistrosBloco attaches the per-discipline breakdown to the day records
// already loaded. A bloco whose day has no registros_dia row is ignored — the
// two are always written together.
func (r *PlanoRepo) loadRegistrosBloco(ctx context.Context, planoID uuid.UUID, s *plano.Salvo) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT data, disciplina, horas::float8, questoes, acertos, nota, concluido,
		        COALESCE(atividade_id::text, '')
		 FROM registros_bloco WHERE plano_id = $1 ORDER BY data, disciplina`,
		planoID,
	)
	if err != nil {
		return fmt.Errorf("querying registros_bloco: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			data time.Time
			b    plano.RegistroBloco
		)

		if err := rows.Scan(
			&data, &b.Disciplina, &b.Horas, &b.Questoes, &b.Acertos, &b.Nota, &b.Concluido,
			&b.AtividadeID,
		); err != nil {
			return fmt.Errorf("scanning registro_bloco: %w", err)
		}

		reg, ok := s.Registros[data.UTC()]
		if !ok {
			continue
		}

		reg.Blocos = append(reg.Blocos, b)
		s.Registros[data.UTC()] = reg
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

	cfg := s.Config.Normalizar()

	err = tx.QueryRow(
		ctx,
		`INSERT INTO planos
		   (user_id, concurso_id, inicio, prova, horas_dia, dias_estudo, dia_revisao, reta_final_dias,
		    tema_ui, simulados, discursiva, pct_questoes, limiar_fraco,
		    blocos_por_dia, minutos_revisao, minutos_bloco, revisao_semanal)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		 ON CONFLICT (user_id, concurso_id) DO UPDATE SET
		   inicio = EXCLUDED.inicio, prova = EXCLUDED.prova, horas_dia = EXCLUDED.horas_dia,
		   dias_estudo = EXCLUDED.dias_estudo, dia_revisao = EXCLUDED.dia_revisao,
		   reta_final_dias = EXCLUDED.reta_final_dias, tema_ui = EXCLUDED.tema_ui,
		   simulados = EXCLUDED.simulados, discursiva = EXCLUDED.discursiva,
		   pct_questoes = EXCLUDED.pct_questoes,
		   limiar_fraco = EXCLUDED.limiar_fraco,
		   blocos_por_dia = EXCLUDED.blocos_por_dia,
		   minutos_revisao = EXCLUDED.minutos_revisao,
		   minutos_bloco = EXCLUDED.minutos_bloco,
		   revisao_semanal = EXCLUDED.revisao_semanal,
		   atualizado_em = now()
		 RETURNING id, criado_em, atualizado_em`,
		s.UserID, s.ConcursoID, cfg.Inicio, cfg.Prova, cfg.HorasDia,
		toInt32Slice(cfg.DiasEstudo), cfg.DiaRevisao, cfg.RetaFinalDias, s.TemaUI,
		string(cfg.Simulados), cfg.Discursiva,
		cfg.PctQuestoes, cfg.LimiarFraco,
		cfg.BlocosPorDia, cfg.MinutosRevisao, cfg.MinutosBloco, cfg.RevisaoSemanal,
	).Scan(&s.ID, &s.CriadoEm, &s.AtualizadoEm)
	if err != nil {
		return plano.Salvo{}, fmt.Errorf("upserting plano: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM plano_disciplinas WHERE plano_id = $1`, s.ID); err != nil {
		return plano.Salvo{}, fmt.Errorf("clearing plano_disciplinas: %w", err)
	}

	for codigo, q := range s.Config.Questoes {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO plano_disciplinas (plano_id, disciplina, questoes, modo, reforco)
			 VALUES ($1, $2, $3, $4, $5)`,
			s.ID, codigo, q, string(cfg.ModoDe(codigo)), cfg.ReforcoDe(codigo),
		); err != nil {
			return plano.Salvo{}, fmt.Errorf("inserting plano_disciplina %s: %w", codigo, err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM plano_ciclo WHERE plano_id = $1`, s.ID); err != nil {
		return plano.Salvo{}, fmt.Errorf("clearing plano_ciclo: %w", err)
	}

	for i, it := range cfg.CicloRevisao {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO plano_ciclo (plano_id, ordem, titulo, questoes) VALUES ($1, $2, $3, $4)`,
			s.ID, i, it.Titulo, it.Questoes,
		); err != nil {
			return plano.Salvo{}, fmt.Errorf("inserting plano_ciclo: %w", err)
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

// UpsertRegistro replaces the whole record for one day — the day row and its
// per-discipline blocks — in a single transaction.
func (r *PlanoRepo) UpsertRegistro(ctx context.Context, planoID uuid.UUID, reg plano.Registro) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO registros_dia (plano_id, data, horas, concluido, questoes, acertos, nota)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (plano_id, data) DO UPDATE SET
		   horas = EXCLUDED.horas, concluido = EXCLUDED.concluido, questoes = EXCLUDED.questoes,
		   acertos = EXCLUDED.acertos, nota = EXCLUDED.nota, atualizado_em = now()`,
		planoID, reg.Data, reg.Horas, reg.Concluido, reg.Questoes, reg.Acertos, reg.Nota,
	); err != nil {
		return fmt.Errorf("upserting registro_dia: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`DELETE FROM registros_bloco WHERE plano_id = $1 AND data = $2`,
		planoID, reg.Data,
	); err != nil {
		return fmt.Errorf("clearing registros_bloco: %w", err)
	}

	for _, b := range reg.Blocos {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO registros_bloco
			   (plano_id, data, disciplina, horas, questoes, acertos, nota, concluido, atividade_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, '')::uuid)`,
			planoID, reg.Data, b.Disciplina, b.Horas, b.Questoes, b.Acertos, b.Nota, b.Concluido,
			b.AtividadeID,
		); err != nil {
			return fmt.Errorf("inserting registro_bloco: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (r *PlanoRepo) DeleteRegistros(ctx context.Context, planoID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM registros_dia WHERE plano_id = $1`, planoID); err != nil {
		return fmt.Errorf("deleting registros_dia: %w", err)
	}

	if _, err := r.pool.Exec(ctx, `DELETE FROM registros_bloco WHERE plano_id = $1`, planoID); err != nil {
		return fmt.Errorf("deleting registros_bloco: %w", err)
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
		`SELECT id, data, disciplina_id, tema, texto, origem, url, proxima_revisao,
		        resolvido, criado_em, atualizado_em
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
			&a.ID, &a.Data, &a.DisciplinaID, &a.Tema, &a.Texto, &a.Origem, &a.URL,
			&a.ProximaRevisao, &a.Resolvido, &a.CriadoEm, &a.AtualizadoEm,
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
		`INSERT INTO anotacoes
		   (plano_id, data, disciplina_id, tema, texto, origem, url, proxima_revisao, resolvido)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, criado_em, atualizado_em`,
		planoID, a.Data, a.DisciplinaID, a.Tema, a.Texto, string(origemDe(a.Origem)),
		a.URL, a.ProximaRevisao, a.Resolvido,
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
		`UPDATE anotacoes SET
		   texto = $3, resolvido = $4, data = $5, disciplina_id = $6,
		   tema = $7, url = $8, proxima_revisao = $9, atualizado_em = now()
		 WHERE id = $1 AND plano_id = $2
		 RETURNING id, criado_em, atualizado_em`,
		a.ID, planoID, a.Texto, a.Resolvido, a.Data, a.DisciplinaID,
		a.Tema, a.URL, a.ProximaRevisao,
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

// origemDe keeps an unknown origin out of the CHECK constraint.
func origemDe(o plano.Origem) plano.Origem {
	switch o {
	case plano.OrigemRevisao, plano.OrigemTEC, plano.OrigemSimulado:
		return o
	default:
		return plano.OrigemManual
	}
}

func (r *PlanoRepo) loadCiclo(ctx context.Context, planoID uuid.UUID, s *plano.Salvo) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT ordem, titulo, questoes FROM plano_ciclo WHERE plano_id = $1 ORDER BY ordem`,
		planoID,
	)
	if err != nil {
		return fmt.Errorf("querying plano_ciclo: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var it concurso.RevItem
		if err := rows.Scan(&it.Ordem, &it.Titulo, &it.Questoes); err != nil {
			return fmt.Errorf("scanning plano_ciclo: %w", err)
		}

		s.Config.CicloRevisao = append(s.Config.CicloRevisao, it)
	}

	return rows.Err()
}

// ListAtividades returns the plan's manually arranged activities. Ordering by
// (data, posicao) here means the domain never has to re-sort what it reads.
func (r *PlanoRepo) ListAtividades(
	ctx context.Context,
	planoID uuid.UUID,
) ([]plano.Atividade, error) {
	const q = `
		SELECT id, data, posicao, disciplina, tema, passada, tipo, duracao_min,
		       origem_dia, origem_pos
		  FROM atividades
		 WHERE plano_id = $1
		 ORDER BY data, posicao`

	rows, err := r.pool.Query(ctx, q, planoID)
	if err != nil {
		return nil, fmt.Errorf("query atividades: %w", err)
	}
	defer rows.Close()

	out := []plano.Atividade{}

	for rows.Next() {
		var (
			a         plano.Atividade
			id        uuid.UUID
			tipo      string
			origemDia *time.Time
			origemPos *int
		)

		if err := rows.Scan(
			&id, &a.Data, &a.Posicao, &a.Disciplina, &a.Tema, &a.Passada, &tipo,
			&a.DuracaoMin, &origemDia, &origemPos,
		); err != nil {
			return nil, fmt.Errorf("scan atividade: %w", err)
		}

		a.ID = id.String()
		a.Tipo = plano.TipoAtividade(tipo)
		a.OrigemDia = origemDia
		a.OrigemPos = origemPos

		out = append(out, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows atividades: %w", err)
	}

	return out, nil
}

// ReplaceAtividades rewrites the whole layout in one transaction. Delete-then-
// insert (rather than per-row updates) is what keeps positions consistent: the
// unique constraint is deferred to commit, so intermediate states never clash.
func (r *PlanoRepo) ReplaceAtividades(
	ctx context.Context,
	planoID uuid.UUID,
	as []plano.Atividade,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if _, err := tx.Exec(ctx, `DELETE FROM atividades WHERE plano_id = $1`, planoID); err != nil {
		return fmt.Errorf("delete atividades: %w", err)
	}

	const ins = `
		INSERT INTO atividades
			(id, plano_id, data, posicao, disciplina, tema, passada, tipo,
			 duracao_min, origem_dia, origem_pos)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	for _, a := range as {
		id, err := uuid.Parse(a.ID)
		if err != nil {
			// A freshly derived activity has no id yet; give it one.
			id = uuid.New()
		}

		if _, err := tx.Exec(ctx, ins,
			id, planoID, a.Data, a.Posicao, a.Disciplina, a.Tema, a.Passada,
			string(a.Tipo), a.DuracaoMin, a.OrigemDia, a.OrigemPos,
		); err != nil {
			return fmt.Errorf("insert atividade: %w", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE planos SET atividades_manuais = true WHERE id = $1`, planoID,
	); err != nil {
		return fmt.Errorf("marcar atividades manuais: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}
