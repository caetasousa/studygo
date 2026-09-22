package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"studygo/internal/domain/usuario"
	"studygo/internal/service"
)

// AuthHandler serve cadastro, login, renovação de token e a conta atual.
type AuthHandler struct {
	auth   *service.AuthService
	logger *slog.Logger

	// refreshTTL é o mesmo prazo do token no banco, e serve só para dar ao
	// cookie um Max-Age igual: um cookie que durasse mais entregaria um token
	// já morto, e um que durasse menos encurtaria a sessão sem motivo.
	refreshTTL time.Duration
}

func NewAuthHandler(auth *service.AuthService, refreshTTL time.Duration, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, refreshTTL: refreshTTL, logger: logger}
}

type registerRequest struct {
	Email string `json:"email"`
	Nome  string `json:"nome"`
	Senha string `json:"senha"`
}

type loginRequest struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// authResponse é o que cadastro, login e renovação devolvem — os três iguais,
// porque para o cliente são a mesma coisa: uma sessão aberta. O refresh token
// NÃO está aqui; ele sai no cookie (ver cookie_sessao.go).
type authResponse struct {
	Usuario         usuarioResponse `json:"usuario"`
	AccessToken     string          `json:"accessToken"`
	AccessExpiresAt time.Time       `json:"accessExpiresAt"`
}

type usuarioResponse struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Nome   string `json:"nome"`
	TemaUI string `json:"temaUi"`
}

func (h *AuthHandler) Cadastrar(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	u, pair, err := h.auth.Cadastrar(r.Context(), req.Email, req.Nome, req.Senha)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	h.gravarRefresh(w, r, pair.RefreshToken)
	writeJSON(w, h.logger, http.StatusCreated, toAuthResponse(u, pair))
}

func (h *AuthHandler) Entrar(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	u, pair, err := h.auth.Entrar(r.Context(), req.Email, req.Senha)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	h.gravarRefresh(w, r, pair.RefreshToken)
	writeJSON(w, h.logger, http.StatusOK, toAuthResponse(u, pair))
}

// Renovar troca o cookie por uma sessão nova. É também o que restaura a sessão
// quando o app abre: o cliente não guarda mais token nenhum entre cargas, então
// esta rota devolve a conta junto — sem ela, toda abertura custaria um /api/me
// a mais só para saber quem é quem.
func (h *AuthHandler) Renovar(w http.ResponseWriter, r *http.Request) {
	token := refreshDaRequisicao(r)
	if token == "" {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	u, pair, err := h.auth.Renovar(r.Context(), token)
	if err != nil {
		// Sessão recusada deixa de existir no navegador também: sem isto o
		// cookie morto voltaria a cada tentativa, e o app tentaria renovar para
		// sempre com um token que o banco já não conhece.
		apagarRefresh(w, r)
		writeError(w, r, h.logger, err)

		return
	}

	h.gravarRefresh(w, r, pair.RefreshToken)
	writeJSON(w, h.logger, http.StatusOK, toAuthResponse(u, pair))
}

// Sair é idempotente: apaga o cookie sempre, e revoga no banco o que houver.
// Quem chega sem cookie já está fora — responder erro só atrapalharia a tela,
// que chama esta rota justamente para garantir que não sobrou sessão.
func (h *AuthHandler) Sair(w http.ResponseWriter, r *http.Request) {
	token := refreshDaRequisicao(r)
	apagarRefresh(w, r)

	if token != "" {
		if err := h.auth.Sair(r.Context(), token); err != nil {
			writeError(w, r, h.logger, err)

			return
		}
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}

func (h *AuthHandler) Eu(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	u, err := h.auth.PorID(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, toUsuarioResponse(u))
}

func toAuthResponse(u usuario.Usuario, pair service.ParDeTokens) authResponse {
	return authResponse{
		Usuario:         toUsuarioResponse(u),
		AccessToken:     pair.AccessToken,
		AccessExpiresAt: pair.AccessExpiraEm,
	}
}

func toUsuarioResponse(u usuario.Usuario) usuarioResponse {
	return usuarioResponse{
		ID:     u.ID.String(),
		Email:  u.Email,
		Nome:   u.Nome,
		TemaUI: string(u.TemaUI),
	}
}

type temaRequest struct {
	TemaUI string `json:"temaUi"`
}

// DefinirTema grava a preferência visual da conta. Ela é do USUÁRIO, não do
// plano: quem estuda para dois concursos não quer dois temas.
func (h *AuthHandler) DefinirTema(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	var req temaRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	if err := h.auth.DefinirTema(r.Context(), id, req.TemaUI); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	u, err := h.auth.PorID(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	writeJSON(w, h.logger, http.StatusOK, toUsuarioResponse(u))
}
