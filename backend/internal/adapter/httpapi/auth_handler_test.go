package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"studygo/internal/domain/usuario"
	"studygo/internal/service"

	"github.com/google/uuid"
)

// O refresh token sai e entra por cookie. Estes testes olham o cookie porque é
// ele o contrato agora: o corpo da resposta não pode mais carregar o token, e os
// atributos (HttpOnly, SameSite, Path) são a proteção — não detalhe de gosto.

type contaFake struct {
	conta  usuario.Usuario
	tokens map[string]uuid.UUID
}

func novaContaFake() *contaFake {
	return &contaFake{
		conta: usuario.Usuario{
			ID: uuid.New(), Email: "a@b.c", Nome: "Ana", TemaUI: usuario.TemaPadrao,
		},
		tokens: map[string]uuid.UUID{},
	}
}

func (f *contaFake) Criar(context.Context, usuario.Usuario) (usuario.Usuario, error) {
	return f.conta, nil
}

func (f *contaFake) PorEmail(context.Context, string) (usuario.Usuario, error) {
	return f.conta, nil
}

func (f *contaFake) PorID(context.Context, uuid.UUID) (usuario.Usuario, error) {
	return f.conta, nil
}

func (f *contaFake) DefinirTema(context.Context, uuid.UUID, usuario.Tema) error { return nil }

func (f *contaFake) GuardarRefreshToken(
	_ context.Context, id uuid.UUID, hash string, _ time.Time,
) error {
	f.tokens[hash] = id

	return nil
}

func (f *contaFake) RefreshTokenValido(_ context.Context, hash string) (uuid.UUID, error) {
	id, ok := f.tokens[hash]
	if !ok {
		return uuid.Nil, usuario.ErrCredenciaisInvalidas
	}

	return id, nil
}

func (f *contaFake) RevogarRefreshToken(_ context.Context, hash string) error {
	delete(f.tokens, hash)

	return nil
}

type senhaFake struct{}

func (senhaFake) Hash(string) (string, error)           { return "hash", nil }
func (senhaFake) Conferir(string, string) (bool, error) { return true, nil }

type relogioFake struct{}

func (relogioFake) Now() time.Time { return time.Unix(1_700_000_000, 0).UTC() }

func authDeTeste(t *testing.T) (*AuthHandler, *contaFake) {
	t.Helper()

	repo := novaContaFake()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewAuthService(repo, senhaFake{}, fakeTokens{id: repo.conta.ID}, relogioFake{}, 720*time.Hour)

	return NewAuthHandler(svc, func(string) bool { return false }, 720*time.Hour, quiet), repo
}

func cookieDaResposta(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieDoRefresh {
			return c
		}
	}

	t.Fatalf("resposta não trouxe o cookie %s", cookieDoRefresh)

	return nil
}

func TestEntrar_PoeORefreshNoCookieENaoNoCorpo(t *testing.T) {
	t.Parallel()

	h, repo := authDeTeste(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@b.c","senha":"x"}`))

	h.Entrar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body)
	}

	if strings.Contains(rec.Body.String(), "refreshToken") {
		t.Errorf("o corpo ainda carrega o refresh token: %s", rec.Body)
	}

	c := cookieDaResposta(t, rec)

	if !c.HttpOnly {
		t.Error("o cookie precisa ser HttpOnly — é o ponto da mudança")
	}

	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, queria Strict", c.SameSite)
	}

	if c.Path != caminhoDoRefresh {
		t.Errorf("Path = %q, queria %q", c.Path, caminhoDoRefresh)
	}

	if c.Secure {
		t.Error("em http o cookie não pode ser Secure, ou o navegador o descarta")
	}

	// O token do cookie é o que o banco conhece.
	if len(repo.tokens) != 1 {
		t.Fatalf("tokens guardados = %d, queria 1", len(repo.tokens))
	}
}

func TestEntrar_SecureQuandoABordaTerminaTLS(t *testing.T) {
	t.Parallel()

	h, _ := authDeTeste(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@b.c","senha":"x"}`))
	req.Header.Set("X-Forwarded-Proto", "https")

	h.Entrar(rec, req)

	if c := cookieDaResposta(t, rec); !c.Secure {
		t.Error("atrás do nginx com TLS o cookie tem de ser Secure")
	}
}

func TestRenovar_GiraOCookieEDevolveAConta(t *testing.T) {
	t.Parallel()

	h, repo := authDeTeste(t)

	login := httptest.NewRecorder()
	h.Entrar(login, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{}`)))
	antigo := cookieDaResposta(t, login)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(antigo)

	h.Renovar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body)
	}

	novo := cookieDaResposta(t, rec)
	if novo.Value == antigo.Value {
		t.Error("o refresh tem de girar: o cookie voltou igual")
	}

	// Renovar é também como o app descobre quem está logado ao abrir.
	var corpo struct {
		Usuario     usuarioResponse `json:"usuario"`
		AccessToken string          `json:"accessToken"`
	}

	if err := json.NewDecoder(rec.Body).Decode(&corpo); err != nil {
		t.Fatalf("decodificando resposta: %v", err)
	}

	if corpo.Usuario.Email != repo.conta.Email {
		t.Errorf("usuario.email = %q, queria %q", corpo.Usuario.Email, repo.conta.Email)
	}

	// O antigo não vale mais: reapresentá-lo é 401.
	repetido := httptest.NewRecorder()
	reqRepetido := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	reqRepetido.AddCookie(antigo)
	h.Renovar(repetido, reqRepetido)

	if repetido.Code != http.StatusUnauthorized {
		t.Errorf("reusar o refresh antigo deu %d, queria 401", repetido.Code)
	}
}

func TestRenovar_SemCookieOuComCookieMorto(t *testing.T) {
	t.Parallel()

	h, _ := authDeTeste(t)

	semCookie := httptest.NewRecorder()
	h.Renovar(semCookie, httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil))

	if semCookie.Code != http.StatusUnauthorized {
		t.Errorf("sem cookie deu %d, queria 401", semCookie.Code)
	}

	morto := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: cookieDoRefresh, Value: "nao-existe"})
	h.Renovar(morto, req)

	if morto.Code != http.StatusUnauthorized {
		t.Fatalf("cookie desconhecido deu %d, queria 401", morto.Code)
	}

	if c := cookieDaResposta(t, morto); c.MaxAge >= 0 {
		t.Errorf("Max-Age = %d; o cookie recusado tem de ser apagado", c.MaxAge)
	}
}

func TestSair_ApagaOCookieERevoga(t *testing.T) {
	t.Parallel()

	h, repo := authDeTeste(t)

	login := httptest.NewRecorder()
	h.Entrar(login, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{}`)))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(cookieDaResposta(t, login))

	h.Sair(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body)
	}

	if len(repo.tokens) != 0 {
		t.Errorf("sobraram %d tokens no banco depois do logout", len(repo.tokens))
	}

	if c := cookieDaResposta(t, rec); c.MaxAge >= 0 {
		t.Errorf("Max-Age = %d; sair tem de apagar o cookie", c.MaxAge)
	}

	// Sair de novo, já sem cookie, continua sendo 204: a tela chama esta rota
	// para garantir que não sobrou sessão, não para descobrir se havia uma.
	denovo := httptest.NewRecorder()
	h.Sair(denovo, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))

	if denovo.Code != http.StatusNoContent {
		t.Errorf("sair sem cookie deu %d, queria 204", denovo.Code)
	}
}
