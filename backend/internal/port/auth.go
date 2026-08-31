package port

import (
	"context"
	"time"

	"studygo/internal/domain/user"

	"github.com/google/uuid"
)

// UserRepository persists accounts and their refresh tokens.
type UserRepository interface {
	CreateUser(ctx context.Context, u user.User) (user.User, error)
	UserByEmail(ctx context.Context, email string) (user.User, error)
	UserByID(ctx context.Context, id uuid.UUID) (user.User, error)

	StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	RefreshTokenValid(ctx context.Context, tokenHash string) (uuid.UUID, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
}

// PasswordHasher hashes and verifies passwords (argon2id).
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encodedHash string) (bool, error)
}

// TokenIssuer mints and parses short-lived access tokens (JWT).
type TokenIssuer interface {
	Issue(userID uuid.UUID) (token string, expiresAt time.Time, err error)
	Parse(token string) (uuid.UUID, error)
}
