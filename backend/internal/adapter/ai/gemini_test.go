package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"annygo/internal/port"
)

// geminiStub records the last request body and replies with a canned payload.
type geminiStub struct {
	srv      *httptest.Server
	bodies   []map[string]any
	status   int
	replies  []string // one per call; the last is reused if calls exceed len
	delay    time.Duration
	calls    int
	uploaded int // bytes received by the Files API stub
}

func newStub(t *testing.T) *geminiStub {
	t.Helper()

	s := &geminiStub{status: http.StatusOK}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Files API: upload start → upload finalize → files.get poll.
		switch {
		case strings.HasPrefix(r.URL.Path, "/upload/"):
			if r.Header.Get("X-Goog-Upload-Command") == "start" {
				w.Header().Set("X-Goog-Upload-URL", s.srv.URL+"/upload/v1beta/files/put")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))

				return
			}

			body, _ := io.ReadAll(r.Body)
			s.uploaded = len(body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"file":{"name":"files/abc","uri":"https://gen.example/files/abc",` +
				`"mimeType":"application/pdf","state":"ACTIVE"}}`))

			return
		case strings.HasPrefix(r.URL.Path, "/v1beta/files/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"state":"ACTIVE"}`))

			return
		}

		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		s.bodies = append(s.bodies, b)

		if s.delay > 0 {
			time.Sleep(s.delay)
		}

		reply := `{}`
		if len(s.replies) > 0 {
			i := s.calls
			if i >= len(s.replies) {
				i = len(s.replies) - 1
			}
			reply = geminiEnvelope(s.replies[i])
		}
		s.calls++

		w.WriteHeader(s.status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(s.srv.Close)

	return s
}

// geminiEnvelope wraps an inner JSON string as a generateContent response, or
// passes it through verbatim when it is already a full envelope / not JSON.
func geminiEnvelope(inner string) string {
	trimmed := strings.TrimSpace(inner)
	if strings.HasPrefix(trimmed, `{"candidates"`) || strings.HasPrefix(trimmed, `{"error"`) || !strings.HasPrefix(trimmed, "{") {
		return inner
	}

	b, _ := json.Marshal(inner)

	return `{"candidates":[{"content":{"parts":[{"text":` + string(b) + `}]}}]}`
}

func analisadorAt(s *geminiStub) *GeminiAnalisador {
	g := NewGeminiAnalisador("test-key", "gemini-test")
	g.http = s.srv.Client()
	g.baseURL = s.srv.URL + "/v1beta/models/%s:generateContent?key=%s"
	g.uploadURL = s.srv.URL + "/upload/v1beta/files?key=%s"
	g.filesURL = s.srv.URL + "/v1beta/%s?key=%s"
	g.backoff = time.Millisecond
	g.pollInterval = time.Millisecond

	return g
}

func TestGeminiAnalisador_Cargos(t *testing.T) {
	t.Parallel()

	t.Run("ignora as partes de raciocínio e junta o texto visível", func(t *testing.T) {
		t.Parallel()

		s := newStub(t)
		// Gemini 3 flash devolve o raciocínio numa parte "thought": true e pode
		// quebrar a resposta em vários pedaços.
		s.replies = []string{`{"candidates":[{"finishReason":"STOP","content":{"parts":[` +
			`{"text":"o edital tem um cargo","thought":true},` +
			`{"text":"{\"banca\":\"FGV\",\"cargos\":[{\"codigo\":\"B02\",\"nome\":"},` +
			`{"text":"\"TI\",\"escolaridade\":\"Médio\",\"vagas\":3}]}"}` +
			`]}}]}`}

		got, err := analisadorAt(s).Cargos(context.Background(), port.EditalEntrada{Texto: "EDITAL..."})
		if err != nil {
			t.Fatalf("Cargos: %v", err)
		}
		if got.Banca != "FGV" || len(got.Cargos) != 1 || got.Cargos[0].Codigo != "B02" {
			t.Fatalf("resultado: %+v", got)
		}
	})

	t.Run("texto vai como texto e volta ecoado", func(t *testing.T) {
		t.Parallel()

		s := newStub(t)
		s.replies = []string{`{"banca":"FCC","cargos":[
			{"codigo":"A01","nome":"Técnico Administrativo","escolaridade":"Médio","vagas":6},
			{"codigo":"B02","nome":"Tecnologia da Informação","escolaridade":"Médio","vagas":10}]}`}

		got, err := analisadorAt(s).Cargos(context.Background(), port.EditalEntrada{Texto: "EDITAL Nº 01/2026 ..."})
		if err != nil {
			t.Fatalf("Cargos: %v", err)
		}

		if got.Banca != "FCC" || len(got.Cargos) != 2 {
			t.Fatalf("resultado: %+v", got)
		}
		if got.Cargos[1].Codigo != "B02" || got.Cargos[1].Vagas != 10 {
			t.Errorf("cargo B02: %+v", got.Cargos[1])
		}
		if got.Texto != "EDITAL Nº 01/2026 ..." {
			t.Errorf("texto = %q", got.Texto)
		}

		if hasInlineData(parts(t, s.bodies[0])) {
			t.Error("não deveria mandar inline_data para entrada de texto")
		}
	})

	t.Run("pdf sobe uma vez pela Files API e volta como URI", func(t *testing.T) {
		t.Parallel()

		s := newStub(t)
		s.replies = []string{`{"banca":"FCC","cargos":[
			{"codigo":"B02","nome":"TI","escolaridade":"Médio","vagas":10}]}`}

		pdf := []byte("%PDF-1.7 bytes de um edital digitalizado")
		got, err := analisadorAt(s).Cargos(context.Background(), port.EditalEntrada{
			PDF:  pdf,
			MIME: "application/pdf",
		})
		if err != nil {
			t.Fatalf("Cargos: %v", err)
		}

		if s.uploaded != len(pdf) {
			t.Errorf("bytes enviados = %d, esperava %d", s.uploaded, len(pdf))
		}
		// Scanned PDF: no text layer to hand back, but a reusable URI.
		if got.Texto != "" {
			t.Errorf("texto = %q, esperava vazio (o modelo não transcreve)", got.Texto)
		}
		if got.ArquivoURI == "" {
			t.Error("esperava a URI do arquivo enviado")
		}

		// generateContent references the file instead of inlining it.
		p := parts(t, s.bodies[0])
		if hasInlineData(p) {
			t.Error("não deveria inlinar o PDF depois do upload")
		}
		if !hasFileData(p) {
			t.Error("faltou file_data apontando para o arquivo enviado")
		}
		if strings.Contains(allText(p), "transcri") {
			t.Error("o prompt não deveria pedir transcrição — o modelo devolve vazio")
		}
	})

	t.Run("a URI viaja nas etapas seguintes, sem novo upload", func(t *testing.T) {
		t.Parallel()

		s := newStub(t)
		s.replies = []string{`{"nome":"X","prova":"2027-01-17","gerais":[{"nome":"P","questoes":10}],"especificas":[]}`}

		entrada := port.EditalEntrada{ArquivoURI: "https://gen.example/files/abc", MIME: "application/pdf"}
		if _, err := analisadorAt(s).Estrutura(context.Background(), entrada, "TI"); err != nil {
			t.Fatalf("Estrutura: %v", err)
		}

		if s.uploaded != 0 {
			t.Errorf("não deveria subir nada de novo; recebeu %d bytes", s.uploaded)
		}
		if !hasFileData(parts(t, s.bodies[0])) {
			t.Error("Estrutura deveria referenciar o arquivo por URI")
		}
	})

	t.Run("sem cargos vira erro", func(t *testing.T) {
		t.Parallel()

		s := newStub(t)
		s.replies = []string{`{"banca":"X","cargos":[]}`}

		if _, err := analisadorAt(s).Cargos(context.Background(), port.EditalEntrada{Texto: "x"}); err == nil {
			t.Fatal("esperava erro de nenhum cargo")
		}
	})

	t.Run("entrada vazia", func(t *testing.T) {
		t.Parallel()

		if _, err := analisadorAt(newStub(t)).Cargos(context.Background(), port.EditalEntrada{}); err == nil {
			t.Fatal("esperava erro")
		}
	})
}

func TestGeminiAnalisador_Estrutura(t *testing.T) {
	t.Parallel()

	s := newStub(t)
	s.replies = []string{`{
		"nome":"TCE-GO — Técnico de Controle Externo (TI)",
		"prova":"2027-01-17","provaDiscursiva":true,
		"totalGerais":25,"totalEspecificas":45,
		"gerais":[{"nome":"Língua Portuguesa","questoes":0},{"nome":"Matemática","questoes":0}],
		"especificas":[{"nome":"Engenharia de Software","questoes":0}]}`}

	got, err := analisadorAt(s).Estrutura(context.Background(), port.EditalEntrada{Texto: "EDITAL..."}, "Tecnologia da Informação")
	if err != nil {
		t.Fatalf("Estrutura: %v", err)
	}

	if got.Nome == "" || got.Prova != "2027-01-17" || !got.ProvaDiscursiva {
		t.Errorf("cabeçalho: %+v", got)
	}
	if len(got.Gerais) != 2 || len(got.Especificas) != 1 {
		t.Fatalf("disciplinas: %d ger, %d esp", len(got.Gerais), len(got.Especificas))
	}
	if got.TotalGerais != 25 || got.TotalEspecificas != 45 {
		t.Errorf("totais = %d/%d", got.TotalGerais, got.TotalEspecificas)
	}

	// the chosen cargo must reach the prompt
	joined := allText(parts(t, s.bodies[0]))
	if !strings.Contains(joined, "Tecnologia da Informação") {
		t.Errorf("o cargo não foi enviado; partes: %q", joined)
	}
}

func TestGeminiAnalisador_Cronograma(t *testing.T) {
	t.Parallel()

	s := newStub(t)
	s.replies = []string{`{"marcos":[
		{"data":"2026-10-05","dataFim":"2026-11-06","titulo":"Inscrições","exigeAcao":true},
		{"data":"","titulo":"lixo sem data"},
		{"data":"2027-01-17","titulo":"Aplicação das provas","exigeAcao":false}]}`}

	got, err := analisadorAt(s).Cronograma(context.Background(), port.EditalEntrada{Texto: "EDITAL..."})
	if err != nil {
		t.Fatalf("Cronograma: %v", err)
	}

	if len(got) != 2 { // blank date dropped
		t.Fatalf("marcos = %d, quero 2", len(got))
	}
	if got[0].DataFim != "2026-11-06" || !got[0].ExigeAcao {
		t.Errorf("marco de inscrição: %+v", got[0])
	}
}

func TestGeminiAnalisador_Conteudo(t *testing.T) {
	t.Parallel()

	s := newStub(t)
	s.replies = []string{`{"disciplinas":[
		{"nome":"Língua Portuguesa","temas":["Crase","Concordância"," "]},
		{"nome":"Engenharia de Software","temas":["Scrum"]}]}`}

	got, err := analisadorAt(s).Conteudo(context.Background(), port.EditalEntrada{Texto: "EDITAL..."},
		[]string{"Língua Portuguesa", "Engenharia de Software"})
	if err != nil {
		t.Fatalf("Conteudo: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("itens = %d", len(got))
	}
	if len(got[0].Temas) != 2 { // blank trimmed out
		t.Errorf("temas de Português: %v", got[0].Temas)
	}

	joined := allText(parts(t, s.bodies[0]))
	if !strings.Contains(joined, "Engenharia de Software") {
		t.Errorf("a lista de disciplinas não foi enviada: %q", joined)
	}
}

func TestGeminiAnalisador_erros(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		prep        func(*geminiStub)
		wantSub     string
		transitorio bool // wrapped in port.ErrProvedorIndisponivel → 503, not 500
	}{
		{"modelo 404", func(s *geminiStub) { s.status = 404; s.replies = []string{`{"error":{"code":404,"message":"gone"}}`} }, "404", false},
		{"resposta não-JSON", func(s *geminiStub) { s.replies = []string{`<html>502</html>`} }, "decoding gemini response", false},
		{"sem candidates", func(s *geminiStub) { s.replies = []string{`{"candidates":[]}`} }, "não retornou conteúdo", false},
		{"json interno inválido", func(s *geminiStub) {
			s.replies = []string{`{"candidates":[{"content":{"parts":[{"text":"desculpe, não sei responder"}]}}]}`}
		}, "json inválido", false},
		{"500 persistente é culpa do provedor", func(s *geminiStub) {
			s.status = http.StatusInternalServerError
			s.replies = []string{`{"error":{"code":500,"message":"internal"}}`}
		}, "500", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newStub(t)
			tt.prep(s)

			_, err := analisadorAt(s).Cargos(context.Background(), port.EditalEntrada{Texto: "x"})
			if err == nil || !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("erro = %v, queria conter %q", err, tt.wantSub)
			}
			if got := errors.Is(err, port.ErrProvedorIndisponivel); got != tt.transitorio {
				t.Errorf("errors.Is(ErrProvedorIndisponivel) = %v, queria %v (erro: %v)", got, tt.transitorio, err)
			}
		})
	}

	t.Run("retry em 503 transitório", func(t *testing.T) {
		t.Parallel()

		s := newStub(t)
		s.status = http.StatusServiceUnavailable
		s.replies = []string{`{"error":{"code":503,"message":"high demand"}}`}

		_, err := analisadorAt(s).Cargos(context.Background(), port.EditalEntrada{Texto: "x"})
		if err == nil {
			t.Fatal("esperava erro após esgotar os retries")
		}
		if s.calls != maxTentativas {
			t.Errorf("deveria ter esgotado as %d tentativas; calls=%d", maxTentativas, s.calls)
		}
		if !errors.Is(err, port.ErrProvedorIndisponivel) {
			t.Errorf("503 esgotado deveria ser ErrProvedorIndisponivel; erro = %v", err)
		}
	})
}

func TestGeminiAnalisador_Disponivel(t *testing.T) {
	t.Parallel()

	if NewGeminiAnalisador("", "").Disponivel() {
		t.Error("sem chave: indisponível")
	}
	if !NewGeminiAnalisador("k", "").Disponivel() {
		t.Error("com chave: disponível")
	}
}

// --- helpers ---

func parts(t *testing.T, body map[string]any) []any {
	t.Helper()

	contents, _ := body["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("contents ausente: %v", body)
	}
	first, _ := contents[0].(map[string]any)
	p, _ := first["parts"].([]any)
	if len(p) == 0 {
		t.Fatalf("parts ausente: %v", first)
	}

	return p
}

func allText(parts []any) string {
	var b strings.Builder
	for _, pt := range parts {
		if m, ok := pt.(map[string]any); ok {
			if s, ok := m["text"].(string); ok {
				b.WriteString(s)
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

func hasInlineData(parts []any) bool { return hasChave(parts, "inline_data") }
func hasFileData(parts []any) bool   { return hasChave(parts, "file_data") }

func hasChave(parts []any, chave string) bool {
	for _, pt := range parts {
		if m, ok := pt.(map[string]any); ok {
			if _, ok := m[chave]; ok {
				return true
			}
		}
	}

	return false
}
