package editalproc

import (
	"context"

	"annygo/internal/port"
)

var _ port.EditalProcessor = Indisponivel{}

// Indisponivel is the no-op EditalProcessor used when EDITAL_PROCESSOR_URL is
// unset. Every call reports the importer is unavailable; the rest of the API is
// unaffected.
type Indisponivel struct{}

func (Indisponivel) Disponivel() bool { return false }

func (Indisponivel) Analisar(context.Context, string, port.EditalUpload) (port.EditalAnalise, error) {
	return port.EditalAnalise{}, port.ErrImportacaoIndisponivel
}

func (Indisponivel) Estrutura(context.Context, string, string, string) (port.EditalEstrutura, error) {
	return port.EditalEstrutura{}, port.ErrImportacaoIndisponivel
}

func (Indisponivel) Conteudo(
	context.Context, string, string, string, []string, port.EditalUpload,
) (port.EditalConteudo, error) {
	return port.EditalConteudo{}, port.ErrImportacaoIndisponivel
}
