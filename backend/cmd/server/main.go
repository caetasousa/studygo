package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"annygo/internal/adapter/ai"
	"annygo/internal/adapter/crypto"
	"annygo/internal/adapter/httpapi"
	"annygo/internal/adapter/postgres"
	"annygo/internal/platform/config"
	"annygo/internal/platform/db"
	"annygo/internal/platform/httpserver"
	"annygo/internal/platform/middleware"
	"annygo/internal/port"
	"annygo/internal/service"
	"annygo/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("server exited", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.RunMigrations {
		if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
			return err
		}

		logger.Info("migrations applied")
	}

	clock := port.SystemClock{}
	hasher := crypto.NewArgon2Hasher(cfg.Argon2)
	tokens := crypto.NewJWTIssuer(cfg.JWTSecret, cfg.AccessTTL)

	userRepo := postgres.NewUserRepo(pool)
	concursoRepo := postgres.NewConcursoRepo(pool)
	planoRepo := postgres.NewPlanoRepo(pool)

	var editalParser port.EditalAnalisador = ai.Indisponivel{}
	if cfg.GeminiAPIKey != "" {
		editalParser = ai.NewGeminiAnalisador(cfg.GeminiAPIKey, cfg.GeminiModel)
		logger.Info("edital import enabled", slog.String("model", cfg.GeminiModel))
	}

	handlers := httpapi.Handlers{
		Health:   httpapi.NewHealthHandler(service.NewHealthService(pool), logger),
		Auth:     httpapi.NewAuthHandler(service.NewAuthService(userRepo, hasher, tokens, clock, cfg.RefreshTTL), logger),
		Concurso: httpapi.NewConcursoHandler(service.NewConcursoService(concursoRepo, editalParser), logger),
		Plano:    httpapi.NewPlanoHandler(service.NewPlanoService(planoRepo, concursoRepo, clock), logger),
	}

	router := httpapi.NewRouter(handlers, tokens, logger)

	handler := middleware.Chain(
		router,
		middleware.RequestID,
		middleware.Recover(logger),
		middleware.Logger(logger),
		middleware.CORS(cfg.CORSOrigin),
	)

	srv := httpserver.New(
		cfg.ServerAddr,
		httpserver.WithHandler(handler),
	)

	logger.Info("server starting", slog.String("addr", cfg.ServerAddr))

	if err := srv.Run(ctx); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
