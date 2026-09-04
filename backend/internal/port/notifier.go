package port

import "context"

// Lembrete é o aviso de revisão de um usuário num dia.
type Lembrete struct {
	Email   string
	Nome    string
	DataISO string
	Itens   []ItemLembrete
	Dica    string // opcional, ex.: sugerir um Áudio Overview no NotebookLM
}

// ItemLembrete é um tema a revisitar. Distancia é há quantos dias ele foi
// respondido pela última vez — a fila espaçada de intervalos fixos não existe
// mais; o que puxa a revisão hoje é o caderno de erros.
type ItemLembrete struct {
	Distancia  int
	Disciplina string
	Tema       string
}

// Notifier entrega lembretes. O adapter de desenvolvimento os escreve no log;
// um de e-mail implementa a mesma interface sem tocar na aplicação.
type Notifier interface {
	EnviarLembrete(ctx context.Context, l Lembrete) error
}
