package mapa

import "strings"

// Texto escreve o mapa de volta no formato do outline — o inverso de Ler. É o
// que a exportação devolve: o banco é a fonte do mapa, e o texto exportado se
// reimporta (ou se edita e reimporta) sem perder nada que o banco guarda.
//
// O `reconhecer:` sai só se o mapa o tiver: o banco não o guarda, porque ele
// serve à sugestão de vínculo na importação, e os vínculos ficam à parte.
func (m Mapa) Texto() string {
	var b strings.Builder

	b.WriteString("# " + m.Titulo + "\n")
	b.WriteString("slug: " + m.Slug + "\n")

	if m.Fonte != "" {
		b.WriteString("fonte: " + m.Fonte + "\n")
	}

	if m.Materia != "" {
		b.WriteString("materia: " + m.Materia + "\n")
	}

	if len(m.Reconhecer) > 0 {
		b.WriteString("reconhecer: " + strings.Join(m.Reconhecer, ", ") + "\n")
	}

	b.WriteString("\n")

	var escrever func(itens []Item, nivel int)
	escrever = func(itens []Item, nivel int) {
		for _, it := range itens {
			b.WriteString(strings.Repeat("  ", nivel) + "- ")

			if it.Marca != SemMarca {
				b.WriteString("[" + string(it.Marca) + "] ")
			}

			b.WriteString(it.Texto + "\n")
			escrever(it.Filhos, nivel+1)
		}
	}
	escrever(m.Ramos, 0)

	return b.String()
}
