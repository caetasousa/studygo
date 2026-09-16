package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Config holds every value the process reads from the environment. It is loaded
// once in the composition root and passed down explicitly.
type Config struct {
	ServerAddr  string
	DatabaseURL string
	CORSOrigin  string

	JWTSecret     string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	Argon2        Argon2Params
	RunMigrations bool

	// EditalProcessorURL / Token point at the internal Python service that
	// processes edital PDFs. Empty URL disables AI import.
	EditalProcessorURL   string
	EditalProcessorToken string

	// Versao e Deploy identificam o que está no ar: a tag (ou o commit, fora de
	// tag) e a pipeline que implantou. Quem preenche é o deploy, não o build — a
	// mesma imagem sobe como versões diferentes, e o que se quer saber olhando o
	// servidor é qual publicação ele está servindo.
	Versao string
	Deploy string

	Provas Provas
}

// Provas configura o catálogo de provas. Sem curadores, ninguém importa nem
// publica, e o worker nem liga a fila — só a consulta funciona.
type Provas struct {
	// Dir é o volume durável compartilhado com o processador.
	Dir       string
	Curadores map[string]bool
	// TodosCuradores faz de qualquer conta curadora (PROVAS_CURADORES=*). É
	// atalho do ambiente local, onde recriar o banco muda o UUID das contas e
	// esvaziaria a lista em silêncio.
	TodosCuradores bool
	// MaxPDF é o teto de cada arquivo. O nginx precisa aceitar o dobro, porque
	// a importação leva prova e gabarito juntos.
	MaxPDF       int64
	MaxPendentes int
	// MaxChamadas e MaxProcessamento são os tetos de custo de uma importação.
	MaxChamadas      int
	MaxProcessamento time.Duration
	// ExigirConferencia faz a conferência do curador bloquear a publicação.
	ExigirConferencia bool
}

// Argon2Params configures the argon2id password hasher. Defaults follow the
// OWASP recommendation (19 MiB, 2 iterations, 1 lane).
type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func Load() (Config, error) {
	cfg := Config{
		ServerAddr:           getEnv("SERVER_ADDR", ":8080"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		CORSOrigin:           getEnv("CORS_ORIGIN", "http://localhost:5173"),
		JWTSecret:            os.Getenv("JWT_SECRET"),
		RunMigrations:        getEnvBool("RUN_MIGRATIONS", true),
		EditalProcessorURL:   os.Getenv("EDITAL_PROCESSOR_URL"),
		EditalProcessorToken: os.Getenv("EDITAL_PROCESSOR_TOKEN"),
		Versao:               getEnv("APP_VERSAO", "dev"),
		Deploy:               os.Getenv("APP_DEPLOY"),
	}

	argon2, err := lerArgon2()
	if err != nil {
		return Config{}, err
	}
	cfg.Argon2 = argon2

	accessTTL, err := time.ParseDuration(getEnv("JWT_ACCESS_TTL", "15m"))
	if err != nil {
		return Config{}, fmt.Errorf("parsing JWT_ACCESS_TTL: %w", err)
	}

	refreshTTL, err := time.ParseDuration(getEnv("JWT_REFRESH_TTL", "720h"))
	if err != nil {
		return Config{}, fmt.Errorf("parsing JWT_REFRESH_TTL: %w", err)
	}

	cfg.AccessTTL = accessTTL
	cfg.RefreshTTL = refreshTTL

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL é obrigatória")
	}

	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET é obrigatório")
	}

	// HS256 assina com o segredo cru: um segredo curto é uma senha curta, e uma
	// senha curta se quebra offline sem falar com o servidor. Trinta e dois
	// caracteres é o tamanho da chave que o algoritmo usa — abaixo disso a
	// margem é ilusória. Recusar na partida é melhor que descobrir depois: o
	// processo não sobe, e ninguém emite token com um segredo fraco por engano.
	if len(cfg.JWTSecret) < tamanhoMinimoSegredo {
		return Config{}, fmt.Errorf(
			"JWT_SECRET tem %d caracteres; o mínimo é %d (use: openssl rand -base64 48)",
			len(cfg.JWTSecret), tamanhoMinimoSegredo,
		)
	}

	provas, err := carregarProvas(cfg.Versao)
	if err != nil {
		return Config{}, err
	}
	cfg.Provas = provas

	return cfg, nil
}

func carregarProvas(versao string) (Provas, error) {
	p := Provas{
		Dir:               getEnv("PROVAS_DIR", "/var/lib/provas"),
		Curadores:         map[string]bool{},
		MaxPDF:            int64(getEnvInt("PROVAS_MAX_UPLOAD_MIB", 25)) << 20,
		MaxPendentes:      getEnvInt("PROVAS_MAX_PENDENTES", 2),
		MaxChamadas:       getEnvInt("PROVAS_MAX_CHAMADAS", 180),
		MaxProcessamento:  time.Duration(getEnvInt("PROVAS_MAX_MINUTOS", 20)) * time.Minute,
		ExigirConferencia: getEnvBool("PROVAS_EXIGIR_CONFERENCIA", true),
	}

	// UUID inválido derruba a partida: uma lista com erro de digitação que
	// subisse em silêncio tiraria a curadoria de alguém sem aviso.
	for _, bruto := range strings.Split(os.Getenv("PROVAS_CURADORES"), ",") {
		bruto = strings.TrimSpace(bruto)
		if bruto == "" {
			continue
		}
		// No ar, "*" deixaria qualquer conta aberta importar PDF, gastando
		// Gemini, e publicar no catálogo. O deploy sempre preenche APP_VERSAO,
		// então a partida recusa, e a pipeline para no smoke test de staging.
		if bruto == "*" {
			if versao != "dev" {
				return Provas{}, fmt.Errorf("PROVAS_CURADORES=* só vale no ambiente local; no deploy, liste os UUIDs")
			}
			p.TodosCuradores = true
			continue
		}
		id, err := uuid.Parse(bruto)
		if err != nil {
			return Provas{}, fmt.Errorf("PROVAS_CURADORES contém um UUID inválido: %q", bruto)
		}
		p.Curadores[id.String()] = true
	}

	if p.MaxPDF <= 0 || p.MaxPendentes <= 0 || p.MaxChamadas <= 0 || p.MaxProcessamento <= 0 {
		return Provas{}, fmt.Errorf("os limites de provas (PROVAS_MAX_*) precisam ser positivos")
	}

	return p, nil
}

// tamanhoMinimoSegredo é o piso do JWT_SECRET, em caracteres.
const tamanhoMinimoSegredo = 32

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

// lerArgon2 lê os parâmetros do hash de senha, recusando o que não cabe no
// tipo que a biblioteca espera. Convertido sem conferir, "ARGON2_MEMORY_KIB=-1"
// viraria 4 bilhões de KiB e o servidor subiria pedindo memória que não existe.
func lerArgon2() (Argon2Params, error) {
	memoria, err := inteiroNaFaixa("ARGON2_MEMORY_KIB", 19*1024, 8, math.MaxUint32)
	if err != nil {
		return Argon2Params{}, err
	}
	iteracoes, err := inteiroNaFaixa("ARGON2_ITERATIONS", 2, 1, math.MaxUint32)
	if err != nil {
		return Argon2Params{}, err
	}
	paralelismo, err := inteiroNaFaixa("ARGON2_PARALLELISM", 1, 1, math.MaxUint8)
	if err != nil {
		return Argon2Params{}, err
	}

	// As três faixas foram conferidas em inteiroNaFaixa, logo acima.
	return Argon2Params{
		Memory:      uint32(memoria),    //nolint:gosec // faixa conferida
		Iterations:  uint32(iteracoes),  //nolint:gosec // faixa conferida
		Parallelism: uint8(paralelismo), //nolint:gosec // faixa conferida
		SaltLength:  16,
		KeyLength:   32,
	}, nil
}

func inteiroNaFaixa(chave string, padrao, minimo, maximo int) (int, error) {
	v := getEnvInt(chave, padrao)
	if v < minimo || v > maximo {
		return 0, fmt.Errorf("%s = %d: precisa estar entre %d e %d", chave, v, minimo, maximo)
	}

	return v, nil
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}

	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}

	return b
}
