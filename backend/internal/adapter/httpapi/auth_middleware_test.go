package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"studygo/internal/domain/user"

	"github.com/google/uuid"
)

type fakeTokens struct {
	id  uuid.UUID
	err error
}

func (f fakeTokens) Issue(uuid.UUID) (string, time.Time, error) { return "", time.Time{}, nil }
func (f fakeTokens) Parse(string) (uuid.UUID, error)            { return f.id, f.err }

type fakePresence struct {
	err error
}

func (f fakePresence) UserByID(context.Context, uuid.UUID) (user.User, error) {
	return user.User{}, f.err
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	uid := uuid.New()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := userID(r.Context()); !ok {
			t.Error("userID ausente no contexto de next")
		}
		w.WriteHeader(http.StatusTeapot)
	})

	tests := []struct {
		name       string
		header     string
		tokens     fakeTokens
		presence   fakePresence
		wantStatus int
	}{
		{
			name:       "ok",
			header:     "Bearer abc",
			tokens:     fakeTokens{id: uid},
			presence:   fakePresence{},
			wantStatus: http.StatusTeapot,
		},
		{
			name:       "sem header",
			header:     "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "token inválido",
			header:     "Bearer abc",
			tokens:     fakeTokens{err: errAnalisadorFalhou},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "usuário não existe mais (JWT válido, conta apagada)",
			header:     "Bearer abc",
			tokens:     fakeTokens{id: uid},
			presence:   fakePresence{err: user.ErrNotFound},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "erro de banco na checagem → 500",
			header:     "Bearer abc",
			tokens:     fakeTokens{id: uid},
			presence:   fakePresence{err: errAnalisadorFalhou},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := Authenticate(tt.tokens, tt.presence, quiet)(next)

			req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, quero %d (body %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}
