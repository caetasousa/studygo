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

// ---- edital import wizard (POST /api/editais/*) ----

// CargosResposta — step 1. The client keeps `texto` and passes it back on the
// next steps.
type CargosResposta struct {
	Texto      string       `json:"texto"`
	ArquivoURI string       `json:"arquivoUri"`
	MIME       string       `json:"mime"`
	Banca      string       `json:"banca"`
	Cargos     []CargoOpcao `json:"cargos"`
}

type CargoOpcao struct {
	Codigo       string `json:"codigo"`
	Nome         string `json:"nome"`
	Escolaridade string `json:"escolaridade"`
	Vagas        int    `json:"vagas"`
}

// EstruturaResposta — step 2, the disciplines + schedule for the chosen cargo.
type EstruturaResposta struct {
	Nome            string            `json:"nome"`
	Prova           string            `json:"prova"`
	ProvaDiscursiva bool              `json:"provaDiscursiva"`
	Gerais          []DisciplinaInput `json:"gerais"`
	Especificas     []DisciplinaInput `json:"especificas"`
	Marcos          []MarcoInput      `json:"marcos"`
	Avisos          []string          `json:"avisos"`
}

// ConteudoEditalResposta — step 3, the syllabus topics per discipline.
type ConteudoEditalResposta struct {
	Itens []ConteudoEditalDisc `json:"itens"`
}

type ConteudoEditalDisc struct {
	Nome  string   `json:"nome"`
	Temas []string `json:"temas"`
}

func cargosParaResposta(c port.EditalCargos) CargosResposta {
	out := CargosResposta{
		Texto:      c.Texto,
		ArquivoURI: c.ArquivoURI,
		MIME:       c.MIME,
		Banca:      strings.TrimSpace(c.Banca),
		Cargos:     []CargoOpcao{},
	}
	for _, cg := range c.Cargos {
		out.Cargos = append(out.Cargos, CargoOpcao{
			Codigo:       cg.Codigo,
			Nome:         cg.Nome,
			Escolaridade: cg.Escolaridade,
			Vagas:        cg.Vagas,
		})
	}

	return out
}

func estruturaParaResposta(e port.EditalEstrutura) EstruturaResposta {
	out := EstruturaResposta{
		Nome:            strings.TrimSpace(e.Nome),
		Prova:           strings.TrimSpace(e.Prova),
		ProvaDiscursiva: e.ProvaDiscursiva,
		Gerais:          discParaInput(e.Gerais, "ger"),
		Especificas:     discParaInput(e.Especificas, "esp"),
		Marcos:          []MarcoInput{},
		Avisos:          []string{},
	}

	if out.Prova == "" {
		out.Avisos = append(out.Avisos, "o edital não trazia data de prova definida — preencha antes de salvar")
	}
	if len(out.Gerais) == 0 && len(out.Especificas) == 0 {
		out.Avisos = append(out.Avisos, "não consegui identificar as disciplinas — ajuste manualmente")
	}

	distGer := distribuirBloco(out.Gerais, "ger", e.TotalGerais)
	distEsp := distribuirBloco(out.Especificas, "esp", e.TotalEspecificas)

	if distGer || distEsp {
		out.Avisos = append(out.Avisos,
			"o edital só informou o total de questões por bloco — distribuí igualmente entre as disciplinas; ajuste se souber a divisão")
	}

	for _, m := range e.Marcos {
		if strings.TrimSpace(m.Data) == "" {
			continue
		}

		out.Marcos = append(out.Marcos, MarcoInput{
			Data:      strings.TrimSpace(m.Data),
			DataFim:   strings.TrimSpace(m.DataFim),
			Titulo:    strings.TrimSpace(m.Titulo),
			ExigeAcao: m.ExigeAcao,
		})
	}

	return out
}

func discParaInput(ds []port.EditalDisciplina, bloco string) []DisciplinaInput {
	out := make([]DisciplinaInput, 0, len(ds))
	for _, d := range ds {
		if strings.TrimSpace(d.Nome) == "" {
			continue
		}

		out = append(out, DisciplinaInput{
			Nome:     strings.TrimSpace(d.Nome),
			Bloco:    bloco,
			Questoes: maxZero(d.Questoes),
			Temas:    []string{},
			Fontes:   []FonteInput{},
		})
	}

	return out
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

// distribuirBloco spreads `total` questions across every discipline of `bloco`
// that currently has 0, largest-remainder style. It only acts when the whole
// block is unset. Returns whether it changed anything.
func distribuirBloco(discs []DisciplinaInput, bloco string, total int) bool {
	if total <= 0 {
		return false
	}

	idx := []int{}
	for i, d := range discs {
		if d.Bloco != bloco {
			continue
		}
		if d.Questoes > 0 {
			return false // the edital did break this block down — leave it
		}
		idx = append(idx, i)
	}

	if len(idx) == 0 {
		return false
	}

	base := total / len(idx)
	resto := total % len(idx)

	for n, i := range idx {
		discs[i].Questoes = base
		if n < resto {
			discs[i].Questoes++
		}
	}

	return true
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

// deacentua maps the accented Latin letters common in Portuguese to their ASCII
// base so slugs stay readable ("técnico" -> "tecnico").
var deacentua = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "ê", "e", "ë", "e",
	"í", "i", "ï", "i",
	"ó", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// slugify builds a URL-safe slug from a name plus a short random suffix so it is
// unique even across concursos with the same name.
func slugify(nome string) string {
	var b strings.Builder
	prevDash := false

	for _, r := range deacentua.Replace(strings.ToLower(nome)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
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
		base = strings.Trim(base[:40], "-")
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
