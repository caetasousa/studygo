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

// maxEditalPDF é o maior PDF que a importação aceita. maxCorpoEdital é o teto
// do multipart INTEIRO: o mesmo PDF mais os campos que viajam ao lado dele e o
// enquadramento do formato. Os dois existem porque limitar só um deles deixa
// passar o outro — e porque o edge (nginx) precisa de um número para casar.
const (
	maxEditalPDF   = 20 << 20 // 20 MiB
	maxCorpoEdital = 24 << 20 // 24 MiB

	// maxMemoriaUpload é quanto do formulário fica em RAM antes de o excedente
	// ir para arquivo temporário. Não é limite de tamanho — quem limita é o
	// MaxBytesReader — e por isso é pequeno de propósito.
	maxMemoriaUpload = 1 << 20 // 1 MiB
)

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

// guardaImport é o preâmbulo comum dos passos do assistente: confere a
// autenticação, confere se a importação está disponível e estica o prazo de
// escrita para caber a ida e volta até a IA.
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

// AnalisarEdital — passo 1 do assistente. Aceita JSON {"texto": "..."} ou
// multipart com um campo "file"; devolve o identificador do documento e os
// cargos encontrados.
func (h *ConcursoHandler) AnalisarEdital(w http.ResponseWriter, r *http.Request) {
	if !h.guardaImport(w, r) {
		return
	}

	up, _, err := lerUploadEdital(w, r)
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

// EstruturaEdital — passo 2. Recebe {"documentoId", "cargo"}.
func (h *ConcursoHandler) EstruturaEdital(w http.ResponseWriter, r *http.Request) {
	if !h.guardaImport(w, r) {
		return
	}

	var body struct {
		DocumentoID string `json:"documentoId"`
		Cargo       string `json:"cargo"`
	}
	if err := decode(w, r, &body); err != nil {
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

// ConteudoEdital — passo 3, e também o "extrair temas" da tela de edição.
// Recebe {"documentoId", "cargo", "disciplinas"} OU um upload novo (multipart
// ou texto) com "cargo" e "disciplinas" ao lado.
func (h *ConcursoHandler) ConteudoEdital(w http.ResponseWriter, r *http.Request) {
	if !h.guardaImport(w, r) {
		return
	}

	up, extras, err := lerUploadEdital(w, r)
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

// editalExtras são os campos que viajam ao lado de um upload novo, e que mudam
// conforme o passo.
type editalExtras struct {
	DocumentoID string
	Cargo       string
	Disciplinas []string
}

// ownerRefDe devolve a referência opaca a que o processador amarra o documento,
// e que ele exige de volta nos passos seguintes. O id do usuário serve: ele não
// sai da rede do compose e, do outro lado, não significa nada além de igualdade.
func ownerRefDe(r *http.Request) string {
	if id, ok := usuarioID(r.Context()); ok {
		return id.String()
	}
	return ""
}

// lerUploadEdital aceita JSON {"texto", "documentoId", "cargo", "disciplinas"}
// ou multipart/form-data com um "file" mais esses campos.
//
// Recebe o writer porque o teto de corpo é aplicado aqui: um upload maior que
// maxCorpoEdital é RECUSADO com 413, não lido pela metade.
func lerUploadEdital(w http.ResponseWriter, r *http.Request) (port.EditalUpload, editalExtras, error) {
	var extras editalExtras

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxCorpoEdital)

		if err := r.ParseMultipartForm(maxMemoriaUpload); err != nil {
			return port.EditalUpload{}, extras, traduzirCorpo(err)
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
			// sem arquivo e sem texto é legítimo quando um documentoId carrega o
			// trabalho de um passo anterior.
			return port.EditalUpload{}, extras, nil
		}
		defer file.Close()

		data, err := lerPDF(file)
		if err != nil {
			return port.EditalUpload{}, extras, err
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
	if err := decodeLimitado(w, r, &body, maxCorpoEdital); err != nil {
		return port.EditalUpload{}, extras, err
	}

	extras.DocumentoID = body.DocumentoID
	extras.Cargo = body.Cargo
	extras.Disciplinas = body.Disciplinas

	return port.EditalUpload{Texto: body.Texto}, extras, nil
}

// lerPDF lê o arquivo inteiro, RECUSANDO o que passar de maxEditalPDF.
//
// Antes isto era um io.LimitReader, que truncava em silêncio: um PDF de 25 MB
// chegava ao processador com 20 MB e o usuário lia "PDF inválido" sobre um
// arquivo perfeitamente válido. Ler um byte a mais que o teto é a forma de
// distinguir "coube exatamente" de "não coube".
func lerPDF(file io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(file, maxEditalPDF+1))
	if err != nil {
		return nil, errRequisicaoInvalida
	}

	if len(data) > maxEditalPDF {
		return nil, errCorpoGrandeDemais
	}

	return data, nil
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
	if err := decode(w, r, &req); err != nil {
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
	if err := decode(w, r, &req); err != nil {
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
