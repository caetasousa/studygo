// Package notifier holds outbound adapters for delivering reminders. The MVP
// ships only the slog adapter; an email adapter can implement port.Notifier
// later without touching the service.
package notifier

import (
	"context"
	"log/slog"

	"annygo/internal/port"
)

var _ port.Notifier = (*SlogNotifier)(nil)

// SlogNotifier logs each reminder it would send. Useful in development and as
// the default until an email provider is chosen.
type SlogNotifier struct {
	logger *slog.Logger
}

func NewSlogNotifier(logger *slog.Logger) *SlogNotifier {
	return &SlogNotifier{logger: logger}
}

func (n *SlogNotifier) EnviarLembrete(ctx context.Context, l port.Lembrete) error {
	temas := make([]string, 0, len(l.Itens))
	for _, it := range l.Itens {
		temas = append(temas, it.Disciplina+": "+it.Tema)
	}

	attrs := []any{
		slog.String("email", l.Email),
		slog.String("data", l.DataISO),
		slog.Int("itens", len(l.Itens)),
		slog.Any("temas", temas),
	}
	if l.Dica != "" {
		attrs = append(attrs, slog.String("dica", l.Dica))
	}

	n.logger.InfoContext(ctx, "lembrete de revisão espaçada", attrs...)

	return nil
}
