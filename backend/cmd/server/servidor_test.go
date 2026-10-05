//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"studygo/internal/adapter/editalproc"
	"studygo/internal/platform/config"
	"studygo/internal/platform/pgtest"
	"studygo/internal/port"
	"studygo/migrations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// O servidor inteiro, como sobe em produção: a mesma montagem (montarHandler),
// o PostgreSQL de verdade (pgtest, Testcontainers) e o cliente HTTP real do
// processador de edital. Cada teste fala com ele por HTTP e confere o que um
// cliente veria — status, corpo e cookie — e, quando importa, o que ficou no
// banco.
//
// O processador de edital é o único serviço de fora, e no lugar dele sobe um
// servidor HTTP de mentira que responde o contrato dele. O caminho feliz do
// assistente com a tela é do E2E (B9, B10); aqui ficam os casos que a tela não
// provoca: upload grande, recusa, sobrecarga, corpo acima do teto.

func TestMain(m *testing.M) {
	codigo := m.Run()
	pgtest.Encerrar()
	os.Exit(codigo)
}

type servidor struct {
	url  string
	pool *pgxpool.Pool
}

// subir monta o servidor sobre um banco novo. Sem processador, a importação
// fica indisponível, como em produção sem EDITAL_PROCESSOR_URL.
func subir(t *testing.T, edital port.EditalProcessor, ajuste func(*config.Config)) *servidor {
	t.Helper()

	cfg := config.Config{
		CORSOrigin: "http://localhost:5173",
		JWTSecret:  strings.Repeat("segredo-de-teste-", 3),
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
		// O mínimo que o argon2id aceita: o custo da senha não está em teste.
		Argon2: config.Argon2Params{Memory: 64, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32},
		Versao: "v-teste",
		Deploy: "4242",
	}
	if ajuste != nil {
		ajuste(&cfg)
	}

	if edital == nil {
		edital = editalproc.Indisponivel{}
	}

	pool := pgtest.Novo(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv := httptest.NewServer(montarHandler(pool, cfg, edital, editalproc.Indisponivel{}, logger))
	t.Cleanup(srv.Close)

	return &servidor{url: srv.URL, pool: pool}
}

// pedido é uma requisição ao servidor. O corpo string vai como JSON.
type pedido struct {
	metodo, rota string
	token        string
	corpo        io.Reader
	tipo         string
	cookies      []*http.Cookie
	cabecalhos   map[string]string
}

func (s *servidor) fazer(t *testing.T, p pedido) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), p.metodo, s.url+p.rota, p.corpo)
	if err != nil {
		t.Fatalf("montando %s %s: %v", p.metodo, p.rota, err)
	}

	if p.tipo != "" {
		req.Header.Set("Content-Type", p.tipo)
	}

	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}

	for k, v := range p.cabecalhos {
		req.Header.Set(k, v)
	}

	for _, c := range p.cookies {
		req.AddCookie(c)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", p.metodo, p.rota, err)
	}

	t.Cleanup(func() { resp.Body.Close() })

	return resp
}

func (s *servidor) json(t *testing.T, metodo, rota, token, corpo string) *http.Response {
	t.Helper()

	return s.fazer(t, pedido{metodo: metodo, rota: rota, token: token, corpo: strings.NewReader(corpo), tipo: "application/json"})
}

func lerJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("lendo a resposta (%d): %v", resp.StatusCode, err)
	}
}

func esperarStatus(t *testing.T, resp *http.Response, quer int) {
	t.Helper()

	if resp.StatusCode != quer {
		corpo, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s = %d, quer %d: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, quer, corpo)
	}
}

func cookieDaSessao(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()

	for _, c := range resp.Cookies() {
		if c.Name == "studygo_refresh" {
			return c
		}
	}

	t.Fatalf("a resposta de %s não trouxe o cookie da sessão", resp.Request.URL.Path)

	return nil
}

// conta é um estudante cadastrado pelo próprio servidor.
type conta struct {
	id      uuid.UUID
	email   string
	token   string
	refresh *http.Cookie
}

func (s *servidor) cadastrar(t *testing.T) conta {
	t.Helper()

	email := uuid.NewString() + "@teste.local"
	resp := s.json(t, http.MethodPost, "/api/auth/register", "",
		`{"email":"`+email+`","nome":"Estudante","senha":"uma-senha-boa-123"}`)
	esperarStatus(t, resp, http.StatusCreated)

	var corpo struct {
		Usuario     struct{ ID uuid.UUID } `json:"usuario"`
		AccessToken string                 `json:"accessToken"`
	}

	refresh := cookieDaSessao(t, resp)
	lerJSON(t, resp, &corpo)

	return conta{id: corpo.Usuario.ID, email: email, token: corpo.AccessToken, refresh: refresh}
}

// ---------------------------------------------------------------- health

// A versão no ar é o que o deploy confere depois de subir; o schema é a última
// migration aplicada no banco.
func TestServidor_HealthDizAVersaoEOSchemaNoAr(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)

	resp := s.fazer(t, pedido{metodo: http.MethodGet, rota: "/health"})
	esperarStatus(t, resp, http.StatusOK)

	var corpo map[string]any
	lerJSON(t, resp, &corpo)

	quer := map[string]any{"status": "ok", "versao": "v-teste", "deploy": "4242", "schema": float64(ultimaMigration(t))}
	for campo, valor := range quer {
		if corpo[campo] != valor {
			t.Errorf("%s = %v, quer %v", campo, corpo[campo], valor)
		}
	}
}

// Fora da pipeline não há deploy: o campo some em vez de aparecer vazio.
func TestServidor_HealthSemDeployOmiteOCampo(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, func(c *config.Config) { c.Deploy = "" })

	resp := s.fazer(t, pedido{metodo: http.MethodGet, rota: "/health"})
	esperarStatus(t, resp, http.StatusOK)

	var corpo map[string]any
	lerJSON(t, resp, &corpo)

	if _, tem := corpo["deploy"]; tem {
		t.Errorf("deploy apareceu fora da pipeline: %v", corpo)
	}
}

// Ler o schema é uma ida ao banco como o ping. Se falhar, o processo não está
// saudável — e é o 503 que faz o deploy devolver a versão anterior.
func TestServidor_HealthCaiQuandoOSchemaNaoSeLe(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)

	if _, err := s.pool.Exec(t.Context(), `ALTER TABLE schema_migrations RENAME TO schema_sumiu`); err != nil {
		t.Fatalf("tirando o schema do lugar: %v", err)
	}

	resp := s.fazer(t, pedido{metodo: http.MethodGet, rota: "/health"})
	esperarStatus(t, resp, http.StatusServiceUnavailable)

	var corpo map[string]any
	lerJSON(t, resp, &corpo)

	if corpo["status"] != "unavailable" {
		t.Errorf("status = %v, quer unavailable", corpo["status"])
	}
}

func ultimaMigration(t *testing.T) int {
	t.Helper()

	nomes, err := fs.Glob(migrations.FS, "*.up.sql")
	if err != nil || len(nomes) == 0 {
		t.Fatalf("listando as migrations: %v", err)
	}

	maior := 0

	for _, n := range nomes {
		v, err := strconv.Atoi(strings.SplitN(n, "_", 2)[0])
		if err != nil {
			t.Fatalf("migration %q sem número: %v", n, err)
		}

		maior = max(maior, v)
	}

	return maior
}

// ---------------------------------------------------------------- sessão

// O refresh mora num cookie que o JavaScript não lê, só viaja para /api/auth e
// nunca aparece no corpo.
func TestServidor_EntrarPoeORefreshNumCookieFechado(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)

	resp := s.json(t, http.MethodPost, "/api/auth/login", "",
		`{"email":"`+c.email+`","senha":"uma-senha-boa-123"}`)
	esperarStatus(t, resp, http.StatusOK)

	cookie := cookieDaSessao(t, resp)

	corpo, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(corpo), cookie.Value) || strings.Contains(string(corpo), "refreshToken") {
		t.Errorf("o corpo carrega o refresh token: %s", corpo)
	}

	if !cookie.HttpOnly {
		t.Error("o cookie precisa ser HttpOnly")
	}

	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, quer Strict", cookie.SameSite)
	}

	if cookie.Path != "/api/auth" {
		t.Errorf("Path = %q, quer /api/auth", cookie.Path)
	}

	if cookie.Secure {
		t.Error("em http o cookie não pode ser Secure, ou o navegador o descarta")
	}

	// Atrás da borda que termina o TLS (o túnel, o nginx), ele é Secure.
	resp = s.fazer(t, pedido{
		metodo: http.MethodPost, rota: "/api/auth/login", tipo: "application/json",
		corpo:      strings.NewReader(`{"email":"` + c.email + `","senha":"uma-senha-boa-123"}`),
		cabecalhos: map[string]string{"X-Forwarded-Proto": "https"},
	})
	esperarStatus(t, resp, http.StatusOK)

	if !cookieDaSessao(t, resp).Secure {
		t.Error("atrás de TLS o cookie tem de ser Secure")
	}
}

// Renovar gira o cookie, diz quem está logado (é assim que o app abre), e o
// cookie antigo deixa de valer.
func TestServidor_RenovarGiraOCookie(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)

	resp := s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/auth/refresh", cookies: []*http.Cookie{c.refresh}})
	esperarStatus(t, resp, http.StatusOK)

	novo := cookieDaSessao(t, resp)
	if novo.Value == c.refresh.Value {
		t.Error("o refresh tem de girar: o cookie voltou igual")
	}

	var corpo struct {
		Usuario     struct{ Email string } `json:"usuario"`
		AccessToken string                 `json:"accessToken"`
	}
	lerJSON(t, resp, &corpo)

	if corpo.Usuario.Email != c.email || corpo.AccessToken == "" {
		t.Errorf("renovar devolveu %+v, quer a conta %s e um access token", corpo, c.email)
	}

	repetido := s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/auth/refresh", cookies: []*http.Cookie{c.refresh}})
	esperarStatus(t, repetido, http.StatusUnauthorized)
}

// Sem cookie, ou com um que o banco não conhece: 401, e o cookie recusado é
// apagado no navegador.
func TestServidor_RenovarSemSessaoValida(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)

	esperarStatus(t, s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/auth/refresh"}), http.StatusUnauthorized)

	morto := s.fazer(t, pedido{
		metodo: http.MethodPost, rota: "/api/auth/refresh",
		cookies: []*http.Cookie{{Name: "studygo_refresh", Value: "nao-existe"}},
	})
	esperarStatus(t, morto, http.StatusUnauthorized)

	if c := cookieDaSessao(t, morto); c.MaxAge >= 0 {
		t.Errorf("Max-Age = %d; o cookie recusado tem de ser apagado", c.MaxAge)
	}
}

// Sair apaga o cookie e revoga a sessão no banco; sair de novo continua 204.
func TestServidor_SairRevogaASessao(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)

	resp := s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/auth/logout", cookies: []*http.Cookie{c.refresh}})
	esperarStatus(t, resp, http.StatusNoContent)

	if cookie := cookieDaSessao(t, resp); cookie.MaxAge >= 0 {
		t.Errorf("Max-Age = %d; sair tem de apagar o cookie", cookie.MaxAge)
	}

	depois := s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/auth/refresh", cookies: []*http.Cookie{c.refresh}})
	esperarStatus(t, depois, http.StatusUnauthorized)

	esperarStatus(t, s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/auth/logout"}), http.StatusNoContent)
}

// A rota protegida exige um access token válido de uma conta que ainda existe.
func TestServidor_RotaProtegidaExigeContaValida(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)

	esperarStatus(t, s.json(t, http.MethodGet, "/api/me", c.token, ""), http.StatusOK)
	esperarStatus(t, s.json(t, http.MethodGet, "/api/me", "", ""), http.StatusUnauthorized)

	adulterado := c.token[:len(c.token)-2] + "xx"
	esperarStatus(t, s.json(t, http.MethodGet, "/api/me", adulterado, ""), http.StatusUnauthorized)

	// JWT ainda válido de uma conta que foi apagada.
	if _, err := s.pool.Exec(t.Context(), `DELETE FROM usuarios WHERE id = $1`, c.id); err != nil {
		t.Fatalf("apagando a conta: %v", err)
	}

	esperarStatus(t, s.json(t, http.MethodGet, "/api/me", c.token, ""), http.StatusUnauthorized)
}

// O banco fora do ar na checagem da conta é erro nosso (500), não "faça login".
func TestServidor_RotaProtegidaComBancoQuebradoE500(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)

	if _, err := s.pool.Exec(t.Context(), `ALTER TABLE usuarios RENAME TO usuarios_sumiu`); err != nil {
		t.Fatalf("tirando a tabela do lugar: %v", err)
	}

	esperarStatus(t, s.json(t, http.MethodGet, "/api/me", c.token, ""), http.StatusInternalServerError)
}

// ---------------------------------------------------------------- concurso

// O cadastro manual grava o concurso, e o código da matéria é atribuído pelo
// domínio — não pelo cliente nem pelo repository.
func TestServidor_CriarConcurso(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)

	resp := s.json(t, http.MethodPost, "/api/concursos", c.token,
		`{"nome":"TJ-SP Escrevente","prova":"2026-05-10","disciplinas":[{"nome":"Direito Constitucional","bloco":"esp","questoes":15}]}`)
	esperarStatus(t, resp, http.StatusCreated)

	var criado struct{ Slug string }
	lerJSON(t, resp, &criado)

	resp = s.json(t, http.MethodGet, "/api/concursos/"+criado.Slug, c.token, "")
	esperarStatus(t, resp, http.StatusOK)

	var detalhe struct {
		Dados struct {
			Disciplinas []struct{ Codigo string }
		}
	}
	lerJSON(t, resp, &detalhe)

	if len(detalhe.Dados.Disciplinas) != 1 || detalhe.Dados.Disciplinas[0].Codigo != "DIRCO" {
		t.Errorf("disciplinas gravadas = %+v, quer uma com o código DIRCO", detalhe.Dados.Disciplinas)
	}

	invalido := s.json(t, http.MethodPost, "/api/concursos", c.token,
		`{"nome":"X","prova":"2026-05-10","disciplinas":[{"nome":"A","bloco":"esp","questoes":0}]}`)
	esperarStatus(t, invalido, http.StatusUnprocessableEntity)
}

// ---------------------------------------------------------------- tetos

// Um POST autenticado não escolhe quanto o servidor lê: acima do teto é 413,
// dizendo o que houve.
func TestServidor_CorpoJSONAcimaDoTetoE413(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)

	gordo := `{"nome":"` + strings.Repeat("a", 1<<20) + `"}`

	resp := s.json(t, http.MethodPost, "/api/concursos", c.token, gordo)
	esperarStatus(t, resp, http.StatusRequestEntityTooLarge)

	var corpo map[string]string
	lerJSON(t, resp, &corpo)

	if corpo["erro"] == "" {
		t.Error("a resposta devia dizer o que houve")
	}
}

// O CSV do TEC tem o mesmo teto do import da planilha: uma rota não contorna o
// limite da outra. Acima do teto do transporte, para na leitura.
func TestServidor_PlanilhaDoTECAcimaDoTetoE413(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)
	slug := s.criarConcurso(t, c)
	rota := "/api/concursos/" + slug + "/plano/tec"

	csvGrande := `{"csv":"` + strings.Repeat("a", 5<<20+1) + `","data":"2026-09-01"}`
	esperarStatus(t, s.json(t, http.MethodPost, rota, c.token, csvGrande), http.StatusRequestEntityTooLarge)

	transporte := `{"csv":"` + strings.Repeat("a", 8<<20+1) + `"}`
	esperarStatus(t, s.json(t, http.MethodPost, rota, c.token, transporte), http.StatusRequestEntityTooLarge)
}

func (s *servidor) criarConcurso(t *testing.T, c conta) string {
	t.Helper()

	resp := s.json(t, http.MethodPost, "/api/concursos", c.token,
		`{"nome":"TCE-GO","prova":"2026-12-15","disciplinas":[{"nome":"Língua Portuguesa","bloco":"ger","questoes":20}]}`)
	esperarStatus(t, resp, http.StatusCreated)

	var criado struct{ Slug string }
	lerJSON(t, resp, &criado)

	return criado.Slug
}

// ---------------------------------------------------------------- edital

// processadorDeMentira responde o contrato HTTP do edital-processor e guarda o
// que recebeu. status diferente de 200 vira a resposta de erro dele.
type processadorDeMentira struct {
	mu       sync.Mutex
	status   int
	erro     string
	recebido struct {
		rota, dono, cargo string
		pdf               int
		disciplinas       []string
	}
}

func (p *processadorDeMentira) subir(t *testing.T) *editalproc.Client {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(p.servir))
	t.Cleanup(srv.Close)

	return editalproc.New(srv.URL, "token-de-teste")
}

func (p *processadorDeMentira) servir(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.recebido.rota = r.URL.Path
	p.recebido.dono = r.Header.Get("X-Owner-Ref")

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		if err := r.ParseMultipartForm(32 << 20); err == nil {
			if f, _, err := r.FormFile("file"); err == nil {
				n, _ := io.Copy(io.Discard, f)
				p.recebido.pdf = int(n)
			}

			p.recebido.cargo = r.FormValue("cargo")
			_ = json.Unmarshal([]byte(r.FormValue("disciplinas")), &p.recebido.disciplinas)
		}
	} else {
		var corpo struct {
			Cargo       string   `json:"cargo"`
			Disciplinas []string `json:"disciplinas"`
		}

		_ = json.NewDecoder(r.Body).Decode(&corpo)
		p.recebido.cargo, p.recebido.disciplinas = corpo.Cargo, corpo.Disciplinas
	}

	w.Header().Set("Content-Type", "application/json")

	if p.status != 0 && p.status != http.StatusOK {
		w.WriteHeader(p.status)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "teste", "message": p.erro})

		return
	}

	switch r.URL.Path {
	case "/internal/editais/analisar":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"documentId": "doc-abc-123", "banca": "FCC", "totalPages": 26, "ocrPages": 26,
			"cargos": []map[string]any{
				{"codigo": "A01", "nome": "Técnico Administrativo", "totalVagas": 6},
				{"codigo": "B02", "nome": "Tecnologia da Informação", "totalVagas": 10},
			},
			"alerts": []any{},
		})
	case "/internal/editais/conteudo":
		itens := []map[string]any{}
		for _, d := range p.recebido.disciplinas {
			itens = append(itens, map[string]any{"disciplina": d, "itens": []string{"Tema de " + d}})
		}

		_ = json.NewEncoder(w).Encode(map[string]any{"itens": itens, "alerts": []any{}})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func multipartComPDF(t *testing.T, tamanho int, campos map[string]string) (io.Reader, string) {
	t.Helper()

	var buf bytes.Buffer

	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile("file", "edital.pdf")
	if err != nil {
		t.Fatalf("montando o upload: %v", err)
	}

	pdf := append([]byte("%PDF-1.7 "), bytes.Repeat([]byte("x"), max(tamanho-9, 0))...)
	if _, err := fw.Write(pdf[:tamanho]); err != nil {
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

// O PDF enviado chega inteiro ao processador, com o dono da requisição, e a
// leitura volta à tela com os cargos e as vagas.
func TestServidor_AnalisarEditalEmPDF(t *testing.T) {
	t.Parallel()

	proc := &processadorDeMentira{}
	s := subir(t, proc.subir(t), nil)
	c := s.cadastrar(t)

	corpo, tipo := multipartComPDF(t, 4096, nil)
	resp := s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/editais/analisar", token: c.token, corpo: corpo, tipo: tipo})
	esperarStatus(t, resp, http.StatusOK)

	var analise struct {
		DocumentoID string `json:"documentoId"`
		Cargos      []struct {
			Codigo string
			Vagas  *int
		}
	}
	lerJSON(t, resp, &analise)

	if analise.DocumentoID != "doc-abc-123" || len(analise.Cargos) != 2 || analise.Cargos[1].Codigo != "B02" {
		t.Errorf("análise = %+v", analise)
	}

	if analise.Cargos[0].Vagas == nil || *analise.Cargos[0].Vagas != 6 {
		t.Errorf("vagas do A01 = %v, quer 6", analise.Cargos[0].Vagas)
	}

	if proc.recebido.pdf != 4096 {
		t.Errorf("o processador recebeu %d bytes de PDF, quer 4096", proc.recebido.pdf)
	}

	if proc.recebido.dono == "" {
		t.Error("o dono da requisição não chegou ao processador")
	}
}

// Um PDF acima do teto é recusado inteiro; antes ele era TRUNCADO e o
// processador respondia "PDF inválido" sobre um arquivo íntegro. No teto
// exato, passa inteiro.
func TestServidor_PDFDoEditalNoTeto(t *testing.T) {
	t.Parallel()

	proc := &processadorDeMentira{}
	s := subir(t, proc.subir(t), nil)
	c := s.cadastrar(t)

	corpo, tipo := multipartComPDF(t, 20<<20+1, nil)
	resp := s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/editais/analisar", token: c.token, corpo: corpo, tipo: tipo})
	esperarStatus(t, resp, http.StatusRequestEntityTooLarge)

	if proc.recebido.rota != "" {
		t.Error("o PDF acima do teto chegou ao processador")
	}

	corpo, tipo = multipartComPDF(t, 20<<20, nil)
	resp = s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/editais/analisar", token: c.token, corpo: corpo, tipo: tipo})
	esperarStatus(t, resp, http.StatusOK)

	if proc.recebido.pdf != 20<<20 {
		t.Errorf("o processador recebeu %d bytes, quer o PDF inteiro (%d)", proc.recebido.pdf, 20<<20)
	}
}

// A tela de edição extrai os temas de um PDF novo, sem documento da análise.
func TestServidor_ConteudoDeUmPDFNovo(t *testing.T) {
	t.Parallel()

	proc := &processadorDeMentira{}
	s := subir(t, proc.subir(t), nil)
	c := s.cadastrar(t)

	corpo, tipo := multipartComPDF(t, 2048, map[string]string{"cargo": "B02", "disciplinas": `["Português"]`})
	resp := s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/editais/conteudo", token: c.token, corpo: corpo, tipo: tipo})
	esperarStatus(t, resp, http.StatusOK)

	var conteudo struct {
		Itens []struct {
			Nome  string
			Temas []string
		}
	}
	lerJSON(t, resp, &conteudo)

	if len(conteudo.Itens) != 1 || conteudo.Itens[0].Nome != "Português" || len(conteudo.Itens[0].Temas) != 1 {
		t.Errorf("conteúdo = %+v", conteudo)
	}

	if proc.recebido.pdf != 2048 || proc.recebido.cargo != "B02" {
		t.Errorf("o processador recebeu pdf=%d cargo=%q", proc.recebido.pdf, proc.recebido.cargo)
	}
}

// O que falta no pedido é recusado aqui, sem gastar uma ida ao processador.
func TestServidor_EditalRecusaPedidoIncompleto(t *testing.T) {
	t.Parallel()

	proc := &processadorDeMentira{}
	s := subir(t, proc.subir(t), nil)
	c := s.cadastrar(t)

	esperarStatus(t, s.json(t, http.MethodPost, "/api/editais/estrutura", c.token,
		`{"documentoId":"x","cargo":""}`), http.StatusBadRequest)
	esperarStatus(t, s.json(t, http.MethodPost, "/api/editais/conteudo", c.token,
		`{"documentoId":"x","disciplinas":[]}`), http.StatusBadRequest)
	esperarStatus(t, s.json(t, http.MethodPost, "/api/editais/analisar", "",
		`{"texto":"x"}`), http.StatusUnauthorized)

	if proc.recebido.rota != "" {
		t.Errorf("o pedido incompleto chegou ao processador (%s)", proc.recebido.rota)
	}
}

// Sem processador configurado, ou com ele sobrecarregado, a resposta é 503 —
// e a sobrecarga diz para tentar de novo ou cadastrar à mão.
func TestServidor_EditalComProcessadorForaDoAr(t *testing.T) {
	t.Parallel()

	semProcessador := subir(t, nil, nil)
	c := semProcessador.cadastrar(t)
	esperarStatus(t, semProcessador.json(t, http.MethodPost, "/api/editais/analisar", c.token,
		`{"texto":"EDITAL"}`), http.StatusServiceUnavailable)

	proc := &processadorDeMentira{status: http.StatusServiceUnavailable, erro: "high demand"}
	s := subir(t, proc.subir(t), nil)
	c = s.cadastrar(t)

	resp := s.json(t, http.MethodPost, "/api/editais/analisar", c.token, `{"texto":"EDITAL"}`)
	esperarStatus(t, resp, http.StatusServiceUnavailable)

	var corpo map[string]string
	lerJSON(t, resp, &corpo)

	if !strings.Contains(corpo["erro"], "sobrecarregada") {
		t.Errorf("mensagem = %q, quer falar em sobrecarga", corpo["erro"])
	}
}

// ---------------------------------------------------------------- mapa

// Excluir pelo caminho com o texto de outro tópico é 409: o mapa mudou desde
// que a tela o abriu, e apagar assim mesmo levaria o item errado. A tela não
// produz esse estado sozinha (é preciso outra aba); aqui ele se provoca.
func TestServidor_ExcluirTopicoDeMapaQueMudouE409(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)

	mapaTexto, _ := json.Marshal(map[string]string{"texto": "# Ciclo\nslug: ciclo\n\n- Evaporação\n  - Calor\n- Chuva\n"})
	esperarStatus(t, s.json(t, http.MethodPost, "/api/mapas", c.token, string(mapaTexto)), http.StatusCreated)

	esperarStatus(t, s.json(t, http.MethodPost, "/api/mapas/ciclo/itens/excluir", c.token,
		`{"caminho":[0,0],"texto":"Outro texto"}`), http.StatusConflict)

	resp := s.json(t, http.MethodPost, "/api/mapas/ciclo/itens/excluir", c.token, `{"caminho":[0,0],"texto":"Calor"}`)
	esperarStatus(t, resp, http.StatusOK)

	var lido struct {
		Mapa   struct{ Itens int }
		Arvore []struct {
			Texto  string
			Filhos []any
		}
	}
	lerJSON(t, resp, &lido)

	if lido.Mapa.Itens != 2 || len(lido.Arvore) != 2 || len(lido.Arvore[0].Filhos) != 0 {
		t.Errorf("mapa depois da exclusão = %+v", lido)
	}

	outra := s.cadastrar(t)
	esperarStatus(t, s.json(t, http.MethodPost, "/api/mapas/ciclo/itens/excluir", outra.token,
		`{"caminho":[0],"texto":"Evaporação"}`), http.StatusNotFound)
}

func multipartDeImagens(t *testing.T, arquivos map[string][]byte) (io.Reader, string) {
	t.Helper()

	var buf bytes.Buffer

	mw := multipart.NewWriter(&buf)

	for nome, dados := range arquivos {
		fw, err := mw.CreateFormFile("imagens", nome)
		if err != nil {
			t.Fatalf("montando o envio: %v", err)
		}

		if _, err := fw.Write(dados); err != nil {
			t.Fatalf("montando o envio: %v", err)
		}
	}

	if err := mw.Close(); err != nil {
		t.Fatalf("montando o envio: %v", err)
	}

	return &buf, mw.FormDataContentType()
}

// A imagem do mapa vai e volta pelo servidor: o envio multipart grava, a
// leitura devolve os mesmos bytes com o tipo conferido e o nosniff, o arquivo
// que não é imagem é recusado com o motivo, e outra conta não alcança nada.
func TestServidor_ImagemDoMapaVaiEVolta(t *testing.T) {
	t.Parallel()

	s := subir(t, nil, nil)
	c := s.cadastrar(t)

	mapaTexto, _ := json.Marshal(map[string]string{"texto": "# Fluxos\nslug: fluxos\n\n- Gateways\n  - ![Gateway exclusivo](gateway.png)\n"})
	esperarStatus(t, s.json(t, http.MethodPost, "/api/mapas", c.token, string(mapaTexto)), http.StatusCreated)

	png := append([]byte("\x89PNG\r\n\x1a\n"), []byte("dados")...)

	corpo, tipo := multipartDeImagens(t, map[string][]byte{"gateway.png": png})
	esperarStatus(t, s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/mapas/fluxos/imagens", token: c.token, corpo: corpo, tipo: tipo}), http.StatusOK)

	resp := s.fazer(t, pedido{metodo: http.MethodGet, rota: "/api/mapas/fluxos/imagens/gateway.png", token: c.token})
	esperarStatus(t, resp, http.StatusOK)

	if got := resp.Header.Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}

	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}

	if dados, _ := io.ReadAll(resp.Body); !bytes.Equal(dados, png) {
		t.Errorf("a imagem voltou diferente: %q", dados)
	}

	var lido struct{ Imagens []string }
	lerJSON(t, s.json(t, http.MethodGet, "/api/mapas/fluxos", c.token, ""), &lido)

	if len(lido.Imagens) != 1 || lido.Imagens[0] != "gateway.png" {
		t.Errorf("a leitura do mapa lista %q", lido.Imagens)
	}

	corpo, tipo = multipartDeImagens(t, map[string][]byte{"falsa.png": []byte("<svg onload=alert(1)>")})
	esperarStatus(t, s.fazer(t, pedido{metodo: http.MethodPost, rota: "/api/mapas/fluxos/imagens", token: c.token, corpo: corpo, tipo: tipo}), http.StatusUnprocessableEntity)

	esperarStatus(t, s.fazer(t, pedido{metodo: http.MethodGet, rota: "/api/mapas/fluxos/imagens/falta.png", token: c.token}), http.StatusNotFound)

	outra := s.cadastrar(t)
	esperarStatus(t, s.fazer(t, pedido{metodo: http.MethodGet, rota: "/api/mapas/fluxos/imagens/gateway.png", token: outra.token}), http.StatusNotFound)
	esperarStatus(t, s.fazer(t, pedido{metodo: http.MethodGet, rota: "/api/mapas/fluxos/imagens/gateway.png"}), http.StatusUnauthorized)
}
