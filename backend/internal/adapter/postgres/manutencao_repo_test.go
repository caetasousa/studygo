//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"studygo/internal/domain/plano"
)

// A faxina periódica: o que a varredura diária apaga e o que ela se recusa a
// carregar. Os dois vazavam do mesmo jeito — crescendo em silêncio até que o
// custo aparecesse como lentidão sem causa aparente.

func TestUsuarioRepo_LimparRefreshTokens(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "faxina@b.c")
	agora := time.Now()

	guardar := func(hash string, expira time.Duration, revogar bool) {
		t.Helper()

		if err := r.usuarios.GuardarRefreshToken(
			t.Context(), u.ID, hash, agora.Add(expira),
		); err != nil {
			t.Fatalf("GuardarRefreshToken(%s): %v", hash, err)
		}

		if revogar {
			if err := r.usuarios.RevogarRefreshToken(t.Context(), hash); err != nil {
				t.Fatalf("RevogarRefreshToken(%s): %v", hash, err)
			}
		}
	}

	guardar("vivo", time.Hour, false)
	guardar("vencido", -time.Hour, false)
	guardar("revogado-mas-no-prazo", time.Hour, true)

	apagados, err := r.usuarios.LimparRefreshTokens(t.Context(), agora)
	if err != nil {
		t.Fatalf("LimparRefreshTokens: %v", err)
	}

	if apagados != 2 {
		t.Errorf("apagados = %d, quer 2 (o vencido e o revogado)", apagados)
	}

	// O que sobrou tem de ser exatamente a sessão que ainda autentica alguém.
	var restantes []string

	linhas, err := r.pool.Query(
		t.Context(), `SELECT token_hash FROM refresh_tokens WHERE usuario_id = $1`, u.ID,
	)
	if err != nil {
		t.Fatalf("consultando tokens: %v", err)
	}
	defer linhas.Close()

	for linhas.Next() {
		var hash string
		if err := linhas.Scan(&hash); err != nil {
			t.Fatalf("lendo token: %v", err)
		}

		restantes = append(restantes, hash)
	}

	if len(restantes) != 1 || restantes[0] != "vivo" {
		t.Errorf("sobraram %v, quer só [vivo]", restantes)
	}

	// O token que sobrou continua servindo: a faxina não pode deslogar ninguém.
	if _, err := r.usuarios.RefreshTokenValido(t.Context(), "vivo"); err != nil {
		t.Errorf("o token vivo parou de valer depois da faxina: %v", err)
	}
}

// Rodar duas vezes não apaga nada na segunda: a faxina é idempotente, e é isso
// que permite chamá-la em toda virada de dia sem pensar.
func TestUsuarioRepo_LimparRefreshTokens_idempotente(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "idem@b.c")
	agora := time.Now()

	if err := r.usuarios.GuardarRefreshToken(
		t.Context(), u.ID, "vencido", agora.Add(-time.Hour),
	); err != nil {
		t.Fatalf("GuardarRefreshToken: %v", err)
	}

	if _, err := r.usuarios.LimparRefreshTokens(t.Context(), agora); err != nil {
		t.Fatalf("primeira faxina: %v", err)
	}

	apagados, err := r.usuarios.LimparRefreshTokens(t.Context(), agora)
	if err != nil {
		t.Fatalf("segunda faxina: %v", err)
	}

	if apagados != 0 {
		t.Errorf("segunda passada apagou %d, quer 0", apagados)
	}
}

// Um concurso cuja prova já passou fica atrasado para sempre — as atividades
// vencidas nunca serão concluídas. Sem o corte por data ele voltava em toda
// varredura, e o worker carregava plano, cronograma e registros inteiros só
// para o caso de uso concluir que não havia futuro em que redistribuir.
func TestPlanoRepo_ComAtraso_ignoraProvaJaRealizada(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "provapassou@b.c")
	c := r.criarConcurso(t, u, "tj-sp")
	p := r.criarPlano(t, u, c)

	ontem := dia(2026, time.September, 1)
	hoje := dia(2026, time.September, 2)

	r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(c.Disciplinas[0], ontem, 0, "vencida e não estudada"),
	})

	// Enquanto a prova está à frente, o atraso conta.
	if got := r.slugsComAtraso(t, hoje); len(got) != 1 {
		t.Fatalf("com atraso = %v, quer o plano listado enquanto a prova não chegou", got)
	}

	// A prova passa a ser ontem. Nada mais no cenário muda.
	if _, err := r.pool.Exec(
		t.Context(), `UPDATE planos SET prova = $2 WHERE id = $1`, p.ID, ontem,
	); err != nil {
		t.Fatalf("antecipando a prova: %v", err)
	}

	if got := r.slugsComAtraso(t, hoje); len(got) != 0 {
		t.Errorf("com atraso = %v, quer vazio depois de a prova passar", got)
	}
}

// O dia da prova também não entra: absorverAtraso desiste quando hoje já não é
// anterior à prova, então listar o plano seria carregar tudo para nada.
func TestPlanoRepo_ComAtraso_ignoraODiaDaProva(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "diadaprova@b.c")
	c := r.criarConcurso(t, u, "trf-1")
	p := r.criarPlano(t, u, c)

	ontem := dia(2026, time.September, 1)
	hoje := dia(2026, time.September, 2)

	r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(c.Disciplinas[0], ontem, 0, "vencida"),
	})

	if _, err := r.pool.Exec(
		t.Context(), `UPDATE planos SET prova = $2 WHERE id = $1`, p.ID, hoje,
	); err != nil {
		t.Fatalf("marcando a prova para hoje: %v", err)
	}

	if got := r.slugsComAtraso(t, hoje); len(got) != 0 {
		t.Errorf("com atraso = %v, quer vazio no dia da prova", got)
	}
}
