package port

import "context"

// Lembrete is a spaced-review reminder for one user on one day.
type Lembrete struct {
	Email   string
	Nome    string
	DataISO string
	Itens   []LembreteItem
	Dica    string // optional, e.g. suggesting a NotebookLM audio overview
}

// LembreteItem is one topic due for review (D-1, D-7 or D-30).
type LembreteItem struct {
	Distancia  int // 1, 7 or 30
	Disciplina string
	Tema       string
}

// Notifier delivers reminders. The MVP adapter logs them; an email adapter can
// implement the same interface later.
type Notifier interface {
	EnviarLembrete(ctx context.Context, l Lembrete) error
}
