package mapa

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Pedido de mapa: o PDF da aula na fila da conta. Quem o atende é o
// edital-processor, com o Claude Code; o resultado volta ao backend, que o
// importa como a tela importaria.

// Situacao é onde o pedido está na fila.
type Situacao string

const (
	NaFila      Situacao = "na_fila"
	Processando Situacao = "processando"
	Pronto      Situacao = "pronto"
	Falhou      Situacao = "falhou"
)

const (
	// MaxBytesPDF é o teto de um PDF: a maior aula baixada até aqui tem 30 MB.
	// Os dois nginx do caminho (frontend/nginx.conf e o vhost da borda)
	// aceitam um pouco mais, para o multipart.
	MaxBytesPDF = 40 << 20 // 40 MiB
	// MaxNomePDF é o tamanho do nome do arquivo guardado.
	MaxNomePDF = 200
	// MaxRelatorio é o tamanho do relatório do processador.
	MaxRelatorio = 20000
	// GuardaDoPDFQueFalhou é quanto o PDF de um pedido que falhou fica no
	// servidor para voltar à fila sem reenviar; depois, a faxina o apaga.
	GuardaDoPDFQueFalhou = 7 * 24 * time.Hour
)

var (
	// ErrPedidoNaoEncontrado vale para o pedido que não existe, o de outra
	// conta e o PDF que já foi descartado.
	ErrPedidoNaoEncontrado = errors.New("pedido de mapa não encontrado")
	// ErrPedidoForaDeHora: o pedido não está na situação que a ação pede —
	// concluir o que ninguém está processando, pôr na fila o que já está nela.
	ErrPedidoForaDeHora = errors.New("o pedido mudou de situação — recarregue a página")
	// ErrPDFInvalido: o arquivo não é um PDF, está vazio ou é grande demais.
	ErrPDFInvalido = errors.New("o arquivo precisa ser um PDF de até 40 MB")
	// ErrPedidoSemPDF: o PDF do pedido que falhou já foi apagado pela faxina.
	ErrPedidoSemPDF = errors.New("o PDF deste pedido já foi apagado: envie-o de novo")
	// ErrRelatorioGrandeDemais: o relatório passa do teto.
	ErrRelatorioGrandeDemais = errors.New("o relatório do processador passa de 20.000 caracteres")
)

// Pedido é um pedido da conta como a fila o mostra.
type Pedido struct {
	ID       uuid.UUID
	Arquivo  string
	Situacao Situacao
	// A matéria em que o mapa vai morar, e o concurso dela; vazios quando o
	// pedido não escolheu matéria (ou ela saiu do concurso).
	DisciplinaID   uuid.NullUUID
	DisciplinaNome string
	ConcursoSlug   string
	// Mapa é o slug do mapa que o pedido gerou, quando pronto.
	Mapa         string
	Relatorio    string
	TemPDF       bool
	CriadoEm     time.Time
	AtualizadoEm time.Time
}

// ConferirPDF devolve o nome que o pedido guarda, ou ErrPDFInvalido. O tipo
// sai dos bytes — o PDF começa por "%PDF-" —, nunca do nome nem do navegador.
func ConferirPDF(nome string, dados []byte) (string, error) {
	if len(dados) == 0 || len(dados) > MaxBytesPDF || !bytes.HasPrefix(dados, []byte("%PDF-")) {
		return "", ErrPDFInvalido
	}

	// O navegador pode mandar o caminho inteiro; guarda-se só o nome, curto.
	nome = strings.TrimSpace(filepath.Base(strings.ReplaceAll(nome, `\`, "/")))
	if nome == "" || nome == "." || nome == "/" {
		nome = "aula.pdf"
	}

	if r := []rune(nome); len(r) > MaxNomePDF {
		nome = string(r[:MaxNomePDF])
	}

	return nome, nil
}

// ConferirRelatorio recusa o relatório que não cabe.
func ConferirRelatorio(r string) (string, error) {
	r = strings.TrimSpace(r)
	if utf8.RuneCountInString(r) > MaxRelatorio {
		return "", ErrRelatorioGrandeDemais
	}

	return r, nil
}

// ErrSlugDeOutroMapa: o resultado do processador usa o slug de um mapa que a
// conta já tem — importar o substituiria. Volta ao processador para trocar.
type ErrSlugDeOutroMapa struct{ Slug string }

func (e ErrSlugDeOutroMapa) Error() string {
	return "o slug \"" + e.Slug + "\" já é de outro mapa da conta e importar o substituiria: escolha outro"
}
