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
}

func NewAuthHandler(auth *service.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, logger: logger}
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

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type authResponse struct {
	Usuario         usuarioResponse `json:"usuario"`
	AccessToken     string          `json:"accessToken"`
	AccessExpiresAt time.Time       `json:"accessExpiresAt"`
	RefreshToken    string          `json:"refreshToken"`
}

type usuarioResponse struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Nome   string `json:"nome"`
	TemaUI string `json:"temaUi"`
}

func (h *AuthHandler) Cadastrar(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	u, pair, err := h.auth.Cadastrar(r.Context(), req.Email, req.Nome, req.Senha)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusCreated, toAuthResponse(u, pair))
}

func (h *AuthHandler) Entrar(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	u, pair, err := h.auth.Entrar(r.Context(), req.Email, req.Senha)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, toAuthResponse(u, pair))
}

func (h *AuthHandler) Renovar(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	pair, err := h.auth.Renovar(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]any{
		"accessToken":     pair.AccessToken,
		"accessExpiresAt": pair.AccessExpiraEm,
		"refreshToken":    pair.RefreshToken,
	})
}

func (h *AuthHandler) Sair(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	if err := h.auth.Sair(r.Context(), req.RefreshToken); err != nil {
		writeError(w, r, h.logger, err)
		return
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
		RefreshToken:    pair.RefreshToken,
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
	if err := decode(r, &req); err != nil {
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
