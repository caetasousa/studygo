package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"annygo/internal/domain/user"
	"annygo/internal/port"

	"github.com/google/uuid"
)

// TokenPair is what a successful register/login/refresh returns.
type TokenPair struct {
	AccessToken     string
	AccessExpiresAt time.Time
	RefreshToken    string
}

// AuthService implements registration, login and refresh-token rotation.
type AuthService struct {
	users      port.UserRepository
	hasher     port.PasswordHasher
	tokens     port.TokenIssuer
	clock      port.Clock
	refreshTTL time.Duration
}

func NewAuthService(
	users port.UserRepository,
	hasher port.PasswordHasher,
	tokens port.TokenIssuer,
	clock port.Clock,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		users:      users,
		hasher:     hasher,
		tokens:     tokens,
		clock:      clock,
		refreshTTL: refreshTTL,
	}
}

func (s *AuthService) Register(ctx context.Context, email, nome, senha string) (user.User, TokenPair, error) {
	if err := user.ValidateRegistration(email, nome, senha); err != nil {
		return user.User{}, TokenPair{}, err
	}

	hash, err := s.hasher.Hash(senha)
	if err != nil {
		return user.User{}, TokenPair{}, fmt.Errorf("hashing password: %w", err)
	}

	created, err := s.users.CreateUser(ctx, user.User{
		Email:        user.NormalizeEmail(email),
		Nome:         nome,
		PasswordHash: hash,
	})
	if err != nil {
		return user.User{}, TokenPair{}, err
	}

	pair, err := s.issuePair(ctx, created.ID)
	if err != nil {
		return user.User{}, TokenPair{}, err
	}

	return created, pair, nil
}

func (s *AuthService) Login(ctx context.Context, email, senha string) (user.User, TokenPair, error) {
	found, err := s.users.UserByEmail(ctx, user.NormalizeEmail(email))
	if errors.Is(err, user.ErrNotFound) {
		return user.User{}, TokenPair{}, user.ErrInvalidCredentials
	}

	if err != nil {
		return user.User{}, TokenPair{}, err
	}

	ok, err := s.hasher.Verify(senha, found.PasswordHash)
	if err != nil {
		return user.User{}, TokenPair{}, fmt.Errorf("verifying password: %w", err)
	}

	if !ok {
		return user.User{}, TokenPair{}, user.ErrInvalidCredentials
	}

	pair, err := s.issuePair(ctx, found.ID)
	if err != nil {
		return user.User{}, TokenPair{}, err
	}

	return found, pair, nil
}

// Refresh rotates the token: the presented refresh token is revoked and a fresh
// pair is issued.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	hash := hashToken(refreshToken)

	userID, err := s.users.RefreshTokenValid(ctx, hash)
	if errors.Is(err, user.ErrInvalidCredentials) {
		return TokenPair{}, user.ErrInvalidCredentials
	}

	if err != nil {
		return TokenPair{}, err
	}

	if err := s.users.RevokeRefreshToken(ctx, hash); err != nil {
		return TokenPair{}, err
	}

	return s.issuePair(ctx, userID)
}

// Logout revokes a refresh token; unknown tokens are a no-op.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.users.RevokeRefreshToken(ctx, hashToken(refreshToken))
}

func (s *AuthService) UserByID(ctx context.Context, id uuid.UUID) (user.User, error) {
	return s.users.UserByID(ctx, id)
}

func (s *AuthService) issuePair(ctx context.Context, userID uuid.UUID) (TokenPair, error) {
	access, accessExp, err := s.tokens.Issue(userID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("issuing access token: %w", err)
	}

	refresh, err := randomToken()
	if err != nil {
		return TokenPair{}, err
	}

	expiresAt := s.clock.Now().Add(s.refreshTTL)
	if err := s.users.StoreRefreshToken(ctx, userID, hashToken(refresh), expiresAt); err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:     access,
		AccessExpiresAt: accessExp,
		RefreshToken:    refresh,
	}, nil
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}
