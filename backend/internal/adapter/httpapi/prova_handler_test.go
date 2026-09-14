package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"studygo/internal/domain/prova"
	"studygo/internal/port"
	"studygo/internal/service"

	"github.com/google/uuid"
)

// fakeProvaRepo atende só o que estes testes tocam; o resto entra em pânico,
// o que é o aviso certo se um handler começar a chamar algo inesperado.
type fakeProvaRepo struct {
	port.ProvaRepository
	importacao prova.Importacao
}

func (f *fakeProvaRepo) Obter(context.Context, string) (prova.Importacao, error) {
	if f.importacao.ID == "" {
		return prova.Importacao{}, prova.ErrNaoEncontrada
	}

	return f.importacao, nil
}

func (f *fakeProvaRepo) Arquivo(context.Context, string, bool) (string, error) {
	return "", prova.ErrNaoEncontrada
}

func novoProvaHandlerDeTeste(curadores ...uuid.UUID) *ProvaHandler {
	lista := map[string]bool{}
	for _, c := range curadores {
		lista[c.String()] = true
	}
	svc := &service.ProvaService{Repo: &fakeProvaRepo{}, Curadores: lista, MaxPendentes: 2}

	return NewProvaHandler(svc, 1<<20, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func comConta(r *http.Request, id uuid.UUID) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), usuarioIDKey, id))
}

// O ServeMux entra em pânico ao registrar padrões ambíguos. Montar o router
// inteiro com as rotas de provas é o que pega isso antes da produção.
func TestRouter_RotasDeProvasNaoConflitam(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("registrar as rotas entrou em pânico: %v", r)
		}
	}()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	NewRouter(Handlers{Prova: novoProvaHandlerDeTeste()}, nil, nil, LimitesPadrao(logger), logger)
}

// Quem não é curador ouve 403 antes de o corpo ser lido: sem isso, qualquer
// conta subiria dois PDFs grandes até ouvir o não.
func TestProvaHandler_ImportarExigeCurador(t *testing.T) {
	t.Parallel()

	var corpo bytes.Buffer
	mw := multipart.NewWriter(&corpo)
	parte, _ := mw.CreateFormFile("prova", "prova.pdf")
	_, _ = parte.Write([]byte("%PDF-1.7"))
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/provas/importacoes", &corpo)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	novoProvaHandlerDeTeste().Importar(rec, comConta(req, uuid.New()))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, quer 403: %s", rec.Code, rec.Body)
	}
}

func TestProvaHandler_RascunhoSoParaCurador(t *testing.T) {
	t.Parallel()

	id := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/api/provas/importacoes/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()

	novoProvaHandlerDeTeste().Importacao(rec, comConta(req, uuid.New()))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, quer 403", rec.Code)
	}
}

// Arquivo de rascunho não existe para quem não é curador: 404, e não 403, para
// não confirmar que o id existe.
func TestProvaHandler_ArquivoDeRascunhoInvisivel(t *testing.T) {
	t.Parallel()

	id := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/api/provas/arquivos/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()

	novoProvaHandlerDeTeste().Arquivo(rec, comConta(req, uuid.New()))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quer 404", rec.Code)
	}
}

func TestProvaHandler_IdMalformadoE404(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/provas/../../etc/passwd", nil)
	req.SetPathValue("id", "../../etc/passwd")
	rec := httptest.NewRecorder()

	novoProvaHandlerDeTeste().Arquivo(rec, comConta(req, uuid.New()))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quer 404", rec.Code)
	}
}

// A importação sai em camelCase e com listas vazias como [], nunca null.
func TestProvaHandler_ImportacaoEmCamelCase(t *testing.T) {
	t.Parallel()

	curador := uuid.New()
	h := novoProvaHandlerDeTeste(curador)
	id := uuid.NewString()
	h.provas.Repo.(*fakeProvaRepo).importacao = prova.Importacao{
		ID: id, Estado: prova.EstadoEmRevisao, Versao: 4,
		Rascunho: prova.Rascunho{Banca: "FCC", Questoes: []prova.Questao{{Numero: 1}}},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/provas/importacoes/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h.Importacao(rec, comConta(req, curador))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var corpo struct {
		Versao   int `json:"versao"`
		Rascunho struct {
			Questoes []struct {
				Blocos       []any `json:"blocos"`
				Alternativas []any `json:"alternativas"`
			} `json:"questoes"`
		} `json:"rascunho"`
		Pendencias []string `json:"pendencias"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
		t.Fatal(err)
	}
	q := corpo.Rascunho.Questoes[0]
	if corpo.Versao != 4 || q.Blocos == nil || q.Alternativas == nil || len(corpo.Pendencias) == 0 {
		t.Fatalf("corpo = %s", rec.Body)
	}
}
