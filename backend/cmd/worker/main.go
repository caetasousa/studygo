// Command worker dispatches the daily spaced-review reminders. It shares the
// domain engine and repositories with the server; only the entrypoint differs.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"studygo/internal/adapter/notifier"
	"studygo/internal/adapter/postgres"
	"studygo/internal/platform/config"
	"studygo/internal/platform/db"
	"studygo/internal/port"
	"studygo/internal/service"
	"studygo/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("worker exited", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	intervalo := 24 * time.Hour
	if v := os.Getenv("LEMBRETE_INTERVALO"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			intervalo = d
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Shares the advisory-locked runner with the server, so whichever process
	// wins the race applies the migrations and the other waits.
	if cfg.RunMigrations {
		if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
			return err
		}
	}

	svc := service.NewNotificacaoService(
		postgres.NewPlanoRepo(pool),
		postgres.NewCronogramaRepo(pool),
		postgres.NewConcursoRepo(pool),
		notifier.NewSlogNotifier(logger),
		port.SystemClock{},
	)

	logger.Info("worker starting", slog.Duration("intervalo", intervalo))

	tick(ctx, logger, svc)

	ticker := time.NewTicker(intervalo)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopped")
			return nil
		case <-ticker.C:
			tick(ctx, logger, svc)
		}
	}
}

func tick(ctx context.Context, logger *slog.Logger, svc *service.NotificacaoService) {
	enviados, err := svc.EnviarLembretesDoDia(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "dispatching lembretes", slog.Any("error", err))
		return
	}

	logger.InfoContext(ctx, "lembretes despachados", slog.Int("enviados", enviados))
}
