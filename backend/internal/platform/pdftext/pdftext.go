// Package pdftext extracts plain text from a PDF byte slice. It is best-effort:
// text-based PDFs come out clean, scanned/image PDFs come out empty and the
// caller should fall back to an OCR-capable path.
package pdftext

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/dslipak/pdf"
)

// MinÚtil is the length below which the extraction is treated as a failure
// (scanned PDF, protected PDF, parser choked).
const MinUtil = 400

var multiEspaco = regexp.MustCompile(`[ \t]{2,}`)

// Extrair returns the concatenated plain text of every page, or an error the
// caller can treat as "extraction failed, try another way".
func Extrair(dados []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(dados), int64(len(dados)))
	if err != nil {
		return "", fmt.Errorf("abrindo pdf: %w", err)
	}

	var b strings.Builder

	total := r.NumPage()
	for i := 1; i <= total; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}

		txt, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}

		b.WriteString(txt)
		b.WriteString("\n")
	}

	limpo := limpar(b.String())
	if len(strings.TrimSpace(limpo)) < MinUtil {
		return limpo, fmt.Errorf("texto extraído muito curto (%d chars) — pdf provavelmente digitalizado", len(limpo))
	}

	return limpo, nil
}

func limpar(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = multiEspaco.ReplaceAllString(s, " ")

	linhas := strings.Split(s, "\n")
	out := make([]string, 0, len(linhas))
	brancasSeguidas := 0

	for _, l := range linhas {
		l = strings.TrimRight(l, " \t")
		if l == "" {
			brancasSeguidas++
			if brancasSeguidas > 1 {
				continue
			}
		} else {
			brancasSeguidas = 0
		}

		out = append(out, l)
	}

	return strings.Join(out, "\n")
}
