//go:build integration

package db_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"studygo/internal/platform/db"
	"studygo/internal/platform/pgtest"
	"studygo/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

// O teste que o resto da suíte não consegue dar: as migrations sobem num
// PostgreSQL VAZIO e produzem o schema que os repositories esperam.
//
// Roda contra um container efêmero — nunca contra um banco que já exista na
// máquina.

// quantasMigrations conta os .up.sql do bundle. Contar em vez de fixar um
// número faz este teste continuar valendo quando uma migration nova entra —
// o que ele afirma é "aplicou cada uma UMA vez", não "existem N".
func quantasMigrations(t *testing.T) int {
	t.Helper()

	entradas, err := fs.Glob(migrations.FS, "*.up.sql")
	if err != nil {
		t.Fatalf("listando migrations: %v", err)
	}

	if len(entradas) == 0 {
		t.Fatal("nenhuma migration no bundle")
	}

	return len(entradas)
}

func TestMain(m *testing.M) {
	codigo := m.Run()
	pgtest.Encerrar()
	os.Exit(codigo)
}

func TestMigrate_CriaSchemaAPartirDeBancoVazio(t *testing.T) {
	t.Parallel()

	pool := pgtest.NovoVazio(t)
	ctx := t.Context()

	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatalf("migrando banco vazio: %v", err)
	}

	esperadas := []string{
		"anotacoes", "atividades", "concursos", "conteudo_programatico",
		"disciplinas", "fontes", "marco_checks", "marcos",
		"plano_disciplinas", "planos", "refresh_tokens", "registros_atividade",
		"registros_dia", "schema_migrations", "temas", "usuarios",
	}

	obtidas := tabelas(t, pool)

	if !slices.Equal(obtidas, esperadas) {
		t.Errorf("tabelas criadas =\n  %v\nquer\n  %v", obtidas, esperadas)
	}
}

// Repetir o deploy é seguro: rodar as migrations de novo não pode falhar nem
// duplicar nada.
func TestMigrate_EIdempotente(t *testing.T) {
	t.Parallel()

	pool := pgtest.NovoVazio(t)
	ctx := t.Context()

	for i := range 2 {
		if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
			t.Fatalf("migração %d: %v", i+1, err)
		}
	}

	var aplicadas int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&aplicadas); err != nil {
		t.Fatalf("contando migrations: %v", err)
	}

	if quer := quantasMigrations(t); aplicadas != quer {
		t.Errorf("schema_migrations tem %d linhas, quer %d (uma por migration)", aplicadas, quer)
	}
}

// O server e o worker sobem juntos e ambos migram. O advisory lock existe para
// que um espere o outro em vez de os dois aplicarem a mesma migration —
// serializar isso é a razão de o runner pegar o lock antes de qualquer coisa.
func TestMigrate_ConcorrenteNaoDuplica(t *testing.T) {
	t.Parallel()

	pool := pgtest.NovoVazio(t)

	const processos = 4

	var (
		grupo sync.WaitGroup
		erros = make([]error, processos)
	)

	grupo.Add(processos)

	for i := range processos {
		go func() {
			defer grupo.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			erros[i] = db.Migrate(ctx, pool, migrations.FS)
		}()
	}

	grupo.Wait()

	for i, err := range erros {
		if err != nil {
			t.Errorf("processo %d falhou ao migrar: %v", i, err)
		}
	}

	var aplicadas int
	if err := pool.QueryRow(
		t.Context(), `SELECT count(*) FROM schema_migrations`,
	).Scan(&aplicadas); err != nil {
		t.Fatalf("contando migrations: %v", err)
	}

	if quer := quantasMigrations(t); aplicadas != quer {
		t.Errorf(
			"schema_migrations tem %d linhas depois de %d migrações concorrentes, quer %d",
			aplicadas, processos, quer,
		)
	}
}

// tabelas lista o schema public, em ordem, para comparar com o esperado.
func tabelas(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()

	rows, err := pool.Query(t.Context(),
		`SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`)
	if err != nil {
		t.Fatalf("listando tabelas: %v", err)
	}
	defer rows.Close()

	var out []string

	for rows.Next() {
		var nome string
		if err := rows.Scan(&nome); err != nil {
			t.Fatalf("lendo nome de tabela: %v", err)
		}

		out = append(out, nome)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterando tabelas: %v", err)
	}

	return out
}

// Um banco de OUTRA linhagem do projeto tem schema_migrations com a versão 1
// registrada — a baseline daquela época — e nenhuma das tabelas desta. O runner
// não teria o que aplicar, e o backend subiria contra o schema errado: /health
// só dá ping, responde 200, e o deploy é declarado bom com tudo quebrado.
func TestMigrate_RecusaBancoDeOutraLinhagem(t *testing.T) {
	t.Parallel()

	pool := pgtest.NovoVazio(t)
	ctx := t.Context()

	// O banco antigo: a versão 1 consta como aplicada, mas o schema é outro.
	if _, err := pool.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version    integer     PRIMARY KEY,
			name       text        NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		);
		INSERT INTO schema_migrations (version, name) VALUES (1, 'initial_schema');
		CREATE TABLE registros_bloco (id uuid PRIMARY KEY)`,
	); err != nil {
		t.Fatalf("montando o banco antigo: %v", err)
	}

	err := db.Migrate(ctx, pool, migrations.FS)
	if err == nil {
		t.Fatal("migrar sobre o banco de outra linhagem devia falhar")
	}

	if !errors.Is(err, db.ErrBancoDeOutraLinhagem) {
		t.Fatalf("erro = %v, quer ErrBancoDeOutraLinhagem", err)
	}

	// A mensagem tem de dizer o que fazer, não só que deu errado.
	if !strings.Contains(err.Error(), "banco vazio") {
		t.Errorf("mensagem = %q, devia apontar a saída", err)
	}
}
