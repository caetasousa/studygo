package httpapi

// O pacote de prova: um .zip com manifest.json e arquivos/<uuid>.<pdf|png>.
// É o formato que sai de um ambiente e entra em outro, então é contrato — o
// snapshot em testdata/prova_pacote.json falha se o manifesto mudar.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"studygo/internal/domain/prova"
	"studygo/internal/service"
)

// formatoDoPacote muda quando o manifesto muda de um jeito que o importador
// antigo não entende.
const formatoDoPacote = "studygo.prova/1"

type pacoteDeProvaDTO struct {
	Formato     string      `json:"formato"`
	ExportadoEm time.Time   `json:"exportadoEm"`
	Prova       rascunhoDTO `json:"prova"`
	Regioes     []origemDTO `json:"regioes"`
	// Documento e Gabarito são nomes dentro de arquivos/, como "<uuid>.pdf".
	Documento     string `json:"documento"`
	Gabarito      string `json:"gabarito"`
	NomeDocumento string `json:"nomeDocumento"`
	NomeGabarito  string `json:"nomeGabarito"`
}

var arquivoDoPacote = regexp.MustCompile(`^arquivos/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.(pdf|png)$`)

// Teto do manifest.json: sessenta questões com texto de apoio ficam longe de 1 MiB.
const maxManifesto = 8 << 20

// escreverPacote grava o .zip. PDF e PNG já vêm comprimidos: vão sem compressão.
func escreverPacote(w io.Writer, p service.ProvaParaLevar, agora time.Time) error {
	manifesto := pacoteDeProvaDTO{
		Formato: formatoDoPacote, ExportadoEm: agora.UTC(),
		Prova: rascunhoParaDTO(p.Conteudo), Regioes: origensParaDTO(p.Regioes),
		Documento: p.Documento + ".pdf", NomeDocumento: p.NomeDocumento, NomeGabarito: p.NomeGabarito,
	}
	if p.GabaritoArquivo != "" {
		manifesto.Gabarito = p.GabaritoArquivo + ".pdf"
	}

	zw := zip.NewWriter(w)
	f, err := zw.Create("manifest.json")
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifesto); err != nil {
		return err
	}
	for _, nome := range slices.Sorted(func(yield func(string) bool) {
		for k := range p.Arquivos {
			if !yield(k) {
				return
			}
		}
	}) {
		if err := copiarParaOZip(zw, "arquivos/"+nome, p.Arquivos[nome]); err != nil {
			return err
		}
	}

	return zw.Close()
}

func copiarParaOZip(zw *zip.Writer, nome, caminho string) error {
	origem, err := os.Open(caminho) //nolint:gosec // caminho montado pelo volume a partir de uuid
	if err != nil {
		return err
	}
	defer origem.Close()
	destino, err := zw.CreateHeader(&zip.FileHeader{Name: nome, Method: zip.Store, Modified: time.Now()})
	if err != nil {
		return err
	}
	_, err = io.Copy(destino, origem)

	return err
}

// lerPacote desmonta o .zip enviado. Só entra o que o formato prevê — nome
// fora do padrão, arquivo acima do teto ou total acima do dobro dele recusam
// o pacote inteiro: é o que impede caminho forjado e bomba de descompressão.
func lerPacote(dados []byte, maxArquivo int64) (service.PacoteDeProva, error) {
	zr, err := zip.NewReader(bytes.NewReader(dados), int64(len(dados)))
	if err != nil {
		return service.PacoteDeProva{}, errPacote("o arquivo não é um .zip de prova")
	}

	var manifesto []byte
	arquivos := map[string][]byte{}
	var total int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		limite := maxArquivo
		if f.Name == "manifest.json" {
			limite = maxManifesto
		} else if !arquivoDoPacote.MatchString(f.Name) {
			return service.PacoteDeProva{}, errPacote(fmt.Sprintf("o pacote tem um arquivo que não é de prova: %q", f.Name))
		}
		if f.UncompressedSize64 > uint64(limite) { //nolint:gosec // limite positivo
			return service.PacoteDeProva{}, errPacote(fmt.Sprintf("%s passa do tamanho máximo", f.Name))
		}
		conteudo, err := lerDoZip(f, limite)
		if err != nil {
			return service.PacoteDeProva{}, err
		}
		if total += int64(len(conteudo)); total > 2*maxArquivo+maxManifesto {
			return service.PacoteDeProva{}, errPacote("o pacote descompactado passa do tamanho máximo")
		}
		if f.Name == "manifest.json" {
			manifesto = conteudo
		} else {
			arquivos[strings.TrimPrefix(f.Name, "arquivos/")] = conteudo
		}
	}
	if manifesto == nil {
		return service.PacoteDeProva{}, errPacote("o pacote não tem manifest.json")
	}

	var m pacoteDeProvaDTO
	if err := json.Unmarshal(manifesto, &m); err != nil {
		return service.PacoteDeProva{}, errPacote("o manifest.json do pacote não é válido")
	}
	if m.Formato != formatoDoPacote {
		return service.PacoteDeProva{}, errPacote(fmt.Sprintf("formato de pacote desconhecido: %q", m.Formato))
	}

	pct := service.PacoteDeProva{
		Conteudo: rascunhoDoDTO(m.Prova), Documento: arquivos[m.Documento],
		NomeDocumento: m.NomeDocumento, NomeGabarito: m.NomeGabarito,
		Figuras: map[string][]byte{},
	}
	if m.Gabarito != "" {
		pct.Gabarito = arquivos[m.Gabarito]
	}
	for _, o := range m.Regioes {
		pct.Regioes = append(pct.Regioes, origemDoDTO(o))
	}
	for nome, conteudo := range arquivos {
		if id, ok := strings.CutSuffix(nome, ".png"); ok {
			pct.Figuras[id] = conteudo
		}
	}

	return pct, nil
}

func lerDoZip(f *zip.File, limite int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, errPacote(fmt.Sprintf("não consegui ler %s do pacote", f.Name))
	}
	defer rc.Close()
	conteudo, err := io.ReadAll(io.LimitReader(rc, limite+1))
	if err != nil {
		return nil, errPacote(fmt.Sprintf("não consegui ler %s do pacote", f.Name))
	}
	if int64(len(conteudo)) > limite {
		return nil, errPacote(fmt.Sprintf("%s passa do tamanho máximo", f.Name))
	}

	return conteudo, nil
}

// nomeDoPacote é o nome do .zip baixado: "prova-trt-18-2023-l12.zip".
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

// errPacote é a recusa do pacote enviado: 422 com o motivo, que o curador lê.
func errPacote(msg string) error { return service.ErrValidacao{Msg: msg} }
