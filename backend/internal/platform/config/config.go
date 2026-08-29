package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
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

	GeminiAPIKey string
	GeminiModel  string
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
		ServerAddr:    getEnv("SERVER_ADDR", ":8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		CORSOrigin:    getEnv("CORS_ORIGIN", "http://localhost:5173"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		RunMigrations: getEnvBool("RUN_MIGRATIONS", true),
		GeminiAPIKey:  os.Getenv("GEMINI_API_KEY"),
		GeminiModel:   getEnv("GEMINI_MODEL", "gemini-flash-lite-latest"),
		Argon2: Argon2Params{
			Memory:      uint32(getEnvInt("ARGON2_MEMORY_KIB", 19*1024)),
			Iterations:  uint32(getEnvInt("ARGON2_ITERATIONS", 2)),
			Parallelism: uint8(getEnvInt("ARGON2_PARALLELISM", 1)),
			SaltLength:  16,
			KeyLength:   32,
		},
	}

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
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
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
