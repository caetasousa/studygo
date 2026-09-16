// Package provafiles guarda os PDFs das provas no volume durável que o
// backend, o worker e o processador compartilham.
//
// O nome de todo arquivo é "<uuid>.<extensão>" e o caminho é montado aqui, a
// partir do uuid validado: nada que venha do cliente vira caminho.
package provafiles

import (
	"os"
	"path/filepath"
	"strings"

	"studygo/internal/port"

	"github.com/google/uuid"
)

var _ port.ProvaArquivos = Store{}

type Store struct{ Root string }

func (s Store) Caminho(id, ext string) (string, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return "", err
	}

	return filepath.Join(s.Root, u.String()+"."+ext), nil
}

// Guardar grava um PDF enviado. Os PNGs dos recortes quem grava é o
// processador; o backend só os lê.
//
// Grava num temporário e renomeia: o worker nunca enxerga um PDF pela metade.
func (s Store) Guardar(id string, conteudo []byte) error {
	return s.GuardarComo(id, "pdf", conteudo)
}

// GuardarComo é Guardar com a extensão dada, só as que o volume conhece.
func (s Store) GuardarComo(id, ext string, conteudo []byte) error {
	if ext != "pdf" && ext != "png" {
		return os.ErrInvalid
	}
	destino, err := s.Caminho(id, ext)
	if err != nil {
		return err
	}
	// 0770, e não 0750: o processador roda com outro usuário, no mesmo grupo
	// (ver o setgid 2770 nos Dockerfiles).
	if err := os.MkdirAll(s.Root, 0o770); err != nil { //nolint:gosec // grupo compartilhado
		return err
	}

	f, err := os.CreateTemp(s.Root, "upload-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)

	if _, err := f.Write(conteudo); err != nil {
		_ = f.Close()
		return err
	}
	// 0660 porque o processador roda com outro usuário, no mesmo grupo.
	if err := f.Chmod(0o660); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, destino)
}

func (s Store) Existe(id, ext string) bool {
	p, err := s.Caminho(id, ext)
	if err != nil {
		return false
	}
	st, err := os.Stat(p)

	return err == nil && st.Mode().IsRegular()
}

// Remover apaga "<uuid>.<extensão>". Arquivo que já não existe não é erro: a
// limpeza pode ter sido interrompida depois de apagar.
func (s Store) Remover(nome string) error {
	ext := filepath.Ext(nome)
	if ext != ".png" && ext != ".pdf" {
		return os.ErrInvalid
	}
	p, err := s.Caminho(strings.TrimSuffix(nome, ext), strings.TrimPrefix(ext, "."))
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
