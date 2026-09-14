// Package provaproc é o cliente das rotas internas de provas do
// edital-processor.
//
// O processador responde com os nomes de campo do domínio (Numero, Blocos,
// Retangulo…), de propósito: o contrato interno foi desenhado para decodificar
// direto em prova.Rascunho, sem uma camada de tradução que só copiaria campo a
// campo. Quem renomear um campo do domínio precisa renomear o alias em
// edital-processor/app/provas/schemas.py.
package provaproc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"studygo/internal/domain/prova"
	"studygo/internal/port"
)

var _ port.ProvaProcessor = (*Client)(nil)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New constrói o cliente. O timeout cobre a etapa mais longa — uma região com
// dez questões e texto de apoio leva perto de um minuto no Gemini — com folga
// para a tentativa única que o processador faz.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 220 * time.Second},
	}
}

type pedido struct {
	Documento string        `json:"documento"`
	Origem    *prova.Origem `json:"origem,omitempty"`
	// Questao é o número que a releitura procura na página.
	Questao int `json:"questao,omitempty"`
}

type wireErro struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Transient bool   `json:"transient"`
}

func (c *Client) chamar(ctx context.Context, rota string, p any, out any) error {
	corpo, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("codificando pedido ao processador: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/provas/"+rota, bytes.NewReader(corpo))
	if err != nil {
		return fmt.Errorf("montando pedido ao processador: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", port.ErrProcessamentoTransitorio, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("%w: lendo resposta: %w", port.ErrProcessamentoTransitorio, err)
	}
	if resp.StatusCode >= 400 {
		return traduzirErro(resp.StatusCode, payload)
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("%w: resposta inválida do processador: %w", port.ErrProcessamentoTransitorio, err)
	}

	return nil
}

// traduzirErro separa falha que passa sozinha de recusa do documento. Só a
// recusa é definitiva; na dúvida, a fila tenta de novo.
func traduzirErro(status int, payload []byte) error {
	var we wireErro
	_ = json.Unmarshal(payload, &we)
	msg := we.Message
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", status)
	}

	if we.Transient || status >= 500 || status == http.StatusTooManyRequests {
		return fmt.Errorf("%w: %s", port.ErrProcessamentoTransitorio, msg)
	}

	return fmt.Errorf("%w: %s", port.ErrDocumentoRecusado, msg)
}

func (c *Client) Preparar(ctx context.Context, documento string) ([]prova.Origem, error) {
	var regioes []prova.Origem
	err := c.chamar(ctx, "preparar", pedido{Documento: documento}, &regioes)

	return regioes, err
}

func (c *Client) Metadados(ctx context.Context, documento string, capa prova.Origem) (prova.Metadados, error) {
	var m prova.Metadados
	err := c.chamar(ctx, "metadados", pedido{Documento: documento, Origem: &capa}, &m)

	return m, err
}

func (c *Client) Extrair(ctx context.Context, documento string, regiao prova.Origem) (prova.Rascunho, error) {
	var r prova.Rascunho
	p := pedido{Documento: documento, Origem: &regiao}
	if n, ok := prova.QuestaoDaReleitura(regiao); ok {
		p.Questao = n
	}
	err := c.chamar(ctx, "extrair", p, &r)

	return r, err
}

func (c *Client) Gabarito(ctx context.Context, arquivo string) (prova.Gabarito, error) {
	var g prova.Gabarito
	err := c.chamar(ctx, "gabarito", pedido{Documento: arquivo}, &g)

	return g, err
}

func (c *Client) Classificar(ctx context.Context, questoes []prova.ResumoDeQuestao) (map[int]string, error) {
	var out struct {
		Materias []struct {
			Numero  int
			Materia string
		}
	}
	if err := c.chamar(ctx, "classificar", map[string]any{"questoes": questoes}, &out); err != nil {
		return nil, err
	}

	materias := make(map[int]string, len(out.Materias))
	for _, m := range out.Materias {
		materias[m.Numero] = m.Materia
	}

	return materias, nil
}

func (c *Client) Recortar(ctx context.Context, documento string, o prova.Origem) (string, error) {
	var out struct {
		Arquivo string `json:"arquivo"`
	}
	if err := c.chamar(ctx, "recortar", pedido{Documento: documento, Origem: &o}, &out); err != nil {
		return "", err
	}

	return out.Arquivo, nil
}
