// Package tec entende a planilha de desempenho do TEC Concursos: como lê-la e
// como casar cada assunto dela com as disciplinas e temas de um concurso.
//
// É regra de domínio pura — normalização de texto, similaridade, agregação —
// sem I/O e sem conhecer HTTP ou banco.
package tec

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"studygo/internal/domain/concurso"
)

// ErrPlanilhaInvalida marca uma planilha que não dá para ler: arquivo
// corrompido, vazia, ou sem as colunas que o TEC exporta. A aplicação a traduz
// numa mensagem para o usuário.
var ErrPlanilhaInvalida = errors.New("planilha do TEC inválida")

// Linha is one row of the TEC Concursos performance spreadsheet, after the
// column names have been resolved.
type Linha struct {
	Assunto  string
	Questoes int
	Acertos  int
}

// Casamento is one imported row matched against the concurso.
type Casamento struct {
	Assunto    string
	Questoes   int
	Acertos    int
	Erros      int
	Pct        int
	Disciplina string // codigo, empty when unmatched
	Nome       string
	Tema       string
}

// Preview is what the import shows before anything is written.
type Preview struct {
	Casados      []Casamento
	SemCorrespon []Casamento
	Questoes     int
	Acertos      int
}

// cabecalhosTEC maps the spreadsheet's column headings to the fields we need.
// The export has changed wording over time, so each field accepts a few names.
var cabecalhosTEC = map[string][]string{
	"assunto":  {"assunto", "materia", "matéria", "disciplina", "topico", "tópico"},
	"questoes": {"questoes", "questões", "resolvidas", "total", "respondidas"},
	"acertos":  {"acertos", "certas", "corretas"},
}

// LerPlanilha parses the TEC performance export. Only CSV is accepted — an
// .xlsx reader would mean a new dependency, and the TEC lets you save as CSV.
func LerPlanilha(r io.Reader) ([]Linha, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("%w: não consegui ler o arquivo: %s", ErrPlanilhaInvalida, err)
	}

	texto := strings.TrimPrefix(string(buf), "\ufeff")

	leitor := csv.NewReader(strings.NewReader(texto))
	leitor.FieldsPerRecord = -1
	leitor.LazyQuotes = true
	leitor.Comma = detectarSeparador(texto)

	registros, err := leitor.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: não consegui ler o CSV: %s", ErrPlanilhaInvalida, err)
	}

	if len(registros) < 2 {
		return nil, fmt.Errorf("%w: a planilha está vazia", ErrPlanilhaInvalida)
	}

	idx, err := mapearColunas(registros[0])
	if err != nil {
		return nil, err
	}

	out := make([]Linha, 0, len(registros)-1)

	for _, rec := range registros[1:] {
		l, ok := linhaTEC(rec, idx)
		if !ok {
			continue
		}

		out = append(out, l)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("%w: não encontrei nenhuma linha com questões nesta planilha", ErrPlanilhaInvalida)
	}

	return out, nil
}

// mapearColunas finds which column holds each field, by heading.
func mapearColunas(cabecalho []string) (map[string]int, error) {
	idx := map[string]int{}

	for i, col := range cabecalho {
		chave := normalizar(col)

		for campo, apelidos := range cabecalhosTEC {
			if _, achado := idx[campo]; achado {
				continue
			}

			for _, a := range apelidos {
				if chave == normalizar(a) {
					idx[campo] = i
					break
				}
			}
		}
	}

	for _, campo := range []string{"assunto", "questoes", "acertos"} {
		if _, ok := idx[campo]; !ok {
			return nil, fmt.Errorf(
				"%w: não achei a coluna de %s na planilha — exporte as estatísticas "+
					"do TEC em CSV, com cabeçalho",
				ErrPlanilhaInvalida, campo,
			)
		}
	}

	return idx, nil
}

func linhaTEC(rec []string, idx map[string]int) (Linha, bool) {
	assunto := strings.TrimSpace(campoCSV(rec, idx["assunto"]))
	if assunto == "" {
		return Linha{}, false
	}

	questoes := inteiroCSV(campoCSV(rec, idx["questoes"]))
	if questoes <= 0 {
		return Linha{}, false
	}

	acertos := inteiroCSV(campoCSV(rec, idx["acertos"]))
	if acertos > questoes {
		acertos = questoes
	}

	return Linha{Assunto: assunto, Questoes: questoes, Acertos: max(acertos, 0)}, true
}

func campoCSV(rec []string, i int) string {
	if i < 0 || i >= len(rec) {
		return ""
	}

	return rec[i]
}

// inteiroCSV reads a count that may carry thousands separators or a percent sign.
func inteiroCSV(s string) int {
	var b strings.Builder

	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}

	n, err := strconv.Atoi(b.String())
	if err != nil {
		return 0
	}

	return n
}

// detectarSeparador reads the heading line: the Brazilian export uses ';'.
func detectarSeparador(texto string) rune {
	primeira := texto
	if i := strings.IndexAny(texto, "\r\n"); i >= 0 {
		primeira = texto[:i]
	}

	if strings.Count(primeira, ";") > strings.Count(primeira, ",") {
		return ';'
	}

	return ','
}

// Casar matches each imported row against the concurso's temas, then its
// discipline names. Matching is on normalized text — accents and case aside —
// and falls back to containment, since the TEC's wording is rarely identical to
// the edital's.
func Casar(c concurso.Concurso, linhas []Linha) Preview {
	out := Preview{Casados: []Casamento{}, SemCorrespon: []Casamento{}}

	for _, l := range linhas {
		cas := Casamento{
			Assunto:  l.Assunto,
			Questoes: l.Questoes,
			Acertos:  l.Acertos,
			Erros:    l.Questoes - l.Acertos,
			Pct:      pctInteiro(l.Questoes, l.Acertos),
		}

		if d, tema := procurarAssunto(c, l.Assunto); d != nil {
			cas.Disciplina = d.Codigo
			cas.Nome = d.Nome
			cas.Tema = tema
			out.Casados = append(out.Casados, cas)
			out.Questoes += l.Questoes
			out.Acertos += l.Acertos

			continue
		}

		out.SemCorrespon = append(out.SemCorrespon, cas)
	}

	sort.SliceStable(out.Casados, func(i, j int) bool {
		return out.Casados[i].Pct < out.Casados[j].Pct
	})

	return out
}

// procurarAssunto returns the discipline (and topic, when one matched) a TEC
// subject belongs to.
func procurarAssunto(c concurso.Concurso, assunto string) (*concurso.Disciplina, string) {
	alvo := normalizar(assunto)

	for i := range c.Disciplinas {
		d := &c.Disciplinas[i]

		for _, t := range d.Temas {
			if casaTexto(normalizar(t), alvo) {
				return d, t
			}
		}
	}

	for i := range c.Disciplinas {
		d := &c.Disciplinas[i]
		if casaTexto(normalizar(d.Nome), alvo) {
			return d, ""
		}
	}

	return nil, ""
}

// casaTexto is equality, then containment in either direction — but only for
// strings long enough that containment still means something.
func casaTexto(a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	if a == b {
		return true
	}

	if len(a) < 5 || len(b) < 5 {
		return false
	}

	return strings.Contains(a, b) || strings.Contains(b, a)
}

// semAcento maps the accented letters Portuguese actually uses onto their bare
// form. A table beats pulling golang.org/x/text in just for NFD folding.
var semAcento = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
	'é': 'e', 'ê': 'e', 'è': 'e', 'ë': 'e',
	'í': 'i', 'î': 'i', 'ì': 'i', 'ï': 'i',
	'ó': 'o', 'ô': 'o', 'õ': 'o', 'ò': 'o', 'ö': 'o',
	'ú': 'u', 'û': 'u', 'ù': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n',
}

// normalizar lowercases, strips accents and collapses whitespace, so
// "Concordância Verbal" and "concordancia verbal" are the same key.
func normalizar(s string) string {
	var b strings.Builder

	espaco := false

	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if sub, ok := semAcento[r]; ok {
			r = sub
		}

		switch {
		case unicode.IsSpace(r):
			espaco = true
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if espaco && b.Len() > 0 {
				b.WriteByte(' ')
			}

			espaco = false

			b.WriteRune(r)
		}
	}

	return b.String()
}

func pctInteiro(questoes, acertos int) int {
	if questoes <= 0 {
		return 0
	}

	return acertos * 100 / questoes
}
