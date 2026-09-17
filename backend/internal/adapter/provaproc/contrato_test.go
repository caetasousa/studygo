package provaproc_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"studygo/internal/domain/prova"
)

// O contrato com o edital-processor, do lado de cá.
//
// O cliente decodifica a resposta dele direto no domínio, sem tradução: os
// aliases de edital-processor/app/provas/schemas.py são os nomes dos campos
// daqui. Quem renomeia um campo de um lado precisa renomear do outro, e sem
// este teste o sintoma só aparecia em produção — prova importada vazia.
//
// O arquivo é gravado pelo processador (tests/unit/test_contrato_provas.py).
// Mudou de propósito? Regrave lá com ATUALIZAR_CONTRATO=1 e rode este teste.
const contrato = "../../../../edital-processor/tests/contrato/provas.json"

func lerContrato(t *testing.T) map[string]json.RawMessage {
	t.Helper()

	dados, err := os.ReadFile(filepath.Clean(contrato))
	if err != nil {
		t.Fatalf("lendo o contrato gravado pelo processador: %v", err)
	}
	var partes map[string]json.RawMessage
	if err := json.Unmarshal(dados, &partes); err != nil {
		t.Fatalf("decodificando o contrato: %v", err)
	}

	return partes
}

func TestContrato_MetadadosEGabarito(t *testing.T) {
	t.Parallel()

	partes := lerContrato(t)

	var m prova.Metadados
	if err := json.Unmarshal(partes["metadados"], &m); err != nil {
		t.Fatal(err)
	}
	if m.Orgao != "TJCE" || m.Ano != 2026 || m.Cargo != "F06" || m.Caderno != "004" || m.Total != 60 ||
		m.CargoNome == "" {
		t.Fatalf("metadados = %+v; algum campo da capa deixou de ser entendido", m)
	}

	var g prova.Gabarito
	if err := json.Unmarshal(partes["gabarito"], &g); err != nil {
		t.Fatal(err)
	}
	if g.Cargo != "F06" || g.Caderno != "4" || g.Tipo != "preliminar" ||
		g.Respostas["1"] != "B" || g.Situacoes["2"] != "Anulada" {
		t.Fatalf("gabarito = %+v", g)
	}
}

func TestContrato_RegiaoExtraida(t *testing.T) {
	t.Parallel()

	var r prova.Rascunho
	if err := json.Unmarshal(lerContrato(t)["extrair"], &r); err != nil {
		t.Fatal(err)
	}

	if len(r.Questoes) != 1 || len(r.Apoios) != 1 || len(r.Extracoes) != 1 || len(r.Alertas) != 1 {
		t.Fatalf("rascunho = %+v; a região lida perdeu uma das listas", r)
	}
	if e := r.Extracoes[0]; e.Modelo == "" || e.TokensEntrada == 0 || e.TokensSaida == 0 ||
		e.Regiao == "" || e.Prompt == "" || e.Versao == "" {
		t.Fatalf("extração = %+v; o registro de consumo perdeu um campo", e)
	}

	q := r.Questoes[0]
	if q.Numero != 24 || q.Disciplina == "" || !q.Completa || !q.LidaPorOCR || len(q.Alternativas) != 5 ||
		len(q.Apoios) != 1 || len(q.Origens) != 1 {
		t.Fatalf("questão = %+v", q)
	}
	if q.Blocos[0].Texto == "" || q.Blocos[0].Formato != "negrito" {
		t.Fatalf("enunciado = %+v", q.Blocos[0])
	}
	figura := q.Blocos[1]
	if figura.Tipo != "imagem" || figura.Arquivo == "" || figura.Descricao == "" ||
		figura.Largura != 60 || !figura.Revisado || figura.Origem == nil {
		t.Fatalf("figura = %+v; o recorte perdeu um campo", figura)
	}
	if o := *figura.Origem; o.Pagina != 7 || o.Regiao != "6" || len(o.Retangulo) != 4 {
		t.Fatalf("origem da figura = %+v", o)
	}
	if a := q.Alternativas[0]; a.Letra != "A" || len(a.Blocos) != 1 || a.Blocos[0].Texto == "" {
		t.Fatalf("alternativa = %+v", a)
	}

	apoio := r.Apoios[0]
	if apoio.ID == "" || len(apoio.Blocos) != 1 || len(apoio.Questoes) != 3 || apoio.Aviso == "" ||
		len(apoio.Origens) != 1 {
		t.Fatalf("texto de apoio = %+v", apoio)
	}
}
