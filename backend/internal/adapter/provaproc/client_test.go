package provaproc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"studygo/internal/adapter/provaproc"
	"studygo/internal/domain/prova"
)

// O processador sabe o que ler pelo pedido: a releitura diz a questão, e o
// trecho marcado em volta de um texto de apoio pede só o texto.
func TestClient_ExtrairDizOQueLer(t *testing.T) {
	t.Parallel()

	var pedidos []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		_ = json.NewDecoder(r.Body).Decode(&p)
		pedidos = append(pedidos, p)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := provaproc.New(srv.URL, "token")

	for _, regiao := range []string{"3", "q7", "ta:r3-t1", "t14"} {
		o := prova.Origem{Pagina: 1, Regiao: regiao, Retangulo: []float64{0, 0, 595, 842}}
		if _, err := c.Extrair(context.Background(), "doc", o); err != nil {
			t.Fatalf("Extrair %s: %v", regiao, err)
		}
	}

	quer := []struct {
		questao any
		apoio   any
	}{{nil, nil}, {float64(7), nil}, {nil, true}, {nil, nil}}
	for k, q := range quer {
		if pedidos[k]["questao"] != q.questao || pedidos[k]["apoio"] != q.apoio {
			t.Errorf("pedido %d = %v; quer questao %v, apoio %v", k, pedidos[k], q.questao, q.apoio)
		}
	}
}

// A folha de alterações traz vários cargos e todos os tipos: o pedido diz qual
// é o desta prova.
func TestClient_AlteracoesDizemOCargoEOTipo(t *testing.T) {
	t.Parallel()

	var pedido map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&pedido)
		_, _ = w.Write([]byte(`{"Respostas":{"17":"A"},"Situacoes":{"17":"Gabarito alterado"}}`))
	}))
	defer srv.Close()

	g, err := provaproc.New(srv.URL, "token").AlteracoesDeGabarito(context.Background(), "arq", "H08", "001")

	if err != nil || g.Respostas["17"] != "A" || g.Situacoes["17"] != "Gabarito alterado" {
		t.Fatalf("alterações = %+v (%v)", g, err)
	}
	if pedido["cargo"] != "H08" || pedido["caderno"] != "001" || pedido["alteracoes"] != true {
		t.Fatalf("pedido = %v", pedido)
	}
}
