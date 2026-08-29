package pdftext

import (
	"os"
	"strings"
	"testing"
)

func TestExtrair(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dados   []byte
		wantErr bool
	}{
		{name: "vazio", dados: nil, wantErr: true},
		{name: "não é pdf", dados: []byte("isso aqui é texto puro"), wantErr: true},
		{
			name:    "pdf sem camada de texto (digitalizado)",
			dados:   []byte("%PDF-1.7\n% cabeçalho válido mas sem conteúdo textual\n"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Extrair(tt.dados)
			if (err != nil) != tt.wantErr {
				t.Fatalf("erro = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestExtrair_ArquivoReal inspects a real file when EDITAL_PDF points at one.
// A scanned edital is expected to fail extraction — the caller then sends the
// PDF itself to the model.
func TestExtrair_ArquivoReal(t *testing.T) {
	p := os.Getenv("EDITAL_PDF")
	if p == "" {
		t.Skip("defina EDITAL_PDF para inspecionar um edital real")
	}

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	txt, err := Extrair(b)
	t.Logf("err=%v len=%d", err, len(txt))

	if err == nil {
		for _, k := range []string{"CONHECIMENTOS", "CRONOGRAMA", "Portuguesa"} {
			t.Logf("%-16s %d ocorrências", k, strings.Count(txt, k))
		}
	}
}
