package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"studygo/internal/domain/usuario"
	"studygo/internal/port"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	_ port.UsuarioRepository = (*UsuarioRepo)(nil)
	_ port.SessaoManutencao  = (*UsuarioRepo)(nil)
)

// UsuarioRepo persiste contas e refresh tokens.
type UsuarioRepo struct {
	pool *pgxpool.Pool
}

func NewUsuarioRepo(pool *pgxpool.Pool) *UsuarioRepo {
	return &UsuarioRepo{pool: pool}
}

func (r *UsuarioRepo) Criar(
	ctx context.Context,
	u usuario.Usuario,
) (usuario.Usuario, error) {
	err := r.pool.QueryRow(
		ctx,
		`INSERT INTO usuarios (email, nome, senha_hash, tema_ui)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, criado_em`,
		u.Email, u.Nome, u.SenhaHash, string(u.TemaUI),
	).Scan(&u.ID, &u.CriadoEm)
	if err != nil {
		if violaUnique(err) {
			return usuario.Usuario{}, usuario.ErrEmailEmUso
		}

		return usuario.Usuario{}, fmt.Errorf("criando usuário: %w", err)
	}

	return u, nil
}

func (r *UsuarioRepo) PorEmail(ctx context.Context, email string) (usuario.Usuario, error) {
	return r.escanear(ctx, `WHERE email = $1`, email)
}

func (r *UsuarioRepo) PorID(ctx context.Context, id uuid.UUID) (usuario.Usuario, error) {
	return r.escanear(ctx, `WHERE id = $1`, id)
}

func (r *UsuarioRepo) escanear(
	ctx context.Context,
	onde string,
	arg any,
) (usuario.Usuario, error) {
	var (
		u    usuario.Usuario
		tema string
	)

	err := r.pool.QueryRow(
		ctx,
		`SELECT id, email, nome, senha_hash, tema_ui, criado_em FROM usuarios `+onde,
		arg,
	).Scan(&u.ID, &u.Email, &u.Nome, &u.SenhaHash, &tema, &u.CriadoEm)

	if errors.Is(err, pgx.ErrNoRows) {
		return usuario.Usuario{}, usuario.ErrNaoEncontrado
	}

	if err != nil {
		return usuario.Usuario{}, fmt.Errorf("consultando usuário: %w", err)
	}

	u.TemaUI = usuario.Tema(tema)

	return u, nil
}

func (r *UsuarioRepo) DefinirTema(
	ctx context.Context,
	id uuid.UUID,
	tema usuario.Tema,
) error {
	if _, err := r.pool.Exec(
		ctx,
		`UPDATE usuarios SET tema_ui = $2, atualizado_em = now() WHERE id = $1`,
		id, string(tema),
	); err != nil {
		return fmt.Errorf("gravando tema: %w", err)
	}

	return nil
}

func (r *UsuarioRepo) GuardarRefreshToken(
	ctx context.Context,
	usuarioID uuid.UUID,
	tokenHash string,
	expiraEm time.Time,
) error {
	if _, err := r.pool.Exec(
		ctx,
		`INSERT INTO refresh_tokens (usuario_id, token_hash, expira_em)
		 VALUES ($1,$2,$3)`,
		usuarioID, tokenHash, expiraEm,
	); err != nil {
		return fmt.Errorf("guardando refresh token: %w", err)
	}

	return nil
}

// RefreshTokenValido devolve o dono de um refresh token ainda utilizável: não
// revogado e dentro da validade. Quem já expirou ou foi revogado é
// indistinguível de inexistente para quem chama — a resposta é a mesma.
func (r *UsuarioRepo) RefreshTokenValido(
	ctx context.Context,
	tokenHash string,
) (uuid.UUID, error) {
	var id uuid.UUID

	err := r.pool.QueryRow(
		ctx,
		`SELECT usuario_id FROM refresh_tokens
		  WHERE token_hash = $1 AND NOT revogado AND expira_em > now()`,
		tokenHash,
	).Scan(&id)

	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, usuario.ErrCredenciaisInvalidas
	}

	if err != nil {
		return uuid.Nil, fmt.Errorf("consultando refresh token: %w", err)
	}

	return id, nil
}

func (r *UsuarioRepo) RevogarRefreshToken(ctx context.Context, tokenHash string) error {
	if _, err := r.pool.Exec(
		ctx,
		`UPDATE refresh_tokens SET revogado = true WHERE token_hash = $1`,
		tokenHash,
	); err != nil {
		return fmt.Errorf("revogando refresh token: %w", err)
	}

	return nil
}

// LimparRefreshTokens apaga o que não autentica mais ninguém.
//
// A tabela só crescia: cada login, cadastro e renovação insere uma linha, e a
// rotação revoga a anterior sem nunca removê-la. Um usuário ativo deixa
// centenas de linhas mortas por ano, e todas participam do índice que a
// renovação consulta a cada quinze minutos.
//
// O vencido sai porque a própria consulta de validação já o ignora; o revogado
// sai porque revogar é definitivo. Nenhum dos dois tem como voltar a valer, e
// por isso apagar aqui não é política — é varrer o que a política já descartou.
func (r *UsuarioRepo) LimparRefreshTokens(ctx context.Context, agora time.Time) (int64, error) {
	ct, err := r.pool.Exec(
		ctx,
		`DELETE FROM refresh_tokens WHERE revogado OR expira_em <= $1`,
		agora,
	)
	if err != nil {
		return 0, fmt.Errorf("limpando refresh tokens: %w", err)
	}

	return ct.RowsAffected(), nil
}
