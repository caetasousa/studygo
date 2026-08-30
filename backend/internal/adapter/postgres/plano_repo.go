package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"annygo/internal/domain/concurso"
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

	var (
		diasEstudo []int32
		intervalos []int32
		horasDia   float64
		pctQuest   float64
		pctRevisao float64
		simulados  string
	)

	err := r.pool.QueryRow(
		ctx,
		`SELECT id, user_id, concurso_id, inicio, prova, horas_dia::float8,
		        dias_estudo, dia_revisao, reta_final_dias, tema_ui, criado_em, atualizado_em,
		        simulados, discursiva, intervalos_revisao, pct_questoes::float8,
		        revisao_por_questoes, questoes_por_revisao, limiar_fraco,
		        blocos_por_dia, pct_revisao::float8
		 FROM planos WHERE user_id = $1 AND concurso_id = $2`,
		userID,
		concursoID,
	).Scan(
		&s.ID, &s.UserID, &s.ConcursoID, &s.Config.Inicio, &s.Config.Prova, &horasDia,
		&diasEstudo, &s.Config.DiaRevisao, &s.Config.RetaFinalDias, &s.TemaUI,
		&s.CriadoEm, &s.AtualizadoEm,
		&simulados, &s.Config.Perfil.Discursiva, &intervalos, &pctQuest,
		&s.Config.Perfil.RevisaoPorQuestoes, &s.Config.Perfil.QuestoesPorRevisao,
		&s.Config.Perfil.LimiarFraco, &s.Config.Perfil.BlocosPorDia, &pctRevisao,
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

	s.Config.Perfil.Simulados = plano.Frequencia(simulados)
	s.Config.Perfil.Intervalos = toIntSlice(intervalos)
	s.Config.Perfil.PctQuestoes = pctQuest
	s.Config.Perfil.PctRevisao = pctRevisao
	s.Config.Perfil.Modos = map[string]plano.Modo{}
	s.Config.Perfil.Reforcos = map[string]float64{}

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

	revisoes, err := r.ListRevisoes(ctx, s.ID)
	if err != nil {
		return plano.Salvo{}, err
	}

	s.Revisoes = revisoes

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
		s.Config.Perfil.Modos[codigo] = plano.Modo(modo)
		s.Config.Perfil.Reforcos[codigo] = reforco
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
		`SELECT data, disciplina, horas::float8, questoes, acertos, nota
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

		if err := rows.Scan(&data, &b.Disciplina, &b.Horas, &b.Questoes, &b.Acertos, &b.Nota); err != nil {
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

	perfil := s.Config.Perfil.Normalizar()

	err = tx.QueryRow(
		ctx,
		`INSERT INTO planos
		   (user_id, concurso_id, inicio, prova, horas_dia, dias_estudo, dia_revisao, reta_final_dias,
		    tema_ui, simulados, discursiva, intervalos_revisao, pct_questoes,
		    revisao_por_questoes, questoes_por_revisao, limiar_fraco,
		    blocos_por_dia, pct_revisao)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		 ON CONFLICT (user_id, concurso_id) DO UPDATE SET
		   inicio = EXCLUDED.inicio, prova = EXCLUDED.prova, horas_dia = EXCLUDED.horas_dia,
		   dias_estudo = EXCLUDED.dias_estudo, dia_revisao = EXCLUDED.dia_revisao,
		   reta_final_dias = EXCLUDED.reta_final_dias, tema_ui = EXCLUDED.tema_ui,
		   simulados = EXCLUDED.simulados, discursiva = EXCLUDED.discursiva,
		   intervalos_revisao = EXCLUDED.intervalos_revisao, pct_questoes = EXCLUDED.pct_questoes,
		   revisao_por_questoes = EXCLUDED.revisao_por_questoes,
		   questoes_por_revisao = EXCLUDED.questoes_por_revisao,
		   limiar_fraco = EXCLUDED.limiar_fraco,
		   blocos_por_dia = EXCLUDED.blocos_por_dia, pct_revisao = EXCLUDED.pct_revisao,
		   atualizado_em = now()
		 RETURNING id, criado_em, atualizado_em`,
		s.UserID, s.ConcursoID, s.Config.Inicio, s.Config.Prova, s.Config.HorasDia,
		toInt32Slice(s.Config.DiasEstudo), s.Config.DiaRevisao, s.Config.RetaFinalDias, s.TemaUI,
		string(perfil.Simulados), perfil.Discursiva, toInt32Slice(perfil.Intervalos),
		perfil.PctQuestoes, perfil.RevisaoPorQuestoes, perfil.QuestoesPorRevisao, perfil.LimiarFraco,
		perfil.BlocosPorDia, perfil.PctRevisao,
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
			s.ID, codigo, q, string(perfil.ModoDe(codigo)), perfil.ReforcoDe(codigo),
		); err != nil {
			return plano.Salvo{}, fmt.Errorf("inserting plano_disciplina %s: %w", codigo, err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM plano_ciclo WHERE plano_id = $1`, s.ID); err != nil {
		return plano.Salvo{}, fmt.Errorf("clearing plano_ciclo: %w", err)
	}

	for i, it := range perfil.CicloRevisao {
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
			`INSERT INTO registros_bloco (plano_id, data, disciplina, horas, questoes, acertos, nota)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			planoID, reg.Data, b.Disciplina, b.Horas, b.Questoes, b.Acertos, b.Nota,
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

	return r.DeleteRevisoes(ctx, planoID)
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

func (r *PlanoRepo) ListRevisoes(ctx context.Context, planoID uuid.UUID) ([]plano.Revisao, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT id, disciplina, tema, origem_data, etapa, vence_em, feita_em, questoes, acertos
		 FROM revisoes WHERE plano_id = $1 AND feita_em IS NULL ORDER BY vence_em, disciplina`,
		planoID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying revisoes: %w", err)
	}
	defer rows.Close()

	out := []plano.Revisao{}

	for rows.Next() {
		var rev plano.Revisao
		if err := rows.Scan(
			&rev.ID, &rev.Disciplina, &rev.Tema, &rev.OrigemData, &rev.Etapa,
			&rev.VenceEm, &rev.FeitaEm, &rev.Questoes, &rev.Acertos,
		); err != nil {
			return nil, fmt.Errorf("scanning revisao: %w", err)
		}

		out = append(out, rev)
	}

	return out, rows.Err()
}

func (r *PlanoRepo) EnfileirarRevisoes(
	ctx context.Context,
	planoID uuid.UUID,
	rs []plano.Revisao,
) error {
	for _, rev := range rs {
		if _, err := r.pool.Exec(
			ctx,
			`INSERT INTO revisoes (plano_id, disciplina, tema, origem_data, etapa, vence_em)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (plano_id, disciplina, tema, etapa) DO NOTHING`,
			planoID, rev.Disciplina, rev.Tema, rev.OrigemData, rev.Etapa, rev.VenceEm,
		); err != nil {
			return fmt.Errorf("enqueueing revisao: %w", err)
		}
	}

	return nil
}

func (r *PlanoRepo) RevisaoByID(
	ctx context.Context,
	planoID, revisaoID uuid.UUID,
) (plano.Revisao, error) {
	var rev plano.Revisao

	err := r.pool.QueryRow(
		ctx,
		`SELECT id, disciplina, tema, origem_data, etapa, vence_em, feita_em, questoes, acertos
		 FROM revisoes WHERE id = $1 AND plano_id = $2`,
		revisaoID, planoID,
	).Scan(
		&rev.ID, &rev.Disciplina, &rev.Tema, &rev.OrigemData, &rev.Etapa,
		&rev.VenceEm, &rev.FeitaEm, &rev.Questoes, &rev.Acertos,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return plano.Revisao{}, plano.ErrNotFound
	}

	if err != nil {
		return plano.Revisao{}, fmt.Errorf("querying revisao: %w", err)
	}

	return rev, nil
}

// ConcluirRevisao closes one review and opens the next stage in one transaction,
// so the queue can never lose a topic halfway through.
func (r *PlanoRepo) ConcluirRevisao(
	ctx context.Context,
	planoID uuid.UUID,
	feita plano.Revisao,
	proxima *plano.Revisao,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(
		ctx,
		`UPDATE revisoes SET feita_em = $3, questoes = $4, acertos = $5
		 WHERE id = $1 AND plano_id = $2`,
		feita.ID, planoID, feita.FeitaEm, feita.Questoes, feita.Acertos,
	)
	if err != nil {
		return fmt.Errorf("closing revisao: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return plano.ErrNotFound
	}

	if proxima != nil {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO revisoes (plano_id, disciplina, tema, origem_data, etapa, vence_em)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (plano_id, disciplina, tema, etapa)
			 DO UPDATE SET vence_em = EXCLUDED.vence_em, feita_em = NULL,
			               questoes = NULL, acertos = NULL`,
			planoID, proxima.Disciplina, proxima.Tema, proxima.OrigemData,
			proxima.Etapa, proxima.VenceEm,
		); err != nil {
			return fmt.Errorf("enqueueing next revisao: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (r *PlanoRepo) DeleteRevisoes(ctx context.Context, planoID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM revisoes WHERE plano_id = $1`, planoID); err != nil {
		return fmt.Errorf("deleting revisoes: %w", err)
	}

	return nil
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

		s.Config.Perfil.CicloRevisao = append(s.Config.Perfil.CicloRevisao, it)
	}

	return rows.Err()
}
