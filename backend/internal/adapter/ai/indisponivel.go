// Package ai holds outbound adapters for AI providers. Today: a Gemini adapter
// for reading editais, and a null adapter for when no key is configured.
package ai

import (
	"context"

	"annygo/internal/port"
)

var _ port.EditalParser = Indisponivel{}

// Indisponivel is the no-op EditalParser used when GEMINI_API_KEY is unset.
type Indisponivel struct{}

func (Indisponivel) Parse(context.Context, port.EditalEntrada) (port.EditalExtraido, error) {
	return port.EditalExtraido{}, port.ErrImportacaoIndisponivel
}

func (Indisponivel) Disponivel() bool { return false }
