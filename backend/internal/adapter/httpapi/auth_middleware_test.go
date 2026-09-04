package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"studygo/internal/domain/usuario"

	"github.com/google/uuid"
)

type fakeTokens struct {
	id  uuid.UUID
	err error
}

func (f fakeTokens) Emitir(uuid.UUID) (string, time.Time, error) { return "", time.Time{}, nil }
func (f fakeTokens) Ler(string) (uuid.UUID, error)               { return f.id, f.err }

type fakeContas struct {
	err error
}

func (f fakeContas) PorID(context.Context, uuid.UUID) (usuario.Usuario, error) {
	return usuario.Usuario{}, f.err
}

func TestAutenticar(t *testing.T) {
	t.Parallel()

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	uid := uuid.New()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := usuarioID(r.Context()); !ok {
			t.Error("usuarioID ausente no contexto de next")
		}
		w.WriteHeader(http.StatusTeapot)
	})

	tests := []struct {
		name       string
		header     string
		tokens     fakeTokens
		contas     fakeContas
		wantStatus int
	}{
		{
			name:       "ok",
			header:     "Bearer abc",
			tokens:     fakeTokens{id: uid},
			contas:     fakeContas{},
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
			contas:     fakeContas{err: usuario.ErrNaoEncontrado},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "erro de banco na checagem → 500",
			header:     "Bearer abc",
			tokens:     fakeTokens{id: uid},
			contas:     fakeContas{err: errAnalisadorFalhou},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := Autenticar(tt.tokens, tt.contas, quiet)(next)

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
