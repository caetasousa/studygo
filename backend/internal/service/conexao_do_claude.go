package service

import (
	"context"

	"studygo/internal/domain/usuario"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// ConexaoDoClaudeService conecta o Claude da conta ao processador de mapas pela
// tela, sem terminal: o processador roda o login do Claude Code numa pasta só
// da conta, e a assinatura de uma conta nunca serve à outra.
type ConexaoDoClaudeService struct {
	processador port.ProcessadorDeMapas
}

func NewConexaoDoClaudeService(processador port.ProcessadorDeMapas) *ConexaoDoClaudeService {
	return &ConexaoDoClaudeService{processador: processador}
}

// Conectar devolve o link de autorização do Claude para a conta.
func (s *ConexaoDoClaudeService) Conectar(ctx context.Context, usuarioID uuid.UUID) (string, error) {
	return s.processador.ConectarClaude(ctx, usuarioID.String())
}

// Concluir entrega o código que a página de autorização mostrou.
func (s *ConexaoDoClaudeService) Concluir(ctx context.Context, usuarioID uuid.UUID, codigo string) (port.ConexaoDoClaude, error) {
	c, err := usuario.ConferirCodigoDoClaude(codigo)
	if err != nil {
		return port.ConexaoDoClaude{}, err
	}

	return s.processador.ConcluirConexaoDoClaude(ctx, usuarioID.String(), c)
}

func (s *ConexaoDoClaudeService) Situacao(ctx context.Context, usuarioID uuid.UUID) (port.ConexaoDoClaude, error) {
	return s.processador.ConexaoDoClaude(ctx, usuarioID.String())
}

func (s *ConexaoDoClaudeService) Desconectar(ctx context.Context, usuarioID uuid.UUID) error {
	return s.processador.DesconectarClaude(ctx, usuarioID.String())
}
