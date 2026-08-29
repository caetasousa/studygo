// Package ai adapts AI providers to the outbound ports. Today: a Gemini
// analyser that reads editais in focused steps, and a null analyser for when no
// key is configured.
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

var _ port.EditalAnalisador = (*GeminiAnalisador)(nil)

const geminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s"

// GeminiAnalisador reads an edital with the Gemini generateContent API.
type GeminiAnalisador struct {
	apiKey  string
	model   string
	http    *http.Client
	baseURL string // fmt template: "…/models/%s:generateContent?key=%s"
	// uploadURL / filesURL back the Files API, used so a big PDF is uploaded
	// once and referenced by URI on the later wizard steps.
	uploadURL    string
	filesURL     string
	backoff      time.Duration
	pollInterval time.Duration
}

func NewGeminiAnalisador(apiKey, model string) *GeminiAnalisador {
	if model == "" {
		model = "gemini-flash-lite-latest"
	}

	return &GeminiAnalisador{
		apiKey:       apiKey,
		model:        model,
		http:         &http.Client{Timeout: 210 * time.Second},
		baseURL:      geminiEndpoint,
		uploadURL:    uploadEndpoint,
		filesURL:     filesEndpoint,
		backoff:      2 * time.Second,
		pollInterval: time.Second,
	}
}

func (g *GeminiAnalisador) Disponivel() bool { return g.apiKey != "" }

// ---- step 1: cargos ----

const promptCargos = `Você recebe um edital de concurso público brasileiro.
Liste em JSON a banca organizadora e TODOS os cargos/especialidades oferecidos.
Para cada cargo: "codigo" (código de opção, ex.: "B02"), "nome" (nome completo),
"escolaridade" (exigência de escolaridade) e "vagas" (nº total de vagas; 0 se não informado).`

func (g *GeminiAnalisador) Cargos(ctx context.Context, in port.EditalEntrada) (port.EditalCargos, error) {
	if in.Vazia() {
		return port.EditalCargos{}, fmt.Errorf("edital vazio")
	}

	// A raw PDF goes up once via the Files API; every later step then references
	// the URI instead of re-uploading the whole file.
	var arq arquivoRemoto
	if len(in.PDF) > 0 && in.ArquivoURI == "" {
		var err error
		if arq, err = g.enviarArquivo(ctx, in.PDF, in.MIME); err != nil {
			return port.EditalCargos{}, err
		}

		in.ArquivoURI = arq.URI
		in.MIME = arq.MIME
		in.PDF = nil
	}

	schema := objeto(map[string]any{
		"banca": str,
		"cargos": lista(objeto(map[string]any{
			"codigo":       str,
			"nome":         str,
			"escolaridade": str,
			"vagas":        inteiro,
		}, "nome")),
	}, "cargos")

	var raw struct {
		Banca  string `json:"banca"`
		Cargos []struct {
			Codigo       string `json:"codigo"`
			Nome         string `json:"nome"`
			Escolaridade string `json:"escolaridade"`
			Vagas        int    `json:"vagas"`
		} `json:"cargos"`
	}

	if err := g.gerar(ctx, partesDe(promptCargos, in), schema, &raw); err != nil {
		return port.EditalCargos{}, err
	}

	out := port.EditalCargos{
		Banca:      raw.Banca,
		Texto:      in.Texto,
		ArquivoURI: in.ArquivoURI,
		MIME:       in.MIME,
		Cargos:     []port.EditalCargo{},
	}

	for _, c := range raw.Cargos {
		out.Cargos = append(out.Cargos, port.EditalCargo{
			Codigo:       strings.TrimSpace(c.Codigo),
			Nome:         strings.TrimSpace(c.Nome),
			Escolaridade: strings.TrimSpace(c.Escolaridade),
			Vagas:        maxZero(c.Vagas),
		})
	}

	if len(out.Cargos) == 0 {
		return port.EditalCargos{}, fmt.Errorf("não identifiquei nenhum cargo no edital")
	}

	return out, nil
}

// ---- step 2: estrutura de um cargo ----

const promptEstrutura = `Você recebe o texto de um edital de concurso público brasileiro e o
nome de UM cargo. Extraia em JSON, referentes SOMENTE a esse cargo:
- "nome": nome curto do concurso (órgão + cargo), ex.: "TCE-GO — Técnico de Controle Externo (TI)".
- "prova": data de aplicação das provas objetivas, formato AAAA-MM-DD (ou "" se não houver).
- "provaDiscursiva": true se há prova discursiva/redação/estudo de caso para o cargo.
- "totalGerais" / "totalEspecificas": total de questões de cada bloco (Conhecimentos Gerais/Básicos
  e Conhecimentos Específicos) conforme a tabela de provas. 0 se não informado.
- "gerais": disciplinas de Conhecimentos Gerais/Básicos do cargo — cada uma com "nome" e "questoes"
  (nº de questões DAQUELA disciplina; 0 quando o edital só dá o total do bloco).
- "especificas": idem para Conhecimentos Específicos do cargo.
Não invente dados. Não inclua o cronograma.`

func (g *GeminiAnalisador) Estrutura(
	ctx context.Context,
	in port.EditalEntrada,
	cargo string,
) (port.EditalEstrutura, error) {
	if in.Vazia() {
		return port.EditalEstrutura{}, fmt.Errorf("edital vazio")
	}

	parts := partesDe(promptEstrutura, in, "CARGO ESCOLHIDO: "+cargo)

	disc := lista(objeto(map[string]any{"nome": str, "questoes": inteiro}, "nome"))
	schema := objeto(map[string]any{
		"nome":             str,
		"prova":            str,
		"provaDiscursiva":  boolean,
		"totalGerais":      inteiro,
		"totalEspecificas": inteiro,
		"gerais":           disc,
		"especificas":      disc,
	}, "nome", "gerais", "especificas")

	var raw struct {
		Nome             string           `json:"nome"`
		Prova            string           `json:"prova"`
		ProvaDiscursiva  bool             `json:"provaDiscursiva"`
		TotalGerais      int              `json:"totalGerais"`
		TotalEspecificas int              `json:"totalEspecificas"`
		Gerais           []disciplinaJSON `json:"gerais"`
		Especificas      []disciplinaJSON `json:"especificas"`
	}

	if err := g.gerar(ctx, parts, schema, &raw); err != nil {
		return port.EditalEstrutura{}, err
	}

	return port.EditalEstrutura{
		Nome:             strings.TrimSpace(raw.Nome),
		Prova:            strings.TrimSpace(raw.Prova),
		ProvaDiscursiva:  raw.ProvaDiscursiva,
		TotalGerais:      maxZero(raw.TotalGerais),
		TotalEspecificas: maxZero(raw.TotalEspecificas),
		Gerais:           toDisciplinas(raw.Gerais),
		Especificas:      toDisciplinas(raw.Especificas),
		Marcos:           []port.EditalMarco{},
	}, nil
}

// ---- step 2b: cronograma (roda em paralelo com Estrutura) ----

const promptCronograma = `Você recebe o texto de um edital de concurso público. Extraia em JSON
o cronograma completo (Anexo de datas / cronograma de provas e publicações): inscrição, isenção,
pagamento, divulgações, convocações, recursos, resultados, heteroidentificação, etc.
"marcos": [{ "data" (AAAA-MM-DD), "dataFim" (só se for período), "titulo",
"exigeAcao" (true quando a data exige ação do candidato: inscrever-se, pagar, recorrer) }].`

func (g *GeminiAnalisador) Cronograma(ctx context.Context, in port.EditalEntrada) ([]port.EditalMarco, error) {
	if in.Vazia() {
		return nil, fmt.Errorf("edital vazio")
	}

	parts := partesDe(promptCronograma, in)

	schema := objeto(map[string]any{
		"marcos": lista(objeto(map[string]any{
			"data":      str,
			"dataFim":   str,
			"titulo":    str,
			"exigeAcao": boolean,
		}, "data", "titulo")),
	}, "marcos")

	var raw struct {
		Marcos []struct {
			Data      string `json:"data"`
			DataFim   string `json:"dataFim"`
			Titulo    string `json:"titulo"`
			ExigeAcao bool   `json:"exigeAcao"`
		} `json:"marcos"`
	}

	if err := g.gerar(ctx, parts, schema, &raw); err != nil {
		return nil, err
	}

	out := make([]port.EditalMarco, 0, len(raw.Marcos))
	for _, m := range raw.Marcos {
		if strings.TrimSpace(m.Data) == "" {
			continue
		}

		out = append(out, port.EditalMarco{
			Data:      strings.TrimSpace(m.Data),
			DataFim:   strings.TrimSpace(m.DataFim),
			Titulo:    strings.TrimSpace(m.Titulo),
			ExigeAcao: m.ExigeAcao,
		})
	}

	return out, nil
}

// ---- step 3: conteúdo programático ----

const promptConteudo = `Você recebe um edital de concurso público e uma lista de disciplinas.
Para CADA disciplina da lista, extraia do Conteúdo Programático (anexo do edital) os temas
correspondentes, um tema por item, mantendo a redação do edital de forma curta.
Retorne em JSON: "disciplinas": [{ "nome": <exatamente como na lista>, "temas": [<strings>] }].
Se não achar o conteúdo de uma disciplina, devolva "temas": [].`

func (g *GeminiAnalisador) Conteudo(
	ctx context.Context,
	in port.EditalEntrada,
	disciplinas []string,
) ([]port.EditalConteudoDisciplina, error) {
	if in.Vazia() || len(disciplinas) == 0 {
		return nil, fmt.Errorf("edital ou lista de disciplinas vazia")
	}

	parts := partesDe(promptConteudo, in,
		"DISCIPLINAS:\n- "+strings.Join(disciplinas, "\n- "))

	schema := objeto(map[string]any{
		"disciplinas": lista(objeto(map[string]any{
			"nome":  str,
			"temas": lista(str),
		}, "nome", "temas")),
	}, "disciplinas")

	var raw struct {
		Disciplinas []struct {
			Nome  string   `json:"nome"`
			Temas []string `json:"temas"`
		} `json:"disciplinas"`
	}

	if err := g.gerar(ctx, parts, schema, &raw); err != nil {
		return nil, err
	}

	out := make([]port.EditalConteudoDisciplina, 0, len(raw.Disciplinas))
	for _, d := range raw.Disciplinas {
		out = append(out, port.EditalConteudoDisciplina{
			Nome:  strings.TrimSpace(d.Nome),
			Temas: limpaTemas(d.Temas),
		})
	}

	return out, nil
}

// ---- HTTP plumbing ----

func (g *GeminiAnalisador) gerar(
	ctx context.Context,
	parts []map[string]any,
	schema map[string]any,
	out any,
) error {
	ctx, cancel := context.WithTimeout(ctx, 200*time.Second)
	defer cancel()

	body := map[string]any{
		"contents": []map[string]any{{"role": "user", "parts": parts}},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"responseSchema":   schema,
			"temperature":      0.1,
		},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	payload, status, err := g.postWithRetry(ctx, fmt.Sprintf(g.baseURL, g.model, g.apiKey), raw)
	if err != nil {
		return err
	}

	if status != http.StatusOK {
		return fmt.Errorf("gemini respondeu %d: %s", status, snippet(payload))
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
		return fmt.Errorf("decoding gemini response: %w", err)
	}

	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("gemini não retornou conteúdo")
	}

	if err := json.Unmarshal([]byte(parsed.Candidates[0].Content.Parts[0].Text), out); err != nil {
		return fmt.Errorf("gemini devolveu json inválido: %w", err)
	}

	return nil
}

// maxTentativas rides out the "high demand" 503s the flash models throw during
// congestion spikes — with the exponential backoff below that is ~30s of waiting
// before giving up.
const maxTentativas = 5

func (g *GeminiAnalisador) postWithRetry(ctx context.Context, url string, body []byte) ([]byte, int, error) {
	var lastErr error

	for tentativa := 1; tentativa <= maxTentativas; tentativa++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, 0, fmt.Errorf("building request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := g.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("calling gemini: %w", err)
		} else {
			payload, _ := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
			resp.Body.Close()

			if !transiente(resp.StatusCode) {
				return payload, resp.StatusCode, nil
			}

			lastErr = fmt.Errorf("gemini respondeu %d: %s", resp.StatusCode, snippet(payload))
		}

		if tentativa == maxTentativas {
			break
		}

		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(g.backoff << (tentativa - 1)): // 2s, 4s, 8s, 16s
		}
	}

	return nil, 0, lastErr
}

func transiente(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// ---- schema + decode helpers ----

type disciplinaJSON struct {
	Nome     string `json:"nome"`
	Questoes int    `json:"questoes"`
}

func toDisciplinas(in []disciplinaJSON) []port.EditalDisciplina {
	out := make([]port.EditalDisciplina, 0, len(in))
	for _, d := range in {
		if strings.TrimSpace(d.Nome) == "" {
			continue
		}

		out = append(out, port.EditalDisciplina{Nome: strings.TrimSpace(d.Nome), Questoes: maxZero(d.Questoes)})
	}

	return out
}

// partesDe builds the request parts: the instruction, any extra context lines,
// and the edital itself. Preference order: an already-uploaded file (cheapest to
// repeat), extracted text, then the raw PDF inline. Asking the model to
// transcribe a whole edital does not work, so the source always travels along.
func partesDe(prompt string, in port.EditalEntrada, extras ...string) []map[string]any {
	parts := make([]map[string]any, 0, len(extras)+2)
	parts = append(parts, map[string]any{"text": prompt})

	for _, e := range extras {
		parts = append(parts, map[string]any{"text": e})
	}

	mime := in.MIME
	if mime == "" {
		mime = "application/pdf"
	}

	switch {
	case in.ArquivoURI != "":
		return append(parts, map[string]any{
			"file_data": map[string]any{"mime_type": mime, "file_uri": in.ArquivoURI},
		})
	case in.Texto != "":
		return append(parts, map[string]any{"text": "EDITAL:\n" + in.Texto})
	default:
		return append(parts, map[string]any{
			"inline_data": map[string]any{
				"mime_type": mime,
				"data":      base64.StdEncoding.EncodeToString(in.PDF),
			},
		})
	}
}

var (
	str     = map[string]any{"type": "STRING"}
	inteiro = map[string]any{"type": "INTEGER"}
	boolean = map[string]any{"type": "BOOLEAN"}
)

func objeto(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "OBJECT", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}

	return m
}

func lista(items map[string]any) map[string]any {
	return map[string]any{"type": "ARRAY", "items": items}
}

func limpaTemas(xs []string) []string {
	out := []string{}
	for _, x := range xs {
		if s := strings.TrimSpace(x); s != "" {
			out = append(out, s)
		}
	}

	return out
}

func maxZero(n int) int {
	if n < 0 {
		return 0
	}

	return n
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		return s[:400] + "…"
	}

	return s
}
