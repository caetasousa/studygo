package httpapi

import (
	"log/slog"
	"net/http"

	"studygo/internal/port"
	"studygo/internal/service"
)

// ProcessadorHandler serve o token do Claude que a conta guarda para o
// processador de mapas: a tela grava e vê só o fim; o backend o entrega ao
// processador junto com cada PDF da conta.
type ProcessadorHandler struct {
	tokens   *service.TokenDoClaudeService
	conexoes *service.ConexaoDoClaudeService
	logger   *slog.Logger
}

func NewProcessadorHandler(
	tokens *service.TokenDoClaudeService, conexoes *service.ConexaoDoClaudeService, logger *slog.Logger,
) *ProcessadorHandler {
	return &ProcessadorHandler{tokens: tokens, conexoes: conexoes, logger: logger}
}

// situacaoDoTokenDTO nunca traz o token, só o fim dele.
type situacaoDoTokenDTO struct {
	Configurado bool   `json:"configurado"`
	Fim         string `json:"fim"`
}

type tokenDoClaudeDTO struct {
	Token string `json:"token"`
}

func (h *ProcessadorHandler) Situacao(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	s, err := h.tokens.Situacao(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, situacaoDoTokenDTO{Configurado: s.Configurado, Fim: s.Fim})
}

func (h *ProcessadorHandler) Guardar(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	var req tokenDoClaudeDTO
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	if err := h.tokens.Guardar(r.Context(), id, req.Token); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}

func (h *ProcessadorHandler) Remover(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	if err := h.tokens.Remover(r.Context(), id); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}

// --- a conexão do Claude, pela tela ---------------------------------------------

type conexaoDoClaudeDTO struct {
	Conectado bool   `json:"conectado"`
	Email     string `json:"email"`
	Plano     string `json:"plano"`
}

type linkDoClaudeDTO struct {
	URL string `json:"url"`
}

type codigoDoClaudeDTO struct {
	Codigo string `json:"codigo"`
}

func conexaoParaDTO(c port.ConexaoDoClaude) conexaoDoClaudeDTO {
	return conexaoDoClaudeDTO{Conectado: c.Conectado, Email: c.Email, Plano: c.Plano}
}

func (h *ProcessadorHandler) Conexao(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	c, err := h.conexoes.Situacao(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, conexaoParaDTO(c))
}

func (h *ProcessadorHandler) Conectar(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	url, err := h.conexoes.Conectar(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, linkDoClaudeDTO{URL: url})
}

func (h *ProcessadorHandler) ConcluirConexao(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	var req codigoDoClaudeDTO
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	c, err := h.conexoes.Concluir(r.Context(), id, req.Codigo)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, conexaoParaDTO(c))
}

func (h *ProcessadorHandler) Desconectar(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	if err := h.conexoes.Desconectar(r.Context(), id); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}
