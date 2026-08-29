package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"annygo/internal/domain/concurso"
	"annygo/internal/port"
	"annygo/internal/service"

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

type fakeAnalisador struct {
	disponivel bool
	cargos     port.EditalCargos
	estrutura  port.EditalEstrutura
	marcos     []port.EditalMarco
	conteudo   []port.EditalConteudoDisciplina
	err        error

	gotEntrada port.EditalEntrada
	gotCargo   string
	gotDiscs   []string
}

func (f *fakeAnalisador) Disponivel() bool { return f.disponivel }

func (f *fakeAnalisador) Cargos(_ context.Context, in port.EditalEntrada) (port.EditalCargos, error) {
	f.gotEntrada = in

	return f.cargos, f.err
}

func (f *fakeAnalisador) Estrutura(
	_ context.Context,
	in port.EditalEntrada,
	cargo string,
) (port.EditalEstrutura, error) {
	f.gotCargo = cargo
	f.gotEntrada = in

	return f.estrutura, f.err
}

func (f *fakeAnalisador) Cronograma(context.Context, port.EditalEntrada) ([]port.EditalMarco, error) {
	return f.marcos, nil
}

func (f *fakeAnalisador) Conteudo(
	_ context.Context,
	in port.EditalEntrada,
	ds []string,
) ([]port.EditalConteudoDisciplina, error) {
	f.gotDiscs = ds
	f.gotEntrada = in

	return f.conteudo, f.err
}

func newHandler(an port.EditalAnalisador) (*ConcursoHandler, *fakeConcursoRepo) {
	repo := &fakeConcursoRepo{}
	svc := service.NewConcursoService(repo, an)

	return NewConcursoHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))), repo
}

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

	cargos := port.EditalCargos{
		Texto: "EDITAL TCE-GO Nº 01/2026 ...",
		Banca: "FCC",
		Cargos: []port.EditalCargo{
			{Codigo: "A01", Nome: "Técnico Administrativo", Vagas: 6},
			{Codigo: "B02", Nome: "Tecnologia da Informação", Vagas: 10},
		},
	}

	t.Run("json com texto", func(t *testing.T) {
		t.Parallel()

		an := &fakeAnalisador{disponivel: true, cargos: cargos}
		h, _ := newHandler(an)

		rec := httptest.NewRecorder()
		h.AnalisarEdital(rec, post("/api/editais/analisar", `{"texto":"EDITAL ..."}`, "application/json"))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if an.gotEntrada.Texto == "" {
			t.Error("o texto não chegou ao analisador")
		}

		var resp service.CargosResposta
		decodeBody(t, rec, &resp)
		if len(resp.Cargos) != 2 || resp.Cargos[1].Codigo != "B02" {
			t.Errorf("cargos = %+v", resp.Cargos)
		}
		if resp.Texto == "" {
			t.Error("a resposta deveria devolver o texto para os próximos passos")
		}
	})

	t.Run("multipart com pdf", func(t *testing.T) {
		t.Parallel()

		an := &fakeAnalisador{disponivel: true, cargos: cargos}
		h, _ := newHandler(an)

		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("pdf", "edital.pdf")
		_, _ = fw.Write([]byte("%PDF-1.7 conteúdo qualquer que não é um pdf de verdade"))
		_ = mw.Close()

		req := withUser(httptest.NewRequest(http.MethodPost, "/api/editais/analisar", &buf))
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()

		h.AnalisarEdital(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		// pdftext.Extrair fails on the fake bytes -> the PDF is forwarded as-is
		if len(an.gotEntrada.PDF) == 0 {
			t.Error("o PDF ilegível deveria ter sido repassado ao analisador")
		}
	})

	t.Run("503 quando indisponível", func(t *testing.T) {
		t.Parallel()

		h, _ := newHandler(&fakeAnalisador{disponivel: false})
		rec := httptest.NewRecorder()
		h.AnalisarEdital(rec, post("/api/editais/analisar", `{"texto":"x"}`, "application/json"))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("erro do analisador vira 500", func(t *testing.T) {
		t.Parallel()

		h, _ := newHandler(&fakeAnalisador{disponivel: true, err: errAnalisadorFalhou})
		rec := httptest.NewRecorder()
		h.AnalisarEdital(rec, post("/api/editais/analisar", `{"texto":"x"}`, "application/json"))

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("sem auth", func(t *testing.T) {
		t.Parallel()

		h, _ := newHandler(&fakeAnalisador{disponivel: true, cargos: cargos})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/editais/analisar", strings.NewReader(`{"texto":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		h.AnalisarEdital(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

// --- wizard: step 2 estrutura ---

func TestConcursoHandler_EstruturaEdital(t *testing.T) {
	t.Parallel()

	an := &fakeAnalisador{
		disponivel: true,
		marcos:     []port.EditalMarco{{Data: "2026-10-05", Titulo: "Inscrições", ExigeAcao: true}},
		estrutura: port.EditalEstrutura{
			Nome:             "TCE-GO — TI",
			Prova:            "2027-01-17",
			TotalGerais:      25,
			TotalEspecificas: 45,
			Gerais:           []port.EditalDisciplina{{Nome: "Português"}, {Nome: "Matemática"}},
			Especificas:      []port.EditalDisciplina{{Nome: "Eng. Software"}},
		},
	}
	h, _ := newHandler(an)

	rec := httptest.NewRecorder()
	h.EstruturaEdital(rec, post("/api/editais/estrutura",
		`{"texto":"EDITAL ...","cargo":"Tecnologia da Informação"}`, "application/json"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if an.gotCargo != "Tecnologia da Informação" {
		t.Errorf("cargo = %q", an.gotCargo)
	}

	var resp service.EstruturaResposta
	decodeBody(t, rec, &resp)

	if len(resp.Gerais) != 2 || len(resp.Especificas) != 1 {
		t.Fatalf("disciplinas: %d ger, %d esp", len(resp.Gerais), len(resp.Especificas))
	}
	if resp.Gerais[0].Bloco != "ger" || resp.Especificas[0].Bloco != "esp" {
		t.Errorf("blocos: %q / %q", resp.Gerais[0].Bloco, resp.Especificas[0].Bloco)
	}
	sg := resp.Gerais[0].Questoes + resp.Gerais[1].Questoes
	if sg != 25 || resp.Especificas[0].Questoes != 45 {
		t.Errorf("distribuição: ger=%d esp=%d", sg, resp.Especificas[0].Questoes)
	}
	// Cronograma runs in parallel and is merged in.
	if len(resp.Marcos) != 1 {
		t.Errorf("marcos = %d, esperava o cronograma mesclado", len(resp.Marcos))
	}
}

// TestConcursoHandler_EstruturaEdital_PDF covers the scanned-PDF path: the file
// travels on step 2 too, since there is no text to reuse.
func TestConcursoHandler_EstruturaEdital_PDF(t *testing.T) {
	t.Parallel()

	an := &fakeAnalisador{
		disponivel: true,
		estrutura: port.EditalEstrutura{
			Nome:   "X",
			Prova:  "2027-01-17",
			Gerais: []port.EditalDisciplina{{Nome: "Português", Questoes: 10}},
		},
	}
	h, _ := newHandler(an)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("pdf", "edital.pdf")
	_, _ = fw.Write([]byte("%PDF-1.7 conteúdo escaneado"))
	_ = mw.WriteField("cargo", "Tecnologia da Informação")
	_ = mw.Close()

	req := withUser(httptest.NewRequest(http.MethodPost, "/api/editais/estrutura", &buf))
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	h.EstruturaEdital(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(an.gotEntrada.PDF) == 0 {
		t.Error("o PDF deveria ter chegado ao analisador na etapa 2")
	}
	if an.gotCargo != "Tecnologia da Informação" {
		t.Errorf("cargo = %q", an.gotCargo)
	}
}

func TestConcursoHandler_EstruturaEdital_semCargo(t *testing.T) {
	t.Parallel()

	h, _ := newHandler(&fakeAnalisador{disponivel: true})
	rec := httptest.NewRecorder()
	h.EstruturaEdital(rec, post("/api/editais/estrutura", `{"texto":"x","cargo":""}`, "application/json"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

// --- wizard: step 3 conteúdo ---

func TestConcursoHandler_ConteudoEdital(t *testing.T) {
	t.Parallel()

	an := &fakeAnalisador{
		disponivel: true,
		conteudo: []port.EditalConteudoDisciplina{
			{Nome: "Português", Temas: []string{"Crase", "Concordância"}},
			{Nome: "Eng. Software", Temas: []string{"Scrum"}},
		},
	}
	h, _ := newHandler(an)

	rec := httptest.NewRecorder()
	h.ConteudoEdital(rec, post("/api/editais/conteudo",
		`{"texto":"EDITAL ...","disciplinas":["Português","Eng. Software"]}`, "application/json"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(an.gotDiscs) != 2 {
		t.Errorf("disciplinas enviadas: %v", an.gotDiscs)
	}

	var resp service.ConteudoEditalResposta
	decodeBody(t, rec, &resp)
	if len(resp.Itens) != 2 || len(resp.Itens[0].Temas) != 2 {
		t.Errorf("itens = %+v", resp.Itens)
	}
}

func TestConcursoHandler_ConteudoEdital_semDisciplinas(t *testing.T) {
	t.Parallel()

	h, _ := newHandler(&fakeAnalisador{disponivel: true})
	rec := httptest.NewRecorder()
	h.ConteudoEdital(rec, post("/api/editais/conteudo", `{"texto":"x","disciplinas":[]}`, "application/json"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

// --- create ---

func TestConcursoHandler_Criar(t *testing.T) {
	t.Parallel()

	h, repo := newHandler(&fakeAnalisador{})

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

	h, _ := newHandler(&fakeAnalisador{})

	body := `{"nome":"X","prova":"2026-05-10","disciplinas":[{"nome":"A","bloco":"esp","questoes":0}]}`
	rec := httptest.NewRecorder()
	h.Criar(rec, post("/api/concursos", body, "application/json"))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
