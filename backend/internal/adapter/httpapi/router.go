package httpapi

import (
	"log/slog"
	"net/http"

	"studygo/internal/port"
)

// Handlers reúne os handlers que o router liga.
type Handlers struct {
	Health   *HealthHandler
	Auth     *AuthHandler
	Concurso *ConcursoHandler
	Plano    *PlanoHandler
}

// NewRouter monta o mux da API. As rotas protegidas são embrulhadas
// individualmente com Autenticar; o middleware transversal (recover, log, CORS)
// é aplicado por quem chama.
func NewRouter(
	h Handlers,
	tokens port.TokenIssuer,
	contas ContaExiste,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()
	exigirAuth := Autenticar(tokens, contas, logger)

	protegida := func(padrao string, fn http.HandlerFunc) {
		mux.Handle(padrao, exigirAuth(fn))
	}

	mux.Handle("GET /health", h.Health)

	mux.HandleFunc("POST /api/auth/register", h.Auth.Cadastrar)
	mux.HandleFunc("POST /api/auth/login", h.Auth.Entrar)
	mux.HandleFunc("POST /api/auth/refresh", h.Auth.Renovar)
	mux.HandleFunc("POST /api/auth/logout", h.Auth.Sair)

	protegida("GET /api/me", h.Auth.Eu)
	protegida("PUT /api/me/tema", h.Auth.DefinirTema)

	protegida("GET /api/concursos", h.Concurso.List)
	protegida("POST /api/concursos", h.Concurso.Criar)
	protegida("POST /api/editais/analisar", h.Concurso.AnalisarEdital)
	protegida("POST /api/editais/estrutura", h.Concurso.EstruturaEdital)
	protegida("POST /api/editais/conteudo", h.Concurso.ConteudoEdital)
	protegida("GET /api/concursos/{slug}", h.Concurso.Get)
	protegida("PUT /api/concursos/{slug}", h.Concurso.Atualizar)
	protegida("DELETE /api/concursos/{slug}", h.Concurso.Remover)

	const base = "/api/concursos/{slug}/plano"

	protegida("GET "+base, h.Plano.Obter)
	protegida("PUT "+base, h.Plano.Salvar)

	// O registro é por ATIVIDADE: é a unidade de trabalho, e é dela que a
	// conclusão do dia é derivada.
	protegida("PUT "+base+"/atividades/{id}/registro", h.Plano.Registrar)
	protegida("PATCH "+base+"/dias/{data}", h.Plano.RegistrarDia)
	protegida("DELETE "+base+"/registros", h.Plano.LimparRegistros)

	protegida("PUT "+base+"/marcos/{id}", h.Plano.MarcarMarco)
	protegida("PATCH "+base+"/disciplinas/{codigo}/caderno", h.Plano.AtualizarCadernoDisciplina)

	protegida("POST "+base+"/atividades/mover", h.Plano.Mover)
	protegida("POST "+base+"/atividades/antecipar", h.Plano.Antecipar)
	protegida("POST "+base+"/dias/{data}/adiar", h.Plano.AdiarDia)
	protegida("POST "+base+"/compactar", h.Plano.Compactar)
	protegida("POST "+base+"/restaurar-ordem", h.Plano.RestaurarOrdem)

	protegida("GET "+base+"/estatisticas", h.Plano.Estatisticas)

	protegida("GET "+base+"/caderno", h.Plano.Caderno)
	protegida("POST "+base+"/anotacoes", h.Plano.CriarAnotacao)
	protegida("PATCH "+base+"/anotacoes/{id}", h.Plano.AtualizarAnotacao)
	protegida("DELETE "+base+"/anotacoes/{id}", h.Plano.RemoverAnotacao)

	protegida("GET "+base+"/dossie", h.Plano.Dossie)
	protegida("GET "+base+"/export.csv", h.Plano.ExportarCSV)

	protegida("POST "+base+"/tec/preview", h.Plano.PreviewTEC)
	protegida("POST "+base+"/tec", h.Plano.ImportarTEC)

	return mux
}
