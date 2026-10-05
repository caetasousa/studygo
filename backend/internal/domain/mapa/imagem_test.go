package mapa

import (
	"strings"
	"testing"
)

// O que a pessoa vê de uma imagem do mapa — escrito antes do código:
//
//	I1  o item `![legenda](arquivo.png)` não é reconhecido e aparece como texto cru
//	I2  um "![" mal fechado, sem legenda ou com nome perigoso (barra, ponto a mais)
//	    passa na importação e quebra a tela depois
//	I3  um arquivo que não é imagem, ou PNG com nome .jpg, ou grande demais é aceito
//	I4  a mesma imagem citada duas vezes conta como duas

var png1x1 = []byte("\x89PNG\r\n\x1a\n" + "\x00\x00\x00\rIHDR")

func TestLer_ItemDeImagem(t *testing.T) {
	t.Parallel()

	m, err := Ler("# BPMN\n\n- Gateways\n  - ![Gateway exclusivo, com X](gateway-exclusivo.png)\n  - Texto com ![] no meio\n- ![Outra vez](gateway-exclusivo.png)\n")
	if err != nil {
		t.Fatalf("Ler: %v", err)
	}

	legenda, nome, ok := ImagemDoItem(m.Ramos[0].Filhos[0].Texto)
	if !ok || legenda != "Gateway exclusivo, com X" || nome != "gateway-exclusivo.png" {
		t.Errorf("ImagemDoItem = %q, %q, %v", legenda, nome, ok)
	}

	if _, _, ok := ImagemDoItem(m.Ramos[0].Filhos[1].Texto); ok {
		t.Error("um texto que só menciona ![] virou imagem")
	}

	if got := m.ImagensCitadas(); len(got) != 1 || got[0] != "gateway-exclusivo.png" {
		t.Errorf("ImagensCitadas = %q, quer a imagem uma vez só", got)
	}
}

func TestLer_ImagemMalEscritaERecusada(t *testing.T) {
	t.Parallel()

	casos := map[string]string{
		"sem fechar":     "![Fluxo](fluxo.png",
		"sem legenda":    "![](fluxo.png)",
		"com barra":      "![Fluxo](../fluxo.png)",
		"sem extensão":   "![Fluxo](fluxo)",
		"maiúsculas":     "![Fluxo](Fluxo.PNG)",
		"texto depois":   "![Fluxo](fluxo.png) e mais",
		"extensão svg":   "![Fluxo](fluxo.svg)",
		"espaço no nome": "![Fluxo](meu fluxo.png)",
	}

	for nome, item := range casos {
		_, err := Ler("# BPMN\n\n- " + item + "\n")
		if err == nil {
			t.Errorf("%s: %q foi aceito", nome, item)

			continue
		}

		if !strings.Contains(err.Error(), "imagem") {
			t.Errorf("%s: a mensagem não fala da imagem: %v", nome, err)
		}
	}
}

func TestNovaImagem(t *testing.T) {
	t.Parallel()

	if img, p := NovaImagem("Gateway-Exclusivo.png", png1x1); p != "" || img.Tipo != "image/png" || img.Nome != "gateway-exclusivo.png" {
		t.Errorf("PNG válido: %+v, %q", img, p)
	}

	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0}
	if img, p := NovaImagem("foto.jpg", jpeg); p != "" || img.Tipo != "image/jpeg" {
		t.Errorf("JPEG válido: %+v, %q", img, p)
	}

	recusas := map[string]struct {
		nome  string
		dados []byte
	}{
		"texto com nome de png": {"fluxo.png", []byte("<svg onload=alert(1)>")},
		"png chamado de jpg":    {"fluxo.jpg", png1x1},
		"vazio":                 {"fluxo.png", nil},
		"grande demais":         {"fluxo.png", append(append([]byte{}, png1x1...), make([]byte, MaxBytesImagem)...)},
		"nome com barra":        {"a/fluxo.png", png1x1},
	}

	for caso, r := range recusas {
		if _, p := NovaImagem(r.nome, r.dados); p == "" {
			t.Errorf("%s: aceito", caso)
		}
	}
}
