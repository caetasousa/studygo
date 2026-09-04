//go:build integration

// Package pgtest sobe um PostgreSQL efêmero para os testes de integração.
//
// Ele existe porque dois pacotes precisam da mesma coisa — os repositories e o
// runner de migrations. Não é um framework de testes: são duas funções, e a
// intenção é que continue assim.
//
// O que ele NÃO faz, de propósito:
//
//   - não lê .env, TEST_DATABASE_URL nem variável de ambiente alguma;
//   - não usa porta fixa: o container publica numa porta efêmera;
//   - não toca em nenhum banco que já exista na máquina.
//
// O harness anterior fazia as três coisas, e apagou o banco de desenvolvimento
// de verdade. Um teste nunca deveria conseguir isso.
package pgtest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"studygo/internal/platform/db"
	"studygo/migrations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// imagemPostgres acompanha o que a aplicação roda em desenvolvimento e em
// produção (docker-compose.yml e ansible/templates/docker-compose.prod.yml.j2).
// Testar contra outra versão testaria outro banco.
const imagemPostgres = "postgres:18-alpine"

// bancoModelo é migrado uma vez por pacote e serve de TEMPLATE para os bancos
// de cada teste: copiar um banco pronto custa milissegundos, migrar custa
// centenas deles.
//
// bancoAdmin é onde as conexões administrativas vivem. Ele existe porque
// CREATE DATABASE ... TEMPLATE exige que NINGUÉM esteja conectado ao template:
// se o pool administrativo apontasse para o modelo, ele bloquearia a própria
// cópia que veio fazer.
const (
	bancoModelo = "modelo"
	bancoAdmin  = "postgres"
)

const (
	usuario = "teste"
	senha   = "teste"
)

// servidor é o container compartilhado pelos testes de UM pacote. Compartilhar
// entre pacotes exigiria estado global entre processos, que é justamente o que
// dá vazamento silencioso.
type servidor struct {
	container *tcpostgres.PostgresContainer
	urlAdmin  string
}

var (
	umaVez sync.Once
	srv    *servidor
	errSrv error
)

// Novo devolve um pool ligado a um banco vazio e recém-migrado, exclusivo deste
// teste.
//
// O container sobe na primeira chamada do pacote; cada teste ganha o seu próprio
// database, então dois testes nunca enxergam os dados um do outro e podem rodar
// em paralelo.
func Novo(t *testing.T) *pgxpool.Pool {
	t.Helper()

	umaVez.Do(func() { srv, errSrv = subir() })

	if errSrv != nil {
		t.Fatalf(
			"não consegui subir o PostgreSQL de teste: %v\n\n"+
				"Estes testes exigem Docker. Rode `make check` para a suíte rápida, "+
				"que não precisa de container.",
			errSrv,
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	nome := nomeDeBanco()

	admin, err := pgxpool.New(ctx, srv.urlAdmin)
	if err != nil {
		t.Fatalf("conectando ao servidor de teste: %v", err)
	}

	// CREATE DATABASE não aceita parâmetro nem roda em transação; o nome é
	// gerado aqui (hex de um uuid), nunca vem de fora.
	_, err = admin.Exec(ctx, fmt.Sprintf(
		`CREATE DATABASE %s TEMPLATE %s`, nome, bancoModelo,
	))

	admin.Close()

	if err != nil {
		t.Fatalf("criando o banco do teste: %v", err)
	}

	pool, err := db.Connect(ctx, srv.urlDe(nome))
	if err != nil {
		t.Fatalf("conectando ao banco do teste: %v", err)
	}

	// O pool fecha ANTES do banco ser derrubado: uma conexão aberta impediria o
	// DROP e deixaria lixo para trás.
	t.Cleanup(func() {
		pool.Close()
		derrubar(nome)
	})

	return pool
}

// NovoVazio devolve um pool ligado a um banco SEM migrations — o ponto de
// partida do teste do runner, que precisa provar que ele cria o schema do zero.
func NovoVazio(t *testing.T) *pgxpool.Pool {
	t.Helper()

	umaVez.Do(func() { srv, errSrv = subir() })

	if errSrv != nil {
		t.Fatalf("não consegui subir o PostgreSQL de teste: %v", errSrv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	nome := nomeDeBanco()

	admin, err := pgxpool.New(ctx, srv.urlAdmin)
	if err != nil {
		t.Fatalf("conectando ao servidor de teste: %v", err)
	}

	_, err = admin.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s`, nome))

	admin.Close()

	if err != nil {
		t.Fatalf("criando o banco do teste: %v", err)
	}

	pool, err := db.Connect(ctx, srv.urlDe(nome))
	if err != nil {
		t.Fatalf("conectando ao banco do teste: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		derrubar(nome)
	})

	return pool
}

// subir cria o container e deixa o banco-modelo migrado e pronto para clonagem.
func subir() (*servidor, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := tcpostgres.Run(ctx, imagemPostgres,
		tcpostgres.WithDatabase(bancoAdmin),
		tcpostgres.WithUsername(usuario),
		tcpostgres.WithPassword(senha),
		// O readiness do próprio módulo: o Postgres registra "ready to accept
		// connections" duas vezes (uma no fim do init), então esperar a segunda
		// é o que evita conectar no servidor temporário do initdb.
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(2*time.Minute),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("subindo o container: %w", err)
	}

	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)

		return nil, fmt.Errorf("obtendo a connection string: %w", err)
	}

	s := &servidor{container: container, urlAdmin: url}

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		_ = container.Terminate(ctx)

		return nil, fmt.Errorf("conectando ao servidor: %w", err)
	}

	_, err = admin.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s`, bancoModelo))

	admin.Close()

	if err != nil {
		_ = container.Terminate(ctx)

		return nil, fmt.Errorf("criando o banco-modelo: %w", err)
	}

	// O modelo é migrado UMA vez; os bancos dos testes o copiam. A conexão é
	// fechada em seguida para liberá-lo como template.
	pool, err := db.Connect(ctx, s.urlDe(bancoModelo))
	if err != nil {
		_ = container.Terminate(ctx)

		return nil, fmt.Errorf("conectando ao banco-modelo: %w", err)
	}

	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		pool.Close()
		_ = container.Terminate(ctx)

		return nil, fmt.Errorf("migrando o banco-modelo: %w", err)
	}

	pool.Close()

	return s, nil
}

// Encerrar derruba o container. Chame de TestMain, depois de m.Run(), para que
// nenhum container fique órfão — inclusive quando um teste falha.
func Encerrar() {
	if srv == nil || srv.container == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_ = srv.container.Terminate(ctx)
	srv = nil
}

// urlDe monta a connection string de outro database do mesmo servidor,
// trocando só o caminho da URL que o container devolveu.
func (s *servidor) urlDe(banco string) string {
	if i := strings.LastIndex(s.urlAdmin, "/"+bancoAdmin); i >= 0 {
		return s.urlAdmin[:i] + "/" + banco + s.urlAdmin[i+len("/"+bancoAdmin):]
	}

	return s.urlAdmin
}

// derrubar apaga o banco de um teste que terminou. Uma falha aqui não quebra o
// teste: o container inteiro morre no fim do pacote de qualquer jeito.
func derrubar(nome string) {
	if srv == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, srv.urlAdmin)
	if err != nil {
		return
	}
	defer admin.Close()

	_, _ = admin.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s (FORCE)`, nome))
}

// nomeDeBanco gera um identificador único e seguro para interpolar: só letras e
// dígitos, começando por letra.
func nomeDeBanco() string {
	return "t" + strings.ReplaceAll(uuid.NewString(), "-", "")
}
