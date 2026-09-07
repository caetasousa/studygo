package httpapi

import (
	"log/slog"
	"net/http"

	"studygo/internal/platform/middleware"
	"studygo/internal/port"
)

// Handlers reúne os handlers que o router liga.
type Handlers struct {
	Health   *HealthHandler
	Auth     *AuthHandler
	Concurso *ConcursoHandler
	Plano    *PlanoHandler
}

// Limites reúne os limitadores das rotas que merecem um teto próprio.
//
// Duas classes, por motivos diferentes: a de autenticação porque um endpoint
// que compara senha é onde a força bruta bate, e a de edital porque cada
// chamada custa OCR, CPU e uma ida paga ao provedor de IA. O resto da API é
// barato e fica com o teto global aplicado por quem monta a cadeia.
type Limites struct {
	Auth   *middleware.Limitador
	Edital *middleware.Limitador
}

// LimitesPadrao é a política que sobe em produção.
//
// Auth: 20 por minuto por IP, rajada de 10 — um humano errando a senha nunca
// chega perto; um script chega na primeira volta.
//
// Edital: 12 por HORA por usuário, rajada de 4. O assistente tem três passos,
// então quatro tentativas seguidas é um wizard inteiro com uma repetição; doze
// por hora é um dia de trabalho legítimo e um teto de custo previsível.
func LimitesPadrao(logger *slog.Logger) Limites {
	return Limites{
		Auth:   middleware.NovoLimitador(20, 10, logger),
		Edital: middleware.NovoLimitador(12*60, 4, logger, middleware.ComChave(chaveDoUsuario)),
	}
}

// chaveDoUsuario limita por conta, não por IP. Vale para as rotas que já
// passaram por Autenticar: dois estudantes atrás do mesmo IP (uma faculdade, um
// celular em NAT) não devem gastar a cota um do outro. Sem id no contexto o
// limite cai no IP, que é o comportamento seguro.
func chaveDoUsuario(r *http.Request) string {
	if id, ok := usuarioID(r.Context()); ok {
		return id.String()
	}

	return middleware.ClienteIP(r)
}

// NewRouter monta o mux da API. As rotas protegidas são embrulhadas
// individualmente com Autenticar; o middleware transversal (recover, log, CORS)
// é aplicado por quem chama.
func NewRouter(
	h Handlers,
	tokens port.TokenIssuer,
	contas ContaExiste,
	limites Limites,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()
	exigirAuth := Autenticar(tokens, contas, logger)

	protegida := func(padrao string, fn http.HandlerFunc) {
		mux.Handle(padrao, exigirAuth(fn))
	}

	// A ORDEM importa: autenticar primeiro, limitar depois. Invertido, o
	// limitador contaria por IP mesmo em rota autenticada, porque o id do
	// usuário ainda não estaria no contexto.
	limitada := func(padrao string, lim *middleware.Limitador, fn http.HandlerFunc) {
		mux.Handle(padrao, exigirAuth(lim.Middleware(fn)))
	}

	// Rota pública com teto: aqui o limitador vem primeiro porque não há
	// autenticação para esperar — e é justamente por isso que ela precisa dele.
	publicaLimitada := func(padrao string, fn http.HandlerFunc) {
		mux.Handle(padrao, limites.Auth.Middleware(fn))
	}

	mux.Handle("GET /health", h.Health)

	publicaLimitada("POST /api/auth/register", h.Auth.Cadastrar)
	publicaLimitada("POST /api/auth/login", h.Auth.Entrar)
	publicaLimitada("POST /api/auth/refresh", h.Auth.Renovar)
	publicaLimitada("POST /api/auth/logout", h.Auth.Sair)

	protegida("GET /api/me", h.Auth.Eu)
	protegida("PUT /api/me/tema", h.Auth.DefinirTema)

	protegida("GET /api/concursos", h.Concurso.List)
	protegida("POST /api/concursos", h.Concurso.Criar)
	limitada("POST /api/editais/analisar", limites.Edital, h.Concurso.AnalisarEdital)
	limitada("POST /api/editais/estrutura", limites.Edital, h.Concurso.EstruturaEdital)
	limitada("POST /api/editais/conteudo", limites.Edital, h.Concurso.ConteudoEdital)
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
	protegida("PATCH "+base+"/disciplinas/{codigo}/links", h.Plano.AtualizarLinksDisciplina)

	protegida("POST "+base+"/atividades/mover", h.Plano.Mover)
	protegida("POST "+base+"/atividades/antecipar", h.Plano.Antecipar)
	protegida("POST "+base+"/dias/{data}/adiar", h.Plano.AdiarDia)
	protegida("POST "+base+"/dias/{data}/reorganizar", h.Plano.Reorganizar)
	protegida("POST "+base+"/compactar", h.Plano.Compactar)
	protegida("POST "+base+"/restaurar-ordem", h.Plano.RestaurarOrdem)

	protegida("GET "+base+"/estatisticas", h.Plano.Estatisticas)

	protegida("GET "+base+"/caderno", h.Plano.Caderno)
	protegida("POST "+base+"/anotacoes", h.Plano.CriarAnotacao)
	protegida("PATCH "+base+"/anotacoes/{id}", h.Plano.AtualizarAnotacao)
	protegida("DELETE "+base+"/anotacoes/{id}", h.Plano.RemoverAnotacao)

	protegida("GET "+base+"/dossie", h.Plano.Dossie)
	protegida("GET "+base+"/export.csv", h.Plano.ExportarCSV)
	protegida("POST "+base+"/importar.csv", h.Plano.ImportarCSV)

	protegida("POST "+base+"/tec/preview", h.Plano.PreviewTEC)
	protegida("POST "+base+"/tec", h.Plano.ImportarTEC)

	return mux
}
