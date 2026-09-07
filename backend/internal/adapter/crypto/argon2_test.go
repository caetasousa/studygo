package crypto

import (
	"errors"
	"strings"
	"testing"

	"studygo/internal/platform/config"
)

// O hasher de senha não tinha teste nenhum, e é o único código do projeto cujo
// erro silencioso deixa qualquer senha entrar. O que estes testes fixam é o
// contrato da porta: hash não é reversível, não repete, e um hash estragado
// falha em vez de passar.

// paramsRapidos: o custo do argon2 é o ponto dele, mas num teste ele só faz a
// suíte demorar. Um mebibyte e uma iteração exercitam o mesmo caminho.
func paramsRapidos() config.Argon2Params {
	return config.Argon2Params{
		Memory:      1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
}

func TestArgon2Hasher_ConfereASenhaCerta(t *testing.T) {
	t.Parallel()

	h := NewArgon2Hasher(paramsRapidos())

	hash, err := h.Hash("senha-do-estudante")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	ok, err := h.Conferir("senha-do-estudante", hash)
	if err != nil {
		t.Fatalf("Conferir: %v", err)
	}

	if !ok {
		t.Error("a senha correta devia conferir")
	}
}

func TestArgon2Hasher_RecusaASenhaErrada(t *testing.T) {
	t.Parallel()

	h := NewArgon2Hasher(paramsRapidos())

	hash, err := h.Hash("senha-do-estudante")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	casos := map[string]string{
		"outra senha":        "outra-senha",
		"vazia":              "",
		"prefixo da correta": "senha-do-estudant",
		"caixa diferente":    "SENHA-DO-ESTUDANTE",
	}

	for nome, senha := range casos {
		t.Run(nome, func(t *testing.T) {
			t.Parallel()

			ok, err := h.Conferir(senha, hash)
			if err != nil {
				t.Fatalf("Conferir: %v", err)
			}

			if ok {
				t.Errorf("a senha %q não devia conferir", senha)
			}
		})
	}
}

// Duas contas com a mesma senha não podem gerar o mesmo hash: é para isso que
// existe o salt, e é o que impede reconhecer senhas repetidas num vazamento.
func TestArgon2Hasher_HashesDiferentesParaAMesmaSenha(t *testing.T) {
	t.Parallel()

	h := NewArgon2Hasher(paramsRapidos())

	primeiro, err := h.Hash("mesma-senha")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	segundo, err := h.Hash("mesma-senha")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if primeiro == segundo {
		t.Error("dois hashes da mesma senha saíram iguais — o salt não está variando")
	}

	// E os dois continuam conferindo.
	for i, hash := range []string{primeiro, segundo} {
		ok, err := h.Conferir("mesma-senha", hash)
		if err != nil || !ok {
			t.Errorf("hash %d não confere: ok=%v err=%v", i+1, ok, err)
		}
	}
}

// A senha em claro não pode aparecer no que é gravado.
func TestArgon2Hasher_NaoGuardaASenhaEmClaro(t *testing.T) {
	t.Parallel()

	h := NewArgon2Hasher(paramsRapidos())
	senha := "umaSenhaBemDistinta123"

	hash, err := h.Hash(senha)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if strings.Contains(hash, senha) {
		t.Error("a senha aparece no hash codificado")
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash = %q, quer o formato PHC do argon2id", hash)
	}
}

// Um hash ilegível precisa dar ERRO, não `false`. A diferença importa: false
// seria indistinguível de senha errada, e um bug de leitura passaria por
// credencial inválida para sempre, sem nunca aparecer no log.
func TestArgon2Hasher_HashEstragadoDaErro(t *testing.T) {
	t.Parallel()

	h := NewArgon2Hasher(paramsRapidos())

	bom, err := h.Hash("qualquer")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	partes := strings.Split(bom, "$")

	casos := map[string]string{
		"vazio":              "",
		"sem cifrão":         "nao-e-um-hash",
		"campos de menos":    "$argon2id$v=19$m=1024,t=1,p=1$c2FsdA",
		"outro algoritmo":    "$argon2i$" + strings.Join(partes[2:], "$"),
		"versão ilegível":    "$argon2id$v=abc$" + strings.Join(partes[3:], "$"),
		"versão desconheci.": "$argon2id$v=1$" + strings.Join(partes[3:], "$"),
		"parâmetros tortos":  "$argon2id$v=19$m=x,t=y,p=z$" + strings.Join(partes[4:], "$"),
		"salt não é base64":  "$argon2id$v=19$m=1024,t=1,p=1$!!!!$" + partes[5],
		"hash não é base64":  "$argon2id$v=19$m=1024,t=1,p=1$" + partes[4] + "$!!!!",
	}

	for nome, estragado := range casos {
		t.Run(nome, func(t *testing.T) {
			t.Parallel()

			ok, err := h.Conferir("qualquer", estragado)

			if ok {
				t.Error("um hash ilegível NUNCA pode conferir")
			}

			if !errors.Is(err, ErrIncompatibleHash) {
				t.Errorf("erro = %v, quer ErrIncompatibleHash", err)
			}
		})
	}
}

// Trocar os parâmetros não invalida quem já tem senha: o custo usado está
// gravado dentro do próprio hash, e é ele que a conferência respeita.
func TestArgon2Hasher_ConfereHashDeParametrosAntigos(t *testing.T) {
	t.Parallel()

	antigos := paramsRapidos()
	hash, err := NewArgon2Hasher(antigos).Hash("senha-antiga")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	novos := antigos
	novos.Memory = 4096
	novos.Iterations = 3

	ok, err := NewArgon2Hasher(novos).Conferir("senha-antiga", hash)
	if err != nil {
		t.Fatalf("Conferir com parâmetros novos: %v", err)
	}

	if !ok {
		t.Error("aumentar o custo não pode deslogar quem já tinha senha")
	}
}
