package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"studygo/internal/domain/mapa"
	"studygo/internal/port"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// colunasDoPedido é o pedido p como a fila o mostra, com a matéria e o concurso
// dela; nunca os bytes do PDF, que só saem por PDFDoPedido.
const colunasDoPedido = `p.id, p.arquivo, p.situacao, p.disciplina_id,
	coalesce(d.nome, ''), coalesce(c.slug, ''), coalesce(p.mapa_slug, ''), p.relatorio,
	p.pdf IS NOT NULL, p.criado_em, p.atualizado_em`

const deOndeVemOPedido = `mapas_pedidos p
	LEFT JOIN disciplinas d ON d.id = p.disciplina_id
	LEFT JOIN concursos c ON c.id = d.concurso_id`

func lerPedido(row pgx.Row) (mapa.Pedido, error) {
	var p mapa.Pedido

	var situacao string

	err := row.Scan(&p.ID, &p.Arquivo, &situacao, &p.DisciplinaID, &p.DisciplinaNome, &p.ConcursoSlug,
		&p.Mapa, &p.Relatorio, &p.TemPDF, &p.CriadoEm, &p.AtualizadoEm)
	p.Situacao = mapa.Situacao(situacao)

	return p, err
}

func (r *MapaRepo) CriarPedido(
	ctx context.Context, usuarioID uuid.UUID, disciplinaID uuid.NullUUID, arquivo string, pdf []byte,
) (mapa.Pedido, error) {
	var id uuid.UUID

	if err := r.pool.QueryRow(ctx,
		`INSERT INTO mapas_pedidos (usuario_id, disciplina_id, arquivo, pdf) VALUES ($1, $2, $3, $4) RETURNING id`,
		usuarioID, disciplinaID, arquivo, pdf,
	).Scan(&id); err != nil {
		return mapa.Pedido{}, fmt.Errorf("gravando o pedido de mapa: %w", err)
	}

	return r.Pedido(ctx, usuarioID, id)
}

func (r *MapaRepo) Pedidos(ctx context.Context, usuarioID uuid.UUID) ([]mapa.Pedido, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+colunasDoPedido+` FROM `+deOndeVemOPedido+`
		  WHERE p.usuario_id = $1
		  ORDER BY p.criado_em DESC, p.id`,
		usuarioID,
	)
	if err != nil {
		return nil, fmt.Errorf("listando pedidos de mapa: %w", err)
	}
	defer rows.Close()

	out := []mapa.Pedido{}

	for rows.Next() {
		p, err := lerPedido(rows)
		if err != nil {
			return nil, fmt.Errorf("lendo pedido de mapa: %w", err)
		}

		out = append(out, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lendo pedidos de mapa: %w", err)
	}

	return out, nil
}

func (r *MapaRepo) Pedido(ctx context.Context, usuarioID, id uuid.UUID) (mapa.Pedido, error) {
	p, err := lerPedido(r.pool.QueryRow(ctx,
		`SELECT `+colunasDoPedido+` FROM `+deOndeVemOPedido+` WHERE p.usuario_id = $1 AND p.id = $2`,
		usuarioID, id,
	))

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return mapa.Pedido{}, mapa.ErrPedidoNaoEncontrado
	case err != nil:
		return mapa.Pedido{}, fmt.Errorf("lendo pedido de mapa: %w", err)
	}

	return p, nil
}

func (r *MapaRepo) PedidoPorID(ctx context.Context, id uuid.UUID) (mapa.Pedido, uuid.UUID, error) {
	var usuarioID uuid.UUID

	err := r.pool.QueryRow(ctx, `SELECT usuario_id FROM mapas_pedidos WHERE id = $1`, id).Scan(&usuarioID)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return mapa.Pedido{}, uuid.Nil, mapa.ErrPedidoNaoEncontrado
	case err != nil:
		return mapa.Pedido{}, uuid.Nil, fmt.Errorf("lendo pedido de mapa: %w", err)
	}

	p, err := r.Pedido(ctx, usuarioID, id)

	return p, usuarioID, err
}

func (r *MapaRepo) PedidosProcessando(ctx context.Context) ([]port.PedidoDaConta, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.usuario_id, `+colunasDoPedido+` FROM `+deOndeVemOPedido+`
		  WHERE p.situacao = 'processando'
		  ORDER BY p.criado_em, p.id`)
	if err != nil {
		return nil, fmt.Errorf("listando pedidos em processamento: %w", err)
	}
	defer rows.Close()

	var out []port.PedidoDaConta

	for rows.Next() {
		var (
			pc       port.PedidoDaConta
			situacao string
		)

		p := &pc.Pedido
		if err := rows.Scan(&pc.UsuarioID, &p.ID, &p.Arquivo, &situacao, &p.DisciplinaID, &p.DisciplinaNome,
			&p.ConcursoSlug, &p.Mapa, &p.Relatorio, &p.TemPDF, &p.CriadoEm, &p.AtualizadoEm); err != nil {
			return nil, fmt.Errorf("lendo pedido em processamento: %w", err)
		}

		p.Situacao = mapa.Situacao(situacao)
		out = append(out, pc)
	}

	return out, rows.Err()
}

func (r *MapaRepo) GravarMapaDoPedido(ctx context.Context, id uuid.UUID, slug string) error {
	if _, err := r.pool.Exec(ctx, `UPDATE mapas_pedidos SET mapa_slug = $2 WHERE id = $1`, id, slug); err != nil {
		return fmt.Errorf("anotando o mapa do pedido: %w", err)
	}

	return nil
}

func (r *MapaRepo) PDFDoPedido(ctx context.Context, usuarioID, id uuid.UUID) ([]byte, error) {
	var pdf []byte

	err := r.pool.QueryRow(ctx,
		`SELECT pdf FROM mapas_pedidos WHERE usuario_id = $1 AND id = $2 AND pdf IS NOT NULL`,
		usuarioID, id,
	).Scan(&pdf)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, mapa.ErrPedidoNaoEncontrado
	case err != nil:
		return nil, fmt.Errorf("lendo o PDF do pedido: %w", err)
	}

	return pdf, nil
}

func (r *MapaRepo) MudarPedido(
	ctx context.Context, usuarioID, id uuid.UUID, de []mapa.Situacao, para mapa.Situacao,
	mapaSlug, relatorio string, descartarPDF bool,
) error {
	origens := make([]string, 0, len(de))
	for _, s := range de {
		origens = append(origens, string(s))
	}

	var slug *string
	if mapaSlug != "" {
		slug = &mapaSlug
	}

	tag, err := r.pool.Exec(ctx,
		`UPDATE mapas_pedidos
		    SET situacao = $4, mapa_slug = coalesce($5, mapa_slug), relatorio = $6, atualizado_em = now(),
		        pdf = CASE WHEN $7 THEN NULL ELSE pdf END
		  WHERE usuario_id = $1 AND id = $2 AND situacao = ANY ($3)`,
		usuarioID, id, origens, string(para), slug, relatorio, descartarPDF,
	)
	if err != nil {
		return fmt.Errorf("mudando o pedido de mapa: %w", err)
	}

	if tag.RowsAffected() == 1 {
		return nil
	}

	// Nada mudou: ou o pedido não é da conta, ou está noutra situação.
	if _, err := r.Pedido(ctx, usuarioID, id); err != nil {
		return err
	}

	return mapa.ErrPedidoForaDeHora
}

func (r *MapaRepo) ExcluirPedido(ctx context.Context, usuarioID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM mapas_pedidos WHERE usuario_id = $1 AND id = $2`, usuarioID, id)
	if err != nil {
		return fmt.Errorf("excluindo pedido de mapa: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return mapa.ErrPedidoNaoEncontrado
	}

	return nil
}

var _ port.FaxinaDosPedidos = (*MapaRepo)(nil)

func (r *MapaRepo) DescartarPDFsDeFalhas(ctx context.Context, antesDe time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE mapas_pedidos SET pdf = NULL
		  WHERE situacao = 'falhou' AND pdf IS NOT NULL AND atualizado_em < $1`,
		antesDe,
	)
	if err != nil {
		return 0, fmt.Errorf("descartando os PDFs dos pedidos que falharam: %w", err)
	}

	return tag.RowsAffected(), nil
}
