// Package postgres holds the outbound adapters that implement the repository
// ports against Postgres, using pgx and hand-written SQL.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"annygo/internal/domain/user"
	"annygo/internal/port"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ port.UserRepository = (*UserRepo)(nil)

// UserRepo persists accounts and refresh tokens.
type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) CreateUser(ctx context.Context, u user.User) (user.User, error) {
	row := r.pool.QueryRow(
		ctx,
		`INSERT INTO users (email, nome, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, criado_em`,
		u.Email,
		u.Nome,
		u.PasswordHash,
	)

	if err := row.Scan(&u.ID, &u.CriadoEm); err != nil {
		if isUniqueViolation(err) {
			return user.User{}, user.ErrEmailTaken
		}

		return user.User{}, fmt.Errorf("inserting user: %w", err)
	}

	return u, nil
}

func (r *UserRepo) UserByEmail(ctx context.Context, email string) (user.User, error) {
	return r.scanUser(ctx, `WHERE email = $1`, email)
}

func (r *UserRepo) UserByID(ctx context.Context, id uuid.UUID) (user.User, error) {
	return r.scanUser(ctx, `WHERE id = $1`, id)
}

func (r *UserRepo) scanUser(ctx context.Context, where string, arg any) (user.User, error) {
	var u user.User

	err := r.pool.QueryRow(
		ctx,
		`SELECT id, email, nome, password_hash, criado_em FROM users `+where,
		arg,
	).Scan(&u.ID, &u.Email, &u.Nome, &u.PasswordHash, &u.CriadoEm)

	if errors.Is(err, pgx.ErrNoRows) {
		return user.User{}, user.ErrNotFound
	}

	if err != nil {
		return user.User{}, fmt.Errorf("querying user: %w", err)
	}

	return u, nil
}

func (r *UserRepo) StoreRefreshToken(
	ctx context.Context,
	userID uuid.UUID,
	tokenHash string,
	expiresAt time.Time,
) error {
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expira_em) VALUES ($1, $2, $3)`,
		userID,
		tokenHash,
		expiresAt,
	)
	if err != nil {
		return fmt.Errorf("storing refresh token: %w", err)
	}

	return nil
}

func (r *UserRepo) RefreshTokenValid(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID

	err := r.pool.QueryRow(
		ctx,
		`SELECT user_id FROM refresh_tokens
		 WHERE token_hash = $1 AND NOT revogado AND expira_em > now()`,
		tokenHash,
	).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, user.ErrInvalidCredentials
	}

	if err != nil {
		return uuid.Nil, fmt.Errorf("validating refresh token: %w", err)
	}

	return userID, nil
}

func (r *UserRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(
		ctx,
		`UPDATE refresh_tokens SET revogado = true WHERE token_hash = $1`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("revoking refresh token: %w", err)
	}

	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}
