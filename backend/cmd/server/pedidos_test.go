//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"studygo/internal/domain/usuario"
	"studygo/internal/port"
)

// A fila de PDFs que viram mapa, pelo servidor inteiro. O processador de
// verdade (o edital-processor, com o Claude) é a fronteira de fora: aqui um
// dublê guarda o trabalho recebido, e o teste faz o papel dele ao devolver o
// resultado pela porta interna.

type processadorDeMapasDeMentira struct {
	mu        sync.Mutex
	recebido  []port.TrabalhoDeMapa
	fora      bool
	iniciados map[string]bool
	conectado map[string]bool
}

// O código que o dublê aceita, como o Claude aceitaria o da página de autorização.
const codigoCertoDoClaude = "codigo-certo"

func (p *processadorDeMapasDeMentira) ConectarClaude(_ context.Context, dono string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.iniciados == nil {
		p.iniciados, p.conectado = map[string]bool{}, map[string]bool{}
	}

	p.iniciados[dono] = true

	return "https://claude.com/cai/oauth/authorize?dono=" + dono, nil
}

func (p *processadorDeMapasDeMentira) ConcluirConexaoDoClaude(_ context.Context, dono, codigo string) (port.ConexaoDoClaude, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.iniciados[dono] {
		return port.ConexaoDoClaude{}, usuario.ErrConexaoDoClaudeNaoIniciada
	}

	delete(p.iniciados, dono)

	if codigo != codigoCertoDoClaude {
		return port.ConexaoDoClaude{}, usuario.ErrCodigoDoClaudeRecusado{Motivo: "o Claude não aceitou o código"}
	}

	p.conectado[dono] = true

	return port.ConexaoDoClaude{Conectado: true, Email: "quem@estuda.dev", Plano: "max"}, nil
}

func (p *processadorDeMapasDeMentira) ConexaoDoClaude(_ context.Context, dono string) (port.ConexaoDoClaude, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.conectado[dono] {
		return port.ConexaoDoClaude{}, nil
	}

	return port.ConexaoDoClaude{Conectado: true, Email: "quem@estuda.dev", Plano: "max"}, nil
}

func (p *processadorDeMapasDeMentira) DesconectarClaude(_ context.Context, dono string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.conectado, dono)

	return nil
}

func (p *processadorDeMapasDeMentira) Processar(_ context.Context, t port.TrabalhoDeMapa) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.fora {
		return errors.New("connection refused")
	}

	p.recebido = append(p.recebido, t)

	return nil
}

func (p *processadorDeMapasDeMentira) ultimo(t *testing.T) port.TrabalhoDeMapa {
	t.Helper()

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.recebido) == 0 {
		t.Fatal("o processador não recebeu trabalho nenhum")
	}

	return p.recebido[len(p.recebido)-1]
}

func (p *processadorDeMapasDeMentira) quantos() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.recebido)
}

func (p *processadorDeMapasDeMentira) foraDoAr(fora bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.fora = fora
}

func multipartDoPedido(t *testing.T, nome string, dados []byte, campos map[string]string) (io.Reader, string) {
	t.Helper()

	var buf bytes.Buffer

	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile("pdf", nome)
	if err != nil {
		t.Fatalf("montando o upload: %v", err)
	}

	if _, err := fw.Write(dados); err != nil {
		t.Fatalf("montando o upload: %v", err)
	}

	for k, v := range campos {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("montando o upload: %v", err)
		}
	}

	if err := mw.Close(); err != nil {
		t.Fatalf("montando o upload: %v", err)
	}

	return &buf, mw.FormDataContentType()
}

type pedidoDeMapa struct {
	ID         string
	Arquivo    string
	Situacao   string
	Concurso   string
	Disciplina string
	Materia    string
	Mapa       string
	Relatorio  string
	TemPDF     bool
}

func (s *servidor) pedirMapa(t *testing.T, c conta, nome string, dados []byte, campos map[string]string, quer int) pedidoDeMapa {
	t.Helper()

	corpo, tipo := multipartDoPedido(t, nome, dados, campos)
	resp := s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/pedidos-de-mapa", token: c.token, corpo: corpo, tipo: tipo})
	esperarStatus(t, resp, quer)

	var p pedidoDeMapa
	if quer == http.StatusCreated {
		lerJSON(t, resp, &p)
	}

	return p
}

func (s *servidor) pedidos(t *testing.T, c conta) []pedidoDeMapa {
	t.Helper()

	var lista struct{ Pedidos []pedidoDeMapa }
	lerJSON(t, s.json(t, http.MethodGet, "/api/pedidos-de-mapa", c.token, ""), &lista)

	return lista.Pedidos
}

// devolver faz o papel do processador: entrega o resultado na porta interna.
func (s *servidor) devolver(t *testing.T, pedidoID, token string, dados map[string]any, imagens map[string][]byte) *http.Response {
	t.Helper()

	var buf bytes.Buffer

	mw := multipart.NewWriter(&buf)
	j, _ := json.Marshal(dados)
	_ = mw.WriteField("dados", string(j))

	for nome, b := range imagens {
		fw, _ := mw.CreateFormFile("imagens", nome)
		_, _ = fw.Write(b)
	}

	_ = mw.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost,
		s.interno+"/internal/pedidos-de-mapa/"+pedidoID+"/resultado", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("devolvendo o resultado: %v", err)
	}

	t.Cleanup(func() { resp.Body.Close() })

	return resp
}

func (s *servidor) interna(t *testing.T, rota, corpo string) *http.Response {
	t.Helper()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, s.interno+rota, strings.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", rota, err)
	}

	t.Cleanup(func() { resp.Body.Close() })

	return resp
}

func (s *servidor) concursoDeES(t *testing.T, c conta) (slug, disciplina string) {
	t.Helper()

	resp := s.json(t, http.MethodPost, "/api/concursos", c.token,
		`{"nome":"TCE-GO","prova":"2026-12-15","disciplinas":[{"nome":"Engenharia de Software","codigo":"ENG","bloco":"esp","questoes":20,"temas":["Testes","Qualidade"]}]}`)
	esperarStatus(t, resp, http.StatusCreated)

	var criado struct{ Slug string }
	lerJSON(t, resp, &criado)

	var lista struct {
		Disciplinas []struct{ DisciplinaID string }
	}
	lerJSON(t, s.json(t, http.MethodGet, "/api/concursos/"+criado.Slug+"/mapas", c.token, ""), &lista)

	return criado.Slug, lista.Disciplinas[0].DisciplinaID
}

const mapaDoProcessador = "# Testes\nslug: testes\n\n- Níveis\n  - [def] **Unitário**\n  - ![A pirâmide](piramide.png)\n- Números e nomes\n  - 5 níveis\n"

// M28, M29: o PDF entra na fila com a matéria e vai na hora ao processador,
// com o contexto do mapa; o que ele devolve vira mapa da conta (questões,
// imagem e tópicos), o pedido fica pronto com o relatório, e o PDF some.
func TestServidor_PedidoDeMapaDoPDFAoMapa(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)
	outra := s.cadastrar(t)
	concurso, disciplina := s.concursoDeES(t, c)

	// Um mapa que a conta já tem: o processador fica sabendo para não usar o slug.
	velho, _ := json.Marshal(map[string]string{"texto": "# Velho\nslug: velho\n\n- Ramo\n"})
	esperarStatus(t, s.json(t, http.MethodPost, "/api/mapas", c.token, string(velho)), http.StatusCreated)

	pdf := []byte("%PDF-1.7 aula de testes")
	campos := map[string]string{"concurso": concurso, "disciplina": disciplina}

	s.pedirMapa(t, c, "aula.pdf", []byte("<html>"), campos, http.StatusUnprocessableEntity)
	s.pedirMapa(t, outra, "aula.pdf", pdf, campos, http.StatusNotFound)

	p := s.pedirMapa(t, c, `C:\Downloads\Aula 05.pdf`, pdf, campos, http.StatusCreated)
	if p.Situacao != "processando" || p.Arquivo != "Aula 05.pdf" || p.Materia != "Engenharia de Software" || !p.TemPDF {
		t.Fatalf("pedido criado = %+v", p)
	}

	tr := s.mapas.ultimo(t)
	if tr.Pedido.String() != p.ID || !bytes.Equal(tr.PDF, pdf) || tr.Materia != "ENG — Engenharia de Software" ||
		!slices.Equal(tr.Temas, []string{"Testes", "Qualidade"}) || !slices.Equal(tr.SlugsExistentes, []string{"velho"}) {
		t.Fatalf("o processador recebeu %+v", tr)
	}

	if ps := s.pedidos(t, outra); len(ps) != 0 {
		t.Fatalf("a outra conta vê %+v", ps)
	}

	png := append([]byte("\x89PNG\r\n\x1a\n"), []byte("dados")...)
	resultado := map[string]any{
		"mapa":      mapaDoProcessador,
		"questoes":  map[string]any{"mapa": "testes", "questoes": []map[string]any{{"id": "e01", "ramo": "Níveis", "origem": "FCC · 2025", "enunciado": "Julgue.", "gabarito": "Certo", "comentario": "Certo."}}},
		"temas":     []string{"Testes"},
		"relatorio": "A questão 3 diverge da tabela.",
	}

	// Sem o token de serviço, a porta interna recusa.
	esperarStatus(t, s.devolver(t, p.ID, "", resultado, nil), http.StatusUnauthorized)

	// O que a importação recusa volta ao processador com o motivo; o pedido segue processando.
	ruim := map[string]any{"mapa": "# Sem itens\nslug: x\n", "relatorio": ""}
	if resp := s.devolver(t, p.ID, s.token, ruim, nil); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("mapa inválido = %d", resp.StatusCode)
	}

	alheio := map[string]any{"mapa": "# Velho de novo\nslug: velho\n\n- Ramo\n", "relatorio": ""}
	if resp := s.devolver(t, p.ID, s.token, alheio, nil); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("slug de outro mapa = %d", resp.StatusCode)
	}

	esperarStatus(t, s.devolver(t, p.ID, s.token, resultado, map[string][]byte{"piramide.png": png}), http.StatusOK)

	ps := s.pedidos(t, c)
	if len(ps) != 1 || ps[0].Situacao != "pronto" || ps[0].Mapa != "testes" ||
		ps[0].Relatorio != "A questão 3 diverge da tabela." || ps[0].TemPDF {
		t.Fatalf("depois de concluir: %+v", ps)
	}

	var lido struct {
		Questoes []struct{ Ramo string }
		Imagens  []string
	}
	lerJSON(t, s.json(t, http.MethodGet, "/api/mapas/testes", c.token, ""), &lido)

	if len(lido.Questoes) != 1 || !slices.Equal(lido.Imagens, []string{"piramide.png"}) {
		t.Fatalf("o mapa importado = %+v", lido)
	}

	var vinculos struct {
		Disciplinas []struct {
			Mapas []struct {
				Slug  string
				Temas []string
			}
		}
	}
	lerJSON(t, s.json(t, http.MethodGet, "/api/concursos/"+concurso+"/mapas", c.token, ""), &vinculos)

	if m := vinculos.Disciplinas[0].Mapas; len(m) != 1 || m[0].Slug != "testes" || !slices.Equal(m[0].Temas, []string{"Testes"}) {
		t.Fatalf("o vínculo = %+v", m)
	}

	// Um segundo resultado do mesmo pedido não muda nada.
	esperarStatus(t, s.devolver(t, p.ID, s.token, resultado, nil), http.StatusConflict)
}

// M30: com o processador fora do ar o pedido fica na fila, dizendo por quê, e
// volta a ele pelo botão; o que falhou volta à fila sem reenviar; o excluído
// não vira mapa; o que perdeu o PDF na faxina pede reenvio; e o processador que
// reiniciou recebe de volta o que tinha em mãos.
func TestServidor_PedidoDeMapaVoltaAFilaESaiDela(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)

	s.mapas.foraDoAr(true)

	p := s.pedirMapa(t, c, "aula.pdf", []byte("%PDF-1.4 x"), nil, http.StatusCreated)
	if p.Situacao != "na_fila" || !strings.Contains(p.Relatorio, "O processador não recebeu o PDF") || p.Disciplina != "" {
		t.Fatalf("com o processador fora do ar: %+v", p)
	}

	s.mapas.foraDoAr(false)
	esperarStatus(t, s.json(t, http.MethodPost, "/api/pedidos-de-mapa/"+p.ID+"/fila", c.token, ""), http.StatusNoContent)

	if ps := s.pedidos(t, c); ps[0].Situacao != "processando" || ps[0].Relatorio != "" {
		t.Fatalf("de volta ao processador: %+v", ps[0])
	}

	esperarStatus(t, s.interna(t, "/internal/pedidos-de-mapa/"+p.ID+"/falha", `{"relatorio":"O PDF é só imagem: sem texto."}`), http.StatusNoContent)

	if ps := s.pedidos(t, c); ps[0].Situacao != "falhou" || ps[0].Relatorio != "O PDF é só imagem: sem texto." || !ps[0].TemPDF {
		t.Fatalf("depois de falhar: %+v", ps[0])
	}

	antes := s.mapas.quantos()
	esperarStatus(t, s.json(t, http.MethodPost, "/api/pedidos-de-mapa/"+p.ID+"/fila", c.token, ""), http.StatusNoContent)

	if s.mapas.quantos() != antes+1 {
		t.Fatal("pôr na fila de novo não reenviou ao processador")
	}

	// O processador reiniciou: pede de volta o que estava processando.
	resp := s.interna(t, "/internal/pedidos-de-mapa/redespacho", "")
	esperarStatus(t, resp, http.StatusOK)

	if s.mapas.quantos() != antes+2 || s.mapas.ultimo(t).Pedido.String() != p.ID {
		t.Fatal("o redespacho não devolveu o pedido ao processador")
	}

	// Excluído, o resultado que chega depois não vira mapa.
	esperarStatus(t, s.json(t, http.MethodDelete, "/api/pedidos-de-mapa/"+p.ID, c.token, ""), http.StatusNoContent)
	esperarStatus(t, s.devolver(t, p.ID, s.token, map[string]any{"mapa": mapaDoProcessador}, nil), http.StatusNotFound)

	// O que falhou há mais de uma semana perdeu o PDF na faxina: não volta, e diz por quê.
	q := s.pedirMapa(t, c, "outra.pdf", []byte("%PDF-1.4 y"), nil, http.StatusCreated)
	esperarStatus(t, s.interna(t, "/internal/pedidos-de-mapa/"+q.ID+"/falha", `{"relatorio":"x"}`), http.StatusNoContent)

	if _, err := s.pool.Exec(t.Context(), `UPDATE mapas_pedidos SET pdf = NULL WHERE id = $1`, q.ID); err != nil {
		t.Fatalf("simulando a faxina: %v", err)
	}

	resp = s.json(t, http.MethodPost, "/api/pedidos-de-mapa/"+q.ID+"/fila", c.token, "")
	esperarStatus(t, resp, http.StatusConflict)

	if corpo, _ := io.ReadAll(resp.Body); !strings.Contains(string(corpo), "envie-o de novo") {
		t.Fatalf("a recusa não diz o que fazer: %s", corpo)
	}
}

// M31: o token do Claude entra pela tela, volta à tela só pelo fim, vai ao
// processador junto com o PDF da conta (e só dela) e, no banco, é ilegível.
func TestServidor_TokenDoClaudeDaConta(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)
	outra := s.cadastrar(t)

	const token = "sk-ant-oat01-um-token-de-teste-bem-comprido-XYZW"

	esperarStatus(t, s.json(t, http.MethodPut, "/api/conta/token-do-claude", c.token, `{"token":"  "}`), http.StatusUnprocessableEntity)
	esperarStatus(t, s.json(t, http.MethodPut, "/api/conta/token-do-claude", c.token, `{"token":"dois tokens"}`), http.StatusUnprocessableEntity)
	esperarStatus(t, s.json(t, http.MethodPut, "/api/conta/token-do-claude", c.token, `{"token":" `+token+` "}`), http.StatusNoContent)

	resp := s.json(t, http.MethodGet, "/api/conta/token-do-claude", c.token, "")
	esperarStatus(t, resp, http.StatusOK)

	corpo, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(corpo), "um-token-de-teste") || !strings.Contains(string(corpo), `"fim":"…XYZW"`) {
		t.Fatalf("a tela recebeu %s", corpo)
	}

	// O processador recebe o token da conta do PDF, e o da outra conta vai sem token.
	s.pedirMapa(t, c, "a.pdf", []byte("%PDF-1.4 a"), nil, http.StatusCreated)

	if got := s.mapas.ultimo(t).TokenClaude; got != token {
		t.Fatalf("o processador recebeu o token %q", got)
	}

	s.pedirMapa(t, outra, "b.pdf", []byte("%PDF-1.4 b"), nil, http.StatusCreated)

	if got := s.mapas.ultimo(t).TokenClaude; got != "" {
		t.Fatalf("o PDF da outra conta foi com o token %q", got)
	}

	var guardado []byte
	if err := s.pool.QueryRow(t.Context(), `SELECT token_claude FROM usuarios WHERE id = $1`, c.id).Scan(&guardado); err != nil {
		t.Fatalf("lendo o banco: %v", err)
	}

	if len(guardado) == 0 || bytes.Contains(guardado, []byte("um-token-de-teste")) {
		t.Fatalf("o banco guarda o token legível: %q", guardado)
	}

	esperarStatus(t, s.json(t, http.MethodDelete, "/api/conta/token-do-claude", c.token, ""), http.StatusNoContent)
	s.pedirMapa(t, c, "c.pdf", []byte("%PDF-1.4 c"), nil, http.StatusCreated)

	if got := s.mapas.ultimo(t).TokenClaude; got != "" {
		t.Fatalf("removido, o token ainda foi: %q", got)
	}
}

// M33: conectar o Claude pela tela, sem terminal. O que pode quebrar: o link
// não chega, o código errado (ou em branco) passa, a conexão de uma conta vale
// para outra, ou desconectar não desconecta.
func TestServidor_ConexaoDoClaudePelaTela(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)
	outra := s.cadastrar(t)

	situacao := func(conta conta) string {
		t.Helper()

		resp := s.json(t, http.MethodGet, "/api/conta/claude", conta.token, "")
		esperarStatus(t, resp, http.StatusOK)

		corpo, _ := io.ReadAll(resp.Body)

		return string(corpo)
	}

	if got := situacao(c); !strings.Contains(got, `"conectado":false`) {
		t.Fatalf("a conta nova já aparece conectada: %s", got)
	}

	// O código sem link pedido não tem a quem ir.
	esperarStatus(t, s.json(t, http.MethodPost, "/api/conta/claude/conexao/codigo", c.token, `{"codigo":"codigo-certo"}`), http.StatusConflict)

	resp := s.json(t, http.MethodPost, "/api/conta/claude/conexao", c.token, "")
	esperarStatus(t, resp, http.StatusOK)

	corpo, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(corpo), `"url":"https://claude.com/cai/oauth/authorize?dono=`+c.id.String()) {
		t.Fatalf("o link não é o desta conta: %s", corpo)
	}

	esperarStatus(t, s.json(t, http.MethodPost, "/api/conta/claude/conexao/codigo", c.token, `{"codigo":"  "}`), http.StatusUnprocessableEntity)
	esperarStatus(t, s.json(t, http.MethodPost, "/api/conta/claude/conexao/codigo", c.token, `{"codigo":"codigo-errado"}`), http.StatusUnprocessableEntity)

	// Recusado, o processo do login acabou: um link novo, e o código certo.
	esperarStatus(t, s.json(t, http.MethodPost, "/api/conta/claude/conexao", c.token, ""), http.StatusOK)

	resp = s.json(t, http.MethodPost, "/api/conta/claude/conexao/codigo", c.token, `{"codigo":" codigo-certo "}`)
	esperarStatus(t, resp, http.StatusOK)

	corpo, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(corpo), `"conectado":true`) || !strings.Contains(string(corpo), `"email":"quem@estuda.dev"`) {
		t.Fatalf("conectado, a tela recebeu %s", corpo)
	}

	if got := situacao(outra); !strings.Contains(got, `"conectado":false`) {
		t.Fatalf("a conexão de uma conta vale para a outra: %s", got)
	}

	esperarStatus(t, s.json(t, http.MethodDelete, "/api/conta/claude", c.token, ""), http.StatusNoContent)

	if got := situacao(c); !strings.Contains(got, `"conectado":false`) {
		t.Fatalf("desconectada, a conta ainda aparece conectada: %s", got)
	}
}
