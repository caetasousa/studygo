package httpapi

import (
	"log/slog"
	"net/http"

	"annygo/internal/port"
)

// Handlers bundles every HTTP handler the router wires.
type Handlers struct {
	Health   *HealthHandler
	Auth     *AuthHandler
	Concurso *ConcursoHandler
	Plano    *PlanoHandler
}

// NewRouter builds the API mux. Protected routes are individually wrapped with
// Authenticate; cross-cutting middleware (recover, logging, CORS) is applied by
// the caller.
func NewRouter(
	h Handlers,
	tokens port.TokenIssuer,
	presence UserPresence,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()
	requireAuth := Authenticate(tokens, presence, logger)

	guard := func(pattern string, fn http.HandlerFunc) {
		mux.Handle(pattern, requireAuth(fn))
	}

	mux.Handle("GET /health", h.Health)

	mux.HandleFunc("POST /api/auth/register", h.Auth.Register)
	mux.HandleFunc("POST /api/auth/login", h.Auth.Login)
	mux.HandleFunc("POST /api/auth/refresh", h.Auth.Refresh)
	mux.HandleFunc("POST /api/auth/logout", h.Auth.Logout)

	guard("GET /api/me", h.Auth.Me)

	guard("GET /api/concursos", h.Concurso.List)
	guard("POST /api/concursos", h.Concurso.Criar)
	guard("POST /api/editais/analisar", h.Concurso.AnalisarEdital)
	guard("POST /api/editais/estrutura", h.Concurso.EstruturaEdital)
	guard("POST /api/editais/conteudo", h.Concurso.ConteudoEdital)
	guard("GET /api/concursos/{slug}", h.Concurso.Get)
	guard("PUT /api/concursos/{slug}", h.Concurso.Atualizar)
	guard("DELETE /api/concursos/{slug}", h.Concurso.Remover)

	guard("GET /api/concursos/{slug}/plano", h.Plano.Get)
	guard("PUT /api/concursos/{slug}/plano", h.Plano.Salvar)
	guard("DELETE /api/concursos/{slug}/plano/registros", h.Plano.LimparRegistros)
	guard("PATCH /api/concursos/{slug}/plano/registros/{data}", h.Plano.RegistrarDia)
	guard("PUT /api/concursos/{slug}/plano/marcos/{id}", h.Plano.MarcarMarco)
	guard("PATCH /api/concursos/{slug}/plano/revisoes/{id}", h.Plano.RegistrarRevisao)
	guard("POST /api/concursos/{slug}/plano/tec/preview", h.Plano.PreviewTEC)
	guard("POST /api/concursos/{slug}/plano/tec", h.Plano.ImportarTEC)
	guard("POST /api/concursos/{slug}/plano/reordenar", h.Plano.Reordenar)
	guard("POST /api/concursos/{slug}/plano/restaurar-ordem", h.Plano.RestaurarOrdem)
	guard("GET /api/concursos/{slug}/plano/estatisticas", h.Plano.Estatisticas)
	guard("GET /api/concursos/{slug}/plano/caderno", h.Plano.Caderno)
	guard("GET /api/concursos/{slug}/plano/dossie", h.Plano.Dossie)
	guard("POST /api/concursos/{slug}/plano/anotacoes", h.Plano.CriarAnotacao)
	guard("PATCH /api/concursos/{slug}/plano/anotacoes/{id}", h.Plano.AtualizarAnotacao)
	guard("DELETE /api/concursos/{slug}/plano/anotacoes/{id}", h.Plano.RemoverAnotacao)
	guard("GET /api/concursos/{slug}/plano/export.csv", h.Plano.ExportarCSV)

	return mux
}
