package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"studygo/internal/service"

	"github.com/google/uuid"
)

// MapaHandler serve a importação, a lista e a leitura dos mapas mentais, e o
// vínculo deles com as matérias do concurso.
type MapaHandler struct {
	mapas  *service.MapaService
	logger *slog.Logger
}

func NewMapaHandler(mapas *service.MapaService, logger *slog.Logger) *MapaHandler {
	return &MapaHandler{mapas: mapas, logger: logger}
}

// maxCorpoMapa cobre o outline do maior mapa que o app aceita (5.000 itens de
// até 500 caracteres) com folga, e passa do teto das rotas comuns (1 MiB).
const maxCorpoMapa = 4 << 20 // 4 MiB

func (h *MapaHandler) Catalogo(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	rs, err := h.mapas.Catalogo(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	out := make([]mapaResumoDTO, 0, len(rs))
	for _, x := range rs {
		out = append(out, mapaResumoParaDTO(x))
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]any{"mapas": out})
}

func (h *MapaHandler) Importar(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	var req importarMapaRequest
	if err := decodeLimitado(w, r, &req, maxCorpoMapa); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	res, err := h.mapas.Importar(r.Context(), id, req.Texto, req.Concurso)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	status := http.StatusOK
	if res.Novo {
		status = http.StatusCreated
	}

	writeJSON(w, h.logger, status, mapaImportadoParaDTO(res))
}

func (h *MapaHandler) Ler(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	m, err := h.mapas.Ler(r.Context(), id, r.PathValue("slug"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, mapaLidoParaDTO(m))
}

func (h *MapaHandler) Excluir(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	if err := h.mapas.Excluir(r.Context(), id, r.PathValue("slug")); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}

func (h *MapaHandler) DoConcurso(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	ms, err := h.mapas.DoConcurso(r.Context(), id, r.PathValue("slug"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	out := make([]mapasDaMateriaDTO, 0, len(ms))
	for _, m := range ms {
		out = append(out, mapasDaMateriaParaDTO(m))
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]any{"disciplinas": out})
}

func (h *MapaHandler) Vincular(w http.ResponseWriter, r *http.Request) {
	h.vincular(w, r, true)
}

func (h *MapaHandler) Desvincular(w http.ResponseWriter, r *http.Request) {
	h.vincular(w, r, false)
}

func (h *MapaHandler) vincular(w http.ResponseWriter, r *http.Request, ligar bool) {
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

	if err := h.mapas.Vincular(r.Context(), id, r.PathValue("slug"), disciplina, r.PathValue("mapa"), ligar); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}

// ExcluirItem tira um tópico do mapa e devolve o mapa como ficou.
func (h *MapaHandler) ExcluirItem(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	var req exclusaoDeItemRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	m, err := h.mapas.ExcluirItem(r.Context(), id, r.PathValue("slug"), req.Caminho, req.Texto)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, mapaLidoParaDTO(m))
}

func (h *MapaHandler) ImportarQuestoes(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	var d arquivoDeQuestoesDTO
	if err := decodeLimitado(w, r, &d, maxCorpoMapa); err != nil {
		if errors.Is(err, errRequisicaoInvalida) {
			err = errQuestoesDoMapaIlegiveis
		}
		writeError(w, r, h.logger, err)
		return
	}

	res, err := h.mapas.ImportarQuestoes(r.Context(), id, r.PathValue("slug"), arquivoDeQuestoesDoDTO(d))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusOK, questoesImportadasDTO{
		Novas: res.Novas, Atualizadas: res.Atualizadas, Desativadas: res.Desativadas, Mantidas: res.Mantidas,
	})
}

func (h *MapaHandler) Responder(w http.ResponseWriter, r *http.Request) {
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

	var req respostaDoMapaRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	c, err := h.mapas.Responder(r.Context(), id, questao, req.Resposta)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, h.logger, http.StatusCreated, correcaoDoMapaParaDTO(c))
}

// maxCorpoImagens cobre um envio de várias imagens de até 2 MiB cada, com folga
// para o envelope do multipart. Quem tem mais manda em mais de uma vez.
const maxCorpoImagens = 24 << 20 // 24 MiB

// EnviarImagens recebe as imagens do mapa num multipart, no campo "imagens".
func (h *MapaHandler) EnviarImagens(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCorpoImagens)

	if err := r.ParseMultipartForm(maxCorpoImagens); err != nil {
		var grande *http.MaxBytesError
		if errors.As(err, &grande) {
			writeError(w, r, h.logger, errCorpoGrandeDemais)
			return
		}

		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck // limpeza de temporário

	var arquivos []service.ArquivoDeImagem

	for _, cab := range r.MultipartForm.File["imagens"] {
		f, err := cab.Open()
		if err != nil {
			writeError(w, r, h.logger, errRequisicaoInvalida)
			return
		}

		dados, err := io.ReadAll(f)
		f.Close() //nolint:errcheck,gosec // leitura de arquivo já recebido

		if err != nil {
			writeError(w, r, h.logger, errRequisicaoInvalida)
			return
		}

		arquivos = append(arquivos, service.ArquivoDeImagem{Nome: cab.Filename, Dados: dados})
	}

	n, err := h.mapas.EnviarImagens(r.Context(), id, r.PathValue("slug"), arquivos)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, imagensEnviadasDTO{Gravadas: n})
}

// Imagem devolve os bytes de uma imagem do mapa. O tipo é o que o envio
// conferiu nos bytes, e o nosniff impede o navegador de achar outro.
func (h *MapaHandler) Imagem(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	img, err := h.mapas.Imagem(r.Context(), id, r.PathValue("slug"), r.PathValue("nome"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	w.Header().Set("Content-Type", img.Tipo)
	w.Header().Set("Content-Length", strconv.Itoa(len(img.Dados)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(img.Dados); err != nil {
		h.logger.DebugContext(r.Context(), "escrevendo imagem do mapa", "erro", err)
	}
}
