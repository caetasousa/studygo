package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"studygo/internal/service"

	"github.com/google/uuid"
)

// LeiHandler serve o catálogo de leis, a captura e a publicação, a importação
// das questões, o leitor, as respostas e os vínculos entre lei e matéria.
type LeiHandler struct {
	leis   *service.LeiService
	logger *slog.Logger
}

func NewLeiHandler(leis *service.LeiService, logger *slog.Logger) *LeiHandler {
	return &LeiHandler{leis: leis, logger: logger}
}

// maxCorpoQuestoesLei cobre o questoes.json de uma lei grande: o da Lei
// Orgânica do TCE-GO, com 133 questões, passa de 1 MiB, o teto das rotas
// comuns.
const maxCorpoQuestoesLei = 16 << 20 // 16 MiB

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

func (h *LeiHandler) Capturar(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	var req capturaRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	captura, err := h.leis.Capturar(r.Context(), id, service.PedidoDeCaptura{
		Link: req.Link, Recorte: req.Recorte, Artigos: req.Artigos, Slug: req.Slug, Inteira: req.Inteira,
	})
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusAccepted, map[string]string{"id": captura})
}

func (h *LeiHandler) Pesquisar(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	var req pesquisaRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	p, err := h.leis.PesquisarTema(r.Context(), id, req.Tema, req.Link)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusOK, pesquisaParaDTO(p))
}

func (h *LeiHandler) ResumirExclusao(w http.ResponseWriter, r *http.Request) {
	res, err := h.leis.ResumirExclusao(r.Context(), r.PathValue("slug"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusOK, exclusaoDTO{Curto: res.Curto, Questoes: res.Questoes, Respostas: res.Respostas})
}

func (h *LeiHandler) Excluir(w http.ResponseWriter, r *http.Request) {
	if err := h.leis.Excluir(r.Context(), r.PathValue("slug")); err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusNoContent, nil)
}

func (h *LeiHandler) Captura(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	c, err := h.leis.Captura(r.Context(), id, r.PathValue("id"), r.URL.Query().Get("concurso"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusOK, capturaParaDTO(c))
}

func (h *LeiHandler) Publicar(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	var req publicacaoRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	res, err := h.leis.Publicar(r.Context(), id, r.PathValue("id"), service.PedidoDePublicacao{
		Slug: req.Slug, Nome: req.Nome, Curto: req.Curto, Reconhecer: req.Reconhecer, Aceitos: req.Aceitos,
	})
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	status := http.StatusOK
	if res.NovaVersao {
		status = http.StatusCreated
	}
	writeJSON(w, h.logger, status, publicacaoParaDTO(res))
}

func (h *LeiHandler) ImportarQuestoes(w http.ResponseWriter, r *http.Request) {
	var d questoesDeLeiDTO
	if err := decodeLimitado(w, r, &d, maxCorpoQuestoesLei); err != nil {
		if errors.Is(err, errRequisicaoInvalida) {
			err = errQuestoesIlegiveis
		}
		writeError(w, r, h.logger, err)
		return
	}

	us, qs := questoesDoDTO(d)
	res, err := h.leis.ImportarQuestoes(r.Context(), r.PathValue("slug"), us, qs)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusOK, importacaoQuestoesParaDTO(res))
}

func (h *LeiHandler) Ler(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	l, err := h.leis.Ler(r.Context(), id, r.PathValue("slug"), r.URL.Query().Get("concurso"))
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

	var req vinculoRequest
	if ligar && r.ContentLength != 0 {
		if err := decode(w, r, &req); err != nil {
			writeError(w, r, h.logger, err)
			return
		}
	}

	if err := h.leis.Vincular(
		r.Context(), id, r.PathValue("slug"), disciplina, r.PathValue("lei"), ligar, req.Recorte, req.Artigos, req.Somar,
	); err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusNoContent, nil)
}
