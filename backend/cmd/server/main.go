package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"studygo/internal/adapter/crypto"
	"studygo/internal/adapter/editalproc"
	"studygo/internal/adapter/httpapi"
	"studygo/internal/adapter/postgres"
	"studygo/internal/platform/config"
	"studygo/internal/platform/db"
	"studygo/internal/platform/httpserver"
	"studygo/internal/platform/middleware"
	"studygo/internal/port"
	"studygo/internal/service"
	"studygo/migrations"
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

	usuarioRepo := postgres.NewUsuarioRepo(pool)
	concursoRepo := postgres.NewConcursoRepo(pool)
	planoRepo := postgres.NewPlanoRepo(pool)
	cronogramaRepo := postgres.NewCronogramaRepo(pool)
	cadernoRepo := postgres.NewCadernoRepo(pool)

	var editalProc port.EditalProcessor = editalproc.Indisponivel{}
	if cfg.EditalProcessorURL != "" {
		editalProc = editalproc.New(cfg.EditalProcessorURL, cfg.EditalProcessorToken)
		logger.Info("edital import enabled", slog.String("processor", cfg.EditalProcessorURL))
	}

	authService := service.NewAuthService(usuarioRepo, hasher, tokens, clock, cfg.RefreshTTL)

	// Os seis casos de uso do plano compartilham as mesmas dependências.
	deps := service.Dependencias{
		Planos:     planoRepo,
		Cronograma: cronogramaRepo,
		Concursos:  concursoRepo,
		Caderno:    cadernoRepo,
		Usuarios:   usuarioRepo,
		Relogio:    clock,
	}

	handlers := httpapi.Handlers{
		Health: httpapi.NewHealthHandler(service.NewHealthService(pool), logger),
		Auth:   httpapi.NewAuthHandler(authService, logger),
		Concurso: httpapi.NewConcursoHandler(
			service.NewConcursoService(concursoRepo, editalProc), logger,
		),
		Plano: httpapi.NewPlanoHandler(
			service.NewPlanoService(deps),
			service.NewCronogramaService(deps),
			service.NewRegistroService(deps),
			service.NewEstatisticaService(deps),
			service.NewCadernoService(deps),
			service.NewDossieService(deps),
			service.NewExportacaoService(deps),
			service.NewImportacaoTECService(deps),
			logger,
		),
	}

	router := httpapi.NewRouter(handlers, tokens, authService, logger)

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
		// The edital-import endpoints wait on Gemini (up to ~90s per call, two
		// calls in a step). Everything else answers in milliseconds; a generous
		// write timeout just keeps a slow-but-successful AI response from being
		// cut off mid-body.
		httpserver.WithWriteTimeout(240*time.Second),
	)

	logger.Info("server starting", slog.String("addr", cfg.ServerAddr))

	if err := srv.Run(ctx); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
