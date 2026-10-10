package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"studygo/internal/domain/mapa"
	"studygo/internal/service"

	"github.com/google/uuid"
)

// PedidosDeMapaHandler serve a fila de PDFs que viram mapa: as rotas da tela
// (enviar, listar, pôr de novo na fila, excluir) e as internas, por onde o
// processador devolve o resultado — estas numa porta que só a rede dos
// containers alcança, e com o token de serviço.
type PedidosDeMapaHandler struct {
	pedidos *service.PedidosDeMapaService
	logger  *slog.Logger
}

func NewPedidosDeMapaHandler(pedidos *service.PedidosDeMapaService, logger *slog.Logger) *PedidosDeMapaHandler {
	return &PedidosDeMapaHandler{pedidos: pedidos, logger: logger}
}

// maxCorpoPedido cobre o maior PDF aceito e o envelope do multipart.
const maxCorpoPedido = mapa.MaxBytesPDF + 1<<20

// maxCorpoResultado cobre o texto, as questões e as imagens de um mapa (100 de
// até 2 MiB é o teto teórico; uma aula real fica em poucos MB).
const maxCorpoResultado = 64 << 20

// Pedir recebe o PDF da aula num multipart: o arquivo no campo "pdf" e,
// opcionais, o concurso e a matéria em que o mapa vai morar.
func (h *PedidosDeMapaHandler) Pedir(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCorpoPedido)

	// O arquivo vai a disco temporário acima de 8 MiB, em vez de ficar todo na
	// memória da VM que o servidor divide com o resto.
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, r, h.logger, erroDoMultipart(err))
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck // limpeza de temporário

	f, cab, err := r.FormFile("pdf")
	if err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}
	defer f.Close() //nolint:errcheck // leitura de arquivo já recebido

	dados, err := io.ReadAll(f)
	if err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}

	var disciplina uuid.NullUUID
	if v := r.FormValue("disciplina"); v != "" {
		d, err := uuid.Parse(v)
		if err != nil {
			writeError(w, r, h.logger, errRequisicaoInvalida)
			return
		}

		disciplina = uuid.NullUUID{UUID: d, Valid: true}
	}

	p, err := h.pedidos.Pedir(r.Context(), id, r.FormValue("concurso"), disciplina, cab.Filename, dados)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusCreated, pedidoDeMapaParaDTO(p))
}

func (h *PedidosDeMapaHandler) Pedidos(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	ps, err := h.pedidos.Pedidos(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	out := make([]pedidoDeMapaDTO, 0, len(ps))
	for _, p := range ps {
		out = append(out, pedidoDeMapaParaDTO(p))
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]any{"pedidos": out})
}

func (h *PedidosDeMapaHandler) Reenfileirar(w http.ResponseWriter, r *http.Request) {
	id, pedido, ok := h.dePedido(w, r)
	if !ok {
		return
	}

	if err := h.pedidos.Reenfileirar(r.Context(), id, pedido); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}

func (h *PedidosDeMapaHandler) Excluir(w http.ResponseWriter, r *http.Request) {
	id, pedido, ok := h.dePedido(w, r)
	if !ok {
		return
	}

	if err := h.pedidos.Excluir(r.Context(), id, pedido); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}

// dePedido tira da requisição a conta e o id do pedido, ou responde o erro.
func (h *PedidosDeMapaHandler) dePedido(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return uuid.Nil, uuid.Nil, false
	}

	pedido, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		// Um id que não é uuid é um pedido que não existe.
		writeError(w, r, h.logger, mapa.ErrPedidoNaoEncontrado)
		return uuid.Nil, uuid.Nil, false
	}

	return id, pedido, true
}

// --- rotas internas ------------------------------------------------------------

// dadosDoResultadoDTO é a parte JSON do resultado; as imagens vêm como arquivos
// do mesmo multipart, no campo "imagens".
type dadosDoResultadoDTO struct {
	Mapa      string                `json:"mapa"`
	Questoes  *arquivoDeQuestoesDTO `json:"questoes"`
	Temas     []string              `json:"temas"`
	Relatorio string                `json:"relatorio"`
}

type falhaDoProcessadorDTO struct {
	Relatorio string `json:"relatorio"`
}

// Resultado é o processador entregando o mapa de um pedido. Recusado pela
// importação, volta com o motivo (422), e o processador o repassa ao Claude.
func (h *PedidosDeMapaHandler) Resultado(w http.ResponseWriter, r *http.Request) {
	pedido, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, h.logger, mapa.ErrPedidoNaoEncontrado)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCorpoResultado)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, r, h.logger, erroDoMultipart(err))
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck // limpeza de temporário

	var dados dadosDoResultadoDTO
	if err := json.Unmarshal([]byte(r.FormValue("dados")), &dados); err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)
		return
	}

	res := service.ResultadoDoProcessador{Texto: dados.Mapa, Temas: dados.Temas, Relatorio: dados.Relatorio}
	if dados.Questoes != nil {
		q := arquivoDeQuestoesDoDTO(*dados.Questoes)
		res.Questoes = &q
	}

	for _, cab := range r.MultipartForm.File["imagens"] {
		f, err := cab.Open()
		if err != nil {
			writeError(w, r, h.logger, errRequisicaoInvalida)
			return
		}

		b, err := io.ReadAll(f)
		f.Close() //nolint:errcheck,gosec // leitura de arquivo já recebido

		if err != nil {
			writeError(w, r, h.logger, errRequisicaoInvalida)
			return
		}

		res.Imagens = append(res.Imagens, service.ArquivoDeImagem{Nome: cab.Filename, Dados: b})
	}

	slug, err := h.pedidos.Concluir(r.Context(), pedido, res)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]string{"mapa": slug})
}

// Falha é o processador desistindo de um pedido, com o motivo.
func (h *PedidosDeMapaHandler) Falha(w http.ResponseWriter, r *http.Request) {
	pedido, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, h.logger, mapa.ErrPedidoNaoEncontrado)
		return
	}

	var req falhaDoProcessadorDTO
	if err := decodeLimitado(w, r, &req, 1<<20); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	if err := h.pedidos.Falhar(r.Context(), pedido, req.Relatorio); err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusNoContent, nil)
}

// Redespacho é o processador, recém-subido, pedindo de volta o que tinha em
// mãos antes de reiniciar.
func (h *PedidosDeMapaHandler) Redespacho(w http.ResponseWriter, r *http.Request) {
	n, err := h.pedidos.Redespachar(r.Context())
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, map[string]int{"redespachados": n})
}

// exigirTokenDeServico guarda as rotas internas: só o processador, com o token
// de serviço que ele já usa no sentido contrário, as chama. A porta interna
// não é publicada; o token é a segunda camada.
func exigirTokenDeServico(token string, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apresentado, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" || subtle.ConstantTimeCompare([]byte(apresentado), []byte(token)) != 1 {
			writeError(w, r, logger, errNaoAutenticado)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// NewRouterInterno monta as rotas que só o processador chama.
func NewRouterInterno(pedidos *PedidosDeMapaHandler, token string, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/pedidos-de-mapa/{id}/resultado", pedidos.Resultado)
	mux.HandleFunc("POST /internal/pedidos-de-mapa/{id}/falha", pedidos.Falha)
	mux.HandleFunc("POST /internal/pedidos-de-mapa/redespacho", pedidos.Redespacho)

	return exigirTokenDeServico(token, logger, mux)
}

func erroDoMultipart(err error) error {
	var grande *http.MaxBytesError
	if errors.As(err, &grande) {
		return errCorpoGrandeDemais
	}

	return errRequisicaoInvalida
}
