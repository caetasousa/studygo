package httpapi

import (
	"encoding/json"
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

// guardaImport is the common preamble for the wizard endpoints: auth check,
// availability check, and a longer write deadline for the LLM round-trip.
func (h *ConcursoHandler) guardaImport(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := userID(r.Context()); !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return false
	}

	if !h.concursos.ImportacaoDisponivel() {
		writeJSON(w, h.logger, http.StatusServiceUnavailable, map[string]string{
			"erro": "importação por IA indisponível — configure GEMINI_API_KEY ou cadastre manualmente",
		})

		return false
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(4 * time.Minute))

	return true
}

// AnalisarEdital — wizard step 1. Accepts JSON {"texto": "..."} or
// multipart/form-data with a "pdf" file; returns the cargos + normalized text.
func (h *ConcursoHandler) AnalisarEdital(w http.ResponseWriter, r *http.Request) {
	if !h.guardaImport(w, r) {
		return
	}

	entrada, err := lerEntradaEdital(r)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resp, err := h.concursos.AnalisarEdital(r.Context(), entrada)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

// EstruturaEdital — wizard step 2. Takes the edital (text or PDF, same shapes as
// step 1) plus the chosen "cargo".
func (h *ConcursoHandler) EstruturaEdital(w http.ResponseWriter, r *http.Request) {
	if !h.guardaImport(w, r) {
		return
	}

	entrada, extras, err := lerEntradaComExtras(r)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	cargo := strings.TrimSpace(extras.Cargo)
	if entrada.Vazia() || cargo == "" {
		writeError(w, r, h.logger, errBadRequest)
		return
	}

	resp, err := h.concursos.EstruturaDoCargo(r.Context(), entrada, cargo)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

// ConteudoEdital — wizard step 3. Takes the edital plus the discipline list.
func (h *ConcursoHandler) ConteudoEdital(w http.ResponseWriter, r *http.Request) {
	if !h.guardaImport(w, r) {
		return
	}

	entrada, extras, err := lerEntradaComExtras(r)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	if entrada.Vazia() || len(extras.Disciplinas) == 0 {
		writeError(w, r, h.logger, errBadRequest)
		return
	}

	resp, err := h.concursos.ConteudoDoEdital(r.Context(), entrada, extras.Disciplinas)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

// editalExtras are the per-step fields that travel next to the edital.
type editalExtras struct {
	Cargo       string   `json:"cargo"`
	Disciplinas []string `json:"disciplinas"`
}

func lerEntradaEdital(r *http.Request) (port.EditalEntrada, error) {
	in, _, err := lerEntradaComExtras(r)

	return in, err
}

// lerEntradaComExtras accepts either JSON {"texto", "cargo", "disciplinas"} or
// multipart/form-data with a "pdf" file plus "cargo"/"disciplinas" fields.
func lerEntradaComExtras(r *http.Request) (port.EditalEntrada, editalExtras, error) {
	var extras editalExtras

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(maxEditalPDF); err != nil {
			return port.EditalEntrada{}, extras, errBadRequest
		}

		extras.Cargo = r.FormValue("cargo")
		if ds := r.FormValue("disciplinas"); ds != "" {
			if err := json.Unmarshal([]byte(ds), &extras.Disciplinas); err != nil {
				return port.EditalEntrada{}, extras, errBadRequest
			}
		}

		file, header, err := r.FormFile("pdf")
		if err != nil {
			return port.EditalEntrada{}, extras, errBadRequest
		}
		defer file.Close()

		data, err := io.ReadAll(io.LimitReader(file, maxEditalPDF))
		if err != nil {
			return port.EditalEntrada{}, extras, errBadRequest
		}

		mime := header.Header.Get("Content-Type")
		if mime == "" {
			mime = "application/pdf"
		}

		return port.EditalEntrada{PDF: data, MIME: mime}, extras, nil
	}

	var body struct {
		Texto       string   `json:"texto"`
		ArquivoURI  string   `json:"arquivoUri"`
		MIME        string   `json:"mime"`
		Cargo       string   `json:"cargo"`
		Disciplinas []string `json:"disciplinas"`
	}

	if err := decode(r, &body); err != nil {
		return port.EditalEntrada{}, extras, err
	}

	if body.Texto == "" && body.ArquivoURI == "" {
		return port.EditalEntrada{}, extras, errBadRequest
	}

	extras.Cargo = body.Cargo
	extras.Disciplinas = body.Disciplinas

	return port.EditalEntrada{
		Texto:      body.Texto,
		ArquivoURI: body.ArquivoURI,
		MIME:       body.MIME,
	}, extras, nil
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
