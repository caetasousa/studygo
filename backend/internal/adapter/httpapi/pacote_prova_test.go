package httpapi

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"studygo/internal/domain/prova"
	"studygo/internal/service"
)

const (
	figuraDoPacote    = "11111111-2222-3333-4444-555555555555"
	documentoDoPacote = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	gabaritoDoPacote  = "99999999-8888-7777-6666-555555555555"
)

var (
	pngMinimo = []byte("\x89PNG\r\n\x1a\nfigura")
	pdfMinimo = []byte("%PDF-1.7 caderno")
)

func provaParaLevar(t *testing.T) service.ProvaParaLevar {
	t.Helper()

	dir := t.TempDir()
	gravar := func(nome string, conteudo []byte) string {
		caminho := filepath.Join(dir, nome)
		if err := os.WriteFile(caminho, conteudo, 0o600); err != nil {
			t.Fatal(err)
		}
		return caminho
	}
	r := rascunhoDeContrato()
	r.Questoes[0].Blocos[0].Arquivo = figuraDoPacote
	r.Questoes[0].Alternativas[0].Blocos[0].Arquivo = figuraDoPacote
	r.Apoios[0].Blocos[0].Arquivo = figuraDoPacote
	r.AnuladasExcluidas = []int{7}

	return service.ProvaParaLevar{
		Conteudo:  r,
		Regioes:   []prova.Origem{{Pagina: 1, Retangulo: []float64{0, 0, 595, 845}, Regiao: "0"}},
		Documento: documentoDoPacote, GabaritoArquivo: gabaritoDoPacote,
		NomeDocumento: "fcc-2023-trt-18-prova.pdf", NomeGabarito: "fcc-2023-trt-18-gabarito.pdf",
		Arquivos: map[string]string{
			documentoDoPacote + ".pdf": gravar("d.pdf", pdfMinimo),
			gabaritoDoPacote + ".pdf":  gravar("g.pdf", []byte("%PDF-1.7 gabarito")),
			figuraDoPacote + ".png":    gravar("f.png", pngMinimo),
		},
	}
}

// O que sai de um ambiente entra no outro igual: conteúdo, regiões, nomes,
// PDFs e figuras.
func TestPacoteDeProva_IdaEVolta(t *testing.T) {
	t.Parallel()

	p := provaParaLevar(t)
	var zip bytes.Buffer
	if err := escreverPacote(&zip, p, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}

	pct, err := lerPacote(zip.Bytes(), 1<<20)
	if err != nil {
		t.Fatalf("lerPacote: %v", err)
	}
	if !reflect.DeepEqual(rascunhoParaDTO(pct.Conteudo), rascunhoParaDTO(rascunhoSemExtracoes(p.Conteudo))) {
		t.Fatalf("conteúdo mudou na ida e volta:\n%+v\n%+v", pct.Conteudo, p.Conteudo)
	}
	if !reflect.DeepEqual(pct.Regioes, p.Regioes) || pct.NomeDocumento != p.NomeDocumento || pct.NomeGabarito != p.NomeGabarito {
		t.Fatalf("regiões ou nomes mudaram: %+v", pct)
	}
	if !bytes.Equal(pct.Documento, pdfMinimo) || !bytes.HasPrefix(pct.Gabarito, []byte("%PDF")) {
		t.Fatalf("PDFs = %q, %q", pct.Documento, pct.Gabarito)
	}
	if len(pct.Figuras) != 1 || !bytes.Equal(pct.Figuras[figuraDoPacote], pngMinimo) {
		t.Fatalf("figuras = %v", pct.Figuras)
	}
}

// As extrações são o registro das chamadas do ambiente de lá: o rascunho que
// chega não as traz (rascunhoDoDTO as ignora, como no salvar).
func rascunhoSemExtracoes(r prova.Rascunho) prova.Rascunho {
	r.Extracoes = nil
	return r
}

func zipCom(t *testing.T, arquivos map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for nome, conteudo := range arquivos {
		f, err := zw.Create(nome)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write(conteudo)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

// Só entra o que o formato prevê: caminho forjado, arquivo gigante, formato de
// outra versão e lixo recusam o pacote com o motivo.
func TestPacoteDeProva_RecusaOQueNaoEPacote(t *testing.T) {
	t.Parallel()

	manifesto := []byte(`{"formato":"studygo.prova/1","prova":{},"documento":"` + documentoDoPacote + `.pdf"}`)
	casos := map[string]struct {
		dados []byte
		quer  string
	}{
		"não é zip":          {[]byte("não sou zip"), "não é um .zip"},
		"sem manifesto":      {zipCom(t, map[string][]byte{"arquivos/" + documentoDoPacote + ".pdf": pdfMinimo}), "não tem manifest.json"},
		"caminho forjado":    {zipCom(t, map[string][]byte{"manifest.json": manifesto, "../../etc/passwd": []byte("x")}), "não é de prova"},
		"extensão estranha":  {zipCom(t, map[string][]byte{"manifest.json": manifesto, "arquivos/" + documentoDoPacote + ".exe": []byte("x")}), "não é de prova"},
		"arquivo grande":     {zipCom(t, map[string][]byte{"manifest.json": manifesto, "arquivos/" + documentoDoPacote + ".pdf": bytes.Repeat([]byte("a"), 2048)}), "tamanho máximo"},
		"outro formato":      {zipCom(t, map[string][]byte{"manifest.json": []byte(`{"formato":"studygo.prova/9"}`)}), "formato de pacote desconhecido"},
		"manifesto quebrado": {zipCom(t, map[string][]byte{"manifest.json": []byte(`{`)}), "não é válido"},
	}
	for nome, c := range casos {
		_, err := lerPacote(c.dados, 1024)
		var v service.ErrValidacao
		if !errors.As(err, &v) || !strings.Contains(v.Msg, c.quer) {
			t.Errorf("%s: err = %v; quer recusa com %q", nome, err, c.quer)
		}
	}
}

func TestNomeDoPacote(t *testing.T) {
	t.Parallel()

	r := prova.Rascunho{Orgao: "TRT 18", Ano: 2023, Cargo: "L12"}
	if got := nomeDoPacote(r); got != "prova-trt-18-2023-l12.zip" {
		t.Fatalf("nome = %q", got)
	}
	if got := nomeDoPacote(prova.Rascunho{Orgao: "TJ/CE \"x\""}); got != "prova-tjce-x.zip" {
		t.Fatalf("nome sem ano nem cargo = %q", got)
	}
}

// O manifesto é o que um ambiente manda ao outro: contrato.
func TestContratoHTTP_PacoteDeProva(t *testing.T) {
	t.Parallel()

	p := provaParaLevar(t)
	m := pacoteDeProvaDTO{
		Formato: formatoDoPacote, ExportadoEm: time.Unix(0, 0), Prova: rascunhoParaDTO(p.Conteudo),
		Regioes: origensParaDTO(p.Regioes), Documento: documentoDoPacote + ".pdf", Gabarito: gabaritoDoPacote + ".pdf",
		NomeDocumento: p.NomeDocumento, NomeGabarito: p.NomeGabarito,
	}

	compararComGolden(t, "prova_pacote.json", forma(t, m))
}
