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

// ConcursoHandler serve o cadastro dos concursos do usuário e o assistente de
// importação de edital.
type ConcursoHandler struct {
	concursos *service.ConcursoService
	logger    *slog.Logger
}

func NewConcursoHandler(concursos *service.ConcursoService, logger *slog.Logger) *ConcursoHandler {
	return &ConcursoHandler{concursos: concursos, logger: logger}
}

func (h *ConcursoHandler) List(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	items, err := h.concursos.Listar(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resumos := make([]concursoResumoDTO, 0, len(items))
	for _, it := range items {
		resumos = append(resumos, resumoParaDTO(it))
	}

	writeJSON(w, h.logger, http.StatusOK, listaConcursosDTO{
		Concursos:        resumos,
		ImportacaoEdital: h.concursos.ImportacaoDisponivel(),
	})
}

// guardaImport is the common preamble for the wizard endpoints: auth check,
// availability check, and a longer write deadline for the LLM round-trip.
func (h *ConcursoHandler) guardaImport(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := usuarioID(r.Context()); !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
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
		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}

	resp, err := h.concursos.AnalisarEdital(r.Context(), ownerRefDe(r), up)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, analiseParaDTO(resp))
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
		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}

	resp, err := h.concursos.EstruturaDoCargo(
		r.Context(), ownerRefDe(r), body.DocumentoID, body.Cargo,
	)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, estruturaParaDTO(resp))
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
		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}
	if extras.DocumentoID == "" && up.Vazia() {
		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}

	resp, err := h.concursos.ConteudoDoEdital(
		r.Context(), ownerRefDe(r), extras.DocumentoID, extras.Cargo, extras.Disciplinas, up,
	)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, conteudoEditalParaDTO(resp))
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
	if id, ok := usuarioID(r.Context()); ok {
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
			return port.EditalUpload{}, extras, errRequisicaoInvalida
		}

		extras.DocumentoID = r.FormValue("documentoId")
		extras.Cargo = r.FormValue("cargo")
		if ds := r.FormValue("disciplinas"); ds != "" {
			if err := json.Unmarshal([]byte(ds), &extras.Disciplinas); err != nil {
				return port.EditalUpload{}, extras, errRequisicaoInvalida
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
			return port.EditalUpload{}, extras, errRequisicaoInvalida
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
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	detalhe, err := h.concursos.Detalhe(r.Context(), id, r.PathValue("slug"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, concursoDetalheDTO{
		Slug:   detalhe.Slug,
		Dados:  comandoParaConcurso(detalhe.Dados),
		Avisos: detalhe.Avisos,
	})
}

func (h *ConcursoHandler) Criar(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	var req concursoRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resumo, avisos, err := h.concursos.Criar(r.Context(), id, concursoParaComando(req))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusCreated, concursoDetalheDTO{
		Slug:   resumo.Slug,
		Dados:  req,
		Avisos: avisos,
	})
}

func (h *ConcursoHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	var req concursoRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resumo, avisos, err := h.concursos.Atualizar(
		r.Context(), id, r.PathValue("slug"), concursoParaComando(req),
	)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, concursoDetalheDTO{
		Slug:   resumo.Slug,
		Dados:  req,
		Avisos: avisos,
	})
}

func (h *ConcursoHandler) Remover(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	if err := h.concursos.Remover(r.Context(), id, r.PathValue("slug")); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}
