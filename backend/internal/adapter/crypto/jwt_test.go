package crypto

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// O emissor de token: o que ele aceita é exatamente quem entra na aplicação.
// Não tinha teste, e as duas recusas que mais importam — token expirado e token
// assinado por outro — nunca tinham sido exercitadas.

const segredo = "um-segredo-de-teste-com-tamanho-suficiente"

func TestJWTIssuer_EmiteELe(t *testing.T) {
	t.Parallel()

	emissor := NewJWTIssuer(segredo, time.Hour)
	id := uuid.New()

	token, expira, err := emissor.Emitir(id)
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}

	if token == "" {
		t.Fatal("token veio vazio")
	}

	if !expira.After(time.Now()) {
		t.Errorf("expiração = %v, quer no futuro", expira)
	}

	lido, err := emissor.Ler(token)
	if err != nil {
		t.Fatalf("Ler: %v", err)
	}

	if lido != id {
		t.Errorf("id = %s, quer %s", lido, id)
	}
}

// Um token vencido tem de ser recusado: o TTL curto é a razão de o refresh
// existir, e aceitá-lo depois do prazo esvaziaria os dois.
func TestJWTIssuer_RecusaTokenExpirado(t *testing.T) {
	t.Parallel()

	emissor := NewJWTIssuer(segredo, time.Hour)

	// Emite como se fosse duas horas atrás, com TTL de uma hora.
	emissor.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }

	token, _, err := emissor.Emitir(uuid.New())
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}

	// A leitura usa o relógio de verdade da biblioteca.
	if _, err := emissor.Ler(token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("erro = %v, quer ErrInvalidToken para um token vencido", err)
	}
}

// Assinatura de outro segredo é a falha que mais importa: é o que separa um
// token nosso de um token que alguém fabricou.
func TestJWTIssuer_RecusaAssinaturaDeOutroSegredo(t *testing.T) {
	t.Parallel()

	token, _, err := NewJWTIssuer("segredo-de-outra-instalacao-qualquer", time.Hour).
		Emitir(uuid.New())
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}

	if _, err := NewJWTIssuer(segredo, time.Hour).Ler(token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("erro = %v, quer ErrInvalidToken", err)
	}
}

// `alg: none` é o ataque clássico contra JWT: um token sem assinatura nenhuma,
// aceito por quem confia no cabeçalho em vez de exigir o algoritmo esperado.
func TestJWTIssuer_RecusaAlgNone(t *testing.T) {
	t.Parallel()

	semAssinatura, err := jwt.NewWithClaims(
		jwt.SigningMethodNone,
		jwt.RegisteredClaims{
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("montando o token sem assinatura: %v", err)
	}

	if _, err := NewJWTIssuer(segredo, time.Hour).Ler(semAssinatura); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("erro = %v, quer ErrInvalidToken para alg=none", err)
	}
}

func TestJWTIssuer_RecusaLixo(t *testing.T) {
	t.Parallel()

	emissor := NewJWTIssuer(segredo, time.Hour)

	casos := map[string]string{
		"vazio":               "",
		"não é jwt":           "isto-nao-e-um-token",
		"partes de menos":     "aaa.bbb",
		"base64 estragado":    "!!!.!!!.!!!",
		"só o prefixo bearer": "Bearer abc.def.ghi",
	}

	for nome, token := range casos {
		t.Run(nome, func(t *testing.T) {
			t.Parallel()

			if _, err := emissor.Ler(token); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("erro = %v, quer ErrInvalidToken", err)
			}
		})
	}
}

// O subject precisa ser um UUID: um token bem assinado com um subject qualquer
// não pode virar um usuário.
func TestJWTIssuer_RecusaSubjectQueNaoEUUID(t *testing.T) {
	t.Parallel()

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "admin",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString([]byte(segredo))
	if err != nil {
		t.Fatalf("assinando: %v", err)
	}

	if _, err := NewJWTIssuer(segredo, time.Hour).Ler(token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("erro = %v, quer ErrInvalidToken", err)
	}
}

// A expiração devolvida por Emitir é a que está DENTRO do token — a tela usa
// esse instante para decidir quando renovar.
func TestJWTIssuer_ExpiracaoDevolvidaBateComOToken(t *testing.T) {
	t.Parallel()

	emissor := NewJWTIssuer(segredo, 15*time.Minute)

	token, expira, err := emissor.Emitir(uuid.New())
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}

	var claims jwt.RegisteredClaims
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil {
		t.Fatalf("lendo as claims: %v", err)
	}

	if diff := claims.ExpiresAt.Time.Sub(expira); diff > time.Second || diff < -time.Second {
		t.Errorf("expiração do token = %v, devolvida = %v", claims.ExpiresAt.Time, expira)
	}

	if !strings.HasPrefix(token, "ey") {
		t.Errorf("token = %q, quer um JWT", token)
	}
}
