package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"studygo/internal/service"

	"github.com/google/uuid"
)

// LeiHandler serve o catálogo de leis, o leitor, as respostas e os vínculos
// entre lei e matéria.
type LeiHandler struct {
	leis   *service.LeiService
	logger *slog.Logger
}

func NewLeiHandler(leis *service.LeiService, logger *slog.Logger) *LeiHandler {
	return &LeiHandler{leis: leis, logger: logger}
}

// maxCorpoPacoteLei cobre a Constituição inteira com as questões: o texto dela
// sozinho passa de 1 MiB, que é o teto das rotas comuns.
const maxCorpoPacoteLei = 32 << 20 // 32 MiB

func (h *LeiHandler) Catalogo(w http.ResponseWriter, r *http.Request) {
	rs, err := h.leis.Catalogo(r.Context())
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	out := make([]leiResumoDTO, 0, len(rs))
	for _, x := range rs {
		out = append(out, resumoLeiParaDTO(x))
	}
	writeJSON(w, h.logger, http.StatusOK, map[string]any{"leis": out})
}

func (h *LeiHandler) Importar(w http.ResponseWriter, r *http.Request) {
	var d pacoteLeiDTO
	if err := decodeLimitado(w, r, &d, maxCorpoPacoteLei); err != nil {
		if errors.Is(err, errRequisicaoInvalida) {
			err = errPacoteIlegivel
		}
		writeError(w, r, h.logger, err)
		return
	}

	res, err := h.leis.Importar(r.Context(), d.paraDominio())
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	status := http.StatusOK
	if res.NovaVersao {
		status = http.StatusCreated
	}
	writeJSON(w, h.logger, status, importacaoLeiParaDTO(res))
}

func (h *LeiHandler) Ler(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	l, err := h.leis.Ler(r.Context(), id, r.PathValue("slug"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusOK, leituraParaDTO(l))
}

type respostaRequest struct {
	Alternativa string `json:"alternativa"`
}

func (h *LeiHandler) Responder(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}
	questao, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}

	var req respostaRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	c, err := h.leis.Responder(r.Context(), id, questao, req.Alternativa)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusCreated, correcaoParaDTO(c))
}

func (h *LeiHandler) DoConcurso(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	ms, err := h.leis.DoConcurso(r.Context(), id, r.PathValue("slug"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	out := make([]leisDaMateriaDTO, 0, len(ms))
	for _, m := range ms {
		out = append(out, leisDaMateriaParaDTO(m))
	}
	writeJSON(w, h.logger, http.StatusOK, map[string]any{"disciplinas": out})
}

func (h *LeiHandler) Vincular(w http.ResponseWriter, r *http.Request) {
	h.vincular(w, r, true)
}

func (h *LeiHandler) Desvincular(w http.ResponseWriter, r *http.Request) {
	h.vincular(w, r, false)
}

func (h *LeiHandler) vincular(w http.ResponseWriter, r *http.Request, ligar bool) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}
	disciplina, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}

	if err := h.leis.Vincular(r.Context(), id, r.PathValue("slug"), disciplina, r.PathValue("lei"), ligar); err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusNoContent, nil)
}
