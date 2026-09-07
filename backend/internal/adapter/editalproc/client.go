// Package editalproc é o cliente HTTP do edital-processor.
//
// É o ÚNICO lugar que conhece o formato de fio daquele serviço; o resto do
// backend fala com ele por port.EditalProcessor e não sabe sequer que existe
// HTTP no meio. Aqui também mora o disjuntor, porque a saúde do processador é
// um fato de transporte, não de domínio.
package editalproc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"studygo/internal/platform/middleware"
	"studygo/internal/port"
)

var _ port.EditalProcessor = (*Client)(nil)

// Client fala com o edital-processor pela rede do compose.
type Client struct {
	baseURL   string
	token     string
	http      *http.Client
	disjuntor *Disjuntor
}

// falhasParaAbrir e esperaDoDisjuntor calibram o corte.
//
// Três falhas seguidas porque uma pode ser azar (um timeout isolado do
// provedor) e duas ainda podem ser coincidência; três seguidas, num serviço que
// normalmente responde, é padrão. Trinta segundos porque é o tempo típico de um
// container reiniciar — curto o bastante para a importação voltar sozinha
// enquanto o estudante ainda está na tela, longo o bastante para não martelar
// um serviço que está subindo.
const (
	falhasParaAbrir   = 3
	esperaDoDisjuntor = 30 * time.Second
)

// New constrói um cliente. Uma baseURL vazia produz um cliente cujo Disponivel()
// responde false — a raiz de composição usa o processador nulo no lugar.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		// O processador faz OCR e chama um LLM por passo; um edital digitalizado
		// pode levar uns 40s. Generoso, mas com teto.
		http:      &http.Client{Timeout: 4 * time.Minute},
		disjuntor: NovoDisjuntor(falhasParaAbrir, esperaDoDisjuntor),
	}
}

// Disponivel responde pela SAÚDE, não pela configuração.
//
// Antes bastava a URL estar definida, e a tela seguia oferecendo o assistente
// com o processador fora do ar: o estudante subia o PDF, esperava, e recebia um
// 503 no fim. Com o disjuntor aberto a importação some da tela e o cadastro
// manual — que sempre funcionou — passa a ser o caminho oferecido.
func (c *Client) Disponivel() bool {
	return c.baseURL != "" && c.disjuntor.Fechado()
}

// --- tipos de fio ----------------------------------------------------------

type wireAlerta struct {
	Codigo    string `json:"code"`
	Gravidade string `json:"severity"`
	Mensagem  string `json:"message"`
	Campo     string `json:"field"`
}

type wireCargo struct {
	Codigo        string `json:"codigo"`
	Nome          string `json:"nome"`
	Especialidade string `json:"especialidade"`
	Escolaridade  string `json:"escolaridade"`
	TotalVagas    *int   `json:"totalVagas"`
}

type wireAnalise struct {
	DocumentID string       `json:"documentId"`
	Banca      string       `json:"banca"`
	TotalPages int          `json:"totalPages"`
	OCRPages   int          `json:"ocrPages"`
	Cargos     []wireCargo  `json:"cargos"`
	Alerts     []wireAlerta `json:"alerts"`
}

type wireDisciplina struct {
	Nome           string   `json:"nome"`
	NumeroQuestoes *int     `json:"numeroQuestoes"`
	Peso           *float64 `json:"peso"`
}

type wireGrupo struct {
	Kind          string           `json:"kind"`
	Rotulo        string           `json:"rotulo"`
	TotalQuestoes *int             `json:"totalQuestoes"`
	Peso          *float64         `json:"peso"`
	PesoScope     string           `json:"pesoScope"`
	Disciplinas   []wireDisciplina `json:"disciplinas"`
}

type wireDiscursiva struct {
	Modalidade string `json:"modalidade"`
	Rotulo     string `json:"rotulo"`
	Questoes   *int   `json:"questoes"`
}

type wireDuracao struct {
	Minutos int    `json:"minutos"`
	Scope   string `json:"scope"`
}

type wireMarco struct {
	DataInicio string `json:"dataInicio"`
	DataFim    string `json:"dataFim"`
	Titulo     string `json:"titulo"`
	ExigeAcao  bool   `json:"exigeAcao"`
}

type wireEstrutura struct {
	NomeSugerido      string           `json:"nomeSugerido"`
	DataProva         string           `json:"dataProva"`
	GruposGerais      []wireGrupo      `json:"gruposGerais"`
	GruposEspecificos []wireGrupo      `json:"gruposEspecificos"`
	ProvaDiscursiva   []wireDiscursiva `json:"provaDiscursiva"`
	Duracao           *wireDuracao     `json:"duracao"`
	Cronograma        []wireMarco      `json:"cronograma"`
	Alerts            []wireAlerta     `json:"alerts"`
}

type wireConteudoItem struct {
	Disciplina string   `json:"disciplina"`
	Itens      []string `json:"itens"`
}

type wireConteudo struct {
	Itens  []wireConteudoItem `json:"itens"`
	Alerts []wireAlerta       `json:"alerts"`
}

type wireError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Transient bool   `json:"transient"`
	RequestID string `json:"requestId"`
}

// --- chamadas --------------------------------------------------------------

// Analisar envia o edital e devolve o identificador do documento e os cargos.
func (c *Client) Analisar(ctx context.Context, ownerRef string, up port.EditalUpload) (port.EditalAnalise, error) {
	body, contentType, err := multipartUpload(up)
	if err != nil {
		return port.EditalAnalise{}, err
	}

	var out wireAnalise
	if err := c.do(ctx, http.MethodPost, "/internal/editais/analisar", ownerRef, contentType, body, &out); err != nil {
		return port.EditalAnalise{}, err
	}

	return port.EditalAnalise{
		DocumentoID:  out.DocumentID,
		Banca:        out.Banca,
		TotalPaginas: out.TotalPages,
		PaginasOCR:   out.OCRPages,
		Cargos:       toCargos(out.Cargos),
		Alertas:      toAlertas(out.Alerts),
	}, nil
}

// Estrutura extrai a composição da prova de um cargo.
func (c *Client) Estrutura(ctx context.Context, ownerRef, documentoID, cargo string) (port.EditalEstrutura, error) {
	payload, _ := json.Marshal(map[string]string{"documentId": documentoID, "cargo": cargo})

	var out wireEstrutura
	if err := c.do(ctx, http.MethodPost, "/internal/editais/estrutura", ownerRef, "application/json", bytes.NewReader(payload), &out); err != nil {
		return port.EditalEstrutura{}, err
	}

	return toEstrutura(out), nil
}

// Conteudo extrai o conteúdo programático das disciplinas pedidas.
//
// Aceita um upload novo (documentoID vazio) porque a tela de EDIÇÃO chama este
// passo sozinha, sem ter passado pelo assistente — e portanto sem documento
// guardado do outro lado.
func (c *Client) Conteudo(ctx context.Context, ownerRef, documentoID, cargo string, disciplinas []string, up port.EditalUpload) (port.EditalConteudo, error) {
	var body io.Reader
	contentType := "application/json"

	if documentoID == "" && !up.Vazia() {
		mp, ct, err := multipartConteudo(up, cargo, disciplinas)
		if err != nil {
			return port.EditalConteudo{}, err
		}
		body, contentType = mp, ct
	} else {
		payload, _ := json.Marshal(map[string]any{
			"documentId":  documentoID,
			"cargo":       cargo,
			"disciplinas": disciplinas,
		})
		body = bytes.NewReader(payload)
	}

	var out wireConteudo
	if err := c.do(ctx, http.MethodPost, "/internal/editais/conteudo", ownerRef, contentType, body, &out); err != nil {
		return port.EditalConteudo{}, err
	}

	itens := make([]port.EditalConteudoDisciplina, 0, len(out.Itens))
	for _, it := range out.Itens {
		itens = append(itens, port.EditalConteudoDisciplina{Nome: it.Disciplina, Temas: it.Itens})
	}

	return port.EditalConteudo{Itens: itens, Alertas: toAlertas(out.Alerts)}, nil
}

// --- encanamento -----------------------------------------------------------

func (c *Client) do(ctx context.Context, method, path, ownerRef, contentType string, body io.Reader, out any) error {
	// Falhar rápido: com o disjuntor aberto, o corpo nem é enviado. É a
	// diferença entre o usuário esperar um minuto para saber e saber agora.
	if !c.disjuntor.Permitir() {
		return fmt.Errorf(
			"%w: o processador de editais falhou nas últimas tentativas",
			port.ErrProvedorIndisponivel,
		)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		// Erro de montagem é bug nosso, não indisponibilidade dele: a sonda que
		// este caminho tomou precisa ser devolvida, ou o disjuntor trava
		// meio-aberto para sempre.
		c.disjuntor.Sucesso()

		return fmt.Errorf("montando requisição: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-Owner-Ref", ownerRef)
	if rid := requestIDFrom(ctx); rid != "" {
		req.Header.Set("X-Request-Id", rid)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.disjuntor.Falha()

		return fmt.Errorf("%w: %w", port.ErrProvedorIndisponivel, err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))

	if resp.StatusCode >= 400 {
		return c.registrar(mapError(resp.StatusCode, payload))
	}

	if err := json.Unmarshal(payload, out); err != nil {
		c.disjuntor.Falha()

		return fmt.Errorf("%w: resposta inválida do processador: %w", port.ErrProvedorIndisponivel, err)
	}

	c.disjuntor.Sucesso()

	return nil
}

// registrar conta no disjuntor apenas o que é indisponibilidade.
//
// A distinção é a razão de o disjuntor ser útil: um edital que o processador
// leu e recusou é resposta SAUDÁVEL. Contá-la abriria o disjuntor por causa de
// um PDF ruim de um usuário, tirando a importação do ar para todos os outros.
func (c *Client) registrar(err error) error {
	if errors.Is(err, port.ErrProvedorIndisponivel) {
		c.disjuntor.Falha()
	} else {
		c.disjuntor.Sucesso()
	}

	return err
}

// mapError traduz o corpo de erro do processador num erro nosso.
//
// Código transitório (ou qualquer 5xx, ou 429) vira ErrProvedorIndisponivel — a
// API o apresenta como 503 com convite a tentar de novo, e o disjuntor o conta.
// O resto é recusa do EDITAL, não do serviço: sobe como erro comum e não conta.
func mapError(status int, payload []byte) error {
	var we wireError
	_ = json.Unmarshal(payload, &we)

	msg := we.Message
	if msg == "" {
		msg = strings.TrimSpace(string(payload))
	}

	switch {
	case we.Transient || status >= 500 || status == http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s (%s)", port.ErrProvedorIndisponivel, msg, we.Code)
	default:
		return fmt.Errorf("processador rejeitou o edital: %s (%s)", msg, we.Code)
	}
}

func multipartUpload(up port.EditalUpload) (io.Reader, string, error) {
	if up.Texto != "" {
		payload, _ := json.Marshal(map[string]string{"texto": up.Texto})
		return bytes.NewReader(payload), "application/json", nil
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "edital.pdf")
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(up.PDF); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}

	return &buf, w.FormDataContentType(), nil
}

func multipartConteudo(up port.EditalUpload, cargo string, disciplinas []string) (io.Reader, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if up.Texto != "" {
		_ = w.WriteField("texto", up.Texto)
	} else {
		part, err := w.CreateFormFile("file", "edital.pdf")
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(up.PDF); err != nil {
			return nil, "", err
		}
	}
	_ = w.WriteField("cargo", cargo)
	ds, _ := json.Marshal(disciplinas)
	_ = w.WriteField("disciplinas", string(ds))

	if err := w.Close(); err != nil {
		return nil, "", err
	}

	return &buf, w.FormDataContentType(), nil
}

// --- tradutores ------------------------------------------------------------

func toCargos(in []wireCargo) []port.EditalCargo {
	out := make([]port.EditalCargo, 0, len(in))
	for _, c := range in {
		if strings.TrimSpace(c.Codigo) == "" {
			continue
		}
		out = append(out, port.EditalCargo{
			Codigo:        strings.TrimSpace(c.Codigo),
			Nome:          strings.TrimSpace(c.Nome),
			Especialidade: strings.TrimSpace(c.Especialidade),
			Escolaridade:  strings.TrimSpace(c.Escolaridade),
			Vagas:         c.TotalVagas,
		})
	}
	return out
}

func toAlertas(in []wireAlerta) []port.EditalAlerta {
	out := make([]port.EditalAlerta, 0, len(in))
	for _, a := range in {
		out = append(out, port.EditalAlerta{
			Codigo:    a.Codigo,
			Gravidade: a.Gravidade,
			Mensagem:  a.Mensagem,
			Campo:     a.Campo,
		})
	}
	return out
}

func toGrupos(in []wireGrupo) []port.EditalGrupo {
	out := make([]port.EditalGrupo, 0, len(in))
	for _, g := range in {
		discs := make([]port.EditalDisciplina, 0, len(g.Disciplinas))
		for _, d := range g.Disciplinas {
			if strings.TrimSpace(d.Nome) == "" {
				continue
			}
			discs = append(discs, port.EditalDisciplina{
				Nome:     strings.TrimSpace(d.Nome),
				Questoes: d.NumeroQuestoes,
				Peso:     d.Peso,
			})
		}
		out = append(out, port.EditalGrupo{
			Kind:        g.Kind,
			Rotulo:      strings.TrimSpace(g.Rotulo),
			Total:       g.TotalQuestoes,
			Peso:        g.Peso,
			PesoEscopo:  g.PesoScope,
			Disciplinas: discs,
		})
	}
	return out
}

func toEstrutura(in wireEstrutura) port.EditalEstrutura {
	var dur *port.EditalDuracao
	if in.Duracao != nil {
		dur = &port.EditalDuracao{Minutos: in.Duracao.Minutos, Escopo: in.Duracao.Scope}
	}

	discursivas := make([]port.EditalDiscursiva, 0, len(in.ProvaDiscursiva))
	for _, d := range in.ProvaDiscursiva {
		discursivas = append(discursivas, port.EditalDiscursiva{
			Modalidade: d.Modalidade,
			Rotulo:     strings.TrimSpace(d.Rotulo),
			Questoes:   d.Questoes,
		})
	}

	marcos := make([]port.EditalMarco, 0, len(in.Cronograma))
	for _, m := range in.Cronograma {
		if strings.TrimSpace(m.DataInicio) == "" {
			continue
		}
		marcos = append(marcos, port.EditalMarco{
			Data:      strings.TrimSpace(m.DataInicio),
			DataFim:   strings.TrimSpace(m.DataFim),
			Titulo:    strings.TrimSpace(m.Titulo),
			ExigeAcao: m.ExigeAcao,
		})
	}

	return port.EditalEstrutura{
		NomeSugerido:      strings.TrimSpace(in.NomeSugerido),
		DataProva:         strings.TrimSpace(in.DataProva),
		GruposGerais:      toGrupos(in.GruposGerais),
		GruposEspecificos: toGrupos(in.GruposEspecificos),
		Discursivas:       discursivas,
		Duracao:           dur,
		Marcos:            marcos,
		Alertas:           toAlertas(in.Alerts),
	}
}

func requestIDFrom(ctx context.Context) string {
	return middleware.RequestIDFrom(ctx)
}
