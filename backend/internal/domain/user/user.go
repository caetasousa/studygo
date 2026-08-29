// Package user holds the account entity and its invariants. No infrastructure
// types appear here.
package user

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNotFound is returned by repositories when no account matches.
	ErrNotFound = errors.New("usuário não encontrado")
	// ErrEmailTaken is returned when registering an already-used email.
	ErrEmailTaken = errors.New("email já cadastrado")
	// ErrInvalidCredentials is returned on a failed login.
	ErrInvalidCredentials = errors.New("email ou senha inválidos")
	// ErrInvalidEmail / ErrWeakPassword are validation failures at registration.
	ErrInvalidEmail    = errors.New("email inválido")
	ErrWeakPassword    = errors.New("a senha precisa de ao menos 8 caracteres")
	ErrNomeObrigatorio = errors.New("nome é obrigatório")
)

// minPasswordLen is the floor enforced at registration; the hasher does not care.
const minPasswordLen = 8

var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// User is a registered account. PasswordHash is the encoded argon2id string.
type User struct {
	ID           uuid.UUID
	Email        string
	Nome         string
	PasswordHash string
	CriadoEm     time.Time
}

// NormalizeEmail lowercases and trims so lookups and uniqueness are stable.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateRegistration checks the raw inputs before any hashing happens.
func ValidateRegistration(email, nome, senha string) error {
	if !emailRegex.MatchString(NormalizeEmail(email)) {
		return ErrInvalidEmail
	}

	if strings.TrimSpace(nome) == "" {
		return ErrNomeObrigatorio
	}

	if len(senha) < minPasswordLen {
		return ErrWeakPassword
	}

	return nil
}
