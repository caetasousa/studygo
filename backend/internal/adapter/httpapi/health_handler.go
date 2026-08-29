package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"annygo/internal/port"
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
	w.Header().Set("Content-Type", "application/json")

	if err := h.checker.Check(r.Context()); err != nil {
		h.logger.ErrorContext(r.Context(), "health check failed", slog.Any("error", err))
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "unavailable"})

		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
