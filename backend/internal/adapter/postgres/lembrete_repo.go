package postgres

import (
	"context"
	"fmt"
	"time"

	"studygo/internal/domain/plano"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// ParaLembrete carrega todo plano de que o worker de lembretes precisa, num
// número de consultas independente de quantos planos existem.
//
// O lembrete só lê a configuração e o cronograma com seus registros, então é
// exatamente isso que é carregado — com WHERE plano_id = ANY($1), de modo que N
// planos custem um punhado de idas ao banco em vez de N × 7. A versão anterior
// rodava o carregamento completo por plano e crescia junto com a base de
// usuários.
func (r *PlanoRepo) ParaLembrete(ctx context.Context) ([]port.PlanoDoUsuario, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT `+colunasPlano+`, u.email, u.nome
		   FROM planos p JOIN usuarios u ON u.id = p.usuario_id`,
	)
	if err != nil {
		return nil, fmt.Errorf("listando planos para lembrete: %w", err)
	}

	type linha struct {
		p     plano.Plano
		email string
		nome  string
	}

	linhas := []linha{}
	ids := []uuid.UUID{}

	for rows.Next() {
		// escanearPlano lê as colunasPlano; email e nome vêm logo depois, então
		// a linha é escaneada aqui inteira, na mesma ordem do SELECT.
		var (
			l          linha
			diasEstudo []int32
			horasDia   float64
			pctQuest   float64
			simulados  string
		)

		l.p = plano.NovoPlano()

		if err := rows.Scan(
			&l.p.ID, &l.p.UsuarioID, &l.p.ConcursoID, &l.p.Config.Inicio,
			&l.p.Config.Prova, &horasDia, &diasEstudo, &l.p.Config.DiaRevisao,
			&l.p.Config.RetaFinalDias, &l.p.Config.BlocosPorDia,
			&l.p.Config.MinutosBloco, &l.p.Config.MinutosRevisao,
			&l.p.Config.RevisaoSemanal, &simulados, &l.p.Config.Discursiva,
			&pctQuest, &l.p.Config.LimiarFraco, &l.p.CriadoEm, &l.p.AtualizadoEm,
			&l.email, &l.nome,
		); err != nil {
			rows.Close()

			return nil, fmt.Errorf("lendo plano para lembrete: %w", err)
		}

		l.p.Config.HorasDia = horasDia
		l.p.Config.DiasEstudo = paraInts(diasEstudo)
		l.p.Config.Simulados = plano.Frequencia(simulados)
		l.p.Config.PctQuestoes = pctQuest
		l.p.Config.Questoes = map[string]int{}
		l.p.Config.Modos = map[string]plano.Modo{}
		l.p.Config.Reforcos = map[string]float64{}

		linhas = append(linhas, l)
		ids = append(ids, l.p.ID)
	}

	rows.Close()

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterando planos para lembrete: %w", err)
	}

	if len(linhas) == 0 {
		return []port.PlanoDoUsuario{}, nil
	}

	porID := make(map[uuid.UUID]*plano.Plano, len(linhas))
	for i := range linhas {
		porID[linhas[i].p.ID] = &linhas[i].p
	}

	if err := r.espalharDisciplinas(ctx, ids, porID); err != nil {
		return nil, err
	}

	if err := r.espalharCiclo(ctx, ids, porID); err != nil {
		return nil, err
	}

	out := make([]port.PlanoDoUsuario, 0, len(linhas))
	for _, l := range linhas {
		out = append(out, port.PlanoDoUsuario{
			Plano:      l.p,
			ConcursoID: l.p.ConcursoID,
			Email:      l.email,
			Nome:       l.nome,
		})
	}

	return out, nil
}

func (r *PlanoRepo) espalharDisciplinas(
	ctx context.Context,
	ids []uuid.UUID,
	porID map[uuid.UUID]*plano.Plano,
) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT pd.plano_id, d.codigo, pd.questoes, pd.modo, pd.reforco::float8
		   FROM plano_disciplinas pd
		   JOIN disciplinas d ON d.id = pd.disciplina_id
		  WHERE pd.plano_id = ANY($1)`,
		ids,
	)
	if err != nil {
		return fmt.Errorf("consultando plano_disciplinas em lote: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			planoID  uuid.UUID
			codigo   string
			questoes int
			modo     string
			reforco  float64
		)

		if err := rows.Scan(&planoID, &codigo, &questoes, &modo, &reforco); err != nil {
			return fmt.Errorf("lendo plano_disciplina em lote: %w", err)
		}

		if p, ok := porID[planoID]; ok {
			p.Config.Questoes[codigo] = questoes
			p.Config.Modos[codigo] = plano.Modo(modo)
			p.Config.Reforcos[codigo] = reforco
		}
	}

	return rows.Err()
}

func (r *PlanoRepo) espalharCiclo(
	ctx context.Context,
	ids []uuid.UUID,
	porID map[uuid.UUID]*plano.Plano,
) error {
	rows, err := r.pool.Query(
		ctx,
		`SELECT plano_id, ordem, titulo, questoes
		   FROM plano_ciclo WHERE plano_id = ANY($1) ORDER BY plano_id, ordem`,
		ids,
	)
	if err != nil {
		return fmt.Errorf("consultando plano_ciclo em lote: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			planoID uuid.UUID
			it      itemCiclo
		)

		if err := rows.Scan(&planoID, &it.Ordem, &it.Titulo, &it.Questoes); err != nil {
			return fmt.Errorf("lendo item do ciclo em lote: %w", err)
		}

		if p, ok := porID[planoID]; ok {
			p.Config.CicloRevisao = append(p.Config.CicloRevisao, it.paraDominio())
		}
	}

	return rows.Err()
}

// ComAtraso encontra os planos com dia vencido cujas atividades ninguém
// registrou.
//
// Uma consulta só, com EXISTS: o interesse é saber SE o plano tem atraso, não
// quanto — parar no primeiro acerto evita varrer o cronograma inteiro de quem
// está em dia. O replanejamento em si é do domínio; aqui só se decide quem
// precisa dele.
//
// A atividade conta como atrasada quando não há registro OU o registro existe
// mas não marca conclusão: um dia aberto pela metade continua sendo um dia que
// não aconteceu.
func (r *PlanoRepo) ComAtraso(ctx context.Context, hoje time.Time) ([]port.PlanoAtrasado, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT p.usuario_id, c.slug
		   FROM planos p
		   JOIN concursos c ON c.id = p.concurso_id
		  WHERE EXISTS (
		        SELECT 1
		          FROM atividades a
		     LEFT JOIN registros_atividade ra ON ra.atividade_id = a.id
		         WHERE a.plano_id = p.id
		           AND a.data < $1
		           AND COALESCE(ra.concluido, false) = false
		        )`,
		hoje,
	)
	if err != nil {
		return nil, fmt.Errorf("listando planos com atraso: %w", err)
	}
	defer rows.Close()

	saida := []port.PlanoAtrasado{}

	for rows.Next() {
		var p port.PlanoAtrasado
		if err := rows.Scan(&p.UsuarioID, &p.Slug); err != nil {
			return nil, fmt.Errorf("lendo plano com atraso: %w", err)
		}

		saida = append(saida, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterando planos com atraso: %w", err)
	}

	return saida, nil
}
