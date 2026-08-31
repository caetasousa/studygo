package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"studygo/internal/domain/user"
	"studygo/internal/port"

	"github.com/google/uuid"
)

type ctxKey int

const userIDKey ctxKey = iota

// UserPresence reports whether an account still exists — a JWT can outlive the
// user it names (e.g. after the account is deleted or the database is reset).
type UserPresence interface {
	UserByID(ctx context.Context, id uuid.UUID) (user.User, error)
}

// Authenticate validates the bearer token, confirms the account still exists,
// and stores the user id in the context.
func Authenticate(
	tokens port.TokenIssuer,
	presence UserPresence,
	logger *slog.Logger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")

			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				writeError(w, r, logger, errUnauthorized)
				return
			}

			id, err := tokens.Parse(token)
			if err != nil {
				writeError(w, r, logger, errUnauthorized)
				return
			}

			if _, err := presence.UserByID(r.Context(), id); err != nil {
				if errors.Is(err, user.ErrNotFound) {
					writeError(w, r, logger, errUnauthorized)
					return
				}

				writeError(w, r, logger, err)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, id)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// userID pulls the authenticated user id set by Authenticate.
func userID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)

	return id, ok
}
