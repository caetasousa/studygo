//go:build integration

package db_test

import (
	"context"
	"os"
	"slices"
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
		"disciplinas", "fontes", "marco_checks", "marcos", "plano_ciclo",
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

	if aplicadas != 1 {
		t.Errorf("schema_migrations tem %d linhas, quer 1 (a baseline)", aplicadas)
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

	if aplicadas != 1 {
		t.Errorf(
			"schema_migrations tem %d linhas depois de %d migrações concorrentes, quer 1",
			aplicadas, processos,
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
