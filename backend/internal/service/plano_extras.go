package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"annygo/internal/domain/concurso"
	"annygo/internal/domain/plano"

	"github.com/google/uuid"
)

// Estatisticas builds the expanded history view.
func (s *PlanoService) Estatisticas(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
) (EstatisticasResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return EstatisticasResposta{}, err
	}

	codes := make([]string, 0, len(c.Disciplinas))
	for _, d := range c.Disciplinas {
		codes = append(codes, d.Codigo)
	}

	res := plano.Gerar(salvo.Config, &c)
	plano.AplicarReordenacoes(res.Dias, salvo.Reordenacoes)
	stats := plano.CalcularStats(res.Dias, codes, salvo.Registros)

	serie := []PontoSerie{}
	semanas := map[int]*ResumoSemana{}
	ordemSemanas := []int{}

	for _, d := range res.Dias {
		rs, ok := semanas[d.Semana]
		if !ok {
			rs = &ResumoSemana{Semana: d.Semana}
			semanas[d.Semana] = rs
			ordemSemanas = append(ordemSemanas, d.Semana)
		}

		rs.HorasPrevisto += salvo.Config.HorasDia

		r, temReg := salvo.Registros[plano.DayOf(d.Data)]
		if !temReg {
			continue
		}

		horas := deref(r.Horas)
		questoes := derefInt(r.Questoes)
		acertos := derefInt(r.Acertos)

		rs.HorasLancado += horas
		rs.Questoes += questoes

		serie = append(serie, PontoSerie{
			Data:      d.Data.Format(isoDate),
			N:         d.N,
			Horas:     round1(horas),
			Questoes:  questoes,
			Acertos:   acertos,
			Concluido: r.Concluido,
		})
	}

	porSemana := make([]ResumoSemana, 0, len(ordemSemanas))
	for _, sm := range ordemSemanas {
		rs := semanas[sm]
		rs.HorasPrevisto = round1(rs.HorasPrevisto)
		rs.HorasLancado = round1(rs.HorasLancado)
		porSemana = append(porSemana, *rs)
	}

	return EstatisticasResposta{
		Serie:         serie,
		PorSemana:     porSemana,
		PorDisciplina: montarBalanceamento(c, salvo.Config, res, stats),
		Streak:        calcularStreak(res.Dias, salvo.Registros, plano.DayOf(s.clock.Now())),
		HorasTotal:    round1(stats.HorasTotal),
		QuestoesTotal: stats.QuestoesTotal,
		AcertosTotal:  stats.AcertosTotal,
	}, nil
}

// calcularStreak counts consecutive concluded plan days working backward from
// the most recent day that is not in the future.
func calcularStreak(dias []plano.Dia, registros map[time.Time]plano.Registro, hoje time.Time) int {
	fim := -1

	for i, d := range dias {
		if d.Data.After(hoje) {
			break
		}

		fim = i
	}

	streak := 0

	for i := fim; i >= 0; i-- {
		r, ok := registros[plano.DayOf(dias[i].Data)]
		if !ok || !r.Concluido {
			break
		}

		streak++
	}

	return streak
}

// Caderno aggregates every note and weak day, plus the dedicated entries.
func (s *PlanoService) Caderno(ctx context.Context, userID uuid.UUID, slug string) (CadernoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return CadernoResposta{}, err
	}

	res := plano.Gerar(salvo.Config, &c)
	plano.AplicarReordenacoes(res.Dias, salvo.Reordenacoes)

	anots, err := s.planos.ListAnotacoes(ctx, salvo.ID)
	if err != nil {
		return CadernoResposta{}, err
	}

	caderno := montarCaderno(c, salvo, res.Dias, anots)

	hoje := plano.DayOf(s.clock.Now())
	caderno.VencendoHoje = revisoesDoDia(plano.VencidasAte(salvo.Revisoes, hoje), salvo.Config, hoje)

	return caderno, nil
}

// CriarAnotacao adds a notebook entry.
func (s *PlanoService) CriarAnotacao(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	in AnotacaoInput,
) (CadernoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return CadernoResposta{}, err
	}

	a, err := anotacaoFromInput(c, plano.Anotacao{}, in)
	if err != nil {
		return CadernoResposta{}, err
	}

	if _, err := s.planos.CreateAnotacao(ctx, salvo.ID, a); err != nil {
		return CadernoResposta{}, err
	}

	return s.Caderno(ctx, userID, slug)
}

// AtualizarAnotacao edits an existing notebook entry.
func (s *PlanoService) AtualizarAnotacao(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	id uuid.UUID,
	in AnotacaoInput,
) (CadernoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return CadernoResposta{}, err
	}

	a, err := anotacaoFromInput(c, plano.Anotacao{ID: id}, in)
	if err != nil {
		return CadernoResposta{}, err
	}

	if _, err := s.planos.UpdateAnotacao(ctx, salvo.ID, a); err != nil {
		return CadernoResposta{}, err
	}

	return s.Caderno(ctx, userID, slug)
}

// RemoverAnotacao deletes a notebook entry.
func (s *PlanoService) RemoverAnotacao(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	id uuid.UUID,
) (CadernoResposta, error) {
	_, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return CadernoResposta{}, err
	}

	if err := s.planos.DeleteAnotacao(ctx, salvo.ID, id); err != nil {
		return CadernoResposta{}, err
	}

	return s.Caderno(ctx, userID, slug)
}

func anotacaoFromInput(c concurso.Concurso, base plano.Anotacao, in AnotacaoInput) (plano.Anotacao, error) {
	if in.Texto == "" {
		return plano.Anotacao{}, ErrValidacao{Msg: "o texto da anotação é obrigatório"}
	}

	base.Texto = in.Texto
	base.Resolvido = in.Resolvido
	base.Tema = strings.TrimSpace(in.Tema)
	base.URL = strings.TrimSpace(in.URL)
	base.Data = nil
	base.DisciplinaID = nil

	if in.Data != nil && *in.Data != "" {
		d, ok := parseISODate(*in.Data)
		if !ok {
			return plano.Anotacao{}, ErrValidacao{Msg: "data inválida"}
		}

		base.Data = &d
	}

	if in.Disciplina != nil && *in.Disciplina != "" {
		d := c.DisciplinaByCodigo(*in.Disciplina)
		if d == nil {
			return plano.Anotacao{}, concurso.ErrNotFound
		}

		id := d.ID
		base.DisciplinaID = &id
	}

	return base, nil
}

func montarCaderno(
	c concurso.Concurso,
	salvo plano.Salvo,
	dias []plano.Dia,
	anots []plano.Anotacao,
) CadernoResposta {
	registros := salvo.Registros
	limiar := salvo.Config.Perfil.Normalizar().LimiarFraco

	codigoPorID := map[uuid.UUID]string{}
	for _, d := range c.Disciplinas {
		codigoPorID[d.ID] = d.Codigo
	}

	anotResp := make([]AnotacaoResposta, 0, len(anots))
	for _, a := range anots {
		ar := AnotacaoResposta{
			ID:        a.ID,
			Tema:      a.Tema,
			Texto:     a.Texto,
			Origem:    string(a.Origem),
			URL:       a.URL,
			Resolvido: a.Resolvido,
			CriadoEm:  a.CriadoEm,
		}

		if a.Data != nil {
			s := a.Data.Format(isoDate)
			ar.Data = &s
		}

		if a.DisciplinaID != nil {
			if cod, ok := codigoPorID[*a.DisciplinaID]; ok {
				ar.Disciplina = &cod
			}
		}

		anotResp = append(anotResp, ar)
	}

	comNota := []DiaNota{}
	fracos := []DiaFraco{}

	for _, d := range dias {
		r, ok := registros[plano.DayOf(d.Data)]
		if !ok {
			continue
		}

		if r.Nota != "" {
			discs := make([]string, 0, len(d.Itens))
			for _, it := range d.Itens {
				discs = append(discs, it.Disciplina)
			}

			comNota = append(comNota, DiaNota{
				Data:        d.Data.Format(isoDate),
				N:           d.N,
				Disciplinas: discs,
				Nota:        r.Nota,
			})
		}

		q := derefInt(r.Questoes)
		a := derefInt(r.Acertos)

		if q > 0 {
			pct := int(math.Round(float64(a) / float64(q) * 100))
			if pct < limiar {
				fracos = append(fracos, DiaFraco{
					Data:     d.Data.Format(isoDate),
					N:        d.N,
					Questoes: q,
					Acertos:  a,
					Pct:      pct,
				})
			}
		}
	}

	return CadernoResposta{
		Anotacoes:    anotResp,
		DiasComNota:  comNota,
		DiasFracos:   fracos,
		VencendoHoje: []RevisaoResposta{},
	}
}

// DossieResposta is GET /api/plano/dossie?disciplina=… — a study brief ready to
// paste into NotebookLM as a source.
type DossieResposta struct {
	Disciplina string        `json:"disciplina"`
	Markdown   string        `json:"markdown"`
	Fontes     []DossieFonte `json:"fontes"`
}

type DossieFonte struct {
	Titulo string `json:"titulo"`
	URL    string `json:"url"`
}

// Dossie builds a single-document brief for one discipline: the ementa, the
// registered laws/materials, and the user's own error log for it.
func (s *PlanoService) Dossie(
	ctx context.Context,
	userID uuid.UUID,
	slug, codigo string,
) (DossieResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return DossieResposta{}, err
	}

	d := c.DisciplinaByCodigo(codigo)
	if d == nil {
		return DossieResposta{}, concurso.ErrNotFound
	}

	res := plano.Gerar(salvo.Config, &c)
	plano.AplicarReordenacoes(res.Dias, salvo.Reordenacoes)

	anots, err := s.planos.ListAnotacoes(ctx, salvo.ID)
	if err != nil {
		return DossieResposta{}, err
	}

	caderno := montarCaderno(c, salvo, res.Dias, anots)

	var b strings.Builder
	fmt.Fprintf(&b, "# %s — %s\n\n", d.Nome, c.Nome)
	b.WriteString("Documento de estudo para colar como fonte no NotebookLM.\n\n")

	b.WriteString("## Ementa\n\n")
	if len(d.Temas) == 0 {
		b.WriteString("_Nenhum tema cadastrado para esta disciplina._\n\n")
	} else {
		for _, t := range d.Temas {
			fmt.Fprintf(&b, "- %s\n", t)
		}
		b.WriteString("\n")
	}

	fontes := make([]DossieFonte, 0, len(d.Fontes))
	b.WriteString("## Fontes\n\n")
	if len(d.Fontes) == 0 {
		b.WriteString("_Nenhuma lei/material cadastrado._\n\n")
	} else {
		for _, f := range d.Fontes {
			fontes = append(fontes, DossieFonte{Titulo: f.Titulo, URL: f.URL})
			if f.URL != "" {
				fmt.Fprintf(&b, "- [%s](%s)\n", f.Titulo, f.URL)
			} else {
				fmt.Fprintf(&b, "- %s\n", f.Titulo)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("## Meu caderno de erros\n\n")
	escreveuAlgo := false

	for _, a := range caderno.Anotacoes {
		if a.Disciplina == nil || *a.Disciplina != codigo {
			continue
		}

		marca := "-"
		if a.Resolvido {
			marca = "- [x]"
		}

		prefixo := ""
		if a.Tema != "" {
			prefixo = "**" + a.Tema + "** — "
		}

		sufixo := ""
		if a.Origem != "" && a.Origem != string(plano.OrigemManual) {
			sufixo = " _(" + a.Origem + ")_"
		}

		fmt.Fprintf(&b, "%s %s%s%s\n", marca, prefixo, a.Texto, sufixo)
		escreveuAlgo = true
	}

	for _, dn := range caderno.DiasComNota {
		if !contemString(dn.Disciplinas, codigo) {
			continue
		}

		fmt.Fprintf(&b, "- (%s) %s\n", dn.Data, dn.Nota)
		escreveuAlgo = true
	}

	if !escreveuAlgo {
		b.WriteString("_Ainda sem anotações para esta disciplina._\n")
	}

	b.WriteString("\n---\n")
	b.WriteString("No NotebookLM: crie um notebook, cole este texto e os links acima como fontes, ")
	b.WriteString("e peça um Guia de Estudos e um Áudio Overview em português.\n")

	return DossieResposta{Disciplina: d.Nome, Markdown: b.String(), Fontes: fontes}, nil
}

func contemString(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}

	return false
}

// ExportarCSV renders the whole plan plus the user's log as a semicolon-CSV,
// porting the artifact's montarCSV().
func (s *PlanoService) ExportarCSV(ctx context.Context, userID uuid.UUID, slug string) ([]byte, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return nil, err
	}

	nomes := map[string]string{}
	for _, d := range c.Disciplinas {
		nomes[d.Codigo] = d.Nome
	}

	res := plano.Gerar(salvo.Config, &c)
	plano.AplicarReordenacoes(res.Dias, salvo.Reordenacoes)

	var b strings.Builder
	b.WriteString("\uFEFF") // BOM so Excel opens the UTF-8 CSV correctly

	header := []string{
		"dia", "data", "semana", "fase",
		"bloco_1_disciplina", "bloco_1_tema", "bloco_2_disciplina", "bloco_2_tema",
		"meta_questoes", "horas", "questoes", "acertos", "concluido", "anotacao",
	}
	writeCSVRow(&b, header)

	for _, d := range res.Dias {
		r := salvo.Registros[plano.DayOf(d.Data)]

		fase := "Conteúdo"
		if d.Fase == plano.FaseReta {
			fase = "Reta final"
		}

		b1d, b1t, b2d, b2t := "", "", "", ""
		switch len(d.Itens) {
		case 0:
			b1d, b1t = rotulo(d.Tipo), d.Tema
		case 1:
			b1d, b1t = nomes[d.Itens[0].Disciplina], d.Itens[0].Tema
		default:
			b1d, b1t = nomes[d.Itens[0].Disciplina], d.Itens[0].Tema
			b2d, b2t = nomes[d.Itens[1].Disciplina], d.Itens[1].Tema
		}

		concl := "não"
		if r.Concluido {
			concl = "sim"
		}

		writeCSVRow(&b, []string{
			strconv.Itoa(d.N),
			d.Data.Format("02/01/2006"),
			strconv.Itoa(d.Semana),
			fase,
			b1d, b1t, b2d, b2t,
			strconv.Itoa(d.Meta),
			floatOrEmpty(r.Horas),
			intOrEmpty(r.Questoes),
			intOrEmpty(r.Acertos),
			concl,
			strings.ReplaceAll(r.Nota, `"`, "'"),
		})
	}

	anots, err := s.planos.ListAnotacoes(ctx, salvo.ID)
	if err != nil {
		return nil, err
	}

	escreverCadernoCSV(&b, c, anots)

	return []byte(b.String()), nil
}

// escreverCadernoCSV appends the error notebook as a second table, so an export
// carries the reasoning and not only the numbers.
func escreverCadernoCSV(b *strings.Builder, c concurso.Concurso, anots []plano.Anotacao) {
	if len(anots) == 0 {
		return
	}

	nomePorID := map[uuid.UUID]string{}
	for _, d := range c.Disciplinas {
		nomePorID[d.ID] = d.Nome
	}

	b.WriteByte('\n')
	writeCSVRow(b, []string{"caderno_data", "caderno_disciplina", "caderno_tema", "caderno_origem", "caderno_texto", "caderno_resolvido", "caderno_link"})

	for _, a := range anots {
		data := ""
		if a.Data != nil {
			data = a.Data.Format("02/01/2006")
		}

		disciplina := ""
		if a.DisciplinaID != nil {
			disciplina = nomePorID[*a.DisciplinaID]
		}

		resolvido := "não"
		if a.Resolvido {
			resolvido = "sim"
		}

		writeCSVRow(b, []string{
			data, disciplina, a.Tema, string(a.Origem),
			strings.ReplaceAll(a.Texto, `"`, "'"), resolvido, a.URL,
		})
	}
}

func writeCSVRow(b *strings.Builder, cols []string) {
	for i, c := range cols {
		if i > 0 {
			b.WriteByte(';')
		}

		b.WriteByte('"')
		b.WriteString(strings.ReplaceAll(c, `"`, `""`))
		b.WriteByte('"')
	}

	b.WriteByte('\n')
}

func rotulo(t plano.Tipo) string {
	switch t {
	case plano.TipoSimulado:
		return "SIMULADO"
	case plano.TipoDiscursiva:
		return "DISCURSIVA"
	case plano.TipoVespera:
		return "VÉSPERA"
	default:
		return "REVISÃO"
	}
}

func floatOrEmpty(p *float64) string {
	if p == nil {
		return ""
	}

	return strconv.FormatFloat(*p, 'f', -1, 64)
}

func intOrEmpty(p *int) string {
	if p == nil {
		return ""
	}

	return strconv.Itoa(*p)
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}

	return *p
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}

	return *p
}
