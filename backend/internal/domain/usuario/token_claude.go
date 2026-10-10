package usuario

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// O token do Claude é o de `claude setup-token`: o processador de mapas o usa para
// trabalhar com a assinatura de quem estuda. A tela o recebe uma vez e nunca o
// mostra de volta inteiro — só o fim, para reconhecer qual está guardado.

const maxTokenDoClaude = 2000

// ErrTokenDoClaudeInvalido: vazio, grande demais ou com espaço no meio — sinal
// de que veio colado errado (com aspas, a linha inteira, dois tokens).
var ErrTokenDoClaudeInvalido = errors.New("o token do Claude veio vazio ou com espaços: cole só o token que o `claude setup-token` imprime")

// ConferirTokenDoClaude devolve o token limpo das pontas, ou o erro.
func ConferirTokenDoClaude(t string) (string, error) {
	t = strings.TrimSpace(t)
	if t == "" || utf8.RuneCountInString(t) > maxTokenDoClaude || strings.IndexFunc(t, unicode.IsSpace) >= 0 {
		return "", ErrTokenDoClaudeInvalido
	}

	return t, nil
}

// FimDoToken é o que a tela mostra do token guardado: os quatro últimos caracteres.
func FimDoToken(t string) string {
	r := []rune(t)
	if len(r) <= 8 {
		return "…"
	}

	return "…" + string(r[len(r)-4:])
}

// A conexão do Claude é feita pela tela: o processador mostra o link de
// autorização, e quem estuda cola o código que a página do Claude mostra.

const maxCodigoDoClaude = 500

var (
	// ErrCodigoDoClaudeInvalido: o código veio vazio, enorme ou com espaço no meio.
	ErrCodigoDoClaudeInvalido = errors.New("cole o código que a página de autorização do Claude mostrou, inteiro e sem espaços")
	// ErrConexaoDoClaudeNaoIniciada: o código chegou sem um link pedido (ou o link venceu).
	ErrConexaoDoClaudeNaoIniciada = errors.New("este link de autorização já não vale: clique em \"Conectar o Claude\" para gerar outro")
	// ErrClaudeIndisponivel: o processador de mapas não respondeu.
	ErrClaudeIndisponivel = errors.New("o processador de mapas não respondeu agora: tente de novo daqui a pouco")
)

// ErrCodigoDoClaudeRecusado: o Claude não aceitou o código; o motivo diz o que fazer.
type ErrCodigoDoClaudeRecusado struct{ Motivo string }

func (e ErrCodigoDoClaudeRecusado) Error() string { return e.Motivo }

// ConferirCodigoDoClaude devolve o código limpo das pontas, ou o erro.
func ConferirCodigoDoClaude(c string) (string, error) {
	c = strings.TrimSpace(c)
	if c == "" || utf8.RuneCountInString(c) > maxCodigoDoClaude || strings.IndexFunc(c, unicode.IsSpace) >= 0 {
		return "", ErrCodigoDoClaudeInvalido
	}

	return c, nil
}
