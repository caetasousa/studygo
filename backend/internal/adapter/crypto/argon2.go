// Package crypto adapts password hashing (argon2id) and access tokens (JWT) to
// the auth ports. It has no knowledge of HTTP or the database.
package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"studygo/internal/platform/config"
	"studygo/internal/port"

	"golang.org/x/crypto/argon2"
)

var _ port.PasswordHasher = (*Argon2Hasher)(nil)

// ErrIncompatibleHash is returned when an encoded hash cannot be parsed.
var ErrIncompatibleHash = errors.New("hash de senha em formato incompatível")

// Argon2Hasher implements port.PasswordHasher with argon2id and the standard
// PHC string encoding.
type Argon2Hasher struct {
	params config.Argon2Params
}

func NewArgon2Hasher(params config.Argon2Params) *Argon2Hasher {
	return &Argon2Hasher{params: params}
}

func (h *Argon2Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	key := argon2.IDKey(
		[]byte(password),
		salt,
		h.params.Iterations,
		h.params.Memory,
		h.params.Parallelism,
		h.params.KeyLength,
	)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.params.Memory,
		h.params.Iterations,
		h.params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)

	return encoded, nil
}

func (h *Argon2Hasher) Verify(password, encodedHash string) (bool, error) {
	params, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	other := argon2.IDKey(
		[]byte(password),
		salt,
		params.Iterations,
		params.Memory,
		params.Parallelism,
		uint32(len(hash)),
	)

	return subtle.ConstantTimeCompare(hash, other) == 1, nil
}

func decodeHash(encoded string) (config.Argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return config.Argon2Params{}, nil, nil, ErrIncompatibleHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return config.Argon2Params{}, nil, nil, ErrIncompatibleHash
	}

	if version != argon2.Version {
		return config.Argon2Params{}, nil, nil, ErrIncompatibleHash
	}

	var params config.Argon2Params
	if _, err := fmt.Sscanf(
		parts[3],
		"m=%d,t=%d,p=%d",
		&params.Memory,
		&params.Iterations,
		&params.Parallelism,
	); err != nil {
		return config.Argon2Params{}, nil, nil, ErrIncompatibleHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return config.Argon2Params{}, nil, nil, ErrIncompatibleHash
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return config.Argon2Params{}, nil, nil, ErrIncompatibleHash
	}

	return params, salt, hash, nil
}
