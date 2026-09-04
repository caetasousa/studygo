package db_test

import (
	"strings"
	"testing"

	"studygo/migrations"
)

// Este teste NÃO precisa de banco: ele só lê os arquivos SQL embutidos. Por isso
// fica fora da tag `integration` e roda em `make check`, junto com a suíte
// rápida — a regra que ele protege vale a cada commit, não só quando alguém
// lembra de subir o Docker.

// Migrations criam ESTRUTURA. Backfill, função e trigger são regra de negócio
// disfarçada de schema: elas ficam invisíveis para os testes de domínio, não
// aparecem no code review como código, e se tornam a segunda fonte de verdade de
// uma regra que já existe em Go.
func TestMigrations_NaoContemLogicaDeNegocio(t *testing.T) {
	t.Parallel()

	entradas, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("lendo migrations: %v", err)
	}

	proibidos := []string{
		"CREATE FUNCTION", "CREATE OR REPLACE FUNCTION", "CREATE TRIGGER",
		"INSERT INTO", "UPDATE ", "DELETE FROM",
	}

	for _, e := range entradas {
		nome := e.Name()
		if !strings.HasSuffix(nome, ".up.sql") {
			continue
		}

		conteudo, err := migrations.FS.ReadFile(nome)
		if err != nil {
			t.Fatalf("lendo %s: %v", nome, err)
		}

		texto := strings.ToUpper(semComentarios(string(conteudo)))

		for _, p := range proibidos {
			if strings.Contains(texto, p) {
				t.Errorf(
					"%s contém %q — migrations criam estrutura; backfill e regra de "+
						"negócio pertencem ao domínio e à aplicação",
					nome, p,
				)
			}
		}
	}
}

// semComentarios remove as linhas `--`, para que a prosa explicativa não
// dispare o teste ao citar um comando.
func semComentarios(sql string) string {
	var b strings.Builder

	for linha := range strings.SplitSeq(sql, "\n") {
		if i := strings.Index(linha, "--"); i >= 0 {
			linha = linha[:i]
		}

		b.WriteString(linha)
		b.WriteByte('\n')
	}

	return b.String()
}
