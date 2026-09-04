package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"studygo/internal/domain/usuario"
	"studygo/internal/port"

	"github.com/google/uuid"
)

type ctxKey int

const usuarioIDKey ctxKey = iota

// ContaExiste diz se uma conta ainda existe — um JWT pode sobreviver ao usuário
// que ele nomeia (conta apagada, banco recriado).
type ContaExiste interface {
	PorID(ctx context.Context, id uuid.UUID) (usuario.Usuario, error)
}

// Autenticar valida o bearer token, confirma que a conta ainda existe e guarda
// o id do usuário no contexto.
func Autenticar(
	tokens port.TokenIssuer,
	contas ContaExiste,
	logger *slog.Logger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")

			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				writeError(w, r, logger, errNaoAutenticado)
				return
			}

			id, err := tokens.Ler(token)
			if err != nil {
				writeError(w, r, logger, errNaoAutenticado)
				return
			}

			if _, err := contas.PorID(r.Context(), id); err != nil {
				if errors.Is(err, usuario.ErrNaoEncontrado) {
					writeError(w, r, logger, errNaoAutenticado)
					return
				}

				writeError(w, r, logger, err)
				return
			}

			ctx := context.WithValue(r.Context(), usuarioIDKey, id)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// usuarioID devolve o id autenticado que Autenticar guardou no contexto.
func usuarioID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(usuarioIDKey).(uuid.UUID)

	return id, ok
}
