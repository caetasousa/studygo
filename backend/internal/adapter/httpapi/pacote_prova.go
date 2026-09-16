package httpapi

// O pacote de provas: um .zip só, com uma pasta por prova —
//
//	LEIA-ME.txt
//	tjce-2026-e05/prova.json        o conteúdo publicado, as regiões e os nomes
//	tjce-2026-e05/prova.pdf         o caderno original
//	tjce-2026-e05/gabarito.pdf
//	tjce-2026-e05/figuras/questao-11-<uuid>.png
//
// Tudo sem compressão (PDF e PNG já vêm comprimidos): o navegador lê o .zip
// sem biblioteca e envia uma prova por vez, porque o arquivo inteiro passaria
// do limite de envio do servidor. O prova.json é contrato entre ambientes — o
// snapshot em testdata/prova_pacote.json falha se ele mudar.

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"studygo/internal/domain/prova"
	"studygo/internal/service"
)

// formatoDoPacote muda quando o prova.json muda de um jeito que o importador
// antigo não entende.
const formatoDoPacote = "studygo.prova/2"

type provaDoPacoteDTO struct {
	Formato     string      `json:"formato"`
	ExportadoEm time.Time   `json:"exportadoEm"`
	Prova       rascunhoDTO `json:"prova"`
	Regioes     []origemDTO `json:"regioes"`
	// Os arquivos da pasta da prova, pelo nome.
	PDFDaProva    string `json:"pdfDaProva"`
	PDFDoGabarito string `json:"pdfDoGabarito"`
	NomeDocumento string `json:"nomeDocumento"`
	NomeGabarito  string `json:"nomeGabarito"`
	// Figuras leva o id que os blocos citam ao arquivo em figuras/.
	Figuras map[string]string `json:"figuras"`
}

const leiaMe = `Provas exportadas do studygo.

Cada pasta é uma prova publicada: prova.json (questões, textos de apoio e
gabarito), prova.pdf (o caderno original), gabarito.pdf e figuras/, com as
imagens das questões pelo número.

Para levar a outro ambiente, importe ESTE .zip inteiro em Questões > Curadoria >
"Importar provas" no ambiente de destino. Não descompacte nem troque os nomes.
`

// Teto do prova.json: sessenta questões com texto de apoio ficam longe de 1 MiB.
const maxManifesto = 8 << 20

// Nomes que um envio de prova pode ter: os PDFs e as figuras que o prova.json cita.
var nomeNoEnvio = regexp.MustCompile(`^(?:prova\.pdf|gabarito\.pdf|[a-z0-9-]*[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.png)$`)

// escreverPacote grava o .zip com as provas, cada uma na sua pasta.
func escreverPacote(w io.Writer, provas []service.ProvaParaLevar, agora time.Time) error {
	zw := zip.NewWriter(w)
	if err := entradaDoZip(zw, "LEIA-ME.txt", strings.NewReader(leiaMe)); err != nil {
		return err
	}
	pastas := map[string]bool{}
	for _, p := range provas {
		pasta := pastaDaProva(p.Conteudo, pastas)
		manifesto := provaDoPacoteDTO{
			Formato: formatoDoPacote, ExportadoEm: agora.UTC(),
			Prova: rascunhoParaDTO(p.Conteudo), Regioes: origensParaDTO(p.Regioes),
			PDFDaProva: "prova.pdf", NomeDocumento: p.NomeDocumento, NomeGabarito: p.NomeGabarito,
			Figuras: map[string]string{},
		}
		if p.GabaritoArquivo != "" {
			manifesto.PDFDoGabarito = "gabarito.pdf"
		}
		for _, id := range p.Conteudo.Arquivos() {
			manifesto.Figuras[id] = nomeDaFigura(p.Conteudo, id)
		}

		json, err := json.MarshalIndent(manifesto, "", "  ")
		if err != nil {
			return err
		}
		if err := entradaDoZip(zw, pasta+"/prova.json", strings.NewReader(string(json))); err != nil {
			return err
		}
		if err := copiarParaOZip(zw, pasta+"/prova.pdf", p.Arquivos[p.Documento+".pdf"]); err != nil {
			return err
		}
		if p.GabaritoArquivo != "" {
			if err := copiarParaOZip(zw, pasta+"/gabarito.pdf", p.Arquivos[p.GabaritoArquivo+".pdf"]); err != nil {
				return err
			}
		}
		for _, id := range slices.Sorted(func(yield func(string) bool) {
			for k := range manifesto.Figuras {
				if !yield(k) {
					return
				}
			}
		}) {
			if err := copiarParaOZip(zw, pasta+"/figuras/"+manifesto.Figuras[id], p.Arquivos[id+".png"]); err != nil {
				return err
			}
		}
	}

	return zw.Close()
}

func entradaDoZip(zw *zip.Writer, nome string, conteudo io.Reader) error {
	destino, err := zw.CreateHeader(&zip.FileHeader{Name: nome, Method: zip.Store, Modified: time.Now()})
	if err != nil {
		return err
	}
	_, err = io.Copy(destino, conteudo)

	return err
}

func copiarParaOZip(zw *zip.Writer, nome, caminho string) error {
	origem, err := os.Open(caminho) //nolint:gosec // caminho montado pelo volume a partir de uuid
	if err != nil {
		return err
	}
	defer origem.Close()

	return entradaDoZip(zw, nome, origem)
}

// pastaDaProva é "tjce-2026-e05"; a segunda com o mesmo nome ganha "-2".
func pastaDaProva(r prova.Rascunho, usadas map[string]bool) string {
	base := strings.TrimSuffix(strings.TrimPrefix(nomeDoPacote(r), "prova-"), ".zip")
	if base == "prova.zip" || base == "" {
		base = "prova"
	}
	pasta := base
	for n := 2; usadas[pasta]; n++ {
		pasta = base + "-" + strconv.Itoa(n)
	}
	usadas[pasta] = true

	return pasta
}

// nomeDaFigura diz de onde a figura é — "questao-11-<uuid>.png" ou
// "texto-<uuid>.png" —, para quem abre o .zip reconhecer as imagens.
func nomeDaFigura(r prova.Rascunho, id string) string {
	usa := func(bs []prova.Bloco) bool {
		return slices.ContainsFunc(bs, func(b prova.Bloco) bool { return b.Arquivo == id })
	}
	for _, q := range r.Questoes {
		if usa(q.Blocos) || slices.ContainsFunc(q.Alternativas, func(a prova.Alternativa) bool { return usa(a.Blocos) }) {
			return fmt.Sprintf("questao-%d-%s.png", q.Numero, id)
		}
	}

	return "texto-" + id + ".png"
}

// provaDoEnvio monta a prova de uma pasta do pacote, que o navegador envia
// como o prova.json e os arquivos pelo nome. Nome fora do que o formato prevê
// recusa o envio.
func provaDoEnvio(manifesto []byte, arquivos map[string][]byte) (service.PacoteDeProva, error) {
	for nome := range arquivos {
		if !nomeNoEnvio.MatchString(nome) {
			return service.PacoteDeProva{}, errPacote(fmt.Sprintf("o envio tem um arquivo que não é de prova: %q", nome))
		}
	}
	var m provaDoPacoteDTO
	if err := json.Unmarshal(manifesto, &m); err != nil {
		return service.PacoteDeProva{}, errPacote("o prova.json não é válido")
	}
	if m.Formato != formatoDoPacote {
		return service.PacoteDeProva{}, errPacote(fmt.Sprintf(
			"formato de pacote desconhecido: %q; exporte de novo do outro ambiente", m.Formato,
		))
	}

	pct := service.PacoteDeProva{
		Conteudo: rascunhoDoDTO(m.Prova), Documento: arquivos[m.PDFDaProva],
		NomeDocumento: m.NomeDocumento, NomeGabarito: m.NomeGabarito,
		Figuras: map[string][]byte{},
	}
	if m.PDFDoGabarito != "" {
		pct.Gabarito = arquivos[m.PDFDoGabarito]
	}
	for _, o := range m.Regioes {
		pct.Regioes = append(pct.Regioes, origemDoDTO(o))
	}
	for id, nome := range m.Figuras {
		pct.Figuras[id] = arquivos[nome]
	}

	return pct, nil
}

// nomeDoPacote é o nome do .zip de uma prova só: "prova-trt-18-2023-l12.zip".
func nomeDoPacote(r prova.Rascunho) string {
	partes := []string{"prova"}
	for _, p := range []string{r.Orgao, fmt.Sprint(r.Ano), r.Cargo} {
		// Órgão, ano e código do cargo são ASCII; o resto sai do nome.
		var b strings.Builder
		for _, c := range strings.ToLower(p) {
			switch {
			case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
				b.WriteRune(c)
			case c == ' ' || c == '-' || c == '_':
				b.WriteRune('-')
			}
		}
		if s := strings.Trim(b.String(), "-"); s != "" && s != "0" {
			partes = append(partes, s)
		}
	}

	return strings.Join(partes, "-") + ".zip"
}

// errPacote é a recusa do envio: 422 com o motivo, que o curador lê.
func errPacote(msg string) error { return service.ErrValidacao{Msg: msg} }
