package editalproc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"

	"studygo/internal/domain/usuario"
	"studygo/internal/port"
)

// Os mapas mentais também moram no mesmo processador: o PDF da aula vai por
// aqui, e o mapa volta pela rota interna do backend quando o Claude termina.
// Ver edital-processor/app/mapas/README.md.

var _ port.ProcessadorDeMapas = (*Client)(nil)

type wireTrabalhoDeMapa struct {
	Pedido          string   `json:"pedido"`
	Arquivo         string   `json:"arquivo"`
	Materia         string   `json:"materia"`
	Temas           []string `json:"temas"`
	SlugsExistentes []string `json:"slugsExistentes"`
	TokenClaude     string   `json:"tokenClaude,omitempty"`
}

type wireTrabalhoAceito struct {
	Pedido string `json:"pedido"`
}

// Processar entrega o PDF e o contexto do mapa; o processador responde 202 e
// trabalha em segundo plano.
func (c *Client) Processar(ctx context.Context, t port.TrabalhoDeMapa) error {
	dados, err := json.Marshal(wireTrabalhoDeMapa{
		Pedido: t.Pedido.String(), Arquivo: t.Arquivo, Materia: t.Materia, Temas: t.Temas,
		SlugsExistentes: t.SlugsExistentes, TokenClaude: t.TokenClaude,
	})
	if err != nil {
		return fmt.Errorf("montando o trabalho: %w", err)
	}

	var buf bytes.Buffer

	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("dados", string(dados)); err != nil {
		return fmt.Errorf("montando o trabalho: %w", err)
	}

	fw, err := mw.CreateFormFile("pdf", "aula.pdf")
	if err != nil {
		return fmt.Errorf("montando o trabalho: %w", err)
	}

	if _, err := fw.Write(t.PDF); err != nil {
		return fmt.Errorf("montando o trabalho: %w", err)
	}

	if err := mw.Close(); err != nil {
		return fmt.Errorf("montando o trabalho: %w", err)
	}

	var out wireTrabalhoAceito

	return c.do(ctx, http.MethodPost, "/internal/mapas/processamentos", t.Dono, mw.FormDataContentType(), &buf, &out)
}

type wireConexaoDoClaude struct {
	Conectado bool   `json:"conectado"`
	Email     string `json:"email"`
	Plano     string `json:"plano"`
}

type wireLinkDoClaude struct {
	URL string `json:"url"`
}

func (w wireConexaoDoClaude) conexao() port.ConexaoDoClaude {
	return port.ConexaoDoClaude{Conectado: w.Conectado, Email: w.Email, Plano: w.Plano}
}

// ConectarClaude começa o login do Claude da conta e devolve o link de autorização.
func (c *Client) ConectarClaude(ctx context.Context, dono string) (string, error) {
	var out wireLinkDoClaude
	if err := c.do(ctx, http.MethodPost, "/internal/claude/conexao", dono, "application/json", bytes.NewReader([]byte("{}")), &out); err != nil {
		return "", erroDaConexao(err)
	}

	return out.URL, nil
}

// ConcluirConexaoDoClaude entrega o código que a página de autorização mostrou.
func (c *Client) ConcluirConexaoDoClaude(ctx context.Context, dono, codigo string) (port.ConexaoDoClaude, error) {
	payload, _ := json.Marshal(map[string]string{"codigo": codigo})

	var out wireConexaoDoClaude
	if err := c.do(ctx, http.MethodPost, "/internal/claude/conexao/codigo", dono, "application/json", bytes.NewReader(payload), &out); err != nil {
		return port.ConexaoDoClaude{}, erroDaConexao(err)
	}

	return out.conexao(), nil
}

func (c *Client) ConexaoDoClaude(ctx context.Context, dono string) (port.ConexaoDoClaude, error) {
	var out wireConexaoDoClaude
	if err := c.do(ctx, http.MethodGet, "/internal/claude", dono, "application/json", nil, &out); err != nil {
		return port.ConexaoDoClaude{}, erroDaConexao(err)
	}

	return out.conexao(), nil
}

func (c *Client) DesconectarClaude(ctx context.Context, dono string) error {
	if err := c.do(ctx, http.MethodDelete, "/internal/claude", dono, "application/json", nil, nil); err != nil {
		return erroDaConexao(err)
	}

	return nil
}

// erroDaConexao traduz a recusa do processador num erro que a tela explica.
func erroDaConexao(err error) error {
	var r recusa
	switch {
	case errors.As(err, &r) && r.Codigo == "codigo_recusado":
		return usuario.ErrCodigoDoClaudeRecusado{Motivo: r.Mensagem}
	case errors.As(err, &r) && r.Codigo == "conexao_nao_iniciada":
		return usuario.ErrConexaoDoClaudeNaoIniciada
	case errors.Is(err, port.ErrProvedorIndisponivel):
		return fmt.Errorf("%w: %w", usuario.ErrClaudeIndisponivel, err)
	default:
		return err
	}
}
