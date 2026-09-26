package httpapi

import (
	"io"
	"log/slog"
	"testing"
)

// O ServeMux do Go entra em pânico ao registrar dois padrões que casam os
// mesmos caminhos sem um ser mais específico ("GET /api/leis/capturas/{id}" ×
// "GET /api/leis/{slug}/exclusao"), e o backend nem sobe. Só montar o
// roteador já pega isso — antes, só o E2E pegava.
func TestRouter_RegistraAsRotasSemConflito(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("o roteador não monta: %v", r)
		}
	}()

	NewRouter(Handlers{}, nil, nil, Limites{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
