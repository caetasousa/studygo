package ai

import (
	"context"

	"annygo/internal/port"
)

var _ port.EditalAnalisador = Indisponivel{}

// Indisponivel is the no-op EditalAnalisador used when GEMINI_API_KEY is unset.
type Indisponivel struct{}

func (Indisponivel) Disponivel() bool { return false }

func (Indisponivel) Cargos(context.Context, port.EditalEntrada) (port.EditalCargos, error) {
	return port.EditalCargos{}, port.ErrImportacaoIndisponivel
}

func (Indisponivel) Estrutura(context.Context, port.EditalEntrada, string) (port.EditalEstrutura, error) {
	return port.EditalEstrutura{}, port.ErrImportacaoIndisponivel
}

func (Indisponivel) Cronograma(context.Context, port.EditalEntrada) ([]port.EditalMarco, error) {
	return nil, port.ErrImportacaoIndisponivel
}

func (Indisponivel) Conteudo(
	context.Context,
	port.EditalEntrada,
	[]string,
) ([]port.EditalConteudoDisciplina, error) {
	return nil, port.ErrImportacaoIndisponivel
}
