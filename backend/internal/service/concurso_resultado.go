package service

import (
	"studygo/internal/domain/concurso"
	"studygo/internal/port"
)

// Comandos e resultados do cadastro de concurso e do assistente de edital.
// Como todo tipo de aplicação, não carregam tag JSON.

// ConcursoCommand é um concurso como o formulário o envia.
type ConcursoCommand struct {
	Nome          string
	Banca         string
	Cargo         string
	Emoji         string
	Prova         string
	RetaFinalDias int
	Resumo        string
	Disciplinas   []DisciplinaCommand
	Marcos        []MarcoCommand
	Conteudo      []ConteudoCommand
}

// DisciplinaCommand é uma matéria do formulário. ID vem preenchido quando a
// matéria já existe: é o que permite editar o concurso sem desligar o
// cronograma dela.
type DisciplinaCommand struct {
	ID         string
	Nome       string
	Bloco      string
	Questoes   int
	Peso       int
	CadernoURL string
	Temas      []string
	Fontes     []FonteCommand
}

type FonteCommand struct {
	Titulo string
	URL    string
	Tipo   string
}

type MarcoCommand struct {
	Data      string
	DataFim   string
	Titulo    string
	ExigeAcao bool
}

type ConteudoCommand struct {
	Tipo  string
	Texto string
}

// ConcursoResumo é uma linha do seletor de concursos.
type ConcursoResumo struct {
	Slug  string
	Nome  string
	Banca string
	Cargo string
	Emoji string
	Prova string
}

// ConcursoDetalhe preenche o formulário de edição.
type ConcursoDetalhe struct {
	Slug   string
	Dados  ConcursoCommand
	Avisos []string
}

// --- assistente de importação do edital ---

// AlertaDoEdital é uma marcação de revisão vinda do processador.
type AlertaDoEdital struct {
	Codigo    string
	Gravidade string
	Mensagem  string
	Campo     string
}

// AnaliseDoEdital é o passo 1: o documento e os fatos baratos do topo.
type AnaliseDoEdital struct {
	DocumentoID  string
	Banca        string
	TotalPaginas int
	PaginasOCR   int
	Cargos       []CargoDoEdital
	Alertas      []AlertaDoEdital
}

type CargoDoEdital struct {
	Codigo        string
	Nome          string
	Especialidade string
	Escolaridade  string
	Vagas         *int
}

// EstruturaDoEdital é o passo 2: os grupos e disciplinas do cargo escolhido.
// Contagens que o edital não detalhou vêm nulas; o assistente faz o usuário
// estimar ou ratear explicitamente antes de salvar.
type EstruturaDoEdital struct {
	Nome        string
	Prova       string
	Gerais      []GrupoDoEdital
	Especificas []GrupoDoEdital
	Discursivas []DiscursivaDoEdital
	Duracao     *DuracaoDoEdital
	Marcos      []MarcoCommand
	Alertas     []AlertaDoEdital
}

type GrupoDoEdital struct {
	Tipo        string
	Rotulo      string
	Total       *int
	Peso        *float64
	PesoEscopo  string
	Disciplinas []DisciplinaDoEdital
}

type DisciplinaDoEdital struct {
	Nome     string
	Questoes *int
	Peso     *float64
}

type DiscursivaDoEdital struct {
	Modalidade string
	Rotulo     string
	Questoes   *int
}

type DuracaoDoEdital struct {
	Minutos int
	Escopo  string
}

// ConteudoDoEdital é o passo 3: os tópicos por disciplina.
type ConteudoDoEdital struct {
	Itens   []DisciplinaComTemas
	Alertas []AlertaDoEdital
}

type DisciplinaComTemas struct {
	Nome  string
	Temas []string
}

func alertasDoEdital(in []port.EditalAlerta) []AlertaDoEdital {
	out := make([]AlertaDoEdital, 0, len(in))

	for _, a := range in {
		out = append(out, AlertaDoEdital{
			Codigo:    a.Codigo,
			Gravidade: a.Gravidade,
			Mensagem:  a.Mensagem,
			Campo:     a.Campo,
		})
	}

	return out
}

func analiseDoEdital(a port.EditalAnalise) AnaliseDoEdital {
	cargos := make([]CargoDoEdital, 0, len(a.Cargos))

	for _, c := range a.Cargos {
		cargos = append(cargos, CargoDoEdital{
			Codigo:        c.Codigo,
			Nome:          c.Nome,
			Especialidade: c.Especialidade,
			Escolaridade:  c.Escolaridade,
			Vagas:         c.Vagas,
		})
	}

	return AnaliseDoEdital{
		DocumentoID:  a.DocumentoID,
		Banca:        a.Banca,
		TotalPaginas: a.TotalPaginas,
		PaginasOCR:   a.PaginasOCR,
		Cargos:       cargos,
		Alertas:      alertasDoEdital(a.Alertas),
	}
}

func gruposDoEdital(in []port.EditalGrupo) []GrupoDoEdital {
	out := make([]GrupoDoEdital, 0, len(in))

	for _, g := range in {
		discs := make([]DisciplinaDoEdital, 0, len(g.Disciplinas))
		for _, d := range g.Disciplinas {
			discs = append(discs, DisciplinaDoEdital{
				Nome: d.Nome, Questoes: d.Questoes, Peso: d.Peso,
			})
		}

		out = append(out, GrupoDoEdital{
			Tipo:        g.Kind,
			Rotulo:      g.Rotulo,
			Total:       g.Total,
			Peso:        g.Peso,
			PesoEscopo:  g.PesoEscopo,
			Disciplinas: discs,
		})
	}

	return out
}

func estruturaDoEdital(e port.EditalEstrutura) EstruturaDoEdital {
	discursivas := make([]DiscursivaDoEdital, 0, len(e.Discursivas))
	for _, d := range e.Discursivas {
		discursivas = append(discursivas, DiscursivaDoEdital{
			Modalidade: d.Modalidade, Rotulo: d.Rotulo, Questoes: d.Questoes,
		})
	}

	marcos := make([]MarcoCommand, 0, len(e.Marcos))
	for _, m := range e.Marcos {
		marcos = append(marcos, MarcoCommand{
			Data: m.Data, DataFim: m.DataFim, Titulo: m.Titulo, ExigeAcao: m.ExigeAcao,
		})
	}

	var duracao *DuracaoDoEdital
	if e.Duracao != nil {
		duracao = &DuracaoDoEdital{Minutos: e.Duracao.Minutos, Escopo: e.Duracao.Escopo}
	}

	return EstruturaDoEdital{
		Nome:        e.NomeSugerido,
		Prova:       e.DataProva,
		Gerais:      gruposDoEdital(e.GruposGerais),
		Especificas: gruposDoEdital(e.GruposEspecificos),
		Discursivas: discursivas,
		Duracao:     duracao,
		Marcos:      marcos,
		Alertas:     alertasDoEdital(e.Alertas),
	}
}

func resumoDe(c concurso.Concurso) ConcursoResumo {
	return ConcursoResumo{
		Slug:  c.Slug,
		Nome:  c.Nome,
		Banca: c.Banca,
		Cargo: c.Cargo,
		Emoji: c.Emoji,
		Prova: c.ProvaPadrao.Format(formatoISO),
	}
}

func detalheDe(c concurso.Concurso) ConcursoDetalhe {
	discs := make([]DisciplinaCommand, 0, len(c.Disciplinas))

	for _, d := range c.Disciplinas {
		fontes := make([]FonteCommand, 0, len(d.Fontes))
		for _, f := range d.Fontes {
			fontes = append(fontes, FonteCommand{Titulo: f.Titulo, URL: f.URL, Tipo: f.Tipo})
		}

		temas := d.Temas
		if temas == nil {
			temas = []string{}
		}

		discs = append(discs, DisciplinaCommand{
			ID:         d.ID.String(),
			Nome:       d.Nome,
			Bloco:      string(d.Bloco),
			Questoes:   d.QuestoesPadrao,
			Peso:       d.Peso,
			CadernoURL: d.CadernoURL,
			Temas:      temas,
			Fontes:     fontes,
		})
	}

	marcos := make([]MarcoCommand, 0, len(c.Marcos))

	for _, m := range c.Marcos {
		mc := MarcoCommand{
			Data:      m.DataInicio.Format(formatoISO),
			Titulo:    m.Titulo,
			ExigeAcao: m.ExigeAcao,
		}

		if m.DataFim != nil {
			mc.DataFim = m.DataFim.Format(formatoISO)
		}

		marcos = append(marcos, mc)
	}

	conteudo := make([]ConteudoCommand, 0, len(c.Conteudo))
	for _, it := range c.Conteudo {
		conteudo = append(conteudo, ConteudoCommand{Tipo: it.Tipo, Texto: it.Texto})
	}

	return ConcursoDetalhe{
		Slug: c.Slug,
		Dados: ConcursoCommand{
			Nome:          c.Nome,
			Banca:         c.Banca,
			Cargo:         c.Cargo,
			Emoji:         c.Emoji,
			Prova:         c.ProvaPadrao.Format(formatoISO),
			RetaFinalDias: c.RetaPadraoDias,
			Resumo:        c.Resumo,
			Disciplinas:   discs,
			Marcos:        marcos,
			Conteudo:      conteudo,
		},
	}
}
