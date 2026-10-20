package httpapi

import (
	"net/http"

	"studygo/internal/service"
)

// mapaProntoDTO é um mapa que vem com o app, como a lista dos prontos o mostra.
type mapaProntoDTO struct {
	Slug     string `json:"slug"`
	Titulo   string `json:"titulo"`
	Ramos    int    `json:"ramos"`
	Questoes int    `json:"questoes"`
	NaConta  bool   `json:"naConta"`
	// Materias: os códigos das matérias do concurso pedido em que o mapa entra.
	Materias []string `json:"materias"`
}

func mapaProntoParaDTO(p service.MapaProntoResumo) mapaProntoDTO {
	materias := p.Materias
	if materias == nil {
		materias = []string{}
	}

	return mapaProntoDTO{
		Slug: p.Slug, Titulo: p.Titulo, Ramos: p.Ramos, Questoes: p.Questoes, NaConta: p.NaConta, Materias: materias,
	}
}

// adicionarMapaProntoRequest leva só o concurso aberto na tela: é nele que o
// mapa ganha o vínculo com a matéria que o texto indica.
type adicionarMapaProntoRequest struct {
	Concurso string `json:"concurso"`
}

func (h *MapaHandler) Prontos(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	// ?concurso= é o concurso aberto na tela: diz em que matéria cada pronto entra.
	ps, err := h.mapas.Prontos(r.Context(), id, r.URL.Query().Get("concurso"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	out := make([]mapaProntoDTO, 0, len(ps))
	for _, p := range ps {
		out = append(out, mapaProntoParaDTO(p))
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]any{"mapas": out})
}

func (h *MapaHandler) AdicionarPronto(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	var req adicionarMapaProntoRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	res, err := h.mapas.AdicionarPronto(r.Context(), id, r.PathValue("slug"), req.Concurso)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	status := http.StatusOK
	if res.Novo {
		status = http.StatusCreated
	}

	writeJSON(w, h.logger, status, pacoteImportadoParaDTO(res))
}
