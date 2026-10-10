package service

import (
	"context"

	"studygo/internal/domain/usuario"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// TokenDoClaudeService guarda o token do Claude da conta para o processador de
// mapas: a tela o grava e vê só o fim dele; o backend o entrega inteiro ao
// processador, com cada PDF. No banco, só cifrado.
type TokenDoClaudeService struct {
	usuarios port.UsuarioRepository
	cifra    port.Cifra
}

func NewTokenDoClaudeService(usuarios port.UsuarioRepository, cifra port.Cifra) *TokenDoClaudeService {
	return &TokenDoClaudeService{usuarios: usuarios, cifra: cifra}
}

// SituacaoDoToken é o que a tela sabe do token guardado.
type SituacaoDoToken struct {
	Configurado bool
	Fim         string
}

func (s *TokenDoClaudeService) Guardar(ctx context.Context, usuarioID uuid.UUID, token string) error {
	token, err := usuario.ConferirTokenDoClaude(token)
	if err != nil {
		return err
	}

	cifrado, err := s.cifra.Cifrar([]byte(token))
	if err != nil {
		return err
	}

	return s.usuarios.GravarTokenDoClaude(ctx, usuarioID, cifrado)
}

func (s *TokenDoClaudeService) Remover(ctx context.Context, usuarioID uuid.UUID) error {
	return s.usuarios.GravarTokenDoClaude(ctx, usuarioID, nil)
}

func (s *TokenDoClaudeService) Situacao(ctx context.Context, usuarioID uuid.UUID) (SituacaoDoToken, error) {
	token, ok, err := s.Token(ctx, usuarioID)
	if err != nil || !ok {
		return SituacaoDoToken{}, err
	}

	return SituacaoDoToken{Configurado: true, Fim: usuario.FimDoToken(token)}, nil
}

// Token devolve o token inteiro, para o processador; false se não há. Um token
// que não se decifra mais (o segredo do servidor mudou) conta como ausente:
// quem estuda cola de novo.
func (s *TokenDoClaudeService) Token(ctx context.Context, usuarioID uuid.UUID) (string, bool, error) {
	cifrado, err := s.usuarios.TokenDoClaude(ctx, usuarioID)
	if err != nil || cifrado == nil {
		return "", false, err
	}

	claro, err := s.cifra.Decifrar(cifrado)
	if err != nil {
		return "", false, nil //nolint:nilerr // ilegível é o mesmo que ausente, de propósito
	}

	return string(claro), true, nil
}
