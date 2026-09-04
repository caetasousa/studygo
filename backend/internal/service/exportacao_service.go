package service

import (
	"context"
	"encoding/csv"
	"strconv"
	"strings"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// ExportacaoService gera o CSV do plano: o cronograma com o que foi lançado em
// cada dia, mais o caderno de erros como segunda tabela — assim a exportação
// leva o raciocínio junto com os números.
type ExportacaoService struct {
	carregador

	repo port.CadernoRepository
}

func NewExportacaoService(deps Dependencias) *ExportacaoService {
	return &ExportacaoService{carregador: deps.carregador(), repo: deps.Caderno}
}

func (s *ExportacaoService) CSV(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) ([]byte, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return nil, err
	}

	nomes := make(map[string]string, len(c.Concurso.Disciplinas))
	for _, d := range c.Concurso.Disciplinas {
		nomes[d.Codigo] = d.Nome
	}

	res := plano.Gerar(c.Plano.Config, &c.Concurso)
	plano.AplicarNosDias(res.Dias, c.Atividades)

	var b strings.Builder

	// BOM para que o Excel abra o UTF-8 corretamente.
	b.WriteString("\uFEFF")

	w := csv.NewWriter(&b)

	cabecalho := []string{
		"dia", "data", "semana", "fase", "tipo",
		"disciplina", "tema", "meta_questoes",
		"horas", "questoes", "acertos", "concluido", "anotacao",
	}

	if err := w.Write(cabecalho); err != nil {
		return nil, err
	}

	for _, d := range res.Dias {
		dt := plano.DayOf(d.Data)

		fase := "Conteúdo"
		if d.Fase == plano.FaseReta {
			fase = "Reta final"
		}

		nota := ""
		if reg, ok := c.Dias[dt]; ok {
			nota = reg.Nota
		}

		// Uma linha por ATIVIDADE: é a unidade em que o estudo é registrado, e
		// escrevê-la assim tira o limite artificial de dois blocos por dia que a
		// versão anterior tinha.
		doDia := plano.AtividadesDoDia(c.Atividades, dt)
		if len(doDia) == 0 {
			continue
		}

		for _, a := range doDia {
			reg := c.Registros[a.ID]

			disciplina := nomes[a.Disciplina]
			if disciplina == "" {
				disciplina = string(a.Tipo)
			}

			concluido := "não"
			if reg.Concluido {
				concluido = "sim"
			}

			linha := []string{
				strconv.Itoa(d.N),
				dt.Format("02/01/2006"),
				strconv.Itoa(d.Semana),
				fase,
				string(d.Tipo),
				disciplina,
				a.Tema,
				strconv.Itoa(d.Meta),
				floatOuVazio(reg.Horas),
				intOuVazio(reg.Questoes),
				intOuVazio(reg.Acertos),
				concluido,
				nota,
			}

			if err := w.Write(linha); err != nil {
				return nil, err
			}
		}
	}

	anotacoes, err := s.repo.Anotacoes(ctx, c.Plano.ID)
	if err != nil {
		return nil, err
	}

	if err := escreverCadernoCSV(w, c.Concurso, anotacoes); err != nil {
		return nil, err
	}

	w.Flush()

	if err := w.Error(); err != nil {
		return nil, err
	}

	return []byte(b.String()), nil
}

func escreverCadernoCSV(
	w *csv.Writer,
	cur concurso.Concurso,
	anotacoes []plano.Anotacao,
) error {
	if len(anotacoes) == 0 {
		return nil
	}

	nomePorID := make(map[uuid.UUID]string, len(cur.Disciplinas))
	for _, d := range cur.Disciplinas {
		nomePorID[d.ID] = d.Nome
	}

	if err := w.Write(nil); err != nil {
		return err
	}

	if err := w.Write([]string{
		"caderno_data", "caderno_disciplina", "caderno_tema",
		"caderno_texto", "caderno_origem", "caderno_resolvido",
	}); err != nil {
		return err
	}

	for _, a := range anotacoes {
		data := ""
		if a.Data != nil {
			data = plano.DayOf(*a.Data).Format("02/01/2006")
		}

		disciplina := ""
		if a.DisciplinaID != nil {
			disciplina = nomePorID[*a.DisciplinaID]
		}

		resolvido := "não"
		if a.Resolvido {
			resolvido = "sim"
		}

		if err := w.Write([]string{
			data, disciplina, a.Tema, a.Texto, string(a.Origem), resolvido,
		}); err != nil {
			return err
		}
	}

	return nil
}

func floatOuVazio(p *float64) string {
	if p == nil {
		return ""
	}

	return strconv.FormatFloat(*p, 'f', -1, 64)
}

func intOuVazio(p *int) string {
	if p == nil {
		return ""
	}

	return strconv.Itoa(*p)
}
