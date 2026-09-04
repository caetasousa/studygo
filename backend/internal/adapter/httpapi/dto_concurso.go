package httpapi

import (
	"studygo/internal/service"

	"github.com/google/uuid"
)

// Contrato de transporte do catálogo, do assistente de edital, das estatísticas
// e do caderno.

type concursoRequest struct {
	Nome          string              `json:"nome"`
	Banca         string              `json:"banca"`
	Cargo         string              `json:"cargo"`
	Emoji         string              `json:"emoji"`
	Prova         string              `json:"prova"`
	RetaFinalDias int                 `json:"retaFinalDias"`
	Resumo        string              `json:"resumo"`
	Disciplinas   []disciplinaRequest `json:"disciplinas"`
	Marcos        []marcoRequest      `json:"marcos"`
	Conteudo      []conteudoItemDTO   `json:"conteudo"`
}

// disciplinaRequest traz o `id` de volta ao servidor nas edições. É o que
// permite renomear uma matéria sem que ela vire outra matéria — e sem que o
// cronograma e o histórico dela se desliguem.
type disciplinaRequest struct {
	ID         string     `json:"id"`
	Nome       string     `json:"nome"`
	Bloco      string     `json:"bloco"`
	Questoes   int        `json:"questoes"`
	Peso       int        `json:"peso"`
	CadernoURL string     `json:"cadernoUrl"`
	Temas      []string   `json:"temas"`
	Fontes     []fonteDTO `json:"fontes"`
}

type marcoRequest struct {
	Data      string `json:"data"`
	DataFim   string `json:"dataFim"`
	Titulo    string `json:"titulo"`
	ExigeAcao bool   `json:"exigeAcao"`
}

type concursoResumoDTO struct {
	Slug  string `json:"slug"`
	Nome  string `json:"nome"`
	Banca string `json:"banca"`
	Cargo string `json:"cargo"`
	Emoji string `json:"emoji"`
	Prova string `json:"prova"`
}

type concursoDetalheDTO struct {
	Slug   string          `json:"slug"`
	Dados  concursoRequest `json:"dados"`
	Avisos []string        `json:"avisos"`
}

type listaConcursosDTO struct {
	Concursos        []concursoResumoDTO `json:"concursos"`
	ImportacaoEdital bool                `json:"importacaoEdital"`
}

// --- assistente de edital ---

type alertaEditalDTO struct {
	Codigo    string `json:"codigo"`
	Gravidade string `json:"gravidade"`
	Mensagem  string `json:"mensagem"`
	Campo     string `json:"campo,omitempty"`
}

type analiseDTO struct {
	DocumentoID  string            `json:"documentoId"`
	Banca        string            `json:"banca"`
	TotalPaginas int               `json:"totalPaginas"`
	PaginasOCR   int               `json:"paginasOcr"`
	Cargos       []cargoDTO        `json:"cargos"`
	Alertas      []alertaEditalDTO `json:"alertas"`
}

type cargoDTO struct {
	Codigo        string `json:"codigo"`
	Nome          string `json:"nome"`
	Especialidade string `json:"especialidade,omitempty"`
	Escolaridade  string `json:"escolaridade,omitempty"`
	Vagas         *int   `json:"vagas"`
}

type estruturaDTO struct {
	Nome        string            `json:"nome"`
	Prova       string            `json:"prova"`
	Gerais      []grupoDTO        `json:"gerais"`
	Especificas []grupoDTO        `json:"especificas"`
	Discursivas []discursivaDTO   `json:"discursivas"`
	Duracao     *duracaoDTO       `json:"duracao"`
	Marcos      []marcoRequest    `json:"marcos"`
	Alertas     []alertaEditalDTO `json:"alertas"`
}

type grupoDTO struct {
	Kind        string                  `json:"kind"`
	Rotulo      string                  `json:"rotulo"`
	Total       *int                    `json:"total"`
	Peso        *float64                `json:"peso"`
	PesoEscopo  string                  `json:"pesoEscopo,omitempty"`
	Disciplinas []disciplinaExtraidaDTO `json:"disciplinas"`
}

type disciplinaExtraidaDTO struct {
	Nome     string   `json:"nome"`
	Questoes *int     `json:"questoes"`
	Peso     *float64 `json:"peso"`
}

type discursivaDTO struct {
	Modalidade string `json:"modalidade"`
	Rotulo     string `json:"rotulo"`
	Questoes   *int   `json:"questoes"`
}

type duracaoDTO struct {
	Minutos int    `json:"minutos"`
	Escopo  string `json:"escopo"`
}

type conteudoEditalDTO struct {
	Itens   []disciplinaComTemasDTO `json:"itens"`
	Alertas []alertaEditalDTO       `json:"alertas"`
}

type disciplinaComTemasDTO struct {
	Nome  string   `json:"nome"`
	Temas []string `json:"temas"`
}

// --- estatísticas e caderno ---

type estatisticasDTO struct {
	Serie         []pontoSerieDTO      `json:"serie"`
	PorSemana     []resumoSemanaDTO    `json:"porSemana"`
	PorDisciplina []linhaBalanceamento `json:"porDisciplina"`
	Streak        int                  `json:"streak"`
	HorasTotal    float64              `json:"horasTotal"`
	QuestoesTotal int                  `json:"questoesTotal"`
	AcertoPct     *int                 `json:"acertoPct"`
}

type pontoSerieDTO struct {
	Data     string  `json:"data"`
	Horas    float64 `json:"horas"`
	Questoes int     `json:"questoes"`
	Acertos  int     `json:"acertos"`
}

type resumoSemanaDTO struct {
	Semana        int     `json:"semana"`
	HorasPrevisto float64 `json:"horasPrevisto"`
	Horas         float64 `json:"horas"`
	Questoes      int     `json:"questoes"`
	Acertos       int     `json:"acertos"`
}

type cadernoDTO struct {
	PorDisciplina []cadernoDisciplinaDTO `json:"porDisciplina"`
	Anotacoes     []anotacaoDTO          `json:"anotacoes"`
	DiasComNota   []diaComNotaDTO        `json:"diasComNota"`
	DiasFracos    []diaFracoDTO          `json:"diasFracos"`
}

type diaComNotaDTO struct {
	Data        string   `json:"data"`
	N           int      `json:"n"`
	Disciplinas []string `json:"disciplinas"`
	Nota        string   `json:"nota"`
}

type cadernoDisciplinaDTO struct {
	Codigo string           `json:"codigo"`
	Nome   string           `json:"nome"`
	Cor    int              `json:"cor"`
	Itens  []itemCadernoDTO `json:"itens"`
}

type itemCadernoDTO struct {
	Tema       string `json:"tema"`
	Questoes   int    `json:"questoes"`
	Acertos    int    `json:"acertos"`
	Erros      int    `json:"erros"`
	Aprov      int    `json:"aprov"`
	UltimaData string `json:"ultimaData"`
}

type anotacaoDTO struct {
	ID         uuid.UUID `json:"id"`
	Data       *string   `json:"data"`
	Disciplina string    `json:"disciplina"`
	Tema       string    `json:"tema"`
	Texto      string    `json:"texto"`
	Origem     string    `json:"origem"`
	URL        string    `json:"url"`
	Resolvido  bool      `json:"resolvido"`
}

type diaFracoDTO struct {
	Data     string `json:"data"`
	N        int    `json:"n"`
	Questoes int    `json:"questoes"`
	Acertos  int    `json:"acertos"`
	Aprov    int    `json:"aprov"`
}

// --- mapeamento ---

func concursoParaComando(req concursoRequest) service.ConcursoCommand {
	discs := make([]service.DisciplinaCommand, 0, len(req.Disciplinas))

	for _, d := range req.Disciplinas {
		fontes := make([]service.FonteCommand, 0, len(d.Fontes))
		for _, f := range d.Fontes {
			fontes = append(fontes, service.FonteCommand{Titulo: f.Titulo, URL: f.URL, Tipo: f.Tipo})
		}

		discs = append(discs, service.DisciplinaCommand{
			ID:         d.ID,
			Nome:       d.Nome,
			Bloco:      d.Bloco,
			Questoes:   d.Questoes,
			Peso:       d.Peso,
			CadernoURL: d.CadernoURL,
			Temas:      d.Temas,
			Fontes:     fontes,
		})
	}

	marcos := make([]service.MarcoCommand, 0, len(req.Marcos))
	for _, m := range req.Marcos {
		marcos = append(marcos, service.MarcoCommand{
			Data: m.Data, DataFim: m.DataFim, Titulo: m.Titulo, ExigeAcao: m.ExigeAcao,
		})
	}

	conteudo := make([]service.ConteudoCommand, 0, len(req.Conteudo))
	for _, it := range req.Conteudo {
		conteudo = append(conteudo, service.ConteudoCommand{Tipo: it.Tipo, Texto: it.Texto})
	}

	return service.ConcursoCommand{
		Nome:          req.Nome,
		Banca:         req.Banca,
		Cargo:         req.Cargo,
		Emoji:         req.Emoji,
		Prova:         req.Prova,
		RetaFinalDias: req.RetaFinalDias,
		Resumo:        req.Resumo,
		Disciplinas:   discs,
		Marcos:        marcos,
		Conteudo:      conteudo,
	}
}

func comandoParaConcurso(cmd service.ConcursoCommand) concursoRequest {
	discs := make([]disciplinaRequest, 0, len(cmd.Disciplinas))

	for _, d := range cmd.Disciplinas {
		fontes := make([]fonteDTO, 0, len(d.Fontes))
		for _, f := range d.Fontes {
			fontes = append(fontes, fonteDTO{Titulo: f.Titulo, URL: f.URL, Tipo: f.Tipo})
		}

		discs = append(discs, disciplinaRequest{
			ID:         d.ID,
			Nome:       d.Nome,
			Bloco:      d.Bloco,
			Questoes:   d.Questoes,
			Peso:       d.Peso,
			CadernoURL: d.CadernoURL,
			Temas:      d.Temas,
			Fontes:     fontes,
		})
	}

	marcos := make([]marcoRequest, 0, len(cmd.Marcos))
	for _, m := range cmd.Marcos {
		marcos = append(marcos, marcoRequest{
			Data: m.Data, DataFim: m.DataFim, Titulo: m.Titulo, ExigeAcao: m.ExigeAcao,
		})
	}

	conteudo := make([]conteudoItemDTO, 0, len(cmd.Conteudo))
	for _, it := range cmd.Conteudo {
		conteudo = append(conteudo, conteudoItemDTO{Tipo: it.Tipo, Texto: it.Texto})
	}

	return concursoRequest{
		Nome:          cmd.Nome,
		Banca:         cmd.Banca,
		Cargo:         cmd.Cargo,
		Emoji:         cmd.Emoji,
		Prova:         cmd.Prova,
		RetaFinalDias: cmd.RetaFinalDias,
		Resumo:        cmd.Resumo,
		Disciplinas:   discs,
		Marcos:        marcos,
		Conteudo:      conteudo,
	}
}

func resumoParaDTO(r service.ConcursoResumo) concursoResumoDTO {
	return concursoResumoDTO{
		Slug: r.Slug, Nome: r.Nome, Banca: r.Banca,
		Cargo: r.Cargo, Emoji: r.Emoji, Prova: r.Prova,
	}
}

func alertasEditalParaDTO(as []service.AlertaDoEdital) []alertaEditalDTO {
	out := make([]alertaEditalDTO, 0, len(as))

	for _, a := range as {
		out = append(out, alertaEditalDTO{
			Codigo: a.Codigo, Gravidade: a.Gravidade,
			Mensagem: a.Mensagem, Campo: a.Campo,
		})
	}

	return out
}

func analiseParaDTO(a service.AnaliseDoEdital) analiseDTO {
	cargos := make([]cargoDTO, 0, len(a.Cargos))

	for _, c := range a.Cargos {
		cargos = append(cargos, cargoDTO{
			Codigo: c.Codigo, Nome: c.Nome, Especialidade: c.Especialidade,
			Escolaridade: c.Escolaridade, Vagas: c.Vagas,
		})
	}

	return analiseDTO{
		DocumentoID:  a.DocumentoID,
		Banca:        a.Banca,
		TotalPaginas: a.TotalPaginas,
		PaginasOCR:   a.PaginasOCR,
		Cargos:       cargos,
		Alertas:      alertasEditalParaDTO(a.Alertas),
	}
}

func gruposParaDTO(gs []service.GrupoDoEdital) []grupoDTO {
	out := make([]grupoDTO, 0, len(gs))

	for _, g := range gs {
		discs := make([]disciplinaExtraidaDTO, 0, len(g.Disciplinas))
		for _, d := range g.Disciplinas {
			discs = append(discs, disciplinaExtraidaDTO{
				Nome: d.Nome, Questoes: d.Questoes, Peso: d.Peso,
			})
		}

		out = append(out, grupoDTO{
			Kind: g.Tipo, Rotulo: g.Rotulo, Total: g.Total,
			Peso: g.Peso, PesoEscopo: g.PesoEscopo, Disciplinas: discs,
		})
	}

	return out
}

func estruturaParaDTO(e service.EstruturaDoEdital) estruturaDTO {
	discursivas := make([]discursivaDTO, 0, len(e.Discursivas))
	for _, d := range e.Discursivas {
		discursivas = append(discursivas, discursivaDTO{
			Modalidade: d.Modalidade, Rotulo: d.Rotulo, Questoes: d.Questoes,
		})
	}

	marcos := make([]marcoRequest, 0, len(e.Marcos))
	for _, m := range e.Marcos {
		marcos = append(marcos, marcoRequest{
			Data: m.Data, DataFim: m.DataFim, Titulo: m.Titulo, ExigeAcao: m.ExigeAcao,
		})
	}

	var duracao *duracaoDTO
	if e.Duracao != nil {
		duracao = &duracaoDTO{Minutos: e.Duracao.Minutos, Escopo: e.Duracao.Escopo}
	}

	return estruturaDTO{
		Nome:        e.Nome,
		Prova:       e.Prova,
		Gerais:      gruposParaDTO(e.Gerais),
		Especificas: gruposParaDTO(e.Especificas),
		Discursivas: discursivas,
		Duracao:     duracao,
		Marcos:      marcos,
		Alertas:     alertasEditalParaDTO(e.Alertas),
	}
}

func conteudoEditalParaDTO(c service.ConteudoDoEdital) conteudoEditalDTO {
	itens := make([]disciplinaComTemasDTO, 0, len(c.Itens))
	for _, it := range c.Itens {
		itens = append(itens, disciplinaComTemasDTO{Nome: it.Nome, Temas: it.Temas})
	}

	return conteudoEditalDTO{Itens: itens, Alertas: alertasEditalParaDTO(c.Alertas)}
}

func estatisticasParaDTO(e service.Estatisticas) estatisticasDTO {
	serie := make([]pontoSerieDTO, 0, len(e.Serie))
	for _, p := range e.Serie {
		serie = append(serie, pontoSerieDTO{
			Data: p.Data, Horas: p.Horas, Questoes: p.Questoes, Acertos: p.Acertos,
		})
	}

	semanas := make([]resumoSemanaDTO, 0, len(e.PorSemana))
	for _, s := range e.PorSemana {
		semanas = append(semanas, resumoSemanaDTO{
			Semana: s.Semana, HorasPrevisto: s.HorasPrevisto, Horas: s.Horas,
			Questoes: s.Questoes, Acertos: s.Acertos,
		})
	}

	return estatisticasDTO{
		Serie:         serie,
		PorSemana:     semanas,
		PorDisciplina: balanceamentoParaDTO(e.PorDisciplina),
		Streak:        e.Streak,
		HorasTotal:    e.HorasTotal,
		QuestoesTotal: e.QuestoesTotal,
		AcertoPct:     e.AcertoPct,
	}
}

func cadernoParaDTO(c service.Caderno) cadernoDTO {
	discs := make([]cadernoDisciplinaDTO, 0, len(c.PorDisciplina))

	for _, d := range c.PorDisciplina {
		itens := make([]itemCadernoDTO, 0, len(d.Itens))
		for _, it := range d.Itens {
			itens = append(itens, itemCadernoDTO{
				Tema: it.Tema, Questoes: it.Questoes, Acertos: it.Acertos,
				Erros: it.Erros, Aprov: it.Aprov, UltimaData: it.UltimaData,
			})
		}

		discs = append(discs, cadernoDisciplinaDTO{
			Codigo: d.Codigo, Nome: d.Nome, Cor: d.Cor, Itens: itens,
		})
	}

	anots := make([]anotacaoDTO, 0, len(c.Anotacoes))
	for _, a := range c.Anotacoes {
		anots = append(anots, anotacaoDTO{
			ID: a.ID, Data: a.Data, Disciplina: a.Disciplina, Tema: a.Tema,
			Texto: a.Texto, Origem: a.Origem, URL: a.URL, Resolvido: a.Resolvido,
		})
	}

	comNota := make([]diaComNotaDTO, 0, len(c.DiasComNota))
	for _, d := range c.DiasComNota {
		comNota = append(comNota, diaComNotaDTO{
			Data: d.Data, N: d.N, Disciplinas: d.Disciplinas, Nota: d.Nota,
		})
	}

	fracos := make([]diaFracoDTO, 0, len(c.DiasFracos))
	for _, d := range c.DiasFracos {
		fracos = append(fracos, diaFracoDTO{
			Data: d.Data, N: d.N, Questoes: d.Questoes, Acertos: d.Acertos, Aprov: d.Aprov,
		})
	}

	return cadernoDTO{
		PorDisciplina: discs, Anotacoes: anots,
		DiasComNota: comNota, DiasFracos: fracos,
	}
}
