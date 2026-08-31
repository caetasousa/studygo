package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"studygo/internal/domain/user"
	"studygo/internal/service"
)

// AuthHandler serves registration, login, refresh and the current-user lookup.
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
	ID    string `json:"id"`
	Email string `json:"email"`
	Nome  string `json:"nome"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	u, pair, err := h.auth.Register(r.Context(), req.Email, req.Nome, req.Senha)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusCreated, toAuthResponse(u, pair))
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	u, pair, err := h.auth.Login(r.Context(), req.Email, req.Senha)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, toAuthResponse(u, pair))
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	pair, err := h.auth.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]any{
		"accessToken":     pair.AccessToken,
		"accessExpiresAt": pair.AccessExpiresAt,
		"refreshToken":    pair.RefreshToken,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	if err := h.auth.Logout(r.Context(), req.RefreshToken); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	u, err := h.auth.UserByID(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, toUsuarioResponse(u))
}

func toAuthResponse(u user.User, pair service.TokenPair) authResponse {
	return authResponse{
		Usuario:         toUsuarioResponse(u),
		AccessToken:     pair.AccessToken,
		AccessExpiresAt: pair.AccessExpiresAt,
		RefreshToken:    pair.RefreshToken,
	}
}

func toUsuarioResponse(u user.User) usuarioResponse {
	return usuarioResponse{
		ID:    u.ID.String(),
		Email: u.Email,
		Nome:  u.Nome,
	}
}
