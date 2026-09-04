package service

import (
	"context"
	"math"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// A montagem do plano para a tela: o cronograma gravado, os registros e os
// números derivados, reunidos num único resultado.
//
// A leitura NÃO grava nada. Antes, montar caía num caminho que materializava
// atividades no meio de um GET; agora o cronograma já existe desde a criação do
// plano, então aqui só se lê.

func (c carregador) montar(ctx context.Context, cx contexto) (PlanoMontado, error) {
	agora := plano.DayOf(c.relogio.Now())

	cfg := cx.Plano.Config.Normalizar()
	cx.Plano.Config = cfg

	res := plano.Gerar(cfg, &cx.Concurso)

	// Os dias vêm do CRONOGRAMA GRAVADO, não da proposta do motor: o que o
	// estudante moveu é o que ele deve ver.
	plano.AplicarNosDias(res.Dias, cx.Atividades)

	codigos := make([]string, 0, len(cx.Concurso.Disciplinas))
	nomes := make(map[string]string, len(cx.Concurso.Disciplinas))

	for _, d := range cx.Concurso.Disciplinas {
		codigos = append(codigos, d.Codigo)
		nomes[d.Codigo] = d.Nome
	}

	stats := plano.CalcularStats(res.Dias, codigos, cx.Atividades, cx.Registros)
	balanceamento := montarBalanceamento(cx.Concurso, cfg, res, stats)

	anotacoes, err := c.anotacoesDoPlano(ctx, cx)
	if err != nil {
		return PlanoMontado{}, err
	}

	ctxBlocos := plano.BlocoCtx{
		Cfg:      cfg,
		Nomes:    nomes,
		Simulado: res.Simulado,
		Cadernos: plano.Caderno(resultadosDoPlano(res.Dias, cx)),
		Revisao:  plano.FilaRevisao(res.Dias, temasPorRevisao(cfg), foiEstudado(cx)),
	}

	dias := make([]DiaDoPlano, 0, len(res.Dias))

	var hojeIndex *int

	for i, d := range res.Dias {
		dt := plano.DayOf(d.Data)

		dd := DiaDoPlano{
			N:         d.N,
			Data:      dt.Format(formatoISO),
			Semana:    d.Semana,
			Fase:      string(d.Fase),
			Tipo:      string(d.Tipo),
			Tema:      d.Tema,
			Meta:      d.Meta,
			Concluido: plano.DiaConcluido(cx.Atividades, cx.Registros, dt),
			Itens:     make([]AtividadeDoDia, 0, len(d.Itens)),
		}

		dd.Horas, dd.Questoes, dd.Acertos = plano.TotaisDoDia(cx.Atividades, cx.Registros, dt)

		for _, a := range plano.AtividadesDoDia(cx.Atividades, dt) {
			if a.Tipo.DeDiaInteiro() && len(d.Itens) > 0 {
				continue
			}

			item := AtividadeDoDia{
				ID:         a.ID,
				Disciplina: a.Disciplina,
				Tema:       a.Tema,
				Passada:    a.Passada,
				Movida:     a.Movida,
			}

			if reg, ok := cx.Registros[a.ID]; ok {
				item.Horas = reg.Horas
				item.Questoes = reg.Questoes
				item.Acertos = reg.Acertos
				item.Erros = errosDe(reg.Questoes, reg.Acertos)
				item.Nota = reg.Nota
				item.Concluido = reg.Concluido
			}

			dd.Itens = append(dd.Itens, item)
		}

		for _, b := range plano.Blocos(d, ctxBlocos) {
			dd.Blocos = append(dd.Blocos, BlocoDoDia{
				Minutos: b.Minutos,
				Titulo:  b.Titulo,
				Detalhe: b.Detalhe,
			})

			if b.Disciplina != "" {
				dd.Revisao = montarRevisao(b.Disciplina, dt, cx.Dias, anotacoes)
			}
		}

		if reg, ok := cx.Dias[dt]; ok {
			dd.Nota = reg.Nota
		}

		if hojeIndex == nil && !dt.Before(agora) {
			idx := i
			hojeIndex = &idx
		}

		dias = append(dias, dd)
	}

	return PlanoMontado{
		Concurso:              montarConcurso(cx.Concurso),
		Config:                montarConfig(cfg, cx.TemaUI),
		Dias:                  dias,
		Marcos:                montarMarcos(cx.Concurso, cx.Plano.Marcos),
		Balanceamento:         balanceamento,
		Props:                 montarProps(cfg, res.Dias, stats, agora),
		Alertas:               montarAlertas(cx.Concurso, cx.Plano.Marcos, balanceamento, agora),
		HojeIndex:             hojeIndex,
		TemMovimentacaoManual: TemMovimentacaoManual(cx.Atividades),
		GeradoEm:              c.relogio.Now(),
	}, nil
}

// anotacoesDoPlano carrega o caderno uma vez, e não por dia: a lista é pequena,
// e reconsultá-la para cada dia seria uma ida ao banco por dia do plano.
func (c carregador) anotacoesDoPlano(
	ctx context.Context,
	cx contexto,
) ([]plano.Anotacao, error) {
	if c.caderno == nil {
		return nil, nil
	}

	return c.caderno.Anotacoes(ctx, cx.Plano.ID)
}

func montarConcurso(c concurso.Concurso) ConcursoDoPlano {
	discs := make([]DisciplinaDoPlano, 0, len(c.Disciplinas))

	for i, d := range c.Disciplinas {
		fontes := make([]FonteDoPlano, 0, len(d.Fontes))
		for _, f := range d.Fontes {
			fontes = append(fontes, FonteDoPlano{Titulo: f.Titulo, URL: f.URL, Tipo: f.Tipo})
		}

		temas := d.Temas
		if temas == nil {
			temas = []string{}
		}

		discs = append(discs, DisciplinaDoPlano{
			Codigo:     d.Codigo,
			Nome:       d.Nome,
			Bloco:      string(d.Bloco),
			Peso:       d.Peso,
			Cor:        i % concurso.TotalCoresDisciplinas,
			CadernoURL: d.CadernoURL,
			Temas:      temas,
			Fontes:     fontes,
		})
	}

	conteudo := make([]ItemDeConteudo, 0, len(c.Conteudo))
	for _, item := range c.Conteudo {
		conteudo = append(conteudo, ItemDeConteudo{Tipo: item.Tipo, Texto: item.Texto})
	}

	return ConcursoDoPlano{
		Slug:        c.Slug,
		Nome:        c.Nome,
		Banca:       c.Banca,
		Cargo:       c.Cargo,
		Emoji:       c.Emoji,
		Resumo:      c.Resumo,
		Disciplinas: discs,
		Conteudo:    conteudo,
	}
}

func montarConfig(cfg plano.Config, tema string) ConfigDoPlano {
	modos := make(map[string]string, len(cfg.Modos))
	for codigo, m := range cfg.Modos {
		modos[codigo] = string(m)
	}

	reforcos := make(map[string]float64, len(cfg.Reforcos))
	for codigo := range cfg.Reforcos {
		reforcos[codigo] = cfg.ReforcoDe(codigo)
	}

	ciclo := make([]ItemDoCiclo, 0, len(cfg.CicloRevisao))
	for _, it := range cfg.CicloRevisao {
		ciclo = append(ciclo, ItemDoCiclo{Titulo: it.Titulo, Questoes: it.Questoes})
	}

	// MinutosBloco é o que a tela de ajustes edita. Quando o plano nunca teve
	// duração explícita, informa a implícita em HorasDia, para que a tela mostre
	// um número real e o primeiro save o solidifique.
	minutos := cfg.MinutosBloco
	if minutos == 0 {
		minutos = minutosImplicitos(cfg)
	}

	return ConfigDoPlano{
		Inicio:         cfg.Inicio.Format(formatoISO),
		Prova:          cfg.Prova.Format(formatoISO),
		HorasDia:       cfg.HorasDia,
		DiasEstudo:     cfg.DiasEstudo,
		DiaRevisao:     cfg.DiaRevisao,
		RetaFinalDias:  cfg.RetaFinalDias,
		TemaUI:         tema,
		Questoes:       cfg.Questoes,
		BlocosPorDia:   cfg.BlocosPorDia,
		MinutosBloco:   minutos,
		MinutosRevisao: cfg.MinutosRevisao,
		Reforcos:       reforcos,
		CicloRevisao:   ciclo,
		RevisaoSemanal: cfg.RevisaoSemanal,
		Simulados:      string(cfg.Simulados),
		Discursiva:     cfg.Discursiva,
		Modos:          modos,
		PctQuestoes:    cfg.PctQuestoes,
		LimiarFraco:    cfg.LimiarFraco,
	}
}

// minutosImplicitos é a duração de bloco que HorasDia implica, quando nenhuma
// foi escolhida.
func minutosImplicitos(cfg plano.Config) int {
	if cfg.BlocosPorDia <= 0 {
		return 0
	}

	uteis := cfg.HorasDia*60 - float64(cfg.MinutosRevisao)
	if uteis <= 0 {
		return 0
	}

	return int(math.Round(uteis / float64(cfg.BlocosPorDia)))
}

func montarMarcos(c concurso.Concurso, checks map[uuid.UUID]bool) []MarcoDoPlano {
	out := make([]MarcoDoPlano, 0, len(c.Marcos))

	for _, m := range c.Marcos {
		var fim *string

		if m.DataFim != nil {
			s := m.DataFim.Format(formatoISO)
			fim = &s
		}

		out = append(out, MarcoDoPlano{
			ID:         m.ID,
			Rotulo:     m.Rotulo,
			DataInicio: m.DataInicio.Format(formatoISO),
			DataFim:    fim,
			Titulo:     m.Titulo,
			ExigeAcao:  m.ExigeAcao,
			EProva:     m.EProva,
			Cumprido:   checks[m.ID],
		})
	}

	return out
}

func montarRevisao(
	disciplina string,
	dt time.Time,
	dias map[time.Time]plano.RegistroDia,
	anotacoes []plano.Anotacao,
) *RevisaoDoDia {
	out := &RevisaoDoDia{Disciplina: disciplina}

	if r, ok := dias[dt]; ok {
		out.Questoes = r.RevisaoQuestoes
		out.Acertos = r.RevisaoAcertos
	}

	if a := anotacaoDaRevisao(anotacoes, dt); a != nil {
		out.Observacao = a.Texto
	}

	return out
}

// foiEstudado diz se uma atividade foi de fato estudada, que é o que lhe dá
// lugar na fila de revisão.
//
// Quem decide é o registro da PRÓPRIA atividade: concluída, ou com horas
// lançadas nela. Antes havia um caminho alternativo que olhava "qualquer bloco
// desta disciplina", e era ele que deixava uma matéria adiantada aparecer como
// já estudada — o estudante a trouxe para o dia justamente porque ela ainda
// estava por fazer.
func foiEstudado(cx contexto) func(plano.ItemDia, plano.Dia) bool {
	return func(it plano.ItemDia, _ plano.Dia) bool {
		reg, ok := cx.Registros[it.AtividadeID]
		if !ok {
			return false
		}

		return reg.Concluido || (reg.Horas != nil && *reg.Horas > 0)
	}
}

// temasPorRevisao é quantos temas um bloco de revisão cobre.
//
// Derivado da duração do próprio bloco, e não configurado à parte: um bloco de
// 20 minutos não pode honestamente pedir seis temas. Cerca de dez minutos cada,
// com ao menos um sempre que houver bloco.
func temasPorRevisao(cfg plano.Config) int {
	cfg = cfg.Normalizar()

	if cfg.MinutosRevisao <= 0 {
		return 0
	}

	return min(max(cfg.MinutosRevisao/10, 1), 6)
}

// resultadosDoPlano reúne cada bateria respondida no plano, que é do que o
// caderno de erros é feito.
func resultadosDoPlano(dias []plano.Dia, cx contexto) []plano.ResultadoTema {
	out := []plano.ResultadoTema{}

	for _, d := range dias {
		for _, a := range plano.AtividadesDoDia(cx.Atividades, plano.DayOf(d.Data)) {
			reg, ok := cx.Registros[a.ID]
			if !ok || reg.Questoes == nil || *reg.Questoes <= 0 || a.Disciplina == "" {
				continue
			}

			acertos := 0
			if reg.Acertos != nil {
				acertos = *reg.Acertos
			}

			out = append(out, plano.ResultadoTema{
				Disciplina: a.Disciplina,
				Tema:       a.Tema,
				Data:       plano.DayOf(a.Data).Format(formatoISO),
				Questoes:   *reg.Questoes,
				Acertos:    acertos,
			})
		}
	}

	return out
}

func errosDe(questoes, acertos *int) *int {
	if questoes == nil || acertos == nil {
		return nil
	}

	e := *questoes - *acertos
	if e < 0 {
		e = 0
	}

	return &e
}

func arredondar1(v float64) float64 {
	return math.Round(v*10) / 10
}
