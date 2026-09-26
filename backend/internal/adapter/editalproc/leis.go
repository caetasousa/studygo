package editalproc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"studygo/internal/domain/lei"
	"studygo/internal/port"
)

// A captura de leis mora no mesmo processador dos editais: mesma rede, mesmo
// token, mesmo disjuntor. Ver edital-processor/app/leis/README.md.

type wireCapturaIniciada struct {
	ID string `json:"id"`
}

type wireCaptura struct {
	ID        string `json:"id"`
	Estado    string `json:"estado"`
	Etapa     string `json:"etapa"`
	Progresso struct {
		Feitos int `json:"feitos"`
		Total  int `json:"total"`
	} `json:"progresso"`
	Erro      string                `json:"erro"`
	Resultado *wireResultadoCaptura `json:"resultado"`
}

type wireResultadoCaptura struct {
	Fonte        string               `json:"fonte"`
	Gemini       bool                 `json:"gemini"`
	Versao       *string              `json:"versao"`
	Dispositivos []wireDispositivoLei `json:"dispositivos"`
	Bloqueios    []string             `json:"bloqueios"`
	Avisos       []struct {
		ID     string `json:"id"`
		Texto  string `json:"texto"`
		Trecho string `json:"trecho"`
	} `json:"avisos"`
	Recorte []string `json:"recorte"`
	Resumo  struct {
		Vigentes    int            `json:"vigentes"`
		Anteriores  int            `json:"anteriores"`
		Notas       int            `json:"notas"`
		Revogados   int            `json:"revogados"`
		Tipos       map[string]int `json:"tipos"`
		Descartados []string       `json:"descartados"`
		Riscados    []string       `json:"riscados"`
		Juncoes     []string       `json:"juncoes"`
	} `json:"resumo"`
}

type wireDispositivoLei struct {
	Ref        string   `json:"ref"`
	Pai        *string  `json:"pai"`
	Tipo       string   `json:"tipo"`
	Rotulo     string   `json:"rotulo"`
	Nome       string   `json:"nome"`
	Texto      string   `json:"texto"`
	Notas      []string `json:"notas"`
	Anteriores []string `json:"anteriores"`
	Revogado   bool     `json:"revogado"`
}

// IniciarCaptura pede a captura do link e devolve o id para consultar.
func (c *Client) IniciarCaptura(ctx context.Context, dono, link string, recorte []string) (string, error) {
	corpo := map[string]any{"link": link}
	if len(recorte) > 0 {
		corpo["recorte"] = recorte
	}
	payload, _ := json.Marshal(corpo)

	var out wireCapturaIniciada
	if err := c.do(ctx, http.MethodPost, "/internal/leis/capturas", dono, "application/json", bytes.NewReader(payload), &out); err != nil {
		return "", erroDeCaptura(err)
	}

	return out.ID, nil
}

// Captura consulta o andamento e, pronta, a lei como o processador a leu.
func (c *Client) Captura(ctx context.Context, dono, id string) (lei.Captura, error) {
	var out wireCaptura
	if err := c.do(ctx, http.MethodGet, "/internal/leis/capturas/"+url.PathEscape(id), dono, "application/json", nil, &out); err != nil {
		return lei.Captura{}, erroDeCaptura(err)
	}

	captura := lei.Captura{
		ID: out.ID, Estado: out.Estado, Etapa: out.Etapa,
		Feitos: out.Progresso.Feitos, Total: out.Progresso.Total, Erro: out.Erro,
	}
	if r := out.Resultado; r != nil {
		res := &lei.ResultadoDaCaptura{
			Fonte: r.Fonte, Gemini: r.Gemini, Bloqueios: r.Bloqueios, Recorte: r.Recorte,
			Resumo: lei.ResumoDaCaptura{
				Vigentes: r.Resumo.Vigentes, Anteriores: r.Resumo.Anteriores, Notas: r.Resumo.Notas,
				Revogados: r.Resumo.Revogados, Tipos: r.Resumo.Tipos, Descartados: r.Resumo.Descartados,
				Riscados: r.Resumo.Riscados, Juncoes: r.Resumo.Juncoes,
			},
		}
		if r.Versao != nil {
			res.Versao = *r.Versao
		}
		for _, a := range r.Avisos {
			res.Avisos = append(res.Avisos, lei.Aviso{ID: a.ID, Texto: a.Texto, Trecho: a.Trecho})
		}
		for _, d := range r.Dispositivos {
			pai := ""
			if d.Pai != nil {
				pai = *d.Pai
			}
			res.Dispositivos = append(res.Dispositivos, lei.Dispositivo{
				Ref: d.Ref, Pai: pai, Tipo: d.Tipo, Rotulo: d.Rotulo, Nome: d.Nome, Texto: d.Texto,
				Notas: d.Notas, Anteriores: d.Anteriores, Revogado: d.Revogado,
			})
		}
		captura.Resultado = res
	}

	return captura, nil
}

type wirePesquisa struct {
	Fonte     string `json:"fonte"`
	Link      string `json:"link"`
	Epigrafe  string `json:"epigrafe"`
	Estrutura []struct {
		Ref      string  `json:"ref"`
		Pai      *string `json:"pai"`
		Tipo     string  `json:"tipo"`
		Rotulo   string  `json:"rotulo"`
		Nome     string  `json:"nome"`
		Texto    string  `json:"texto"`
		Revogado bool    `json:"revogado"`
	} `json:"estrutura"`
}

// Pesquisar acha a fonte da norma que o tópico cita e devolve a estrutura dela.
func (c *Client) Pesquisar(ctx context.Context, dono, tema, link string) (lei.Pesquisa, error) {
	corpo := map[string]string{"tema": tema}
	if link != "" {
		corpo["link"] = link
	}
	payload, _ := json.Marshal(corpo)

	var out wirePesquisa
	if err := c.do(ctx, http.MethodPost, "/internal/leis/pesquisas", dono, "application/json", bytes.NewReader(payload), &out); err != nil {
		return lei.Pesquisa{}, erroDeCaptura(err)
	}

	p := lei.Pesquisa{Fonte: out.Fonte, Link: out.Link, Epigrafe: out.Epigrafe}
	for _, d := range out.Estrutura {
		pai := ""
		if d.Pai != nil {
			pai = *d.Pai
		}
		p.Estrutura = append(p.Estrutura, lei.Dispositivo{
			Ref: d.Ref, Pai: pai, Tipo: d.Tipo, Rotulo: d.Rotulo, Nome: d.Nome, Texto: d.Texto, Revogado: d.Revogado,
		})
	}

	return p, nil
}

// erroDeCaptura traduz a resposta do processador em erro da lei.
func erroDeCaptura(err error) error {
	var r recusa
	switch {
	case errors.As(err, &r) && r.Codigo == "fonte_invalida":
		return lei.ErrLinkInvalido{Motivo: r.Mensagem}
	case errors.As(err, &r) && r.Codigo == "captura_nao_encontrada":
		return lei.ErrCapturaNaoEncontrada
	case errors.As(err, &r) && r.Codigo == "fonte_nao_encontrada":
		return lei.ErrFonteNaoEncontrada
	case errors.Is(err, port.ErrProvedorIndisponivel):
		return fmt.Errorf("%w: %w", lei.ErrCapturaIndisponivel, err)
	default:
		return err
	}
}
