package config

import (
	"strings"
	"testing"
	"time"
)

// A configuração é lida uma vez, na partida. Tudo que ela deixa passar vira
// comportamento do processo até o próximo deploy — por isso o que ela RECUSA
// importa mais do que o que ela aceita.
//
// Sem t.Parallel: estes testes mexem no ambiente do processo (t.Setenv).

const segredoBom = "0123456789abcdef0123456789abcdef" // exatamente 32

func ambienteMinimo(t *testing.T) {
	t.Helper()

	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("JWT_SECRET", segredoBom)
}

func TestLoad_padroesRazoaveis(t *testing.T) {
	ambienteMinimo(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.ServerAddr != ":8080" {
		t.Errorf("ServerAddr = %q, quer :8080", cfg.ServerAddr)
	}

	if cfg.AccessTTL != 15*time.Minute {
		t.Errorf("AccessTTL = %v, quer 15m", cfg.AccessTTL)
	}

	if cfg.RefreshTTL != 720*time.Hour {
		t.Errorf("RefreshTTL = %v, quer 720h", cfg.RefreshTTL)
	}

	if !cfg.RunMigrations {
		t.Error("RunMigrations devia vir ligado por padrão")
	}

	// Os parâmetros do argon2 seguem a recomendação da OWASP.
	if cfg.Argon2.Memory != 19*1024 || cfg.Argon2.Iterations != 2 {
		t.Errorf("Argon2 = %+v, quer 19 MiB e 2 iterações", cfg.Argon2)
	}
}

// A regra nova: HS256 assina com o segredo cru, então um segredo curto é uma
// senha curta — quebrável offline, sem falar com o servidor. Recusar na partida
// é o que impede alguém de emitir token com ele por engano.
func TestLoad_recusaSegredoCurto(t *testing.T) {
	ambienteMinimo(t)
	t.Setenv("JWT_SECRET", strings.Repeat("a", tamanhoMinimoSegredo-1))

	_, err := Load()
	if err == nil {
		t.Fatal("um JWT_SECRET curto devia impedir a partida")
	}

	// A mensagem precisa dizer o tamanho e como gerar um bom: descobrir isso
	// olhando o código é o tipo de atrito que faz alguém baixar o mínimo.
	for _, esperado := range []string{"31", "32", "openssl"} {
		if !strings.Contains(err.Error(), esperado) {
			t.Errorf("mensagem = %q, quer conter %q", err, esperado)
		}
	}
}

func TestLoad_aceitaSegredoNoTamanhoExato(t *testing.T) {
	ambienteMinimo(t)
	t.Setenv("JWT_SECRET", strings.Repeat("a", tamanhoMinimoSegredo))

	if _, err := Load(); err != nil {
		t.Errorf("um segredo no tamanho mínimo devia passar: %v", err)
	}
}

func TestLoad_exigeOEssencial(t *testing.T) {
	casos := map[string]struct {
		db     string
		jwt    string
		contem string
	}{
		"sem DATABASE_URL": {db: "", jwt: segredoBom, contem: "DATABASE_URL"},
		"sem JWT_SECRET":   {db: "postgres://x", jwt: "", contem: "JWT_SECRET"},
	}

	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			t.Setenv("DATABASE_URL", caso.db)
			t.Setenv("JWT_SECRET", caso.jwt)

			_, err := Load()
			if err == nil {
				t.Fatal("quer erro")
			}

			if !strings.Contains(err.Error(), caso.contem) {
				t.Errorf("mensagem = %q, quer nomear %s", err, caso.contem)
			}
		})
	}
}

func TestLoad_recusaDuracaoIlegivel(t *testing.T) {
	ambienteMinimo(t)
	t.Setenv("JWT_ACCESS_TTL", "quinze minutos")

	if _, err := Load(); err == nil {
		t.Error("uma duração ilegível devia impedir a partida, não virar o padrão")
	}
}

// Um número inválido numa variável opcional cai no padrão em vez de derrubar o
// processo: são valores de ajuste fino, e falhar neles custaria mais do que
// ignorá-los.
func TestLoad_numeroInvalidoCaiNoPadrao(t *testing.T) {
	ambienteMinimo(t)
	t.Setenv("ARGON2_ITERATIONS", "muitas")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Argon2.Iterations != 2 {
		t.Errorf("Iterations = %d, quer o padrão 2", cfg.Argon2.Iterations)
	}
}

func TestLoad_ambienteSobrescreveOsPadroes(t *testing.T) {
	ambienteMinimo(t)
	t.Setenv("SERVER_ADDR", ":9090")
	t.Setenv("CORS_ORIGIN", "https://exemplo.tld")
	t.Setenv("RUN_MIGRATIONS", "false")
	t.Setenv("JWT_ACCESS_TTL", "5m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.ServerAddr != ":9090" || cfg.CORSOrigin != "https://exemplo.tld" {
		t.Errorf("cfg = %+v", cfg)
	}

	if cfg.RunMigrations {
		t.Error("RUN_MIGRATIONS=false devia desligar as migrations")
	}

	if cfg.AccessTTL != 5*time.Minute {
		t.Errorf("AccessTTL = %v, quer 5m", cfg.AccessTTL)
	}
}

// Sem EDITAL_PROCESSOR_URL a importação por IA fica desligada, e o cadastro
// manual continua funcionando — é o que a raiz de composição verifica.
func TestLoad_semProcessadorDeEdital(t *testing.T) {
	ambienteMinimo(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.EditalProcessorURL != "" {
		t.Errorf("EditalProcessorURL = %q, quer vazio", cfg.EditalProcessorURL)
	}
}
