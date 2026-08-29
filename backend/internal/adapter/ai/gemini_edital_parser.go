package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"annygo/internal/port"
)

var _ port.EditalParser = (*GeminiEditalParser)(nil)

const geminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s"

const editalPrompt = `Você recebe o edital (ou um trecho) de um concurso público brasileiro.
Extraia SOMENTE o que estiver no texto, em JSON, seguindo o schema.

Regras:
- "nome": nome curto do concurso (órgão + cargo), ex.: "TJ-SP — Escrevente Técnico Judiciário".
- "prova": data da aplicação das provas objetivas no formato AAAA-MM-DD. Se o edital não traz
  data definida, deixe "".
- Para cada disciplina das provas objetivas: "nome", "questoes" (nº de questões; 0 se o edital
  não especifica por disciplina), "temas" (cada item do conteúdo programático como uma linha
  curta) e "bloco":
    - "ger" para Conhecimentos Gerais / Básicos (Língua Portuguesa, Matemática/RLM, Informática
      básica, Atualidades, Legislação geral/institucional comum a vários cargos).
    - "esp" para Conhecimentos Específicos do cargo.
- "marcos": datas do cronograma do edital (inscrição, isenção, pagamento, convocação, recursos,
  resultado…). "exigeAcao" = true quando a data exige ação do candidato (inscrever-se, pagar,
  recorrer). "dataFim" só quando for um período.
- Não invente dados. Campos desconhecidos ficam vazios / 0 / [].`

// GeminiEditalParser reads an edital with the Gemini generateContent API.
type GeminiEditalParser struct {
	apiKey string
	model  string
	http   *http.Client
}

func NewGeminiEditalParser(apiKey, model string) *GeminiEditalParser {
	if model == "" {
		model = "gemini-3.6-flash"
	}

	return &GeminiEditalParser{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: 130 * time.Second},
	}
}

func (p *GeminiEditalParser) Disponivel() bool { return p.apiKey != "" }

func (p *GeminiEditalParser) Parse(ctx context.Context, in port.EditalEntrada) (port.EditalExtraido, error) {
	if strings.TrimSpace(in.Texto) == "" && len(in.PDF) == 0 {
		return port.EditalExtraido{}, fmt.Errorf("edital vazio")
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	parts := []map[string]any{{"text": editalPrompt}}

	if len(in.PDF) > 0 {
		mime := in.MIME
		if mime == "" {
			mime = "application/pdf"
		}

		parts = append(parts, map[string]any{
			"inline_data": map[string]any{
				"mime_type": mime,
				"data":      base64.StdEncoding.EncodeToString(in.PDF),
			},
		})
	} else {
		parts = append(parts, map[string]any{"text": "EDITAL:\n" + in.Texto})
	}

	body := map[string]any{
		"contents": []map[string]any{{"role": "user", "parts": parts}},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"responseSchema":   editalSchema(),
			"temperature":      0.1,
		},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return port.EditalExtraido{}, fmt.Errorf("encoding request: %w", err)
	}

	url := fmt.Sprintf(geminiEndpoint, p.model, p.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return port.EditalExtraido{}, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return port.EditalExtraido{}, fmt.Errorf("calling gemini: %w", err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	if resp.StatusCode != http.StatusOK {
		return port.EditalExtraido{}, fmt.Errorf("gemini respondeu %d: %s", resp.StatusCode, snippet(payload))
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return port.EditalExtraido{}, fmt.Errorf("decoding gemini response: %w", err)
	}

	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return port.EditalExtraido{}, fmt.Errorf("gemini não retornou conteúdo")
	}

	return decodeExtraido(parsed.Candidates[0].Content.Parts[0].Text)
}

func decodeExtraido(jsonText string) (port.EditalExtraido, error) {
	var e struct {
		Nome            string `json:"nome"`
		Banca           string `json:"banca"`
		Cargo           string `json:"cargo"`
		Orgao           string `json:"orgao"`
		Prova           string `json:"prova"`
		ProvaDiscursiva bool   `json:"provaDiscursiva"`
		Disciplinas     []struct {
			Nome     string   `json:"nome"`
			Bloco    string   `json:"bloco"`
			Questoes int      `json:"questoes"`
			Temas    []string `json:"temas"`
		} `json:"disciplinas"`
		Marcos []struct {
			Data      string `json:"data"`
			DataFim   string `json:"dataFim"`
			Titulo    string `json:"titulo"`
			ExigeAcao bool   `json:"exigeAcao"`
		} `json:"marcos"`
	}

	if err := json.Unmarshal([]byte(jsonText), &e); err != nil {
		return port.EditalExtraido{}, fmt.Errorf("gemini devolveu json inválido: %w", err)
	}

	out := port.EditalExtraido{
		Nome:            e.Nome,
		Banca:           e.Banca,
		Cargo:           e.Cargo,
		Orgao:           e.Orgao,
		Prova:           e.Prova,
		ProvaDiscursiva: e.ProvaDiscursiva,
		Disciplinas:     make([]port.EditalDisciplina, 0, len(e.Disciplinas)),
		Marcos:          make([]port.EditalMarco, 0, len(e.Marcos)),
	}

	for _, d := range e.Disciplinas {
		out.Disciplinas = append(out.Disciplinas, port.EditalDisciplina{
			Nome:     d.Nome,
			Bloco:    d.Bloco,
			Questoes: d.Questoes,
			Temas:    d.Temas,
		})
	}

	for _, m := range e.Marcos {
		out.Marcos = append(out.Marcos, port.EditalMarco{
			Data:      m.Data,
			DataFim:   m.DataFim,
			Titulo:    m.Titulo,
			ExigeAcao: m.ExigeAcao,
		})
	}

	return out, nil
}

func editalSchema() map[string]any {
	str := map[string]any{"type": "STRING"}

	return map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"nome":            str,
			"banca":           str,
			"cargo":           str,
			"orgao":           str,
			"prova":           str,
			"provaDiscursiva": map[string]any{"type": "BOOLEAN"},
			"disciplinas": map[string]any{
				"type": "ARRAY",
				"items": map[string]any{
					"type": "OBJECT",
					"properties": map[string]any{
						"nome":     str,
						"bloco":    map[string]any{"type": "STRING", "enum": []string{"esp", "ger"}},
						"questoes": map[string]any{"type": "INTEGER"},
						"temas":    map[string]any{"type": "ARRAY", "items": str},
					},
					"required": []string{"nome", "bloco"},
				},
			},
			"marcos": map[string]any{
				"type": "ARRAY",
				"items": map[string]any{
					"type": "OBJECT",
					"properties": map[string]any{
						"data":      str,
						"dataFim":   str,
						"titulo":    str,
						"exigeAcao": map[string]any{"type": "BOOLEAN"},
					},
					"required": []string{"data", "titulo"},
				},
			},
		},
		"required": []string{"nome", "disciplinas"},
	}
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		return s[:400] + "…"
	}

	return s
}
