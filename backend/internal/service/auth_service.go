package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"studygo/internal/domain/usuario"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// ParDeTokens é o que um cadastro, login ou refresh bem-sucedido devolve.
type ParDeTokens struct {
	AccessToken    string
	AccessExpiraEm time.Time
	RefreshToken   string
}

// AuthService cuida de cadastro, login e rotação de refresh token.
type AuthService struct {
	usuarios   port.UsuarioRepository
	hasher     port.HasherDeSenha
	tokens     port.TokenIssuer
	relogio    port.Clock
	refreshTTL time.Duration
}

func NewAuthService(
	usuarios port.UsuarioRepository,
	hasher port.HasherDeSenha,
	tokens port.TokenIssuer,
	relogio port.Clock,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		usuarios:   usuarios,
		hasher:     hasher,
		tokens:     tokens,
		relogio:    relogio,
		refreshTTL: refreshTTL,
	}
}

func (s *AuthService) Cadastrar(
	ctx context.Context,
	email, nome, senha string,
) (usuario.Usuario, ParDeTokens, error) {
	if err := usuario.ValidarCadastro(email, nome, senha); err != nil {
		return usuario.Usuario{}, ParDeTokens{}, err
	}

	hash, err := s.hasher.Hash(senha)
	if err != nil {
		return usuario.Usuario{}, ParDeTokens{}, fmt.Errorf("gerando hash da senha: %w", err)
	}

	criado, err := s.usuarios.Criar(ctx, usuario.Usuario{
		Email:     usuario.NormalizarEmail(email),
		Nome:      nome,
		SenhaHash: hash,
		TemaUI:    usuario.TemaPadrao,
	})
	if err != nil {
		return usuario.Usuario{}, ParDeTokens{}, err
	}

	par, err := s.emitirPar(ctx, criado.ID)
	if err != nil {
		return usuario.Usuario{}, ParDeTokens{}, err
	}

	return criado, par, nil
}

func (s *AuthService) Entrar(
	ctx context.Context,
	email, senha string,
) (usuario.Usuario, ParDeTokens, error) {
	achado, err := s.usuarios.PorEmail(ctx, usuario.NormalizarEmail(email))

	// Conta inexistente e senha errada devolvem a mesma coisa: distinguir as
	// duas diria a um atacante quais e-mails estão cadastrados.
	if errors.Is(err, usuario.ErrNaoEncontrado) {
		return usuario.Usuario{}, ParDeTokens{}, usuario.ErrCredenciaisInvalidas
	}

	if err != nil {
		return usuario.Usuario{}, ParDeTokens{}, err
	}

	ok, err := s.hasher.Conferir(senha, achado.SenhaHash)
	if err != nil {
		return usuario.Usuario{}, ParDeTokens{}, fmt.Errorf("conferindo senha: %w", err)
	}

	if !ok {
		return usuario.Usuario{}, ParDeTokens{}, usuario.ErrCredenciaisInvalidas
	}

	par, err := s.emitirPar(ctx, achado.ID)
	if err != nil {
		return usuario.Usuario{}, ParDeTokens{}, err
	}

	return achado, par, nil
}

// Renovar gira o token: o refresh apresentado é revogado e um par novo é
// emitido.
func (s *AuthService) Renovar(ctx context.Context, refreshToken string) (ParDeTokens, error) {
	hash := hashDoToken(refreshToken)

	usuarioID, err := s.usuarios.RefreshTokenValido(ctx, hash)
	if err != nil {
		return ParDeTokens{}, err
	}

	if err := s.usuarios.RevogarRefreshToken(ctx, hash); err != nil {
		return ParDeTokens{}, err
	}

	return s.emitirPar(ctx, usuarioID)
}

// Sair revoga um refresh token; um token desconhecido não faz nada.
func (s *AuthService) Sair(ctx context.Context, refreshToken string) error {
	return s.usuarios.RevogarRefreshToken(ctx, hashDoToken(refreshToken))
}

func (s *AuthService) PorID(ctx context.Context, id uuid.UUID) (usuario.Usuario, error) {
	return s.usuarios.PorID(ctx, id)
}

// DefinirTema grava a preferência visual da conta.
func (s *AuthService) DefinirTema(ctx context.Context, id uuid.UUID, tema string) error {
	return s.usuarios.DefinirTema(ctx, id, usuario.TemaValido(tema))
}

func (s *AuthService) emitirPar(ctx context.Context, usuarioID uuid.UUID) (ParDeTokens, error) {
	access, expiraEm, err := s.tokens.Emitir(usuarioID)
	if err != nil {
		return ParDeTokens{}, fmt.Errorf("emitindo access token: %w", err)
	}

	refresh, err := tokenAleatorio()
	if err != nil {
		return ParDeTokens{}, err
	}

	if err := s.usuarios.GuardarRefreshToken(
		ctx, usuarioID, hashDoToken(refresh), s.relogio.Now().Add(s.refreshTTL),
	); err != nil {
		return ParDeTokens{}, err
	}

	return ParDeTokens{
		AccessToken:    access,
		AccessExpiraEm: expiraEm,
		RefreshToken:   refresh,
	}, nil
}

func tokenAleatorio() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gerando token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashDoToken guarda só o hash do refresh token: um vazamento do banco não
// entrega sessões ativas.
func hashDoToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}
