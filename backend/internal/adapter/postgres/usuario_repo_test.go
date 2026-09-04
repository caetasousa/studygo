//go:build integration

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"studygo/internal/domain/usuario"
)

// A conta e os refresh tokens: o que o banco garante e o que o repository
// precisa traduzir de volta em erro de domínio.

func TestUsuarioRepo_RoundTrip(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")

	porID, err := r.usuarios.PorID(t.Context(), u.ID)
	if err != nil {
		t.Fatalf("PorID: %v", err)
	}

	if porID.Email != "a@b.c" || porID.Nome != "Fulano" || porID.SenhaHash != "$argon2id$fake" {
		t.Errorf("round-trip perdeu campos: %+v", porID)
	}

	if porID.TemaUI != usuario.TemaPadrao {
		t.Errorf("tema = %q, quer %q", porID.TemaUI, usuario.TemaPadrao)
	}

	if porID.CriadoEm.IsZero() {
		t.Error("criado_em veio zerado — o default da coluna não chegou ao domínio")
	}

	// citext: a busca por e-mail ignora a caixa.
	porEmail, err := r.usuarios.PorEmail(t.Context(), "A@B.C")
	if err != nil {
		t.Fatalf("PorEmail com outra caixa: %v", err)
	}

	if porEmail.ID != u.ID {
		t.Error("PorEmail devia achar a mesma conta ignorando a caixa")
	}
}

// "Não encontrado" é parte do contrato da porta: o service distingue isso de
// erro de infraestrutura para decidir entre 404 e 500.
func TestUsuarioRepo_NaoEncontrado(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)

	_, err := r.usuarios.PorEmail(t.Context(), "ninguem@lugar.nenhum")
	if !errors.Is(err, usuario.ErrNaoEncontrado) {
		t.Errorf("PorEmail inexistente = %v, quer ErrNaoEncontrado", err)
	}
}

// A UNIQUE de e-mail vira ErrEmailEmUso: o cadastro precisa dizer "já existe",
// não "erro interno".
func TestUsuarioRepo_EmailDuplicadoViraErroDeDominio(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	r.criarUsuario(t, "a@b.c")

	_, err := r.usuarios.Criar(t.Context(), usuario.Usuario{
		// Outra caixa: citext trata como o MESMO e-mail.
		Email: "A@B.C", Nome: "Outro", SenhaHash: "y", TemaUI: usuario.TemaPadrao,
	})

	if !errors.Is(err, usuario.ErrEmailEmUso) {
		t.Errorf("erro = %v, quer ErrEmailEmUso", err)
	}
}

func TestUsuarioRepo_DefinirTema(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")

	if err := r.usuarios.DefinirTema(t.Context(), u.ID, usuario.TemaClaro); err != nil {
		t.Fatalf("DefinirTema: %v", err)
	}

	lido, err := r.usuarios.PorID(t.Context(), u.ID)
	if err != nil {
		t.Fatalf("PorID: %v", err)
	}

	if lido.TemaUI != usuario.TemaClaro {
		t.Errorf("tema = %q, quer %q", lido.TemaUI, usuario.TemaClaro)
	}
}

// Um refresh token só vale enquanto não expirou e não foi revogado. As três
// situações são indistinguíveis para quem chama — todas devolvem credenciais
// inválidas —, mas precisam ser distinguidas AQUI, ou uma sessão revogada
// continuaria emitindo tokens novos.
func TestUsuarioRepo_RefreshToken(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome    string
		expira  time.Duration
		revogar bool
		valido  bool
	}{
		{nome: "válido", expira: time.Hour, valido: true},
		{nome: "expirado", expira: -time.Hour, valido: false},
		{nome: "revogado", expira: time.Hour, revogar: true, valido: false},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			t.Parallel()

			r := novoRepos(t)
			u := r.criarUsuario(t, "a@b.c")

			hash := "hash-" + caso.nome

			if err := r.usuarios.GuardarRefreshToken(
				t.Context(), u.ID, hash, time.Now().Add(caso.expira),
			); err != nil {
				t.Fatalf("GuardarRefreshToken: %v", err)
			}

			if caso.revogar {
				if err := r.usuarios.RevogarRefreshToken(t.Context(), hash); err != nil {
					t.Fatalf("RevogarRefreshToken: %v", err)
				}
			}

			id, err := r.usuarios.RefreshTokenValido(t.Context(), hash)

			if caso.valido {
				if err != nil {
					t.Fatalf("token devia valer: %v", err)
				}

				if id != u.ID {
					t.Errorf("dono = %s, quer %s", id, u.ID)
				}

				return
			}

			if !errors.Is(err, usuario.ErrCredenciaisInvalidas) {
				t.Errorf("erro = %v, quer ErrCredenciaisInvalidas", err)
			}
		})
	}
}

// Apagar a conta leva junto os refresh tokens (FK ON DELETE CASCADE): uma sessão
// não pode sobreviver ao usuário que ela autentica.
func TestUsuarioRepo_ApagarContaLevaOsTokens(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")

	if err := r.usuarios.GuardarRefreshToken(
		t.Context(), u.ID, "hash", time.Now().Add(time.Hour),
	); err != nil {
		t.Fatalf("GuardarRefreshToken: %v", err)
	}

	if _, err := r.pool.Exec(t.Context(), `DELETE FROM usuarios WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("apagando usuário: %v", err)
	}

	var restantes int
	if err := r.pool.QueryRow(
		t.Context(), `SELECT count(*) FROM refresh_tokens WHERE usuario_id = $1`, u.ID,
	).Scan(&restantes); err != nil {
		t.Fatalf("contando tokens: %v", err)
	}

	if restantes != 0 {
		t.Errorf("sobraram %d refresh tokens de uma conta apagada", restantes)
	}
}
