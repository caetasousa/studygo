// Command recapturar refaz a fixture do edital a partir de um PDF real.
//
// Ele existe para que a fixture nunca seja editada à mão: quando o contrato do
// edital-processor muda, a resposta certa é rodar o assistente de novo contra um
// edital de verdade e regravar o arquivo inteiro.
//
// Uso, com o stack de pé e GEMINI_API_KEY definida:
//
//	go run ./internal/adapter/editalproc/cmd/recapturar -pdf edital.pdf
//
// A saída vai para internal/adapter/editalproc/testdata/edital_fcc_tcego.json.
// Confira o diff antes de commitar.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"studygo/internal/adapter/editalproc"
	"studygo/internal/port"
)

func main() {
	if err := executar(); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func executar() error {
	pdf := flag.String("pdf", "", "caminho do edital em PDF (obrigatório)")
	url := flag.String("processor", "http://localhost:8000", "URL do edital-processor")
	token := flag.String("token", "dev-processor-token", "token do processor")
	saida := flag.String(
		"saida",
		filepath.Join("internal", "adapter", "editalproc", "testdata", "edital_fcc_tcego.json"),
		"arquivo de destino",
	)
	flag.Parse()

	if *pdf == "" {
		flag.Usage()

		return fmt.Errorf("informe -pdf")
	}

	dados, err := os.ReadFile(*pdf)
	if err != nil {
		return fmt.Errorf("lendo o PDF: %w", err)
	}

	// A extração inteira leva alguns minutos: são três chamadas ao modelo por
	// cargo, e cada uma pode demorar.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	cliente := editalproc.New(*url, *token)
	up := port.EditalUpload{PDF: dados, MIME: "application/pdf"}

	fmt.Println("1/3 analisando o edital…")

	analise, err := cliente.Analisar(ctx, "recaptura", up)
	if err != nil {
		return fmt.Errorf("analisando: %w", err)
	}

	fmt.Printf("    banca %q, %d páginas, %d cargos\n",
		analise.Banca, analise.TotalPaginas, len(analise.Cargos))

	fixture := map[string]any{
		"_sobre": "Extração real de um edital, capturada por " +
			"internal/adapter/editalproc/cmd/recapturar. Não edite à mão: recapture.",
		"_origem": map[string]any{
			"banca":       analise.Banca,
			"paginas":     analise.TotalPaginas,
			"paginasOcr":  analise.PaginasOCR,
			"capturadoEm": time.Now().Format(time.DateOnly),
		},
		"analise": map[string]any{
			"banca":        analise.Banca,
			"totalPaginas": analise.TotalPaginas,
			"paginasOcr":   analise.PaginasOCR,
			"cargos":       analise.Cargos,
			"alertas":      analise.Alertas,
		},
	}

	estruturas := map[string]any{}
	conteudos := map[string]any{}

	for i, cargo := range analise.Cargos {
		fmt.Printf("2/3 estrutura do cargo %s (%d/%d)…\n", cargo.Codigo, i+1, len(analise.Cargos))

		e, err := cliente.Estrutura(ctx, "recaptura", analise.DocumentoID, cargo.Codigo)
		if err != nil {
			return fmt.Errorf("estrutura de %s: %w", cargo.Codigo, err)
		}

		estruturas[cargo.Codigo] = paraJSON(e)

		nomes := []string{}
		for _, g := range append(e.GruposGerais, e.GruposEspecificos...) {
			for _, d := range g.Disciplinas {
				nomes = append(nomes, d.Nome)
			}
		}

		fmt.Printf("3/3 conteúdo de %s (%d disciplinas)…\n", cargo.Codigo, len(nomes))

		c, err := cliente.Conteudo(
			ctx, "recaptura", analise.DocumentoID, cargo.Codigo, nomes, port.EditalUpload{},
		)
		if err != nil {
			return fmt.Errorf("conteúdo de %s: %w", cargo.Codigo, err)
		}

		conteudos[cargo.Codigo] = map[string]any{
			"itens": c.Itens, "alertas": c.Alertas,
		}
	}

	fixture["estrutura"] = estruturas
	fixture["conteudo"] = conteudos

	bruto, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return fmt.Errorf("serializando: %w", err)
	}

	if err := os.WriteFile(*saida, append(bruto, '\n'), 0o600); err != nil {
		return fmt.Errorf("gravando %s: %w", *saida, err)
	}

	fmt.Printf("\nfixture regravada: %s (%d KB)\n", *saida, len(bruto)/1024)
	fmt.Println("confira o diff antes de commitar.")

	return nil
}

// paraJSON converte a estrutura para as chaves que a fixture usa, que são as do
// contrato HTTP do processor — não as do tipo Go.
func paraJSON(e port.EditalEstrutura) map[string]any {
	return map[string]any{
		"nome":        e.NomeSugerido,
		"prova":       e.DataProva,
		"gerais":      grupos(e.GruposGerais),
		"especificas": grupos(e.GruposEspecificos),
		"discursivas": e.Discursivas,
		"duracao":     e.Duracao,
		"marcos":      e.Marcos,
		"alertas":     e.Alertas,
	}
}

func grupos(gs []port.EditalGrupo) []map[string]any {
	out := make([]map[string]any, 0, len(gs))

	for _, g := range gs {
		out = append(out, map[string]any{
			"kind":        g.Kind,
			"rotulo":      g.Rotulo,
			"total":       g.Total,
			"peso":        g.Peso,
			"pesoEscopo":  g.PesoEscopo,
			"disciplinas": g.Disciplinas,
		})
	}

	return out
}
