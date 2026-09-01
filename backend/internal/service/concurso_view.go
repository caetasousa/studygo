package service

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/port"
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
	Nome     string `json:"nome"`
	Bloco    string `json:"bloco"` // "esp" | "ger"
	Questoes int    `json:"questoes"`
	// Peso is the points a question of this discipline is worth. 0 means "use
	// the block default" (1 for ger, 2 for esp) — preserved for manual
	// registration. A positive value from the user overrides it.
	Peso int `json:"peso"`
	// CadernoURL is an optional link to this subject's external error notebook.
	CadernoURL string       `json:"cadernoUrl"`
	Temas      []string     `json:"temas"`
	Fontes     []FonteInput `json:"fontes"`
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

// EditalAlertaResposta is a review flag from the edital processor.
type EditalAlertaResposta struct {
	Codigo    string `json:"codigo"`
	Gravidade string `json:"gravidade"` // "info" | "warning" | "blocker"
	Mensagem  string `json:"mensagem"`
	Campo     string `json:"campo,omitempty"`
}

// AnaliseResposta — step 1. The client keeps `documentoId` and passes it back on
// the next steps.
type AnaliseResposta struct {
	DocumentoID  string                 `json:"documentoId"`
	Banca        string                 `json:"banca"`
	TotalPaginas int                    `json:"totalPaginas"`
	PaginasOCR   int                    `json:"paginasOcr"`
	Cargos       []CargoOpcao           `json:"cargos"`
	Alertas      []EditalAlertaResposta `json:"alertas"`
}

type CargoOpcao struct {
	Codigo        string `json:"codigo"`
	Nome          string `json:"nome"`
	Especialidade string `json:"especialidade,omitempty"`
	Escolaridade  string `json:"escolaridade,omitempty"`
	// Vagas is null when the edital did not state a number.
	Vagas *int `json:"vagas"`
}

// EstruturaResposta — step 2: the groups and disciplines for the chosen cargo.
// Question counts the edital did not break down are null; the wizard makes the
// user fill an estimate or ratear explicitly before saving.
type EstruturaResposta struct {
	Nome        string                 `json:"nome"`
	Prova       string                 `json:"prova"`
	Gerais      []GrupoResposta        `json:"gerais"`
	Especificas []GrupoResposta        `json:"especificas"`
	Discursivas []DiscursivaResposta   `json:"discursivas"`
	Duracao     *DuracaoResposta       `json:"duracao"`
	Marcos      []MarcoInput           `json:"marcos"`
	Alertas     []EditalAlertaResposta `json:"alertas"`
}

type GrupoResposta struct {
	Kind        string               `json:"kind"` // "ger" | "esp" | "outro"
	Rotulo      string               `json:"rotulo"`
	Total       *int                 `json:"total"`
	Peso        *float64             `json:"peso"`
	PesoEscopo  string               `json:"pesoEscopo,omitempty"`
	Disciplinas []DisciplinaExtraida `json:"disciplinas"`
}

type DisciplinaExtraida struct {
	Nome     string   `json:"nome"`
	Questoes *int     `json:"questoes"` // null unless the edital broke it down
	Peso     *float64 `json:"peso"`     // null unless stated per discipline
}

type DiscursivaResposta struct {
	Modalidade string `json:"modalidade"` // "redacao" | "estudo_de_caso" | "outro"
	Rotulo     string `json:"rotulo"`
	Questoes   *int   `json:"questoes"`
}

type DuracaoResposta struct {
	Minutos int    `json:"minutos"`
	Escopo  string `json:"escopo"` // "exam_set" | "single_prova" | "unknown"
}

// ConteudoEditalResposta — step 3, the syllabus topics per discipline.
type ConteudoEditalResposta struct {
	Itens   []ConteudoEditalDisc   `json:"itens"`
	Alertas []EditalAlertaResposta `json:"alertas"`
}

type ConteudoEditalDisc struct {
	Nome  string   `json:"nome"`
	Temas []string `json:"temas"`
}

func editalAlertasParaResposta(in []port.EditalAlerta) []EditalAlertaResposta {
	out := make([]EditalAlertaResposta, 0, len(in))
	for _, a := range in {
		out = append(out, EditalAlertaResposta{
			Codigo:    a.Codigo,
			Gravidade: a.Gravidade,
			Mensagem:  a.Mensagem,
			Campo:     a.Campo,
		})
	}
	return out
}

func analiseParaResposta(a port.EditalAnalise) AnaliseResposta {
	out := AnaliseResposta{
		DocumentoID:  a.DocumentoID,
		Banca:        strings.TrimSpace(a.Banca),
		TotalPaginas: a.TotalPaginas,
		PaginasOCR:   a.PaginasOCR,
		Cargos:       []CargoOpcao{},
		Alertas:      editalAlertasParaResposta(a.Alertas),
	}

	for _, cg := range a.Cargos {
		out.Cargos = append(out.Cargos, CargoOpcao{
			Codigo:        cg.Codigo,
			Nome:          cg.Nome,
			Especialidade: cg.Especialidade,
			Escolaridade:  cg.Escolaridade,
			Vagas:         cg.Vagas,
		})
	}

	return out
}

func gruposParaResposta(in []port.EditalGrupo) []GrupoResposta {
	out := make([]GrupoResposta, 0, len(in))
	for _, g := range in {
		discs := make([]DisciplinaExtraida, 0, len(g.Disciplinas))
		for _, d := range g.Disciplinas {
			discs = append(discs, DisciplinaExtraida{
				Nome:     strings.TrimSpace(d.Nome),
				Questoes: d.Questoes,
				Peso:     d.Peso,
			})
		}
		out = append(out, GrupoResposta{
			Kind:        g.Kind,
			Rotulo:      strings.TrimSpace(g.Rotulo),
			Total:       g.Total,
			Peso:        g.Peso,
			PesoEscopo:  g.PesoEscopo,
			Disciplinas: discs,
		})
	}
	return out
}

func estruturaParaResposta(e port.EditalEstrutura) EstruturaResposta {
	out := EstruturaResposta{
		Nome:        strings.TrimSpace(e.NomeSugerido),
		Prova:       strings.TrimSpace(e.DataProva),
		Gerais:      gruposParaResposta(e.GruposGerais),
		Especificas: gruposParaResposta(e.GruposEspecificos),
		Discursivas: []DiscursivaResposta{},
		Marcos:      []MarcoInput{},
		Alertas:     editalAlertasParaResposta(e.Alertas),
	}

	for _, d := range e.Discursivas {
		out.Discursivas = append(out.Discursivas, DiscursivaResposta{
			Modalidade: d.Modalidade,
			Rotulo:     strings.TrimSpace(d.Rotulo),
			Questoes:   d.Questoes,
		})
	}

	if e.Duracao != nil {
		out.Duracao = &DuracaoResposta{Minutos: e.Duracao.Minutos, Escopo: e.Duracao.Escopo}
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
			Nome:       d.Nome,
			Bloco:      string(d.Bloco),
			Questoes:   d.QuestoesPadrao,
			Peso:       d.Peso,
			CadernoURL: d.CadernoURL,
			Temas:      append([]string{}, d.Temas...),
			Fontes:     []FonteInput{},
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

		peso := concurso.Peso[bloco]
		if di.Peso > 0 {
			peso = di.Peso
		}

		d := concurso.Disciplina{
			Nome:           strings.TrimSpace(di.Nome),
			Bloco:          bloco,
			Peso:           peso,
			QuestoesPadrao: maxZero(di.Questoes),
			CadernoURL:     strings.TrimSpace(di.CadernoURL),
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
		// O marco da própria prova é o que cai na data dela — não é um campo do formulário.
		m.EProva = !c.ProvaPadrao.IsZero() && data.Equal(c.ProvaPadrao)

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
