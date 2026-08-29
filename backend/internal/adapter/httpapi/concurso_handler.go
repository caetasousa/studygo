package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"annygo/internal/port"
	"annygo/internal/service"
)

const maxEditalPDF = 20 << 20 // 20 MiB

// ConcursoHandler serves the CRUD for a user's registered concursos.
type ConcursoHandler struct {
	concursos *service.ConcursoService
	logger    *slog.Logger
}

func NewConcursoHandler(concursos *service.ConcursoService, logger *slog.Logger) *ConcursoHandler {
	return &ConcursoHandler{concursos: concursos, logger: logger}
}

func (h *ConcursoHandler) List(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	items, err := h.concursos.Listar(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]any{
		"concursos":        items,
		"importacaoEdital": h.concursos.ImportacaoDisponivel(),
	})
}

// ImportarEdital accepts either JSON {"texto": "..."} or multipart/form-data
// with a "pdf" file, and returns the prefilled create form. Nothing is saved.
func (h *ConcursoHandler) ImportarEdital(w http.ResponseWriter, r *http.Request) {
	if _, ok := userID(r.Context()); !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	if !h.concursos.ImportacaoDisponivel() {
		writeJSON(w, h.logger, http.StatusServiceUnavailable, map[string]string{
			"erro": "importação por IA indisponível — configure GEMINI_API_KEY ou cadastre manualmente",
		})

		return
	}

	// The LLM round-trip can take well over the server's default write timeout.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(3 * time.Minute))

	entrada, err := lerEntradaEdital(r)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resp, err := h.concursos.ImportarEdital(r.Context(), entrada)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

func lerEntradaEdital(r *http.Request) (port.EditalEntrada, error) {
	ct := r.Header.Get("Content-Type")

	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxEditalPDF); err != nil {
			return port.EditalEntrada{}, errBadRequest
		}

		file, header, err := r.FormFile("pdf")
		if err != nil {
			return port.EditalEntrada{}, errBadRequest
		}
		defer file.Close()

		data, err := io.ReadAll(io.LimitReader(file, maxEditalPDF))
		if err != nil {
			return port.EditalEntrada{}, errBadRequest
		}

		mime := header.Header.Get("Content-Type")
		if mime == "" {
			mime = "application/pdf"
		}

		return port.EditalEntrada{PDF: data, MIME: mime}, nil
	}

	var body struct {
		Texto string `json:"texto"`
	}

	if err := decode(r, &body); err != nil {
		return port.EditalEntrada{}, err
	}

	if body.Texto == "" {
		return port.EditalEntrada{}, errBadRequest
	}

	return port.EditalEntrada{Texto: body.Texto}, nil
}

func (h *ConcursoHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	detalhe, err := h.concursos.Detalhe(r.Context(), id, r.PathValue("slug"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, detalhe)
}

func (h *ConcursoHandler) Criar(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	var in service.ConcursoInput
	if err := decode(r, &in); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resumo, err := h.concursos.Criar(r.Context(), id, in)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusCreated, resumo)
}

func (h *ConcursoHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	var in service.ConcursoInput
	if err := decode(r, &in); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resumo, err := h.concursos.Atualizar(r.Context(), id, r.PathValue("slug"), in)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resumo)
}

func (h *ConcursoHandler) Remover(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	if err := h.concursos.Remover(r.Context(), id, r.PathValue("slug")); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}
