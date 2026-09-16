package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
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

// entradasDoZip lê o .zip como o navegador lê: nome → conteúdo, sem descompactar.
func entradasDoZip(t *testing.T, dados []byte) map[string][]byte {
	t.Helper()

	zr, err := zip.NewReader(bytes.NewReader(dados), int64(len(dados)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		if f.Method != zip.Store {
			t.Errorf("%s está comprimido; o navegador lê o pacote sem descompactar", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		conteudo, _ := io.ReadAll(rc)
		_ = rc.Close()
		out[f.Name] = conteudo
	}

	return out
}

// Um arquivo só, com uma pasta por prova e as figuras com nome que se reconhece.
func TestPacoteDeProvas_UmZipComUmaPastaPorProva(t *testing.T) {
	t.Parallel()

	p := provaParaLevar(t)
	outra := provaParaLevar(t)
	outra.GabaritoArquivo = ""
	var buf bytes.Buffer
	if err := escreverPacote(&buf, []service.ProvaParaLevar{p, outra}, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}

	entradas := entradasDoZip(t, buf.Bytes())
	nomes := slices.Sorted(func(yield func(string) bool) {
		for k := range entradas {
			if !yield(k) {
				return
			}
		}
	})
	figura := "figuras/questao-44-" + figuraDoPacote + ".png"
	quer := []string{
		"LEIA-ME.txt",
		"tjce-2026-e05-2/figuras/questao-44-" + figuraDoPacote + ".png",
		"tjce-2026-e05-2/prova.json",
		"tjce-2026-e05-2/prova.pdf",
		"tjce-2026-e05/" + figura,
		"tjce-2026-e05/gabarito.pdf",
		"tjce-2026-e05/prova.json",
		"tjce-2026-e05/prova.pdf",
	}
	if !slices.Equal(nomes, quer) {
		t.Fatalf("entradas =\n%s\nquer\n%s", strings.Join(nomes, "\n"), strings.Join(quer, "\n"))
	}
	if !bytes.Equal(entradas["tjce-2026-e05/"+figura], pngMinimo) || !bytes.Equal(entradas["tjce-2026-e05/prova.pdf"], pdfMinimo) {
		t.Fatal("a figura ou o PDF não são os do volume")
	}
}

// O que sai de um ambiente entra no outro igual: conteúdo, regiões, nomes,
// PDFs e figuras.
func TestPacoteDeProvas_IdaEVolta(t *testing.T) {
	t.Parallel()

	p := provaParaLevar(t)
	var buf bytes.Buffer
	if err := escreverPacote(&buf, []service.ProvaParaLevar{p}, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}

	// O navegador manda a pasta: o prova.json e os arquivos pelo nome, sem a pasta.
	envio := map[string][]byte{}
	var manifesto []byte
	for nome, conteudo := range entradasDoZip(t, buf.Bytes()) {
		switch {
		case nome == "tjce-2026-e05/prova.json":
			manifesto = conteudo
		case strings.HasPrefix(nome, "tjce-2026-e05/"):
			envio[path.Base(nome)] = conteudo
		}
	}
	pct, err := provaDoEnvio(manifesto, envio)
	if err != nil {
		t.Fatalf("provaDoEnvio: %v", err)
	}

	semExtracoes := p.Conteudo
	semExtracoes.Extracoes = nil
	if !reflect.DeepEqual(rascunhoParaDTO(pct.Conteudo), rascunhoParaDTO(semExtracoes)) {
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

// Só entra o que o formato prevê: nome forjado, formato de outra versão e lixo
// recusam o envio com o motivo.
func TestPacoteDeProvas_RecusaOQueNaoEProva(t *testing.T) {
	t.Parallel()

	manifesto, _ := json.Marshal(provaDoPacoteDTO{Formato: formatoDoPacote, PDFDaProva: "prova.pdf"})
	casos := map[string]struct {
		manifesto []byte
		arquivos  map[string][]byte
		quer      string
	}{
		"caminho forjado":    {manifesto, map[string][]byte{"../../etc/passwd": []byte("x")}, "não é de prova"},
		"executável":         {manifesto, map[string][]byte{"prova.exe": []byte("x")}, "não é de prova"},
		"outro formato":      {[]byte(`{"formato":"studygo.prova/1"}`), nil, "formato de pacote desconhecido"},
		"manifesto quebrado": {[]byte(`{`), nil, "não é válido"},
	}
	for nome, c := range casos {
		_, err := provaDoEnvio(c.manifesto, c.arquivos)
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

// O prova.json é o que um ambiente manda ao outro: contrato.
func TestContratoHTTP_PacoteDeProva(t *testing.T) {
	t.Parallel()

	p := provaParaLevar(t)
	m := provaDoPacoteDTO{
		Formato: formatoDoPacote, ExportadoEm: time.Unix(0, 0), Prova: rascunhoParaDTO(p.Conteudo),
		Regioes: origensParaDTO(p.Regioes), PDFDaProva: "prova.pdf", PDFDoGabarito: "gabarito.pdf",
		NomeDocumento: p.NomeDocumento, NomeGabarito: p.NomeGabarito,
		Figuras: map[string]string{figuraDoPacote: "questao-44-" + figuraDoPacote + ".png"},
	}

	compararComGolden(t, "prova_pacote.json", forma(t, m))
}
