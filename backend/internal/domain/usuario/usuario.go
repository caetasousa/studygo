// Package usuario guarda a conta e suas invariantes. Nenhum tipo de
// infraestrutura aparece aqui.
package usuario

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNaoEncontrado é devolvido pelos repositories quando nenhuma conta bate.
	ErrNaoEncontrado = errors.New("usuário não encontrado")
	// ErrEmailEmUso é devolvido ao cadastrar um e-mail já usado.
	ErrEmailEmUso = errors.New("email já cadastrado")
	// ErrCredenciaisInvalidas é devolvido quando o login falha.
	ErrCredenciaisInvalidas = errors.New("email ou senha inválidos")
	// ErrEmailInvalido, ErrSenhaFraca e ErrNomeObrigatorio são as falhas de
	// validação do cadastro.
	ErrEmailInvalido   = errors.New("email inválido")
	ErrSenhaFraca      = errors.New("a senha precisa de ao menos 8 caracteres")
	ErrNomeObrigatorio = errors.New("nome é obrigatório")
	// ErrTemaInvalido é devolvido quando a preferência visual não é uma das
	// três conhecidas.
	ErrTemaInvalido = errors.New("tema inválido")
)

// tamanhoMinimoSenha é o piso exigido no cadastro; o hasher não se importa.
const tamanhoMinimoSenha = 8

var regexEmail = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Tema é a preferência visual da conta. Ela é do usuário, não do plano: quem
// estuda para dois concursos não quer dois temas.
type Tema string

const (
	TemaClaro   Tema = "light"
	TemaEscuro  Tema = "dark"
	TemaSistema Tema = "system"
)

// TemaPadrao é o visual com que uma conta nova começa.
const TemaPadrao = TemaEscuro

// TemaValido converte um texto na preferência correspondente, caindo no padrão
// quando o valor não é reconhecido.
func TemaValido(s string) Tema {
	switch Tema(strings.TrimSpace(s)) {
	case TemaClaro:
		return TemaClaro
	case TemaEscuro:
		return TemaEscuro
	case TemaSistema:
		return TemaSistema
	default:
		return TemaPadrao
	}
}

// Usuario é uma conta cadastrada. SenhaHash é a string argon2id codificada.
type Usuario struct {
	ID        uuid.UUID
	Email     string
	Nome      string
	SenhaHash string
	TemaUI    Tema
	CriadoEm  time.Time
}

// NormalizarEmail deixa em minúsculas e sem espaços, para que busca e
// unicidade sejam estáveis.
func NormalizarEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidarCadastro confere as entradas cruas antes de qualquer hash acontecer.
func ValidarCadastro(email, nome, senha string) error {
	if !regexEmail.MatchString(NormalizarEmail(email)) {
		return ErrEmailInvalido
	}

	if strings.TrimSpace(nome) == "" {
		return ErrNomeObrigatorio
	}

	if len(senha) < tamanhoMinimoSenha {
		return ErrSenhaFraca
	}

	return nil
}
