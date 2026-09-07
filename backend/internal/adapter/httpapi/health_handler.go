package httpapi

import (
	"log/slog"
	"net/http"

	"studygo/internal/port"
)

type HealthHandler struct {
	checker port.HealthChecker
	logger  *slog.Logger
}

func NewHealthHandler(checker port.HealthChecker, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		checker: checker,
		logger:  logger,
	}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.checker.Check(r.Context()); err != nil {
		h.logger.ErrorContext(r.Context(), "health check falhou", slog.Any("error", err))
		writeJSON(w, h.logger, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
		})

		return
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]string{"status": "ok"})
}
