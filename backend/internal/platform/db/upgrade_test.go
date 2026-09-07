//go:build integration

package db_test

import (
	"strings"
	"testing"

	"studygo/internal/platform/db"
	"studygo/internal/platform/pgtest"
	"studygo/migrations"
)

// O caminho que o deploy percorre de verdade: um banco JÁ MIGRADO recebendo a
// migration nova.
//
// A suíte cobria banco vazio, repetição e concorrência — nenhum deles exercita
// o upgrade incremental, que é exatamente o que acontece em produção. Um banco
// vazio aplica tudo de uma vez e nunca esbarra numa migration que depende do
// que a anterior criou.
func TestMigrate_AplicaSomenteAsPendentes(t *testing.T) {
	t.Parallel()

	pool := pgtest.NovoVazio(t)
	ctx := t.Context()

	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatalf("primeira migração: %v", err)
	}

	total := quantasMigrations(t)

	// Volta o banco para o estado de quem só tinha a baseline: a linha some de
	// schema_migrations e a coluna que a última migration criou é desfeita.
	if _, err := pool.Exec(ctx,
		`DELETE FROM schema_migrations WHERE version = (SELECT max(version) FROM schema_migrations)`,
	); err != nil {
		t.Fatalf("rebobinando schema_migrations: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`ALTER TABLE disciplinas DROP COLUMN IF EXISTS notebook_url`,
	); err != nil {
		t.Fatalf("rebobinando a coluna: %v", err)
	}

	// O runner precisa aplicar só o que falta, sem tropeçar no que já existe.
	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatalf("upgrade incremental: %v", err)
	}

	var aplicadas int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&aplicadas); err != nil {
		t.Fatalf("contando migrations: %v", err)
	}

	if aplicadas != total {
		t.Errorf("schema_migrations tem %d linhas, quer %d", aplicadas, total)
	}

	var existe bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			 WHERE table_name = 'disciplinas' AND column_name = 'notebook_url'
		)`).Scan(&existe); err != nil {
		t.Fatalf("conferindo a coluna: %v", err)
	}

	if !existe {
		t.Error("a migration pendente não foi aplicada no banco já existente")
	}
}

// A checagem de linhagem roda ANTES de aplicar.
//
// Enquanto a única migration pendente era um DROP ... IF EXISTS, ela passava em
// qualquer banco e conferir a linhagem no fim bastava. A primeira migration que
// toca numa tabela existente desfaz isso: num banco de outra linhagem ela falha
// com um erro do Postgres, e era ESSE erro que chegava ao operador — não o
// diagnóstico que diz o que fazer.
//
// Este teste prende a ordem. Ele falha se alguém devolver a verificação para
// depois do laço.
func TestMigrate_LinhagemEChecadaAntesDeAplicar(t *testing.T) {
	t.Parallel()

	pool := pgtest.NovoVazio(t)
	ctx := t.Context()

	// Banco de outra linhagem: registra a versão 1 e não tem nenhuma tabela
	// desta aplicação — nem `disciplinas`, que a migration pendente altera.
	if _, err := pool.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version    integer     PRIMARY KEY,
			name       text        NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		);
		INSERT INTO schema_migrations (version, name) VALUES (1, 'initial_schema')`,
	); err != nil {
		t.Fatalf("montando o banco antigo: %v", err)
	}

	err := db.Migrate(ctx, pool, migrations.FS)
	if err == nil {
		t.Fatal("migrar sobre banco de outra linhagem devia falhar")
	}

	// O erro do Postgres não pode vazar no lugar do diagnóstico: quem lê o log
	// do deploy precisa saber o que fazer, não que falta uma relação.
	for _, esperado := range []string{"banco de outra linhagem", "banco vazio"} {
		if !strings.Contains(err.Error(), esperado) {
			t.Errorf("mensagem = %q; quer conter %q, não o erro cru do banco", err, esperado)
		}
	}

	// E nada pode ter sido registrado como aplicado.
	var aplicadas int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&aplicadas); err != nil {
		t.Fatalf("contando migrations: %v", err)
	}

	if aplicadas != 1 {
		t.Errorf("schema_migrations tem %d linhas, quer só a 1 que já estava lá", aplicadas)
	}
}
