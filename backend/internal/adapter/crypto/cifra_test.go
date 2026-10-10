package crypto_test

import (
	"bytes"
	"errors"
	"testing"

	"studygo/internal/adapter/crypto"
)

// O token do Claude volta igual com a mesma chave, e não volta com dado
// adulterado nem com o segredo do servidor trocado.
func TestCifra_IdaEVolta(t *testing.T) {
	t.Parallel()

	c, err := crypto.NewCifra("segredo-do-servidor-de-teste")
	if err != nil {
		t.Fatal(err)
	}

	cifrado, err := c.Cifrar([]byte("sk-ant-oat01-abc"))
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(cifrado, []byte("sk-ant")) {
		t.Fatal("o cifrado contém o texto claro")
	}

	if claro, err := c.Decifrar(cifrado); err != nil || string(claro) != "sk-ant-oat01-abc" {
		t.Fatalf("decifrar = %q, %v", claro, err)
	}

	adulterado := bytes.Clone(cifrado)
	adulterado[len(adulterado)-1] ^= 1
	if _, err := c.Decifrar(adulterado); !errors.Is(err, crypto.ErrCifraIlegivel) {
		t.Fatalf("adulterado: %v", err)
	}

	outra, _ := crypto.NewCifra("outro-segredo")
	if _, err := outra.Decifrar(cifrado); !errors.Is(err, crypto.ErrCifraIlegivel) {
		t.Fatalf("outra chave: %v", err)
	}
}
