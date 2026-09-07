package editalproc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"studygo/internal/port"
)

// O cliente pelo lado que importa para quem usa o app: o que ele faz quando o
// processador some, e o que ele NÃO faz quando o processador responde que o
// edital é que está ruim.

func clienteApontandoPara(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	return New(srv.URL, "token-de-teste")
}

func analisar(c *Client) error {
	_, err := c.Analisar(context.Background(), "dono-1", port.EditalUpload{Texto: "edital colado"})

	return err
}

// Depois de três indisponibilidades seguidas, a quarta chamada nem chega à
// rede — é a diferença entre esperar um minuto para saber e saber agora.
func TestClient_abreODisjuntorEFalhaRapido(t *testing.T) {
	t.Parallel()

	chamadas := 0
	c := clienteApontandoPara(t, func(w http.ResponseWriter, _ *http.Request) {
		chamadas++
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	for range falhasParaAbrir {
		if err := analisar(c); !errors.Is(err, port.ErrProvedorIndisponivel) {
			t.Fatalf("erro = %v, quer ErrProvedorIndisponivel", err)
		}
	}

	if c.Disponivel() {
		t.Error("Disponivel() devia passar a responder false com o disjuntor aberto")
	}

	antes := chamadas

	if err := analisar(c); !errors.Is(err, port.ErrProvedorIndisponivel) {
		t.Fatalf("com o disjuntor aberto, erro = %v, quer ErrProvedorIndisponivel", err)
	}

	if chamadas != antes {
		t.Errorf("a chamada atravessou a rede com o disjuntor aberto (%d -> %d)", antes, chamadas)
	}
}

// A distinção que dá sentido ao disjuntor: um edital recusado é resposta
// SAUDÁVEL. Contá-la tiraria a importação do ar para todo mundo por causa de um
// PDF ruim de um usuário só.
func TestClient_editalRecusadoNaoAbreODisjuntor(t *testing.T) {
	t.Parallel()

	c := clienteApontandoPara(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"INVALID_PDF","message":"não é um PDF","transient":false}`))
	})

	for range falhasParaAbrir + 2 {
		err := analisar(c)

		if err == nil {
			t.Fatal("o processador recusou: o cliente devia devolver erro")
		}

		if errors.Is(err, port.ErrProvedorIndisponivel) {
			t.Fatalf("erro = %v, quer uma recusa comum e não indisponibilidade", err)
		}
	}

	if !c.Disponivel() {
		t.Error("recusar editais ruins não pode derrubar a importação de todos")
	}
}

// Um sucesso no meio zera a contagem: falhas espalhadas não somam até abrir.
func TestClient_sucessoNoMeioMantemDisponivel(t *testing.T) {
	t.Parallel()

	falhar := true
	c := clienteApontandoPara(t, func(w http.ResponseWriter, _ *http.Request) {
		if falhar {
			w.WriteHeader(http.StatusBadGateway)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"documentId":"doc-1","banca":"FGV","totalPages":3,"ocrPages":0,"cargos":[],"alerts":[]}`))
	})

	_ = analisar(c)
	_ = analisar(c)

	falhar = false

	if err := analisar(c); err != nil {
		t.Fatalf("a chamada boa devia passar: %v", err)
	}

	falhar = true
	_ = analisar(c)
	_ = analisar(c)

	if !c.Disponivel() {
		t.Error("duas falhas depois de um sucesso não deviam abrir o disjuntor")
	}
}

// Sem URL configurada não há o que consultar: o comportamento antigo continua
// valendo, e é a raiz de composição que troca pelo processador nulo.
func TestClient_semURLNaoEstaDisponivel(t *testing.T) {
	t.Parallel()

	if New("", "").Disponivel() {
		t.Error("cliente sem baseURL devia responder indisponível")
	}
}
