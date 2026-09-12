package db_test

import (
	"regexp"
	"slices"
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

// Rollback de código não desfaz schema: o runner só aplica .up.sql. Se a versão
// que sai removeu ou mudou algo que a anterior ainda usa, voltar para ela
// quebra — e o botão de rollback não tem como saber disso.
//
// A saída é expand/contract: primeiro uma publicação que para de usar, depois
// outra que remove. Este teste não consegue verificar a ordem das publicações,
// mas obriga quem escreve a migration a declarar qual versão já parou de usar
// o que ela tira. Sem o marcador, o build falha.
func TestMigrations_DestrutivaDeclaraOContract(t *testing.T) {
	t.Parallel()

	entradas, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("lendo migrations: %v", err)
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

		if achados := destrutivosSemContract(string(conteudo)); len(achados) > 0 {
			t.Errorf(
				"%s tem %s sem o marcador de contract. Rollback de código não desfaz "+
					"schema: se a versão anterior ainda usa o que isto tira, voltar para ela "+
					"quebra. Faça em duas publicações (expand/contract, docs/ci-cd.md) e "+
					"declare no arquivo quem já parou de usar:\n"+
					"  -- contract: a versão vAAAA.MM.DD parou de ler tabela.coluna",
				nome, strings.Join(achados, ", "),
			)
		}
	}
}

// O que conta como destrutivo, fixado caso a caso. O falso positivo mais fácil é
// o NOT NULL: ADD COLUMN ... NOT NULL DEFAULT '' é aditivo — é o que a 000003
// faz —, e só o SET NOT NULL numa coluna existente quebra quem insere sem ela.
func TestDestrutivosSemContract(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome string
		sql  string
		quer []string
	}{
		{"coluna nova NOT NULL com default", `ALTER TABLE disciplinas ADD COLUMN notebook_url text NOT NULL DEFAULT '';`, nil},
		{"tabela e índice novos", "CREATE TABLE x (id int);\nCREATE INDEX x_id ON x (id);", nil},
		{"prosa citando o comando", "-- antes havia um DROP TABLE aqui\nCREATE TABLE x (id int);", nil},
		{"drop table", `DROP TABLE IF EXISTS plano_ciclo;`, []string{"DROP TABLE"}},
		{"drop column", `ALTER TABLE planos DROP COLUMN ciclo;`, []string{"DROP COLUMN"}},
		{"rename column", `ALTER TABLE planos RENAME COLUMN a TO b;`, []string{"RENAME"}},
		{"rename table", `ALTER TABLE planos RENAME TO planos_antigos;`, []string{"RENAME"}},
		{"troca de tipo", `ALTER TABLE planos ALTER COLUMN horas TYPE numeric;`, []string{"ALTER COLUMN ... TYPE"}},
		{"troca de tipo, forma longa", `ALTER TABLE planos ALTER horas SET DATA TYPE numeric;`, []string{"ALTER COLUMN ... TYPE"}},
		{"set not null", `ALTER TABLE planos ALTER COLUMN nome SET NOT NULL;`, []string{"SET NOT NULL"}},
		{"truncate", `TRUNCATE registros_dia;`, []string{"TRUNCATE"}},
		{"minúsculas também", `alter table planos drop column ciclo;`, []string{"DROP COLUMN"}},
		{"com o marcador", "-- contract: a versão v2026.09.05 parou de ler planos.ciclo\nALTER TABLE planos DROP COLUMN ciclo;", nil},
		{"marcador vazio não vale", "-- contract:\nALTER TABLE planos DROP COLUMN ciclo;", []string{"DROP COLUMN"}},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			if got := destrutivosSemContract(c.sql); !slices.Equal(got, c.quer) {
				t.Errorf("destrutivosSemContract = %v, quer %v", got, c.quer)
			}
		})
	}
}

// destrutivos são os comandos que tornam inseguro voltar o código: o código
// antigo continua lendo ou escrevendo o que eles tiram ou mudam.
var destrutivos = []struct {
	nome string
	re   *regexp.Regexp
}{
	{"DROP TABLE", regexp.MustCompile(`\bDROP\s+TABLE\b`)},
	{"DROP COLUMN", regexp.MustCompile(`\bDROP\s+COLUMN\b`)},
	// Qualquer RENAME: tabela, coluna ou constraint, o código antigo procura o
	// nome velho.
	{"RENAME", regexp.MustCompile(`\bRENAME\b`)},
	// COLUMN é opcional no Postgres, e SET DATA TYPE é a forma longa.
	{"ALTER COLUMN ... TYPE", regexp.MustCompile(`\bALTER\s+(COLUMN\s+)?\S+\s+(SET\s+DATA\s+)?TYPE\b`)},
	// SET NOT NULL, e não NOT NULL solto: o solto casaria o ADD COLUMN aditivo.
	{"SET NOT NULL", regexp.MustCompile(`\bSET\s+NOT\s+NULL\b`)},
	{"TRUNCATE", regexp.MustCompile(`\bTRUNCATE\b`)},
}

// marcadorContract exige texto depois dos dois-pontos: um marcador vazio só
// silenciaria o teste sem dizer quem parou de usar o quê.
// Espaço e tab, não \s: o \s atravessaria a quebra de linha e aceitaria o
// comando da linha seguinte como se fosse o texto do marcador.
var marcadorContract = regexp.MustCompile(`(?im)^[ \t]*--[ \t]*contract:[ \t]*\S`)

// destrutivosSemContract lista os comandos destrutivos de uma migration que não
// declara o contract. Com o marcador, nada é cobrado.
func destrutivosSemContract(sql string) []string {
	if marcadorContract.MatchString(sql) {
		return nil
	}

	texto := strings.ToUpper(semComentarios(sql))

	var achados []string

	for _, d := range destrutivos {
		if d.re.MatchString(texto) {
			achados = append(achados, d.nome)
		}
	}

	return achados
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
