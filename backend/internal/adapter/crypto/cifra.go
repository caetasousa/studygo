package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// Cifra guarda segredos de quem estuda (o token do Claude) cifrados no banco:
// AES-256-GCM, com a chave derivada do segredo do JWT pelo HKDF. Assim não há
// um segredo novo para o deploy cuidar, e a chave de cifra nunca é a mesma do
// JWT. Trocar o segredo do JWT torna os tokens guardados ilegíveis — quem
// estuda cola o token de novo.
type Cifra struct {
	aead cipher.AEAD
}

// ErrCifraIlegivel: o dado não foi cifrado com esta chave (ou foi adulterado).
var ErrCifraIlegivel = errors.New("dado cifrado ilegível")

func NewCifra(segredo string) (*Cifra, error) {
	chave := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, []byte(segredo), nil, []byte("studygo: segredos da conta")), chave); err != nil {
		return nil, fmt.Errorf("derivando a chave de cifra: %w", err)
	}

	bloco, err := aes.NewCipher(chave)
	if err != nil {
		return nil, fmt.Errorf("montando a cifra: %w", err)
	}

	aead, err := cipher.NewGCM(bloco)
	if err != nil {
		return nil, fmt.Errorf("montando a cifra: %w", err)
	}

	return &Cifra{aead: aead}, nil
}

// Cifrar devolve o nonce seguido do texto cifrado.
func (c *Cifra) Cifrar(claro []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("gerando nonce: %w", err)
	}

	return c.aead.Seal(nonce, nonce, claro, nil), nil
}

func (c *Cifra) Decifrar(cifrado []byte) ([]byte, error) {
	n := c.aead.NonceSize()
	if len(cifrado) < n {
		return nil, ErrCifraIlegivel
	}

	claro, err := c.aead.Open(nil, cifrado[:n], cifrado[n:], nil)
	if err != nil {
		return nil, ErrCifraIlegivel
	}

	return claro, nil
}
