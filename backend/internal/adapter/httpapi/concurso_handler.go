package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"studygo/internal/port"
	"studygo/internal/service"
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
			"erro": "importação por IA indisponível — o processador de editais não está configurado; cadastre manualmente",
		})

		return false
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(4 * time.Minute))

	return true
}

// AnalisarEdital — wizard step 1. Accepts JSON {"texto": "..."} or
// multipart/form-data with a "file" field; returns the document handle + cargos.
func (h *ConcursoHandler) AnalisarEdital(w http.ResponseWriter, r *http.Request) {
	if !h.guardaImport(w, r) {
		return
	}

	up, _, err := lerUploadEdital(r)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	if up.Vazia() {
		writeError(w, r, h.logger, errBadRequest)
		return
	}

	resp, err := h.concursos.AnalisarEdital(r.Context(), ownerRefDe(r), up)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

// EstruturaEdital — wizard step 2. Takes {"documentoId", "cargo"}.
func (h *ConcursoHandler) EstruturaEdital(w http.ResponseWriter, r *http.Request) {
	if !h.guardaImport(w, r) {
		return
	}

	var body struct {
		DocumentoID string `json:"documentoId"`
		Cargo       string `json:"cargo"`
	}
	if err := decode(r, &body); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	if strings.TrimSpace(body.DocumentoID) == "" || strings.TrimSpace(body.Cargo) == "" {
		writeError(w, r, h.logger, errBadRequest)
		return
	}

	resp, err := h.concursos.EstruturaDoCargo(r.Context(), ownerRefDe(r), body.DocumentoID, body.Cargo)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

// ConteudoEdital — wizard step 3, and the edit screen's "extract topics". Takes
// {"documentoId", "cargo", "disciplinas"} OR a fresh upload (multipart / text)
// plus "cargo" and "disciplinas".
func (h *ConcursoHandler) ConteudoEdital(w http.ResponseWriter, r *http.Request) {
	if !h.guardaImport(w, r) {
		return
	}

	up, extras, err := lerUploadEdital(r)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	if len(extras.Disciplinas) == 0 {
		writeError(w, r, h.logger, errBadRequest)
		return
	}
	if extras.DocumentoID == "" && up.Vazia() {
		writeError(w, r, h.logger, errBadRequest)
		return
	}

	resp, err := h.concursos.ConteudoDoEdital(
		r.Context(), ownerRefDe(r), extras.DocumentoID, extras.Cargo, extras.Disciplinas, up,
	)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

// editalExtras are the per-step fields that travel next to a fresh upload.
type editalExtras struct {
	DocumentoID string
	Cargo       string
	Disciplinas []string
}

// ownerRefDe returns the opaque per-user handle the processor binds documents
// to. The user id is fine — it never leaves the compose network.
func ownerRefDe(r *http.Request) string {
	if id, ok := userID(r.Context()); ok {
		return id.String()
	}
	return ""
}

// lerUploadEdital accepts JSON {"texto", "documentoId", "cargo", "disciplinas"}
// or multipart/form-data with a "file" plus those fields.
func lerUploadEdital(r *http.Request) (port.EditalUpload, editalExtras, error) {
	var extras editalExtras

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(maxEditalPDF); err != nil {
			return port.EditalUpload{}, extras, errBadRequest
		}

		extras.DocumentoID = r.FormValue("documentoId")
		extras.Cargo = r.FormValue("cargo")
		if ds := r.FormValue("disciplinas"); ds != "" {
			if err := json.Unmarshal([]byte(ds), &extras.Disciplinas); err != nil {
				return port.EditalUpload{}, extras, errBadRequest
			}
		}

		if txt := r.FormValue("texto"); txt != "" {
			return port.EditalUpload{Texto: txt}, extras, nil
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			// no file and no text is allowed when a documentoId carries the work
			return port.EditalUpload{}, extras, nil
		}
		defer file.Close()

		data, err := io.ReadAll(io.LimitReader(file, maxEditalPDF))
		if err != nil {
			return port.EditalUpload{}, extras, errBadRequest
		}

		mime := header.Header.Get("Content-Type")
		if mime == "" {
			mime = "application/pdf"
		}

		return port.EditalUpload{PDF: data, MIME: mime}, extras, nil
	}

	var body struct {
		Texto       string   `json:"texto"`
		DocumentoID string   `json:"documentoId"`
		Cargo       string   `json:"cargo"`
		Disciplinas []string `json:"disciplinas"`
	}
	if err := decode(r, &body); err != nil {
		return port.EditalUpload{}, extras, err
	}

	extras.DocumentoID = body.DocumentoID
	extras.Cargo = body.Cargo
	extras.Disciplinas = body.Disciplinas

	return port.EditalUpload{Texto: body.Texto}, extras, nil
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
