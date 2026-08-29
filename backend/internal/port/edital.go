package port

import (
	"context"
	"errors"
)

// ErrImportacaoIndisponivel is returned by the null parser when no AI provider
// is configured.
var ErrImportacaoIndisponivel = errors.New("importação por IA indisponível")

// EditalEntrada is the raw edital: either pasted text or an uploaded PDF.
type EditalEntrada struct {
	Texto string
	PDF   []byte
	MIME  string
}

// EditalExtraido is the structured result of reading an edital. It is a plain
// value — the service maps it to the concurso wire input.
type EditalExtraido struct {
	Nome            string
	Banca           string
	Cargo           string
	Orgao           string
	Prova           string // YYYY-MM-DD
	ProvaDiscursiva bool
	Disciplinas     []EditalDisciplina
	Marcos          []EditalMarco
}

type EditalDisciplina struct {
	Nome     string
	Bloco    string // "esp" | "ger"
	Questoes int
	Temas    []string
}

type EditalMarco struct {
	Data      string // YYYY-MM-DD
	DataFim   string // YYYY-MM-DD, optional
	Titulo    string
	ExigeAcao bool
}

// EditalParser turns a raw edital into structured data.
type EditalParser interface {
	Parse(ctx context.Context, in EditalEntrada) (EditalExtraido, error)
	// Disponivel reports whether a real provider is wired (false for the null
	// adapter) so the API can advertise the feature.
	Disponivel() bool
}
