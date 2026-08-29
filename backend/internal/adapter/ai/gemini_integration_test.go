//go:build integration

// Integration test that calls the real Gemini API through the whole wizard.
// Run with:
//
//	GEMINI_API_KEY=... go test -tags=integration ./internal/adapter/ai/ -run Real -v
//
// Skipped (not failed) when GEMINI_API_KEY is unset.
package ai

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"annygo/internal/port"
)

func TestGeminiAnalisador_Real(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY não definida — pulando teste de integração")
	}

	edital, err := os.ReadFile("testdata/edital_tcego_b02.txt")
	if err != nil {
		t.Fatalf("lendo fixture: %v", err)
	}

	g := NewGeminiAnalisador(key, os.Getenv("GEMINI_MODEL"))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// --- step 1: cargos ---
	t0 := time.Now()
	entrada := port.EditalEntrada{Texto: string(edital)}
	cargos, err := g.Cargos(ctx, entrada)
	if err != nil {
		t.Fatalf("Cargos: %v", err)
	}
	t.Logf("cargos (%s): banca=%q", time.Since(t0).Round(time.Millisecond), cargos.Banca)
	for _, c := range cargos.Cargos {
		t.Logf("  %s — %s (%d vagas)", c.Codigo, c.Nome, c.Vagas)
	}

	if !containsFold(cargos.Banca, "FCC") && !containsFold(cargos.Banca, "Carlos Chagas") {
		t.Errorf("banca = %q", cargos.Banca)
	}
	if len(cargos.Cargos) < 2 {
		t.Fatalf("esperava >= 2 cargos (A01 e B02), veio %d", len(cargos.Cargos))
	}

	var ti string
	for _, c := range cargos.Cargos {
		if containsFold(c.Nome, "Tecnologia") || c.Codigo == "B02" {
			ti = c.Nome
		}
	}
	if ti == "" {
		t.Fatalf("não achei o cargo de TI entre %d cargos", len(cargos.Cargos))
	}

	// --- step 2: estrutura do cargo de TI ---
	t1 := time.Now()
	est, err := g.Estrutura(ctx, entrada, ti)
	if err != nil {
		t.Fatalf("Estrutura: %v", err)
	}
	t.Logf("estrutura (%s): nome=%q prova=%q discursiva=%v totais=%d/%d",
		time.Since(t1).Round(time.Millisecond), est.Nome, est.Prova, est.ProvaDiscursiva,
		est.TotalGerais, est.TotalEspecificas)
	for _, d := range est.Gerais {
		t.Logf("  [ger] %s (%dq)", d.Nome, d.Questoes)
	}
	for _, d := range est.Especificas {
		t.Logf("  [esp] %s (%dq)", d.Nome, d.Questoes)
	}

	if est.Prova != "2027-01-17" {
		t.Errorf("prova = %q, quero 2027-01-17", est.Prova)
	}
	if est.TotalGerais != 25 {
		t.Errorf("totalGerais = %d, quero 25", est.TotalGerais)
	}
	if est.TotalEspecificas != 45 {
		t.Errorf("totalEspecificas = %d, quero 45", est.TotalEspecificas)
	}
	if len(est.Gerais) < 2 || len(est.Especificas) < 8 {
		t.Errorf("disciplinas: %d gerais / %d específicas", len(est.Gerais), len(est.Especificas))
	}

	// --- step 2b: cronograma (roda em paralelo na vida real) ---
	tc := time.Now()
	marcos, err := g.Cronograma(ctx, entrada)
	if err != nil {
		t.Fatalf("Cronograma: %v", err)
	}
	t.Logf("cronograma (%s): %d marcos", time.Since(tc).Round(time.Millisecond), len(marcos))

	if len(marcos) < 10 {
		t.Errorf("marcos = %d, esperava o cronograma inteiro", len(marcos))
	}

	var achouInscricao bool
	for _, m := range marcos {
		if m.Data == "2026-10-05" && m.DataFim == "2026-11-06" && m.ExigeAcao {
			achouInscricao = true
		}
	}
	if !achouInscricao {
		t.Error("não achei o período de inscrições (05/10 a 06/11/2026, exige ação)")
	}

	// --- step 3: conteúdo programático de algumas disciplinas ---
	alvo := []string{}
	for _, d := range est.Especificas {
		alvo = append(alvo, d.Nome)
		if len(alvo) == 3 {
			break
		}
	}

	t2 := time.Now()
	cont, err := g.Conteudo(ctx, entrada, alvo)
	if err != nil {
		t.Fatalf("Conteudo: %v", err)
	}
	t.Logf("conteúdo (%s):", time.Since(t2).Round(time.Millisecond))
	for _, c := range cont {
		t.Logf("  %s — %d temas", c.Nome, len(c.Temas))
	}

	if len(cont) != len(alvo) {
		t.Errorf("conteúdo devolveu %d disciplinas, pedi %d", len(cont), len(alvo))
	}
	var comTemas int
	for _, c := range cont {
		if len(c.Temas) > 0 {
			comTemas++
		}
	}
	if comTemas == 0 {
		t.Error("nenhuma disciplina veio com temas")
	}

	t.Logf("TOTAL: %s", time.Since(t0).Round(time.Millisecond))
}

// TestGeminiAnalisador_RealPDF runs the wizard against a real PDF file. Scanned
// editais have no text layer, so the file itself travels on every step — this is
// the path that was silently returning no disciplines.
//
//	GEMINI_API_KEY=... EDITAL_PDF=/caminho/edital.pdf \
//	  go test -tags=integration ./internal/adapter/ai/ -run RealPDF -v
func TestGeminiAnalisador_RealPDF(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	path := os.Getenv("EDITAL_PDF")
	if key == "" || path == "" {
		t.Skip("defina GEMINI_API_KEY e EDITAL_PDF")
	}

	pdf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lendo pdf: %v", err)
	}
	t.Logf("pdf: %d bytes", len(pdf))

	entrada := port.EditalEntrada{PDF: pdf, MIME: "application/pdf"}
	g := NewGeminiAnalisador(key, os.Getenv("GEMINI_MODEL"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t0 := time.Now()
	cargos, err := g.Cargos(ctx, entrada)
	if err != nil {
		t.Fatalf("Cargos: %v", err)
	}
	t.Logf("cargos (%s): %d encontrados, banca=%q",
		time.Since(t0).Round(time.Millisecond), len(cargos.Cargos), cargos.Banca)
	for _, c := range cargos.Cargos {
		t.Logf("  %s — %s (%d vagas)", c.Codigo, c.Nome, c.Vagas)
	}

	if len(cargos.Cargos) == 0 {
		t.Fatal("nenhum cargo identificado no PDF")
	}

	alvo := cargos.Cargos[0].Nome
	for _, c := range cargos.Cargos {
		if containsFold(c.Nome, "Tecnologia") {
			alvo = c.Nome
		}
	}

	t1 := time.Now()
	est, err := g.Estrutura(ctx, entrada, alvo)
	if err != nil {
		t.Fatalf("Estrutura: %v", err)
	}
	t.Logf("estrutura (%s): %q prova=%q totais=%d/%d — %d gerais, %d específicas",
		time.Since(t1).Round(time.Millisecond), est.Nome, est.Prova,
		est.TotalGerais, est.TotalEspecificas, len(est.Gerais), len(est.Especificas))
	for _, d := range est.Gerais {
		t.Logf("  [ger] %s (%dq)", d.Nome, d.Questoes)
	}
	for _, d := range est.Especificas {
		t.Logf("  [esp] %s (%dq)", d.Nome, d.Questoes)
	}

	// This is the regression: a scanned PDF used to reach this step with an empty
	// transcription and come back with zero disciplines.
	if len(est.Gerais) == 0 && len(est.Especificas) == 0 {
		t.Fatal("nenhuma disciplina extraída do PDF")
	}
	if est.Prova == "" {
		t.Error("data da prova vazia")
	}

	nomes := []string{}
	for _, d := range est.Especificas {
		nomes = append(nomes, d.Nome)
		if len(nomes) == 3 {
			break
		}
	}

	t2 := time.Now()
	cont, err := g.Conteudo(ctx, entrada, nomes)
	if err != nil {
		t.Fatalf("Conteudo: %v", err)
	}
	t.Logf("conteúdo (%s):", time.Since(t2).Round(time.Millisecond))
	for _, c := range cont {
		t.Logf("  %s — %d temas", c.Nome, len(c.Temas))
	}

	var comTemas int
	for _, c := range cont {
		if len(c.Temas) > 0 {
			comTemas++
		}
	}
	if comTemas == 0 {
		t.Error("nenhuma disciplina veio com temas")
	}

	t.Logf("TOTAL: %s", time.Since(t0).Round(time.Millisecond))
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
