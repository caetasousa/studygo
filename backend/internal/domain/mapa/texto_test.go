package mapa_test

import (
	"os"
	"reflect"
	"testing"

	"studygo/internal/domain/mapa"
)

// A exportação só vale se o texto exportado, reimportado, der o mesmo mapa:
// cada item no seu lugar, com a marca, o negrito, a imagem e o colchete que
// não é marca ("[Figura: …]").
func TestTexto_IdaEVolta(t *testing.T) {
	t.Parallel()

	exemplo, err := os.ReadFile("../../../../e2e/fixtures/mapa-exemplo.md")
	if err != nil {
		t.Fatal(err)
	}

	textos := map[string]string{
		"o exemplo do E2E": string(exemplo),
		"os casos difíceis": "# Casos: difíceis\nslug: casos\nfonte: Aula 01 · Prof. X\nmateria: Engenharia de Software\nreconhecer: UML, BPMN\n\n" +
			"- Ramo com **negrito**\n" +
			"  - [def] Definição: com dois-pontos\n" +
			"  - [Figura: um desenho descrito entre colchetes]\n" +
			"    - [pegadinha] Fundo\n" +
			"      - [cai] Mais fundo\n" +
			"  - ![Legenda da imagem](fluxo-1.png)\n" +
			"- Outro ramo\n",
	}

	for nome, texto := range textos {
		t.Run(nome, func(t *testing.T) {
			t.Parallel()

			lido, err := mapa.Ler(texto)
			if err != nil {
				t.Fatalf("Ler(original): %v", err)
			}

			relido, err := mapa.Ler(lido.Texto())
			if err != nil {
				t.Fatalf("Ler(exportado): %v\n%s", err, lido.Texto())
			}

			if !reflect.DeepEqual(lido, relido) {
				t.Fatalf("o mapa mudou na ida e volta\nantes: %+v\ndepois: %+v\ntexto:\n%s", lido, relido, lido.Texto())
			}
		})
	}
}
