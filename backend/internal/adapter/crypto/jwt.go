package crypto

import (
	"errors"
	"fmt"
	"time"

	"annygo/internal/port"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var _ port.TokenIssuer = (*JWTIssuer)(nil)

// ErrInvalidToken is returned when an access token fails validation.
var ErrInvalidToken = errors.New("token inválido")

// JWTIssuer mints HS256 access tokens with the user id as subject.
type JWTIssuer struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewJWTIssuer(secret string, ttl time.Duration) *JWTIssuer {
	return &JWTIssuer{
		secret: []byte(secret),
		ttl:    ttl,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func (j *JWTIssuer) Issue(userID uuid.UUID) (string, time.Time, error) {
	expiresAt := j.now().Add(j.ttl)

	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(j.now()),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("signing token: %w", err)
	}

	return token, expiresAt, nil
}

func (j *JWTIssuer) Parse(token string) (uuid.UUID, error) {
	parsed, err := jwt.ParseWithClaims(
		token,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrInvalidToken
			}

			return j.secret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil || !parsed.Valid {
		return uuid.Nil, ErrInvalidToken
	}

	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return uuid.Nil, ErrInvalidToken
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	return id, nil
}
