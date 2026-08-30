package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"annygo/internal/domain/concurso"
	"annygo/internal/domain/plano"
	"annygo/internal/domain/user"
	"annygo/internal/port"
	"annygo/internal/service"
)

// writeJSON serializes v with the given status. Encoding errors are logged, not
// surfaced — the status line is already sent.
func writeJSON(w http.ResponseWriter, logger *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if v == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.Error("encoding response", slog.Any("error", err))
	}
}

// writeError maps a domain/service error to a status code and a user-safe
// message. Unexpected errors are logged with the request id and returned as 500.
func writeError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	status, msg := classify(err)

	switch {
	case errors.Is(err, port.ErrProvedorIndisponivel):
		// Not our bug — the AI provider is overloaded or slow. Worth a line for
		// ops visibility, but a warning, not an error.
		logger.WarnContext(
			r.Context(),
			"edital import: provider unavailable",
			slog.String("path", r.URL.Path),
			slog.Any("error", err),
		)
	case status >= http.StatusInternalServerError:
		logger.ErrorContext(
			r.Context(),
			"request failed",
			slog.String("path", r.URL.Path),
			slog.Any("error", err),
		)
	}

	writeJSON(w, logger, status, map[string]string{"erro": msg})
}

func classify(err error) (int, string) {
	var validacao service.ErrValidacao
	if errors.As(err, &validacao) {
		return http.StatusUnprocessableEntity, validacao.Msg
	}

	switch {
	case errors.Is(err, user.ErrEmailTaken):
		return http.StatusConflict, user.ErrEmailTaken.Error()
	case errors.Is(err, user.ErrInvalidCredentials):
		return http.StatusUnauthorized, user.ErrInvalidCredentials.Error()
	case errors.Is(err, user.ErrInvalidEmail),
		errors.Is(err, user.ErrWeakPassword),
		errors.Is(err, user.ErrNomeObrigatorio):
		return http.StatusUnprocessableEntity, err.Error()
	case errors.Is(err, user.ErrNotFound),
		errors.Is(err, concurso.ErrNotFound),
		errors.Is(err, plano.ErrNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, errBadRequest):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, errUnauthorized):
		return http.StatusUnauthorized, "não autenticado"
	case errors.Is(err, port.ErrProvedorIndisponivel):
		return http.StatusServiceUnavailable,
			"a IA está sobrecarregada agora — tente de novo em alguns minutos ou cadastre o concurso manualmente"
	default:
		return http.StatusInternalServerError, "erro interno"
	}
}

var (
	errBadRequest   = errors.New("requisição inválida")
	errUnauthorized = errors.New("não autenticado")
)

// decode reads a JSON body into v, rejecting unknown fields.
func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(v); err != nil {
		return errBadRequest
	}

	return nil
}
