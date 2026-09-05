// Command worker runs the two daily chores: rescheduling the plans that fell
// behind and dispatching the spaced-review reminders. It shares the domain
// engine and repositories with the server; only the entrypoint differs.
package main

import (
	"context"
	"fmt"
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

	planos := postgres.NewPlanoRepo(pool)
	cronogramas := postgres.NewCronogramaRepo(pool)
	concursos := postgres.NewConcursoRepo(pool)

	svc := service.NewNotificacaoService(
		planos,
		cronogramas,
		concursos,
		notifier.NewSlogNotifier(logger),
		port.SystemClock{},
	)

	replanejamento := service.NewCronogramaService(service.Dependencias{
		Planos:     planos,
		Cronograma: cronogramas,
		Concursos:  concursos,
		Caderno:    postgres.NewCadernoRepo(pool),
		Usuarios:   postgres.NewUsuarioRepo(pool),
		Relogio:    port.SystemClock{},
	})

	// Override de desenvolvimento: esperar a meia-noite para ver o worker rodar
	// torna o ciclo de trabalho impraticável. O nome é legado de quando ele só
	// mandava lembrete — trocá-lo quebraria os .env que já existem.
	var intervaloFixo time.Duration

	if v := os.Getenv("LEMBRETE_INTERVALO"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("LEMBRETE_INTERVALO inválido (%q): %w", v, err)
		}

		intervaloFixo = d
	}

	logger.Info(
		"worker starting",
		slog.String("fuso", port.Fuso.String()),
		slog.Bool("intervalo_fixo", intervaloFixo > 0),
	)

	// Uma passada agora, antes de esperar a virada: se o processo ficou fora do
	// ar durante uma meia-noite, o atraso daquele dia continua lá esperando.
	tick(ctx, logger, svc, replanejamento)

	for {
		espera := proximaVirada(time.Now().In(port.Fuso))
		if intervaloFixo > 0 {
			espera = intervaloFixo
		}

		logger.Info(
			"aguardando a virada do dia",
			slog.Duration("em", espera.Truncate(time.Second)),
		)

		timer := time.NewTimer(espera)

		select {
		case <-ctx.Done():
			timer.Stop()
			logger.Info("worker stopped")

			return nil
		case <-timer.C:
			tick(ctx, logger, svc, replanejamento)
		}
	}
}

// proximaVirada é quanto falta para as 00:00 do dia seguinte, no fuso dado.
//
// Recalculado a cada volta em vez de um ticker de 24h: um ticker que começa às
// 15h dispara às 15h para sempre, e num fuso com horário de verão ele iria
// derivando uma hora por mudança. Somar um dia de calendário e zerar acerta a
// meia-noite sempre.
//
// O fuso é o mesmo de port.Fuso, que é onde o domínio decide o que é "hoje":
// a varredura roda no instante exato em que o dia vira para o cronograma.
func proximaVirada(agora time.Time) time.Duration {
	meiaNoite := time.Date(
		agora.Year(), agora.Month(), agora.Day()+1,
		0, 0, 0, 0, agora.Location(),
	)

	return meiaNoite.Sub(agora)
}

// O replanejamento vem ANTES do lembrete de propósito: o lembrete conta o que
// estudar hoje, e hoje só está certo depois que os dias perdidos foram
// absorvidos. Na ordem inversa o estudante receberia a agenda de ontem.
func tick(
	ctx context.Context,
	logger *slog.Logger,
	svc *service.NotificacaoService,
	replanejamento *service.CronogramaService,
) {
	replanejados, err := replanejamento.AbsorverAtrasosDoDia(ctx)
	if err != nil {
		// Erro num plano não impede o lembrete dos outros.
		logger.ErrorContext(ctx, "absorvendo atrasos", slog.Any("error", err))
	}

	if replanejados > 0 {
		logger.InfoContext(ctx, "planos replanejados", slog.Int("planos", replanejados))
	}

	enviados, err := svc.EnviarLembretesDoDia(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "dispatching lembretes", slog.Any("error", err))
		return
	}

	logger.InfoContext(ctx, "lembretes despachados", slog.Int("enviados", enviados))
}
