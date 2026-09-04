package postgres

import (
	"context"
	"errors"
	"fmt"

	"studygo/internal/domain/plano"
	"studygo/internal/port"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ port.PlanoRepository = (*PlanoRepo)(nil)

// PlanoRepo persiste o plano e sua configuração. O cronograma e os registros
// ficam no CronogramaRepo; o caderno, no CadernoRepo.
type PlanoRepo struct {
	pool *pgxpool.Pool
}

func NewPlanoRepo(pool *pgxpool.Pool) *PlanoRepo {
	return &PlanoRepo{pool: pool}
}

// colunasPlano é a lista que PorUsuario e ParaLembrete leem, na mesma ordem em
// que escanearPlano as consome. Uma lista só, para que as duas consultas não
// possam divergir.
const colunasPlano = `p.id, p.usuario_id, p.concurso_id, p.inicio, p.prova,
	p.horas_dia::float8, p.dias_estudo, p.dia_revisao, p.reta_final_dias,
	p.blocos_por_dia, p.minutos_bloco, p.minutos_revisao, p.revisao_semanal,
	p.simulados, p.discursiva, p.pct_questoes::float8, p.limiar_fraco,
	p.criado_em, p.atualizado_em`

// escanearPlano lê uma linha de planos com as colunasPlano acima.
func escanearPlano(linha pgx.Row) (plano.Plano, error) {
	p := plano.NovoPlano()

	var (
		diasEstudo []int32
		horasDia   float64
		pctQuest   float64
		simulados  string
	)

	err := linha.Scan(
		&p.ID, &p.UsuarioID, &p.ConcursoID, &p.Config.Inicio, &p.Config.Prova,
		&horasDia, &diasEstudo, &p.Config.DiaRevisao, &p.Config.RetaFinalDias,
		&p.Config.BlocosPorDia, &p.Config.MinutosBloco, &p.Config.MinutosRevisao,
		&p.Config.RevisaoSemanal, &simulados, &p.Config.Discursiva, &pctQuest,
		&p.Config.LimiarFraco, &p.CriadoEm, &p.AtualizadoEm,
	)
	if err != nil {
		return plano.Plano{}, err
	}

	p.Config.HorasDia = horasDia
	p.Config.DiasEstudo = paraInts(diasEstudo)
	p.Config.Simulados = plano.Frequencia(simulados)
	p.Config.PctQuestoes = pctQuest
	p.Config.Questoes = map[string]int{}
	p.Config.Modos = map[string]plano.Modo{}
	p.Config.Reforcos = map[string]float64{}

	return p, nil
}

func (r *PlanoRepo) PorUsuario(
	ctx context.Context,
	usuarioID, concursoID uuid.UUID,
) (plano.Plano, error) {
	p, err := escanearPlano(r.pool.QueryRow(
		ctx,
		`SELECT `+colunasPlano+` FROM planos p
		 WHERE p.usuario_id = $1 AND p.concurso_id = $2`,
		usuarioID, concursoID,
	))

	if errors.Is(err, pgx.ErrNoRows) {
		return plano.Plano{}, plano.ErrNaoEncontrado
	}

	if err != nil {
		return plano.Plano{}, fmt.Errorf("consultando plano: %w", err)
	}

	if err := r.carregarDisciplinas(ctx, &p); err != nil {
		return plano.Plano{}, err
	}

	if err := r.carregarCiclo(ctx, &p); err != nil {
		return plano.Plano{}, err
	}

	if err := r.carregarMarcos(ctx, &p); err != nil {
		return plano.Plano{}, err
	}

	return p, nil
}

// carregarDisciplinas traz os ajustes por disciplina. A chave no domínio é o
// CÓDIGO (o motor pensa em códigos), mas a coluna é o id — é o join que traduz.
func (r *PlanoRepo) carregarDisciplinas(ctx context.Context, p *plano.Plano) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT d.codigo, pd.questoes, pd.modo, pd.reforco::float8
		 FROM plano_disciplinas pd
		 JOIN disciplinas d ON d.id = pd.disciplina_id
		 WHERE pd.plano_id = $1`,
		p.ID,
	)
	if err != nil {
		return fmt.Errorf("consultando plano_disciplinas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			codigo   string
			questoes int
			modo     string
			reforco  float64
		)

		if err := rows.Scan(&codigo, &questoes, &modo, &reforco); err != nil {
			return fmt.Errorf("lendo plano_disciplina: %w", err)
		}

		p.Config.Questoes[codigo] = questoes
		p.Config.Modos[codigo] = plano.Modo(modo)
		p.Config.Reforcos[codigo] = reforco
	}

	return rows.Err()
}

func (r *PlanoRepo) carregarCiclo(ctx context.Context, p *plano.Plano) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT ordem, titulo, questoes FROM plano_ciclo WHERE plano_id = $1 ORDER BY ordem`,
		p.ID,
	)
	if err != nil {
		return fmt.Errorf("consultando plano_ciclo: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var it itemCiclo
		if err := rows.Scan(&it.Ordem, &it.Titulo, &it.Questoes); err != nil {
			return fmt.Errorf("lendo item do ciclo: %w", err)
		}

		p.Config.CicloRevisao = append(p.Config.CicloRevisao, it.paraDominio())
	}

	return rows.Err()
}

func (r *PlanoRepo) carregarMarcos(ctx context.Context, p *plano.Plano) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT marco_id, cumprido FROM marco_checks WHERE plano_id = $1`,
		p.ID,
	)
	if err != nil {
		return fmt.Errorf("consultando marco_checks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id       uuid.UUID
			cumprido bool
		)

		if err := rows.Scan(&id, &cumprido); err != nil {
			return fmt.Errorf("lendo marco_check: %w", err)
		}

		p.Marcos[id] = cumprido
	}

	return rows.Err()
}

// Salvar grava o plano e substitui por inteiro seus ajustes por disciplina e o
// ciclo de revisão — são listas pequenas e completas, então recriá-las é mais
// simples e mais correto que diferenciar.
func (r *PlanoRepo) Salvar(ctx context.Context, p plano.Plano) (plano.Plano, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return plano.Plano{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	cfg := p.Config

	err = tx.QueryRow(
		ctx,
		`INSERT INTO planos
		   (usuario_id, concurso_id, inicio, prova, horas_dia, dias_estudo,
		    dia_revisao, reta_final_dias, blocos_por_dia, minutos_bloco,
		    minutos_revisao, revisao_semanal, simulados, discursiva,
		    pct_questoes, limiar_fraco)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		 ON CONFLICT (usuario_id, concurso_id) DO UPDATE SET
		   inicio = EXCLUDED.inicio, prova = EXCLUDED.prova,
		   horas_dia = EXCLUDED.horas_dia, dias_estudo = EXCLUDED.dias_estudo,
		   dia_revisao = EXCLUDED.dia_revisao,
		   reta_final_dias = EXCLUDED.reta_final_dias,
		   blocos_por_dia = EXCLUDED.blocos_por_dia,
		   minutos_bloco = EXCLUDED.minutos_bloco,
		   minutos_revisao = EXCLUDED.minutos_revisao,
		   revisao_semanal = EXCLUDED.revisao_semanal,
		   simulados = EXCLUDED.simulados, discursiva = EXCLUDED.discursiva,
		   pct_questoes = EXCLUDED.pct_questoes,
		   limiar_fraco = EXCLUDED.limiar_fraco,
		   atualizado_em = now()
		 RETURNING id, criado_em, atualizado_em`,
		p.UsuarioID, p.ConcursoID, cfg.Inicio, cfg.Prova, cfg.HorasDia,
		paraInt32s(cfg.DiasEstudo), cfg.DiaRevisao, cfg.RetaFinalDias,
		cfg.BlocosPorDia, cfg.MinutosBloco, cfg.MinutosRevisao, cfg.RevisaoSemanal,
		string(cfg.Simulados), cfg.Discursiva, cfg.PctQuestoes, cfg.LimiarFraco,
	).Scan(&p.ID, &p.CriadoEm, &p.AtualizadoEm)
	if err != nil {
		return plano.Plano{}, fmt.Errorf("gravando plano: %w", err)
	}

	if err := substituirDisciplinasDoPlano(ctx, tx, p); err != nil {
		return plano.Plano{}, err
	}

	if err := substituirCicloDoPlano(ctx, tx, p); err != nil {
		return plano.Plano{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return plano.Plano{}, fmt.Errorf("commit: %w", err)
	}

	return p, nil
}

func substituirDisciplinasDoPlano(ctx context.Context, tx pgx.Tx, p plano.Plano) error {
	if _, err := tx.Exec(
		ctx, `DELETE FROM plano_disciplinas WHERE plano_id = $1`, p.ID,
	); err != nil {
		return fmt.Errorf("limpando plano_disciplinas: %w", err)
	}

	if len(p.Config.Questoes) == 0 {
		return nil
	}

	lote := &pgx.Batch{}

	for codigo, questoes := range p.Config.Questoes {
		modo := p.Config.ModoDe(codigo)
		reforco := p.Config.ReforcoDe(codigo)

		// O id vem do código pelo mesmo join da leitura, resolvido pelo banco:
		// assim uma disciplina removida do concurso simplesmente não insere nada,
		// em vez de estourar a FK.
		lote.Queue(
			`INSERT INTO plano_disciplinas (plano_id, disciplina_id, questoes, modo, reforco)
			 SELECT $1, d.id, $3, $4, $5
			   FROM disciplinas d
			   JOIN planos p ON p.concurso_id = d.concurso_id
			  WHERE p.id = $1 AND d.codigo = $2`,
			p.ID, codigo, questoes, string(modo), reforco,
		)
	}

	if err := tx.SendBatch(ctx, lote).Close(); err != nil {
		return fmt.Errorf("gravando plano_disciplinas: %w", err)
	}

	return nil
}

func substituirCicloDoPlano(ctx context.Context, tx pgx.Tx, p plano.Plano) error {
	if _, err := tx.Exec(ctx, `DELETE FROM plano_ciclo WHERE plano_id = $1`, p.ID); err != nil {
		return fmt.Errorf("limpando plano_ciclo: %w", err)
	}

	if len(p.Config.CicloRevisao) == 0 {
		return nil
	}

	lote := &pgx.Batch{}

	for _, it := range p.Config.CicloRevisao {
		lote.Queue(
			`INSERT INTO plano_ciclo (plano_id, ordem, titulo, questoes) VALUES ($1,$2,$3,$4)`,
			p.ID, it.Ordem, it.Titulo, it.Questoes,
		)
	}

	if err := tx.SendBatch(ctx, lote).Close(); err != nil {
		return fmt.Errorf("gravando plano_ciclo: %w", err)
	}

	return nil
}

func (r *PlanoRepo) MarcarMarco(
	ctx context.Context,
	planoID, marcoID uuid.UUID,
	cumprido bool,
) error {
	if _, err := r.pool.Exec(
		ctx,
		`INSERT INTO marco_checks (plano_id, marco_id, cumprido) VALUES ($1,$2,$3)
		 ON CONFLICT (plano_id, marco_id) DO UPDATE SET cumprido = EXCLUDED.cumprido`,
		planoID, marcoID, cumprido,
	); err != nil {
		return fmt.Errorf("gravando marco_check: %w", err)
	}

	return nil
}

func paraInts(xs []int32) []int {
	out := make([]int, len(xs))
	for i, x := range xs {
		out[i] = int(x)
	}

	return out
}

func paraInt32s(xs []int) []int32 {
	out := make([]int32, len(xs))
	for i, x := range xs {
		out[i] = int32(x)
	}

	return out
}
