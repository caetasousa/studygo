package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"annygo/internal/port"

	"github.com/google/uuid"
)

type ctxKey int

const userIDKey ctxKey = iota

// Authenticate validates the bearer token and stores the user id in the context.
func Authenticate(tokens port.TokenIssuer, logger *slog.Logger) func(http.Handler) http.Handler {
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
