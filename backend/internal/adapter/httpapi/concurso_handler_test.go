package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studygo/internal/domain/concurso"
	"studygo/internal/port"
	"studygo/internal/service"

	"github.com/google/uuid"
)

var errAnalisadorFalhou = errors.New("gemini respondeu 500")

// --- fakes ---

type fakeConcursoRepo struct {
	port.ConcursoRepository
	criado concurso.Concurso
}

func (f *fakeConcursoRepo) ListByOwner(context.Context, uuid.UUID) ([]concurso.Concurso, error) {
	return nil, nil
}

func (f *fakeConcursoRepo) CreateConcurso(_ context.Context, c concurso.Concurso) (concurso.Concurso, error) {
	c.ID = uuid.New()
	f.criado = c

	return c, nil
}

type fakeProcessor struct {
	disponivel bool
	analise    port.EditalAnalise
	estrutura  port.EditalEstrutura
	conteudo   port.EditalConteudo
	err        error

	gotOwnerRef string
	gotDocID    string
	gotCargo    string
	gotDiscs    []string
	gotUpload   port.EditalUpload
}

func (f *fakeProcessor) Disponivel() bool { return f.disponivel }

func (f *fakeProcessor) Analisar(_ context.Context, ownerRef string, up port.EditalUpload) (port.EditalAnalise, error) {
	f.gotOwnerRef = ownerRef
	f.gotUpload = up
	return f.analise, f.err
}

func (f *fakeProcessor) Estrutura(_ context.Context, ownerRef, docID, cargo string) (port.EditalEstrutura, error) {
	f.gotOwnerRef = ownerRef
	f.gotDocID = docID
	f.gotCargo = cargo
	return f.estrutura, f.err
}

func (f *fakeProcessor) Conteudo(_ context.Context, ownerRef, docID, cargo string, ds []string, up port.EditalUpload) (port.EditalConteudo, error) {
	f.gotOwnerRef = ownerRef
	f.gotDocID = docID
	f.gotCargo = cargo
	f.gotDiscs = ds
	f.gotUpload = up
	return f.conteudo, f.err
}

func newHandler(an port.EditalProcessor) (*ConcursoHandler, *fakeConcursoRepo) {
	repo := &fakeConcursoRepo{}
	svc := service.NewConcursoService(repo, an)

	return NewConcursoHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))), repo
}

func ptrInt(n int) *int { return &n }

func withUser(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userIDKey, uuid.New()))
}

func post(path, body, ct string) *http.Request {
	r := withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	r.Header.Set("Content-Type", ct)

	return r
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()

	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decoding %q: %v", rec.Body.String(), err)
	}
}

// --- wizard: step 1 analisar ---

func TestConcursoHandler_AnalisarEdital(t *testing.T) {
	t.Parallel()

	analise := port.EditalAnalise{
		DocumentoID:  "doc-abc-123",
		Banca:        "FCC",
		TotalPaginas: 26,
		PaginasOCR:   26,
		Cargos: []port.EditalCargo{
			{Codigo: "A01", Nome: "Técnico Administrativo", Vagas: ptrInt(6)},
			{Codigo: "B02", Nome: "Tecnologia da Informação", Vagas: ptrInt(10)},
		},
	}

	t.Run("json com texto", func(t *testing.T) {
		t.Parallel()

		p := &fakeProcessor{disponivel: true, analise: analise}
		h, _ := newHandler(p)

		rec := httptest.NewRecorder()
		h.AnalisarEdital(rec, post("/api/editais/analisar", `{"texto":"EDITAL ..."}`, "application/json"))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if p.gotUpload.Texto == "" {
			t.Error("o texto não chegou ao processador")
		}
		if p.gotOwnerRef == "" {
			t.Error("o ownerRef não foi propagado")
		}

		var resp service.AnaliseResposta
		decodeBody(t, rec, &resp)
		if resp.DocumentoID != "doc-abc-123" {
			t.Errorf("documentoId = %q", resp.DocumentoID)
		}
		if len(resp.Cargos) != 2 || resp.Cargos[1].Codigo != "B02" {
			t.Errorf("cargos = %+v", resp.Cargos)
		}
		if resp.Cargos[0].Vagas == nil || *resp.Cargos[0].Vagas != 6 {
			t.Errorf("vagas A01 = %v", resp.Cargos[0].Vagas)
		}
	})

	t.Run("multipart com pdf", func(t *testing.T) {
		t.Parallel()

		p := &fakeProcessor{disponivel: true, analise: analise}
		h, _ := newHandler(p)

		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", "edital.pdf")
		_, _ = fw.Write([]byte("%PDF-1.7 conteúdo"))
		_ = mw.Close()

		req := withUser(httptest.NewRequest(http.MethodPost, "/api/editais/analisar", &buf))
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()

		h.AnalisarEdital(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if len(p.gotUpload.PDF) == 0 {
			t.Error("o PDF deveria ter chegado ao processador")
		}
	})

	t.Run("503 quando indisponível", func(t *testing.T) {
		t.Parallel()

		h, _ := newHandler(&fakeProcessor{disponivel: false})
		rec := httptest.NewRecorder()
		h.AnalisarEdital(rec, post("/api/editais/analisar", `{"texto":"x"}`, "application/json"))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("erro do processador vira 500", func(t *testing.T) {
		t.Parallel()

		h, _ := newHandler(&fakeProcessor{disponivel: true, err: errAnalisadorFalhou})
		rec := httptest.NewRecorder()
		h.AnalisarEdital(rec, post("/api/editais/analisar", `{"texto":"x"}`, "application/json"))

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("processador sobrecarregado vira 503 com dica de retry", func(t *testing.T) {
		t.Parallel()

		falha := fmt.Errorf("%w: processador respondeu 503: high demand", port.ErrProvedorIndisponivel)
		h, _ := newHandler(&fakeProcessor{disponivel: true, err: falha})
		rec := httptest.NewRecorder()
		h.AnalisarEdital(rec, post("/api/editais/analisar", `{"texto":"x"}`, "application/json"))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}

		var body struct{ Erro string }
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if !strings.Contains(body.Erro, "sobrecarregada") {
			t.Errorf("mensagem = %q, queria falar em sobrecarga", body.Erro)
		}
	})

	t.Run("sem auth", func(t *testing.T) {
		t.Parallel()

		h, _ := newHandler(&fakeProcessor{disponivel: true, analise: analise})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/editais/analisar", strings.NewReader(`{"texto":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		h.AnalisarEdital(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestConcursoHandler_EstruturaEdital(t *testing.T) {
	t.Parallel()

	p := &fakeProcessor{
		disponivel: true,
		estrutura: port.EditalEstrutura{
			NomeSugerido: "TCE-GO — TI",
			DataProva:    "2027-01-17",
			GruposGerais: []port.EditalGrupo{
				{Kind: "ger", Rotulo: "Conhecimentos Gerais", Total: ptrInt(25),
					Disciplinas: []port.EditalDisciplina{{Nome: "Português"}, {Nome: "Matemática"}}},
			},
			GruposEspecificos: []port.EditalGrupo{
				{Kind: "esp", Rotulo: "Conhecimentos Específicos", Total: ptrInt(45),
					Disciplinas: []port.EditalDisciplina{{Nome: "Eng. Software", Questoes: ptrInt(45)}}},
			},
			Marcos: []port.EditalMarco{{Data: "2026-10-05", Titulo: "Inscrições", ExigeAcao: true}},
		},
	}
	h, _ := newHandler(p)

	rec := httptest.NewRecorder()
	h.EstruturaEdital(rec, post("/api/editais/estrutura",
		`{"documentoId":"doc-abc-123","cargo":"B02"}`, "application/json"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if p.gotDocID != "doc-abc-123" || p.gotCargo != "B02" {
		t.Errorf("docID=%q cargo=%q", p.gotDocID, p.gotCargo)
	}

	var resp service.EstruturaResposta
	decodeBody(t, rec, &resp)

	if len(resp.Gerais) != 1 || len(resp.Especificas) != 1 {
		t.Fatalf("grupos: %d ger, %d esp", len(resp.Gerais), len(resp.Especificas))
	}
	// The group total is kept; disciplines the edital did not break down stay null.
	if resp.Gerais[0].Total == nil || *resp.Gerais[0].Total != 25 {
		t.Errorf("total geral = %v", resp.Gerais[0].Total)
	}
	for _, d := range resp.Gerais[0].Disciplinas {
		if d.Questoes != nil {
			t.Errorf("disciplina %q recebeu questões inventadas", d.Nome)
		}
	}
	if len(resp.Marcos) != 1 {
		t.Errorf("marcos = %d", len(resp.Marcos))
	}
}

func TestConcursoHandler_EstruturaEdital_semCargo(t *testing.T) {
	t.Parallel()

	h, _ := newHandler(&fakeProcessor{disponivel: true})
	rec := httptest.NewRecorder()
	h.EstruturaEdital(rec, post("/api/editais/estrutura", `{"documentoId":"x","cargo":""}`, "application/json"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestConcursoHandler_ConteudoEdital(t *testing.T) {
	t.Parallel()

	p := &fakeProcessor{
		disponivel: true,
		conteudo: port.EditalConteudo{
			Itens: []port.EditalConteudoDisciplina{
				{Nome: "Português", Temas: []string{"Crase", "Concordância"}},
				{Nome: "Eng. Software", Temas: []string{"Scrum"}},
			},
		},
	}
	h, _ := newHandler(p)

	rec := httptest.NewRecorder()
	h.ConteudoEdital(rec, post("/api/editais/conteudo",
		`{"documentoId":"doc-abc-123","cargo":"B02","disciplinas":["Português","Eng. Software"]}`, "application/json"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(p.gotDiscs) != 2 || p.gotDocID != "doc-abc-123" {
		t.Errorf("docID=%q discs=%v", p.gotDocID, p.gotDiscs)
	}

	var resp service.ConteudoEditalResposta
	decodeBody(t, rec, &resp)
	if len(resp.Itens) != 2 || len(resp.Itens[0].Temas) != 2 {
		t.Errorf("itens = %+v", resp.Itens)
	}
}

func TestConcursoHandler_ConteudoEdital_uploadFresco(t *testing.T) {
	t.Parallel()

	// The edit-screen flow: no documentoId, a fresh PDF plus disciplines.
	p := &fakeProcessor{
		disponivel: true,
		conteudo:   port.EditalConteudo{Itens: []port.EditalConteudoDisciplina{{Nome: "X", Temas: []string{"t"}}}},
	}
	h, _ := newHandler(p)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "edital.pdf")
	_, _ = fw.Write([]byte("%PDF-1.7 x"))
	_ = mw.WriteField("cargo", "B02")
	_ = mw.WriteField("disciplinas", `["X"]`)
	_ = mw.Close()

	req := withUser(httptest.NewRequest(http.MethodPost, "/api/editais/conteudo", &buf))
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	h.ConteudoEdital(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(p.gotUpload.PDF) == 0 {
		t.Error("o PDF fresco deveria ter chegado ao processador")
	}
	if p.gotDocID != "" {
		t.Errorf("não deveria haver documentoId: %q", p.gotDocID)
	}
}

func TestConcursoHandler_ConteudoEdital_semDisciplinas(t *testing.T) {
	t.Parallel()

	h, _ := newHandler(&fakeProcessor{disponivel: true})
	rec := httptest.NewRecorder()
	h.ConteudoEdital(rec, post("/api/editais/conteudo", `{"documentoId":"x","disciplinas":[]}`, "application/json"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

// --- create ---

func TestConcursoHandler_Criar(t *testing.T) {
	t.Parallel()

	h, repo := newHandler(&fakeProcessor{})

	body := `{"nome":"TJ-SP Escrevente","prova":"2026-05-10",
	          "disciplinas":[{"nome":"Direito Constitucional","bloco":"esp","questoes":15}]}`
	rec := httptest.NewRecorder()
	h.Criar(rec, post("/api/concursos", body, "application/json"))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.criado.Slug == "" || repo.criado.OwnerID == uuid.Nil {
		t.Errorf("concurso criado sem slug/owner: %+v", repo.criado)
	}
	if repo.criado.Disciplinas[0].Codigo != "" {
		t.Errorf("service não deveria atribuir codigo (é o repo): %q", repo.criado.Disciplinas[0].Codigo)
	}
}

func TestConcursoHandler_Criar_invalido(t *testing.T) {
	t.Parallel()

	h, _ := newHandler(&fakeProcessor{})

	body := `{"nome":"X","prova":"2026-05-10","disciplinas":[{"nome":"A","bloco":"esp","questoes":0}]}`
	rec := httptest.NewRecorder()
	h.Criar(rec, post("/api/concursos", body, "application/json"))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
