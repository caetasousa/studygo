package port

import (
	"context"
	"time"

	"studygo/internal/domain/usuario"

	"github.com/google/uuid"
)

// UsuarioRepository persiste contas e seus refresh tokens.
type UsuarioRepository interface {
	Criar(ctx context.Context, u usuario.Usuario) (usuario.Usuario, error)
	PorEmail(ctx context.Context, email string) (usuario.Usuario, error)
	PorID(ctx context.Context, id uuid.UUID) (usuario.Usuario, error)

	// DefinirTema grava a preferência visual da conta.
	DefinirTema(ctx context.Context, id uuid.UUID, tema usuario.Tema) error

	GuardarRefreshToken(ctx context.Context, usuarioID uuid.UUID, tokenHash string, expiraEm time.Time) error
	RefreshTokenValido(ctx context.Context, tokenHash string) (uuid.UUID, error)
	RevogarRefreshToken(ctx context.Context, tokenHash string) error
}

// SessaoManutencao é a faxina periódica das sessões. É uma porta SEPARADA de
// UsuarioRepository de propósito: quem autentica não precisa saber varrer, e
// quem varre é só o worker. Juntá-las obrigaria todo caso de uso de conta a
// carregar um método que não usa.
type SessaoManutencao interface {
	// LimparRefreshTokens apaga os tokens que já não servem para nada — os
	// revogados e os vencidos — e devolve quantos saíram.
	LimparRefreshTokens(ctx context.Context, agora time.Time) (int64, error)
}

// HasherDeSenha gera e confere hashes de senha (argon2id).
type HasherDeSenha interface {
	Hash(senha string) (string, error)
	Conferir(senha, hashCodificado string) (bool, error)
}

// TokenIssuer emite e lê os access tokens de vida curta (JWT).
type TokenIssuer interface {
	Emitir(usuarioID uuid.UUID) (token string, expiraEm time.Time, err error)
	Ler(token string) (uuid.UUID, error)
}
