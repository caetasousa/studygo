package service

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"annygo/internal/domain/concurso"
	"annygo/internal/port"
)

// Wire contract for the concurso endpoints. The same shape is used to create,
// to prefill the edit form, and as the target of the edital import.

// ConcursoInput is the create/update body.
type ConcursoInput struct {
	Nome          string            `json:"nome"`
	Banca         string            `json:"banca"`
	Cargo         string            `json:"cargo"`
	Emoji         string            `json:"emoji"`
	Prova         string            `json:"prova"` // YYYY-MM-DD
	RetaFinalDias int               `json:"retaFinalDias"`
	Disciplinas   []DisciplinaInput `json:"disciplinas"`
	Marcos        []MarcoInput      `json:"marcos"`
	Conteudo      []ConteudoInput   `json:"conteudo"`
}

type DisciplinaInput struct {
	Nome     string       `json:"nome"`
	Bloco    string       `json:"bloco"` // "esp" | "ger"
	Questoes int          `json:"questoes"`
	Temas    []string     `json:"temas"`
	Fontes   []FonteInput `json:"fontes"`
}

type FonteInput struct {
	Titulo string `json:"titulo"`
	URL    string `json:"url"`
	Tipo   string `json:"tipo"`
}

type MarcoInput struct {
	Data      string `json:"data"`
	DataFim   string `json:"dataFim"`
	Titulo    string `json:"titulo"`
	ExigeAcao bool   `json:"exigeAcao"`
}

type ConteudoInput struct {
	Tipo  string `json:"tipo"`
	Texto string `json:"texto"`
}

// ConcursoResumo is a row in the concurso picker.
type ConcursoResumo struct {
	Slug  string `json:"slug"`
	Nome  string `json:"nome"`
	Banca string `json:"banca"`
	Cargo string `json:"cargo"`
	Emoji string `json:"emoji"`
	Prova string `json:"prova"`
}

// ConcursoDetalhe prefills the edit form.
type ConcursoDetalhe struct {
	Slug string        `json:"slug"`
	Data ConcursoInput `json:"dados"`
}

// ImportarEditalResposta is what POST /api/concursos/importar returns — the
// prefilled form plus any caveats the user should double-check.
type ImportarEditalResposta struct {
	Concurso ConcursoInput `json:"concurso"`
	Avisos   []string      `json:"avisos"`
}

func resumoDe(c concurso.Concurso) ConcursoResumo {
	return ConcursoResumo{
		Slug:  c.Slug,
		Nome:  c.Nome,
		Banca: c.Banca,
		Cargo: c.Cargo,
		Emoji: c.Emoji,
		Prova: c.ProvaPadrao.Format(isoDate),
	}
}

func detalheDe(c concurso.Concurso) ConcursoDetalhe {
	in := ConcursoInput{
		Nome:          c.Nome,
		Banca:         c.Banca,
		Cargo:         c.Cargo,
		Emoji:         c.Emoji,
		Prova:         c.ProvaPadrao.Format(isoDate),
		RetaFinalDias: c.RetaPadraoDias,
		Disciplinas:   []DisciplinaInput{},
		Marcos:        []MarcoInput{},
		Conteudo:      []ConteudoInput{},
	}

	for _, d := range c.Disciplinas {
		di := DisciplinaInput{
			Nome:     d.Nome,
			Bloco:    string(d.Bloco),
			Questoes: d.QuestoesPadrao,
			Temas:    append([]string{}, d.Temas...),
			Fontes:   []FonteInput{},
		}

		for _, f := range d.Fontes {
			di.Fontes = append(di.Fontes, FonteInput{Titulo: f.Titulo, URL: f.URL, Tipo: f.Tipo})
		}

		in.Disciplinas = append(in.Disciplinas, di)
	}

	for _, m := range c.Marcos {
		mi := MarcoInput{Data: m.DataInicio.Format(isoDate), Titulo: m.Titulo, ExigeAcao: m.ExigeAcao}
		if m.DataFim != nil {
			mi.DataFim = m.DataFim.Format(isoDate)
		}

		in.Marcos = append(in.Marcos, mi)
	}

	for _, item := range c.Conteudo {
		in.Conteudo = append(in.Conteudo, ConteudoInput{Tipo: item.Tipo, Texto: item.Texto})
	}

	return ConcursoDetalhe{Slug: c.Slug, Data: in}
}

// concursoFromInput maps the wire input to the domain aggregate, applying
// defaults. It does not validate — the caller runs c.Validar().
func concursoFromInput(in ConcursoInput) (concurso.Concurso, []string) {
	avisos := []string{}

	c := concurso.Concurso{
		Nome:           strings.TrimSpace(in.Nome),
		Banca:          strings.TrimSpace(in.Banca),
		Cargo:          strings.TrimSpace(in.Cargo),
		Emoji:          firstEmoji(in.Emoji),
		RetaPadraoDias: in.RetaFinalDias,
		Disciplinas:    []concurso.Disciplina{},
		Marcos:         []concurso.Marco{},
		Conteudo:       []concurso.ConteudoItem{},
	}

	if c.RetaPadraoDias < 7 {
		c.RetaPadraoDias = 28
	}

	if prova, ok := parseISODate(in.Prova); ok {
		c.ProvaPadrao = prova
	} else if in.Prova != "" {
		avisos = append(avisos, "não entendi a data da prova ("+in.Prova+") — confira")
	}

	for _, di := range in.Disciplinas {
		bloco := concurso.Bloco(strings.ToLower(strings.TrimSpace(di.Bloco)))
		if bloco != concurso.BlocoEspecifico && bloco != concurso.BlocoGeral {
			bloco = concurso.BlocoEspecifico
		}

		d := concurso.Disciplina{
			Nome:           strings.TrimSpace(di.Nome),
			Bloco:          bloco,
			Peso:           concurso.Peso[bloco],
			QuestoesPadrao: maxZero(di.Questoes),
			Temas:          limparLinhas(di.Temas),
			Fontes:         []concurso.Fonte{},
		}

		if d.QuestoesPadrao == 0 {
			avisos = append(avisos, `a disciplina "`+d.Nome+`" está sem número de questões — estime um valor`)
		}

		for _, fi := range di.Fontes {
			if strings.TrimSpace(fi.Titulo) == "" && strings.TrimSpace(fi.URL) == "" {
				continue
			}

			d.Fontes = append(d.Fontes, concurso.Fonte{
				Titulo: strings.TrimSpace(fi.Titulo),
				URL:    strings.TrimSpace(fi.URL),
				Tipo:   strings.TrimSpace(fi.Tipo),
			})
		}

		c.Disciplinas = append(c.Disciplinas, d)
	}

	for _, mi := range in.Marcos {
		data, ok := parseISODate(mi.Data)
		if !ok {
			continue
		}

		m := concurso.Marco{Titulo: strings.TrimSpace(mi.Titulo), ExigeAcao: mi.ExigeAcao}
		m.DataInicio = data

		if fim, ok := parseISODate(mi.DataFim); ok {
			m.DataFim = &fim
		}

		c.Marcos = append(c.Marcos, m)
	}

	for _, ci := range in.Conteudo {
		tipo := strings.TrimSpace(ci.Tipo)
		switch tipo {
		case "ficha", "rot", "h", "p":
		default:
			tipo = "p"
		}

		if strings.TrimSpace(ci.Texto) == "" {
			continue
		}

		c.Conteudo = append(c.Conteudo, concurso.ConteudoItem{Tipo: tipo, Texto: strings.TrimSpace(ci.Texto)})
	}

	return c, avisos
}

// editalParaInput maps the AI extraction to the create-form shape.
func editalParaInput(e port.EditalExtraido) (ConcursoInput, []string) {
	avisos := []string{}

	nome := strings.TrimSpace(e.Nome)
	if nome == "" {
		nome = strings.TrimSpace(strings.TrimSpace(e.Orgao + " " + e.Cargo))
	}

	in := ConcursoInput{
		Nome:        nome,
		Banca:       strings.TrimSpace(e.Banca),
		Cargo:       strings.TrimSpace(e.Cargo),
		Prova:       strings.TrimSpace(e.Prova),
		Disciplinas: []DisciplinaInput{},
		Marcos:      []MarcoInput{},
		Conteudo:    []ConteudoInput{},
	}

	if in.Prova == "" {
		avisos = append(avisos, "o edital não trazia data de prova definida — preencha antes de salvar")
	}

	if len(e.Disciplinas) == 0 {
		avisos = append(avisos, "não consegui identificar as disciplinas — cadastre manualmente")
	}

	for _, d := range e.Disciplinas {
		bloco := strings.ToLower(strings.TrimSpace(d.Bloco))
		if bloco != "esp" && bloco != "ger" {
			bloco = "esp"
		}

		di := DisciplinaInput{
			Nome:     strings.TrimSpace(d.Nome),
			Bloco:    bloco,
			Questoes: maxZero(d.Questoes),
			Temas:    limparLinhas(d.Temas),
			Fontes:   []FonteInput{},
		}

		if di.Questoes == 0 {
			avisos = append(avisos, `"`+di.Nome+`": o edital não separou o nº de questões — estime`)
		}

		in.Disciplinas = append(in.Disciplinas, di)
	}

	for _, m := range e.Marcos {
		in.Marcos = append(in.Marcos, MarcoInput{
			Data:      strings.TrimSpace(m.Data),
			DataFim:   strings.TrimSpace(m.DataFim),
			Titulo:    strings.TrimSpace(m.Titulo),
			ExigeAcao: m.ExigeAcao,
		})
	}

	return in, avisos
}

func limparLinhas(xs []string) []string {
	out := []string{}
	for _, x := range xs {
		if s := strings.TrimSpace(x); s != "" {
			out = append(out, s)
		}
	}

	return out
}

func firstEmoji(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) == 0 {
		return "📚"
	}

	return string(r[0])
}

// slugify builds a URL-safe slug from a name plus a short random suffix so it is
// unique even across concursos with the same name.
func slugify(nome string) string {
	var b strings.Builder
	prevDash := false

	for _, r := range strings.ToLower(nome) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_' || r == '/' || r == '.':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}

	base := strings.Trim(b.String(), "-")
	if base == "" {
		base = "concurso"
	}

	if len(base) > 40 {
		base = base[:40]
	}

	return base + "-" + randHex(2)
}

func randHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().Format("150405")
	}

	return hex.EncodeToString(buf)
}
