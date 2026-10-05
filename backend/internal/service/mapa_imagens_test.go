//go:build integration

package service

import (
	"bytes"
	"errors"
	"testing"

	"studygo/internal/domain/mapa"

	"github.com/google/uuid"
)

// As imagens do mapa, no banco de verdade. Como pode falhar — escrito antes do
// código:
//
//	G1  a imagem enviada não volta igual (bytes, tipo) ou some ao reimportar o mapa
//	G2  enviar de novo o mesmo nome duplica em vez de trocar
//	G3  um envio com um arquivo ruim grava os bons pela metade
//	G4  outra conta lê ou envia imagem para um mapa que não é dela
//	G5  passa do teto de imagens por mapa
//	G6  a leitura do mapa não diz quais imagens já estão lá

var (
	pngA = append([]byte("\x89PNG\r\n\x1a\n"), []byte("primeira")...)
	pngB = append([]byte("\x89PNG\r\n\x1a\n"), []byte("segunda")...)
)

func TestMapa_ImagemEnviadaVoltaIgualESobreviveAReimportacao(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDoMapa(t)

	n, err := ce.svc.EnviarImagens(t.Context(), ce.dono, "ciclo-da-agua", []ArquivoDeImagem{
		{Nome: "ciclo.png", Dados: pngA},
	})
	if err != nil || n != 1 {
		t.Fatalf("EnviarImagens = %d, %v", n, err)
	}

	if _, err := ce.svc.Importar(t.Context(), ce.dono, mapaDoTeste, ""); err != nil {
		t.Fatalf("reimportando: %v", err)
	}

	img, err := ce.svc.Imagem(t.Context(), ce.dono, "ciclo-da-agua", "ciclo.png")
	if err != nil {
		t.Fatalf("Imagem: %v", err)
	}

	if img.Tipo != "image/png" || !bytes.Equal(img.Dados, pngA) {
		t.Errorf("voltou %q com %d bytes", img.Tipo, len(img.Dados))
	}

	lido, err := ce.svc.Ler(t.Context(), ce.dono, "ciclo-da-agua")
	if err != nil {
		t.Fatalf("Ler: %v", err)
	}

	if len(lido.Imagens) != 1 || lido.Imagens[0] != "ciclo.png" {
		t.Errorf("a leitura lista as imagens %q, quer só ciclo.png", lido.Imagens)
	}
}

func TestMapa_EnviarDeNovoTrocaENaoDuplica(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDoMapa(t)

	for _, d := range [][]byte{pngA, pngB} {
		if _, err := ce.svc.EnviarImagens(t.Context(), ce.dono, "ciclo-da-agua", []ArquivoDeImagem{{Nome: "ciclo.png", Dados: d}}); err != nil {
			t.Fatalf("EnviarImagens: %v", err)
		}
	}

	img, err := ce.svc.Imagem(t.Context(), ce.dono, "ciclo-da-agua", "ciclo.png")
	if err != nil {
		t.Fatalf("Imagem: %v", err)
	}

	if !bytes.Equal(img.Dados, pngB) {
		t.Error("enviar de novo não trocou a imagem")
	}

	lido, _ := ce.svc.Ler(t.Context(), ce.dono, "ciclo-da-agua")
	if len(lido.Imagens) != 1 {
		t.Errorf("ficaram %d imagens, quer 1", len(lido.Imagens))
	}
}

func TestMapa_EnvioComArquivoRuimNaoGravaNada(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDoMapa(t)

	_, err := ce.svc.EnviarImagens(t.Context(), ce.dono, "ciclo-da-agua", []ArquivoDeImagem{
		{Nome: "boa.png", Dados: pngA},
		{Nome: "ruim.png", Dados: []byte("<svg/>")},
	})

	var invalidas mapa.ErrImagensInvalidas
	if !errors.As(err, &invalidas) || len(invalidas.Problemas) != 1 {
		t.Fatalf("erro = %v, quer ErrImagensInvalidas com um problema", err)
	}

	if _, err := ce.svc.Imagem(t.Context(), ce.dono, "ciclo-da-agua", "boa.png"); !errors.Is(err, mapa.ErrImagemNaoEncontrada) {
		t.Errorf("a boa foi gravada mesmo com o envio recusado: %v", err)
	}
}

func TestMapa_ImagemDeOutraContaNaoAcha(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDoMapa(t)

	if _, err := ce.svc.EnviarImagens(t.Context(), ce.dono, "ciclo-da-agua", []ArquivoDeImagem{{Nome: "ciclo.png", Dados: pngA}}); err != nil {
		t.Fatalf("EnviarImagens: %v", err)
	}

	outra := uuid.New()

	if _, err := ce.svc.Imagem(t.Context(), outra, "ciclo-da-agua", "ciclo.png"); !errors.Is(err, mapa.ErrNaoEncontrado) {
		t.Errorf("ler a imagem de outra conta: %v, quer ErrNaoEncontrado", err)
	}

	if _, err := ce.svc.EnviarImagens(t.Context(), outra, "ciclo-da-agua", []ArquivoDeImagem{{Nome: "x.png", Dados: pngA}}); !errors.Is(err, mapa.ErrNaoEncontrado) {
		t.Errorf("enviar para o mapa de outra conta: %v, quer ErrNaoEncontrado", err)
	}
}

func TestMapa_TetoDeImagens(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDoMapa(t)

	var arquivos []ArquivoDeImagem
	for i := range mapa.MaxImagens {
		arquivos = append(arquivos, ArquivoDeImagem{Nome: "i" + string(rune('a'+i/26)) + string(rune('a'+i%26)) + ".png", Dados: pngA})
	}

	if _, err := ce.svc.EnviarImagens(t.Context(), ce.dono, "ciclo-da-agua", arquivos); err != nil {
		t.Fatalf("as %d primeiras: %v", mapa.MaxImagens, err)
	}

	// Trocar uma que já existe continua valendo; uma a mais, não.
	if _, err := ce.svc.EnviarImagens(t.Context(), ce.dono, "ciclo-da-agua", arquivos[:1]); err != nil {
		t.Errorf("trocar uma existente no teto: %v", err)
	}

	_, err := ce.svc.EnviarImagens(t.Context(), ce.dono, "ciclo-da-agua", []ArquivoDeImagem{{Nome: "uma-a-mais.png", Dados: pngA}})

	var invalidas mapa.ErrImagensInvalidas
	if !errors.As(err, &invalidas) {
		t.Errorf("passou do teto: %v", err)
	}
}
