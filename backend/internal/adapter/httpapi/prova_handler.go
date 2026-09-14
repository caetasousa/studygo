package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"slices"
	"strconv"

	"studygo/internal/domain/prova"
	"studygo/internal/port"
	"studygo/internal/service"

	"github.com/google/uuid"
)

// maxCorpoRascunho cobre o rascunho de uma prova de 60 questões com texto de
// apoio, que passa de 1 MiB mas não chega perto disto.
const maxCorpoRascunho = 8 << 20 // 8 MiB

// ProvaHandler serve o catálogo de provas e a curadoria das importações.
type ProvaHandler struct {
	provas *service.ProvaService
	// maxPDF é o teto de CADA arquivo; o multipart da importação leva até
	// dois (prova e gabarito). O nginx precisa aceitar pelo menos o dobro.
	maxPDF int64
	logger *slog.Logger
}

func NewProvaHandler(provas *service.ProvaService, maxPDF int64, logger *slog.Logger) *ProvaHandler {
	return &ProvaHandler{provas: provas, maxPDF: maxPDF, logger: logger}
}

func (h *ProvaHandler) usuario(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return "", false
	}

	return id.String(), true
}

// idDaRota recusa o que não é uuid antes de chegar ao banco. Um id malformado
// é, para quem pediu, uma prova que não existe.
func (h *ProvaHandler) idDaRota(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, h.logger, http.StatusNotFound, map[string]string{"erro": "prova ou importação não encontrada"})
		return "", false
	}

	return id.String(), true
}

// --- catálogo ---------------------------------------------------------------

func (h *ProvaHandler) Catalogo(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))

	provas, err := h.provas.Catalogo(r.Context(), port.FiltroCatalogo{
		Ano: q.Get("ano"), Orgao: q.Get("orgao"), Cargo: q.Get("cargo"),
		Disciplina: q.Get("disciplina"), Offset: offset,
	})
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	out := make([]provaResumoDTO, 0, len(provas))
	for _, p := range provas {
		out = append(out, provaResumoParaDTO(p))
	}
	writeJSON(w, h.logger, http.StatusOK, out)
}

// QuestoesAvulsas lista as questões publicadas para treinar por matéria.
// ?disciplina= se repete para pedir várias; ?ano= é o ano da prova.
func (h *ProvaHandler) QuestoesAvulsas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ano, _ := strconv.Atoi(q.Get("ano"))

	qs, err := h.provas.QuestoesAvulsas(r.Context(), prova.FiltroDeAvulsas{Materias: q["disciplina"], Ano: ano})
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	out := make([]questaoAvulsaDTO, 0, len(qs))
	for _, a := range qs {
		out = append(out, questaoAvulsaParaDTO(a))
	}
	writeJSON(w, h.logger, http.StatusOK, out)
}

// Prova devolve uma prova publicada. ?numero= e ?disciplina= filtram as
// questões; os materiais de apoio vêm sempre inteiros.
func (h *ProvaHandler) Prova(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	numero, _ := strconv.Atoi(q.Get("numero"))

	p, err := h.provas.Publicacao(r.Context(), id, numero, q.Get("disciplina"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, provaParaDTO(p))
}

func (h *ProvaHandler) Revisar(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}

	i, err := h.provas.Revisar(r.Context(), usuario, id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusCreated, importacaoProvaParaDTO(i))
}

// Reextrair enfileira uma extração nova do caderno de uma prova publicada.
func (h *ProvaHandler) Reextrair(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}

	i, err := h.provas.Reextrair(r.Context(), usuario, id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusAccepted, importacaoProvaParaDTO(i))
}

func (h *ProvaHandler) Retirar(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}

	if err := h.provas.Retirar(r.Context(), usuario, id); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Arquivo entrega um PDF ou recorte. O id é uuid e o caminho é montado pelo
// service; nada da requisição vira caminho.
func (h *ProvaHandler) Arquivo(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}

	caminho, err := h.provas.Arquivo(r.Context(), usuario, id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	// O conteúdo de um id nunca muda; o acesso a ele pode mudar (a prova sai
	// do catálogo), então o cache é curto e só do navegador.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, caminho)
}

// --- curadoria --------------------------------------------------------------

func (h *ProvaHandler) ListarImportacoes(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}

	lista, err := h.provas.Listar(r.Context(), usuario)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	out := make([]importacaoResumoDTO, 0, len(lista))
	for _, i := range lista {
		out = append(out, importacaoResumoParaDTO(i))
	}
	writeJSON(w, h.logger, http.StatusOK, out)
}

// Importar recebe multipart com "prova" e, opcionalmente, "gabarito".
func (h *ProvaHandler) Importar(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	// Recusa antes de ler o corpo: sem isso, qualquer conta subiria 50 MiB
	// até ouvir o não.
	if !h.provas.Curador(usuario) {
		writeError(w, r, h.logger, prova.ErrAcesso)
		return
	}

	if err := h.lerMultipart(w, r, 2); err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck // limpeza de temporário

	pdf, err := h.lerArquivo(r, "prova")
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	if pdf == nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}
	gabarito, err := h.lerArquivo(r, "gabarito")
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	i, err := h.provas.Importar(r.Context(), usuario, pdf, gabarito)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusAccepted, importacaoProvaParaDTO(i))
}

func (h *ProvaHandler) Importacao(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}

	i, err := h.provas.Obter(r.Context(), usuario, id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, importacaoProvaParaDTO(i))
}

func (h *ProvaHandler) SalvarImportacao(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}
	var req importacaoEdicaoRequest
	if err := decodeLimitado(w, r, &req, maxCorpoRascunho); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	i, err := h.provas.Salvar(r.Context(), usuario, id, req.Versao, rascunhoDoDTO(req.Rascunho))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, importacaoProvaParaDTO(i))
}

func (h *ProvaHandler) Publicar(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}
	var req versaoRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	provaID, err := h.provas.Publicar(r.Context(), usuario, id, req.Versao)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]string{"provaId": provaID})
}

func (h *ProvaHandler) Cancelar(w http.ResponseWriter, r *http.Request) {
	h.acaoComVersao(w, r, h.provas.Cancelar)
}

func (h *ProvaHandler) Reprocessar(w http.ResponseWriter, r *http.Request) {
	h.acaoComVersao(w, r, h.provas.Reprocessar)
}

func (h *ProvaHandler) Reler(w http.ResponseWriter, r *http.Request) {
	h.acaoComVersao(w, r, h.provas.Reler)
}

func (h *ProvaHandler) ProcurarCadastradas(w http.ResponseWriter, r *http.Request) {
	h.acaoComVersao(w, r, h.provas.ProcurarCadastradas)
}

func (h *ProvaHandler) Excluir(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}
	var req versaoRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	if err := h.provas.Excluir(r.Context(), usuario, id, req.Versao); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type acaoDeImportacao func(ctx context.Context, usuario, id string, versao int) (service.ImportacaoDeProva, error)

func (h *ProvaHandler) acaoComVersao(w http.ResponseWriter, r *http.Request, acao acaoDeImportacao) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}
	var req versaoRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	i, err := acao(r.Context(), usuario, id, req.Versao)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, importacaoProvaParaDTO(i))
}

func (h *ProvaHandler) Recortar(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}
	var req recorteRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	arquivo, err := h.provas.Recortar(r.Context(), usuario, id, req.Versao, origemDoDTO(req.Origem))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]string{"arquivo": arquivo})
}

// SugerirMaterias devolve a matéria que a IA sugere para cada questão do
// rascunho salvo. Nada é gravado: a tela aplica e o curador salva.
func (h *ProvaHandler) SugerirMaterias(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}

	materias, err := h.provas.SugerirMaterias(r.Context(), usuario, id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	out := make([]materiaSugeridaDTO, 0, len(materias))
	for numero, materia := range materias {
		out = append(out, materiaSugeridaDTO{Numero: numero, Materia: materia})
	}
	slices.SortFunc(out, func(a, b materiaSugeridaDTO) int { return a.Numero - b.Numero })
	writeJSON(w, h.logger, http.StatusOK, out)
}

// AtualizarGabarito recebe multipart com "gabarito" e "versao".
func (h *ProvaHandler) AtualizarGabarito(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}
	if !h.provas.Curador(usuario) {
		writeError(w, r, h.logger, prova.ErrAcesso)
		return
	}

	if err := h.lerMultipart(w, r, 1); err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck // limpeza de temporário

	pdf, err := h.lerArquivo(r, "gabarito")
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	versao, err := strconv.Atoi(r.FormValue("versao"))
	if pdf == nil || err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}

	i, err := h.provas.AtualizarGabarito(r.Context(), usuario, id, versao, pdf)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusAccepted, importacaoProvaParaDTO(i))
}

// lerMultipart limita o corpo inteiro a `arquivos` PDFs mais 1 MiB de
// enquadramento, e recusa com 413 o que passar.
func (h *ProvaHandler) lerMultipart(w http.ResponseWriter, r *http.Request, arquivos int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, arquivos*h.maxPDF+(1<<20))
	if err := r.ParseMultipartForm(maxMemoriaUpload); err != nil {
		return traduzirCorpo(err)
	}

	return nil
}

// lerArquivo devolve nil quando o campo não veio. Lê um byte além do teto para
// distinguir "coube exatamente" de "não coube", como lerPDF.
func (h *ProvaHandler) lerArquivo(r *http.Request, campo string) ([]byte, error) {
	f, _, err := r.FormFile(campo)
	if errors.Is(err, http.ErrMissingFile) {
		return nil, nil
	}
	if err != nil {
		return nil, errRequisicaoInvalida
	}
	defer f.Close()

	return lerLimitado(f, h.maxPDF)
}

func lerLimitado(f multipart.File, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, errRequisicaoInvalida
	}
	if int64(len(data)) > max {
		return nil, errCorpoGrandeDemais
	}

	return data, nil
}

// --- anotações do estudante -------------------------------------------------

func (h *ProvaHandler) Anotacoes(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}

	anotacoes, err := h.provas.Anotacoes(r.Context(), usuario, id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	out := make([]anotacaoDeQuestaoDTO, 0, len(anotacoes))
	for _, a := range anotacoes {
		out = append(out, anotacaoDeQuestaoParaDTO(a))
	}
	writeJSON(w, h.logger, http.StatusOK, out)
}

func (h *ProvaHandler) Anotar(w http.ResponseWriter, r *http.Request) {
	usuario, ok := h.usuario(w, r)
	if !ok {
		return
	}
	id, ok := h.idDaRota(w, r)
	if !ok {
		return
	}
	numero, err := strconv.Atoi(r.PathValue("numero"))
	if err != nil || numero < 1 {
		writeJSON(w, h.logger, http.StatusNotFound, map[string]string{"erro": "questão não encontrada"})
		return
	}
	var req anotacaoDeQuestaoRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	a, err := h.provas.Anotar(r.Context(), usuario, id, numero, req.Texto)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, anotacaoDeQuestaoParaDTO(a))
}
