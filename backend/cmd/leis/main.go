// Command leis confere as questões escritas sobre uma lei capturada e monta o
// pacote que a curadoria importa ("studygo.lei/1").
//
// Roda na máquina de quem estuda, sobre conteudo/leis/<slug>/:
//
//	lei.json       o texto, da captura (edital-processor)
//	questoes.json  as unidades e as questões, escritas localmente
//
// Uso (a partir de backend/):
//
//	go run ./cmd/leis validar [-atualizar] [slug...]
//	go run ./cmd/leis pacote  [slug...]
//
// A validação é a mesma da importação (lei.Pacote.Validar): o que passa aqui
// passa em produção. -atualizar só preenche o hash das unidades que ainda não
// têm; uma unidade com hash de outra redação continua barrada até alguém
// revisar as questões dela e apagar o hash de propósito.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"studygo/internal/adapter/httpapi"
	"studygo/internal/domain/lei"
)

func main() {
	if err := rodar(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rodar(args []string) error {
	if len(args) == 0 {
		return errors.New("uso: leis validar|pacote [-atualizar] [slug...]")
	}

	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	conteudo := flags.String("conteudo", "../conteudo/leis", "pasta das leis capturadas")
	atualizar := flags.Bool("atualizar", false, "preenche o hash das unidades novas")
	saida := flags.String("saida", "", "pasta dos pacotes (padrão: <conteudo>/pacotes)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}

	slugs := flags.Args()
	if len(slugs) == 0 {
		var err error
		if slugs, err = capturadas(*conteudo); err != nil {
			return err
		}
	}

	switch args[0] {
	case "validar":
		return validar(*conteudo, slugs, *atualizar)
	case "pacote":
		destino := *saida
		if destino == "" {
			destino = filepath.Join(*conteudo, "pacotes")
		}

		return empacotar(*conteudo, slugs, destino)
	default:
		return fmt.Errorf("comando desconhecido %q: use validar ou pacote", args[0])
	}
}

// capturadas são as leis com lei.json gravado.
func capturadas(conteudo string) ([]string, error) {
	arquivos, err := filepath.Glob(filepath.Join(conteudo, "*", "lei.json"))
	if err != nil {
		return nil, err
	}

	var slugs []string
	for _, a := range arquivos {
		slugs = append(slugs, filepath.Base(filepath.Dir(a)))
	}
	sort.Strings(slugs)

	return slugs, nil
}

// montar lê o texto e as questões de uma lei. Sem questoes.json, o pacote é só
// de leitura (normas B e C, por enquanto).
func montar(conteudo, slug string, atualizar bool) (lei.Pacote, bool, error) {
	pasta := filepath.Join(conteudo, slug)

	arquivo, err := os.Open(filepath.Join(pasta, "lei.json"))
	if err != nil {
		return lei.Pacote{}, false, fmt.Errorf("%s: sem lei.json — capture primeiro (make leis-capturar slug=%s)", slug, slug)
	}
	p, err := httpapi.LerPacoteLei(arquivo)
	arquivo.Close()
	if err != nil {
		return lei.Pacote{}, false, fmt.Errorf("%s/lei.json: %w", slug, err)
	}

	caminho := filepath.Join(pasta, "questoes.json")
	bruto, err := os.ReadFile(caminho)
	if errors.Is(err, os.ErrNotExist) {
		return p, false, nil
	}
	if err != nil {
		return lei.Pacote{}, false, err
	}

	us, qs, err := httpapi.LerQuestoesLei(bytes.NewReader(bruto))
	if err != nil {
		return lei.Pacote{}, false, fmt.Errorf("%s/questoes.json: %w", slug, err)
	}

	if atualizar {
		mudou := false
		for i := range us {
			if us[i].Hash == "" {
				us[i].Hash = lei.HashUnidade(p.Dispositivos, us[i].Dispositivos)
				mudou = true
			}
		}
		if mudou {
			var buf bytes.Buffer
			if err := httpapi.EscreverQuestoesLei(&buf, us, qs); err != nil {
				return lei.Pacote{}, false, err
			}
			if err := os.WriteFile(caminho, buf.Bytes(), 0o644); err != nil { //nolint:gosec // conteúdo público, versionado
				return lei.Pacote{}, false, err
			}
		}
	}

	p.Unidades, p.Questoes = us, qs

	return p, true, nil
}

func validar(conteudo string, slugs []string, atualizar bool) error {
	falhas := 0
	for _, slug := range slugs {
		p, comQuestoes, err := montar(conteudo, slug, atualizar)
		if err != nil {
			falhas++
			fmt.Println("✗", err)

			continue
		}

		if err := p.Validar(); err != nil {
			falhas++
			fmt.Printf("✗ %s:\n", slug)
			var invalido lei.ErrPacoteInvalido
			if errors.As(err, &invalido) {
				for _, problema := range invalido.Problemas {
					fmt.Println("   ", problema)
				}
			} else {
				fmt.Println("   ", err)
			}

			continue
		}

		if comQuestoes {
			fmt.Printf("✓ %s: %d unidades, %d questões\n", slug, len(p.Unidades), len(p.Questoes))
		} else {
			fmt.Printf("✓ %s: só leitura (sem questoes.json)\n", slug)
		}
	}

	if falhas > 0 {
		return fmt.Errorf("%d lei(s) com problema", falhas)
	}

	return nil
}

func empacotar(conteudo string, slugs []string, destino string) error {
	if err := validar(conteudo, slugs, false); err != nil {
		return fmt.Errorf("pacote não montado: %w", err)
	}
	if err := os.MkdirAll(destino, 0o755); err != nil { //nolint:gosec // pasta local de trabalho
		return err
	}

	for _, slug := range slugs {
		p, _, err := montar(conteudo, slug, false)
		if err != nil {
			return err
		}

		var buf bytes.Buffer
		if err := httpapi.EscreverPacoteLei(&buf, p); err != nil {
			return err
		}
		arquivo := filepath.Join(destino, slug+".json")
		if err := os.WriteFile(arquivo, buf.Bytes(), 0o644); err != nil { //nolint:gosec // pacote público
			return err
		}
		fmt.Printf("  pacote: %s (%d KiB)\n", arquivo, buf.Len()/1024)
	}

	return nil
}
