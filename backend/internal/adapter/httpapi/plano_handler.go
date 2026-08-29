package httpapi

import (
	"log/slog"
	"net/http"
	"strings"

	"annygo/internal/service"

	"github.com/google/uuid"
)

// PlanoHandler serves every endpoint under /api/concursos/{slug}/plano.
type PlanoHandler struct {
	planos *service.PlanoService
	logger *slog.Logger
}

func NewPlanoHandler(planos *service.PlanoService, logger *slog.Logger) *PlanoHandler {
	return &PlanoHandler{planos: planos, logger: logger}
}

func (h *PlanoHandler) ctx(r *http.Request) (uuid.UUID, string, bool) {
	id, ok := userID(r.Context())
	if !ok {
		return uuid.Nil, "", false
	}

	return id, r.PathValue("slug"), true
}

func (h *PlanoHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	resp, err := h.planos.Obter(r.Context(), id, slug)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

func (h *PlanoHandler) Salvar(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	var in service.ConfigInput
	if err := decode(r, &in); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resp, err := h.planos.Salvar(r.Context(), id, slug, in)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

func (h *PlanoHandler) RegistrarDia(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	var in service.RegistroInput
	if err := decode(r, &in); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resp, err := h.planos.RegistrarDia(r.Context(), id, slug, r.PathValue("data"), in)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

func (h *PlanoHandler) LimparRegistros(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	resp, err := h.planos.LimparRegistros(r.Context(), id, slug)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

type marcarMarcoRequest struct {
	Cumprido bool `json:"cumprido"`
}

func (h *PlanoHandler) MarcarMarco(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	marcoID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, h.logger, errBadRequest)
		return
	}

	var req marcarMarcoRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resp, err := h.planos.MarcarMarco(r.Context(), id, slug, marcoID, req.Cumprido)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

// RegistrarRevisao records the result of one queued spaced review.
func (h *PlanoHandler) RegistrarRevisao(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	revisaoID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, h.logger, errBadRequest)
		return
	}

	var req service.RevisaoInput
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	req.ID = revisaoID

	resp, err := h.planos.RegistrarRevisao(r.Context(), id, slug, req)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

// PreviewTEC parses an uploaded TEC spreadsheet and reports what would be
// imported, without writing anything.
func (h *PlanoHandler) PreviewTEC(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	var req service.ImportacaoTECInput
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resp, err := h.planos.PreviewImportacaoTEC(r.Context(), id, slug, strings.NewReader(req.CSV))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

// ImportarTEC applies a confirmed TEC spreadsheet to the plan.
func (h *PlanoHandler) ImportarTEC(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	var req service.ImportacaoTECInput
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resp, err := h.planos.ImportarTEC(r.Context(), id, slug, req)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

type reordenarRequest struct {
	DataA string `json:"dataA"`
	DataB string `json:"dataB"`
}

func (h *PlanoHandler) Reordenar(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	var req reordenarRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resp, err := h.planos.Reordenar(r.Context(), id, slug, req.DataA, req.DataB)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

func (h *PlanoHandler) RestaurarOrdem(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	resp, err := h.planos.RestaurarOrdem(r.Context(), id, slug)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

func (h *PlanoHandler) Estatisticas(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	resp, err := h.planos.Estatisticas(r.Context(), id, slug)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

func (h *PlanoHandler) Caderno(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	resp, err := h.planos.Caderno(r.Context(), id, slug)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

func (h *PlanoHandler) CriarAnotacao(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	var in service.AnotacaoInput
	if err := decode(r, &in); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resp, err := h.planos.CriarAnotacao(r.Context(), id, slug, in)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusCreated, resp)
}

func (h *PlanoHandler) AtualizarAnotacao(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	anotID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, h.logger, errBadRequest)
		return
	}

	var in service.AnotacaoInput
	if err := decode(r, &in); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	resp, err := h.planos.AtualizarAnotacao(r.Context(), id, slug, anotID, in)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

func (h *PlanoHandler) RemoverAnotacao(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	anotID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, h.logger, errBadRequest)
		return
	}

	resp, err := h.planos.RemoverAnotacao(r.Context(), id, slug, anotID)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

func (h *PlanoHandler) Dossie(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	codigo := r.URL.Query().Get("disciplina")
	if codigo == "" {
		writeError(w, r, h.logger, errBadRequest)
		return
	}

	resp, err := h.planos.Dossie(r.Context(), id, slug, codigo)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, resp)
}

func (h *PlanoHandler) ExportarCSV(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.ctx(r)
	if !ok {
		writeError(w, r, h.logger, errUnauthorized)
		return
	}

	csv, err := h.planos.ExportarCSV(r.Context(), id, slug)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="plano-`+slug+`.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(csv)
}
