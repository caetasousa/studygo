package httpapi

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Os tetos de corpo. Antes deles, um POST autenticado podia mandar o que
// quisesse: o decoder lia até o fim, e o fim era escolhido por quem chamava.

func TestDecode_recusaCorpoGrandeDemais(t *testing.T) {
	t.Parallel()

	h, _ := newHandler(&fakeProcessor{})

	// Um JSON sintaticamente válido e maior que o teto. O tamanho é o que está
	// em teste, não o conteúdo.
	gordo := `{"nome":"` + strings.Repeat("a", maxCorpoJSON+1) + `"}`

	rec := httptest.NewRecorder()
	h.Criar(rec, post("/api/concursos", gordo, "application/json"))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, quer 413", rec.Code)
	}

	var corpo map[string]string
	decodeBody(t, rec, &corpo)

	if corpo["erro"] == "" {
		t.Error("a resposta devia dizer o que houve")
	}
}

// O corpo dentro do teto continua passando: o limite não pode custar o caso
// normal.
func TestDecode_aceitaCorpoNormal(t *testing.T) {
	t.Parallel()

	h, repo := newHandler(&fakeProcessor{})

	corpo := `{"nome":"TCE-GO","prova":"2026-12-15","disciplinas":[` +
		`{"nome":"Língua Portuguesa","bloco":"ger","questoes":20}]}`

	rec := httptest.NewRecorder()
	h.Criar(rec, post("/api/concursos", corpo, "application/json"))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, quer 201: %s", rec.Code, rec.Body.String())
	}

	if repo.criado.Nome != "TCE-GO" {
		t.Errorf("nome gravado = %q, quer TCE-GO", repo.criado.Nome)
	}
}

// A regressão que motivou lerPDF: um PDF acima do teto era TRUNCADO e seguia
// viagem, e o processador respondia "PDF inválido" sobre um arquivo íntegro.
// Recusar é a única resposta honesta.
func TestLerPDF_recusaEmVezDeTruncar(t *testing.T) {
	t.Parallel()

	grande := bytes.NewReader(make([]byte, maxEditalPDF+1))

	if _, err := lerPDF(grande); !errors.Is(err, errCorpoGrandeDemais) {
		t.Errorf("erro = %v, quer errCorpoGrandeDemais", err)
	}
}

func TestLerPDF_aceitaExatamenteOTeto(t *testing.T) {
	t.Parallel()

	dados, err := lerPDF(bytes.NewReader(make([]byte, maxEditalPDF)))
	if err != nil {
		t.Fatalf("um PDF exatamente no teto devia passar: %v", err)
	}

	if len(dados) != maxEditalPDF {
		t.Errorf("leu %d bytes, quer %d", len(dados), maxEditalPDF)
	}
}

// Ler menos que o teto devolve tudo que veio, sem sobra nem falta.
func TestLerPDF_leOArquivoInteiro(t *testing.T) {
	t.Parallel()

	original := []byte("%PDF-1.7 conteúdo qualquer")

	dados, err := lerPDF(bytes.NewReader(original))
	if err != nil {
		t.Fatalf("lerPDF: %v", err)
	}

	if !bytes.Equal(dados, original) {
		t.Errorf("lerPDF devolveu %q, quer %q", dados, original)
	}
}

// O CSV do TEC que chega dentro do JSON tinha teto no import da planilha e
// nenhum no do TEC — importar por uma rota contornava o limite da outra.
func TestImportarTEC_recusaCSVAcimaDoTeto(t *testing.T) {
	t.Parallel()

	h := &PlanoHandler{logger: quietoHTTP()}

	corpo := `{"csv":"` + strings.Repeat("a", maxPlanilhaTEC+1) + `","data":"2026-09-01"}`

	rec := httptest.NewRecorder()
	h.ImportarTEC(rec, comSlug(post("/api/concursos/x/plano/tec", corpo, "application/json")))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, quer 413", rec.Code)
	}
}

// Um corpo acima do teto de transporte para na leitura, antes mesmo de o CSV
// ser olhado — é o que impede a alocação que se quer evitar.
func TestImportarTEC_recusaCorpoAcimaDoTransporte(t *testing.T) {
	t.Parallel()

	h := &PlanoHandler{logger: quietoHTTP()}

	corpo := `{"csv":"` + strings.Repeat("a", maxCorpoPlanilha+1) + `"}`

	rec := httptest.NewRecorder()
	h.ImportarTEC(rec, comSlug(post("/api/concursos/x/plano/tec", corpo, "application/json")))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, quer 413", rec.Code)
	}
}

// comSlug preenche o path value que o PlanoHandler lê da rota.
func comSlug(r *http.Request) *http.Request {
	r.SetPathValue("slug", "x")

	return r
}

func quietoHTTP() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
