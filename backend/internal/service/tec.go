package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"annygo/internal/domain/concurso"
	"annygo/internal/domain/plano"

	"github.com/google/uuid"
)

// LinhaTEC is one row of the TEC Concursos performance spreadsheet, after the
// column names have been resolved.
type LinhaTEC struct {
	Assunto  string
	Questoes int
	Acertos  int
}

// CasamentoTEC is one imported row matched against the concurso.
type CasamentoTEC struct {
	Assunto    string `json:"assunto"`
	Questoes   int    `json:"questoes"`
	Acertos    int    `json:"acertos"`
	Erros      int    `json:"erros"`
	Pct        int    `json:"pct"`
	Disciplina string `json:"disciplina"` // codigo, empty when unmatched
	Nome       string `json:"nome"`
	Tema       string `json:"tema"`
}

// PreviewTEC is what the import shows before anything is written.
type PreviewTEC struct {
	Casados      []CasamentoTEC `json:"casados"`
	SemCorrespon []CasamentoTEC `json:"semCorrespondencia"`
	Questoes     int            `json:"questoes"`
	Acertos      int            `json:"acertos"`
}

// cabecalhosTEC maps the spreadsheet's column headings to the fields we need.
// The export has changed wording over time, so each field accepts a few names.
var cabecalhosTEC = map[string][]string{
	"assunto":  {"assunto", "materia", "matéria", "disciplina", "topico", "tópico"},
	"questoes": {"questoes", "questões", "resolvidas", "total", "respondidas"},
	"acertos":  {"acertos", "certas", "corretas"},
}

// LerPlanilhaTEC parses the TEC performance export. Only CSV is accepted — an
// .xlsx reader would mean a new dependency, and the TEC lets you save as CSV.
func LerPlanilhaTEC(r io.Reader) ([]LinhaTEC, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, ErrValidacao{Msg: "não consegui ler o arquivo: " + err.Error()}
	}

	texto := strings.TrimPrefix(string(buf), "\ufeff")

	leitor := csv.NewReader(strings.NewReader(texto))
	leitor.FieldsPerRecord = -1
	leitor.LazyQuotes = true
	leitor.Comma = detectarSeparador(texto)

	registros, err := leitor.ReadAll()
	if err != nil {
		return nil, ErrValidacao{Msg: "não consegui ler o CSV: " + err.Error()}
	}

	if len(registros) < 2 {
		return nil, ErrValidacao{Msg: "a planilha está vazia"}
	}

	idx, err := mapearColunas(registros[0])
	if err != nil {
		return nil, err
	}

	out := make([]LinhaTEC, 0, len(registros)-1)

	for _, rec := range registros[1:] {
		l, ok := linhaTEC(rec, idx)
		if !ok {
			continue
		}

		out = append(out, l)
	}

	if len(out) == 0 {
		return nil, ErrValidacao{Msg: "não encontrei nenhuma linha com questões nesta planilha"}
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
			return nil, ErrValidacao{Msg: fmt.Sprintf(
				"não achei a coluna de %s na planilha — exporte as estatísticas do TEC em CSV, com cabeçalho",
				campo,
			)}
		}
	}

	return idx, nil
}

func linhaTEC(rec []string, idx map[string]int) (LinhaTEC, bool) {
	assunto := strings.TrimSpace(campoCSV(rec, idx["assunto"]))
	if assunto == "" {
		return LinhaTEC{}, false
	}

	questoes := inteiroCSV(campoCSV(rec, idx["questoes"]))
	if questoes <= 0 {
		return LinhaTEC{}, false
	}

	acertos := inteiroCSV(campoCSV(rec, idx["acertos"]))
	if acertos > questoes {
		acertos = questoes
	}

	return LinhaTEC{Assunto: assunto, Questoes: questoes, Acertos: maxZero(acertos)}, true
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

// CasarTEC matches each imported row against the concurso's temas, then its
// discipline names. Matching is on normalized text — accents and case aside —
// and falls back to containment, since the TEC's wording is rarely identical to
// the edital's.
func CasarTEC(c concurso.Concurso, linhas []LinhaTEC) PreviewTEC {
	out := PreviewTEC{Casados: []CasamentoTEC{}, SemCorrespon: []CasamentoTEC{}}

	for _, l := range linhas {
		cas := CasamentoTEC{
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

// PreviewImportacaoTEC parses the spreadsheet and shows what would be written,
// without touching anything. Two steps, like the edital wizard: the user sees
// what matched before confirming.
func (s *PlanoService) PreviewImportacaoTEC(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	planilha io.Reader,
) (PreviewTEC, error) {
	c, _, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PreviewTEC{}, err
	}

	linhas, err := LerPlanilhaTEC(planilha)
	if err != nil {
		return PreviewTEC{}, err
	}

	return CasarTEC(c, linhas), nil
}

// ImportarTEC applies a parsed spreadsheet to the given day: the matched rows
// become that day's per-discipline record, every weak subject opens a notebook
// entry, and any queued review of a matched topic is settled with the numbers
// the user actually scored on the TEC.
func (s *PlanoService) ImportarTEC(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	in ImportacaoTECInput,
) (PreviewTEC, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PreviewTEC{}, err
	}

	linhas, err := LerPlanilhaTEC(strings.NewReader(in.CSV))
	if err != nil {
		return PreviewTEC{}, err
	}

	data := plano.DayOf(s.clock.Now())
	if in.Data != "" {
		d, ok := parseISODate(in.Data)
		if !ok {
			return PreviewTEC{}, ErrValidacao{Msg: "data inválida"}
		}

		data = d
	}

	// Um registro fora dos dias do plano não aparece em lugar nenhum e não entra
	// nas estatísticas — melhor recusar do que gravar uma linha órfã.
	if err := s.exigirDiaDoPlano(salvo, c, data); err != nil {
		return PreviewTEC{}, err
	}

	prev := CasarTEC(c, linhas)
	if len(prev.Casados) == 0 {
		return prev, ErrValidacao{Msg: "nenhum assunto da planilha casou com as disciplinas deste concurso"}
	}

	if err := s.gravarRegistroTEC(ctx, salvo, data, prev); err != nil {
		return PreviewTEC{}, err
	}

	limiar := salvo.Config.Normalizar().LimiarFraco

	for _, cas := range prev.Casados {
		if cas.Pct >= limiar {
			continue
		}

		if err := s.anotarErroTEC(ctx, c, salvo, data, cas); err != nil {
			return PreviewTEC{}, err
		}
	}

	if err := s.liquidarRevisoesTEC(ctx, salvo, data, prev); err != nil {
		return PreviewTEC{}, err
	}

	return prev, nil
}

// gravarRegistroTEC folds the matched rows into one per-discipline day record.
func (s *PlanoService) gravarRegistroTEC(
	ctx context.Context,
	salvo plano.Salvo,
	data time.Time,
	prev PreviewTEC,
) error {
	porDisc := map[string]*plano.RegistroBloco{}
	ordem := []string{}

	for _, cas := range prev.Casados {
		b, ok := porDisc[cas.Disciplina]
		if !ok {
			b = &plano.RegistroBloco{Disciplina: cas.Disciplina, Questoes: new(int), Acertos: new(int)}
			porDisc[cas.Disciplina] = b
			ordem = append(ordem, cas.Disciplina)
		}

		*b.Questoes += cas.Questoes
		*b.Acertos += cas.Acertos
	}

	reg := salvo.Registros[data]
	reg.Data = data
	reg.Blocos = make([]plano.RegistroBloco, 0, len(ordem))

	for _, codigo := range ordem {
		reg.Blocos = append(reg.Blocos, *porDisc[codigo])
	}

	reg.Horas, reg.Questoes, reg.Acertos = reg.Totais()
	reg.Concluido = true

	return s.planos.UpsertRegistro(ctx, salvo.ID, reg)
}

func (s *PlanoService) anotarErroTEC(
	ctx context.Context,
	c concurso.Concurso,
	salvo plano.Salvo,
	data time.Time,
	cas CasamentoTEC,
) error {
	a := plano.Anotacao{
		Data:   &data,
		Tema:   cas.Tema,
		Origem: plano.OrigemTEC,
		Texto: fmt.Sprintf(
			"%d%% em %q no TEC (%d de %d, %d erros) — anote por que errou, não só a resposta.",
			cas.Pct, cas.Assunto, cas.Acertos, cas.Questoes, cas.Erros,
		),
	}

	if d := c.DisciplinaByCodigo(cas.Disciplina); d != nil {
		id := d.ID
		a.DisciplinaID = &id
		a.URL = urlQuestoes(*d, cas.Tema)
	}

	_, err := s.planos.CreateAnotacao(ctx, salvo.ID, a)

	return err
}

// liquidarRevisoesTEC settles any queued review whose topic the spreadsheet
// covers — solving it on the TEC is solving the review.
func (s *PlanoService) liquidarRevisoesTEC(
	ctx context.Context,
	salvo plano.Salvo,
	data time.Time,
	prev PreviewTEC,
) error {
	for _, rev := range plano.VencidasAte(salvo.Revisoes, data) {
		cas, ok := acharCasamento(prev.Casados, rev)
		if !ok {
			continue
		}

		feita := rev
		feita.FeitaEm = &data
		feita.Questoes = &cas.Questoes
		feita.Acertos = &cas.Acertos

		var proxima *plano.Revisao
		if p, segue := rev.Resultado(salvo.Config, data, cas.Questoes, cas.Acertos); segue {
			proxima = &p
		}

		if err := s.planos.ConcluirRevisao(ctx, salvo.ID, feita, proxima); err != nil {
			return err
		}
	}

	return nil
}

func acharCasamento(casados []CasamentoTEC, rev plano.Revisao) (CasamentoTEC, bool) {
	alvo := normalizar(rev.Tema)

	for _, cas := range casados {
		if cas.Disciplina != rev.Disciplina {
			continue
		}

		if cas.Tema != "" && casaTexto(normalizar(cas.Tema), alvo) {
			return cas, true
		}
	}

	return CasamentoTEC{}, false
}

// urlQuestoes builds the discipline's question-bank link, substituting {tema}
// when the source URL carries the placeholder.
func urlQuestoes(d concurso.Disciplina, tema string) string {
	for _, f := range d.Fontes {
		if f.Tipo != "questoes" || f.URL == "" {
			continue
		}

		return strings.ReplaceAll(f.URL, "{tema}", url.QueryEscape(tema))
	}

	return ""
}

// exigirDiaDoPlano rejects a date the plan does not cover, naming the range so
// the user can pick a day that exists.
func (s *PlanoService) exigirDiaDoPlano(salvo plano.Salvo, c concurso.Concurso, data time.Time) error {
	res := plano.Gerar(salvo.Config, &c)
	if len(res.Dias) == 0 {
		return ErrValidacao{Msg: "este plano ainda não tem dias de estudo"}
	}

	for _, d := range res.Dias {
		if plano.DayOf(d.Data).Equal(data) {
			return nil
		}
	}

	primeiro := res.Dias[0].Data.Format(isoDate)
	ultimo := res.Dias[len(res.Dias)-1].Data.Format(isoDate)

	return ErrValidacao{Msg: fmt.Sprintf(
		"%s não é um dia de estudo do plano — escolha uma data entre %s e %s",
		data.Format(isoDate), primeiro, ultimo,
	)}
}
