package httpapi

import (
	"log/slog"
	"net/http"

	"studygo/internal/service"
)

type HealthHandler struct {
	saude  *service.HealthService
	logger *slog.Logger
}

func NewHealthHandler(saude *service.HealthService, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		saude:  saude,
		logger: logger,
	}
}

// saudeResponse diz, além de "estou de pé", o que está de pé. Deploy some fora
// de uma implantação da pipeline, onde não há número a mostrar.
type saudeResponse struct {
	Status string `json:"status"`
	Versao string `json:"versao"`
	Deploy string `json:"deploy,omitempty"`
	Schema int    `json:"schema"`
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	saude, err := h.saude.Check(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "health check falhou", slog.Any("error", err))
		writeJSON(w, h.logger, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
		})

		return
	}

	writeJSON(w, h.logger, http.StatusOK, saudeResponse{
		Status: "ok",
		Versao: saude.Versao,
		Deploy: saude.Deploy,
		Schema: saude.Schema,
	})
}
