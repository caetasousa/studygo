package httpapi

import (
	"time"

	"studygo/internal/service"

	"github.com/google/uuid"
)

// O contrato de transporte do plano.
//
// Toda tag JSON do sistema vive nos DTOs deste adapter. A aplicação devolve
// tipos sem tag (service.PlanoMontado e companhia) e aqui eles viram payload:
// assim o formato do JSON pode mudar sem mexer no caso de uso, e o caso de uso
// pode ganhar um campo sem que ele vaze para a API por descuido.
//
// Os mappers são por AGREGADO, não por struct: `planoDTO` converte o plano
// inteiro. Uma função de uma linha por tipo seria cerimônia sem ganho.

type planoDTO struct {
	Concurso      concursoDoPlanoDTO   `json:"concurso"`
	Config        configDTO            `json:"config"`
	Dias          []diaDTO             `json:"dias"`
	Marcos        []marcoDTO           `json:"marcos"`
	Balanceamento []linhaBalanceamento `json:"balanceamento"`
	Props         propsDTO             `json:"props"`
	Alertas       []alertaDTO          `json:"alertas"`
	HojeIndex     *int                 `json:"hojeIndex"`
	// TemMovimentacaoManual habilita o botão de restaurar a ordem automática.
	TemMovimentacaoManual bool      `json:"temMovimentacaoManual"`
	GeradoEm              time.Time `json:"geradoEm"`
}

type concursoDoPlanoDTO struct {
	Slug        string            `json:"slug"`
	Nome        string            `json:"nome"`
	Banca       string            `json:"banca"`
	Cargo       string            `json:"cargo"`
	Emoji       string            `json:"emoji"`
	Resumo      string            `json:"resumo"`
	Disciplinas []disciplinaDTO   `json:"disciplinas"`
	Conteudo    []conteudoItemDTO `json:"conteudo"`
}

// disciplinaDTO não tem campo "sigla": o `codigo` É o mnemônico exibido. Havia
// os dois, e o frontend calculava a sigla por conta própria com outra regra —
// era isso que fazia o chip da tela discordar do que estava gravado.
type disciplinaDTO struct {
	Codigo     string     `json:"codigo"`
	Nome       string     `json:"nome"`
	Bloco      string     `json:"bloco"`
	Peso       int        `json:"peso"`
	Cor        int        `json:"cor"`
	CadernoURL string     `json:"cadernoUrl"`
	Temas      []string   `json:"temas"`
	Fontes     []fonteDTO `json:"fontes"`
}

type fonteDTO struct {
	Titulo string `json:"titulo"`
	URL    string `json:"url"`
	Tipo   string `json:"tipo"`
}

type conteudoItemDTO struct {
	Tipo  string `json:"tipo"`
	Texto string `json:"texto"`
}

type configDTO struct {
	Inicio        string         `json:"inicio"`
	Prova         string         `json:"prova"`
	HorasDia      float64        `json:"horasDia"`
	DiasEstudo    []int          `json:"diasEstudo"`
	DiaRevisao    int            `json:"diaRevisao"`
	RetaFinalDias int            `json:"retaFinalDias"`
	TemaUI        string         `json:"temaUi"`
	Questoes      map[string]int `json:"questoes"`

	BlocosPorDia   int                `json:"blocosPorDia"`
	MinutosBloco   int                `json:"minutosBloco"`
	MinutosRevisao int                `json:"minutosRevisao"`
	Reforcos       map[string]float64 `json:"reforcos"`
	CicloRevisao   []itemCicloDTO     `json:"cicloRevisao"`
	RevisaoSemanal bool               `json:"revisaoSemanal"`
	Simulados      string             `json:"simulados"`
	Discursiva     bool               `json:"discursiva"`
	Modos          map[string]string  `json:"modos"`
	PctQuestoes    float64            `json:"pctQuestoes"`
	LimiarFraco    int                `json:"limiarFraco"`
}

type itemCicloDTO struct {
	Titulo   string `json:"titulo"`
	Questoes int    `json:"questoes"`
}

type diaDTO struct {
	N      int            `json:"n"`
	Data   string         `json:"data"`
	Semana int            `json:"semana"`
	Fase   string         `json:"fase"`
	Tipo   string         `json:"tipo"`
	Itens  []atividadeDTO `json:"itens"`
	Tema   string         `json:"tema"`
	Meta   int            `json:"meta"`
	Blocos []blocoDTO     `json:"blocos"`
	// Rotulo é o nome do dia na tela ("SIMULADO", "VÉSPERA"). Vem do servidor
	// para que exista uma única tabela desses nomes: o frontend tinha a sua, e
	// as duas já divergiam.
	Rotulo    string      `json:"rotulo"`
	Concluido bool        `json:"concluido"`
	Horas     *float64    `json:"horas"`
	Questoes  *int        `json:"questoes"`
	Acertos   *int        `json:"acertos"`
	Nota      string      `json:"nota"`
	Revisao   *revisaoDTO `json:"revisao"`
}

type atividadeDTO struct {
	ID         uuid.UUID `json:"id"`
	Disciplina string    `json:"disciplina"`
	Tema       string    `json:"tema"`
	Passada    int       `json:"passada"`
	Movida     bool      `json:"movida"`
	Horas      *float64  `json:"horas"`
	Questoes   *int      `json:"questoes"`
	Acertos    *int      `json:"acertos"`
	Erros      *int      `json:"erros"`
	Nota       string    `json:"nota"`
	Concluido  bool      `json:"concluido"`
}

type blocoDTO struct {
	Minutos int    `json:"minutos"`
	Titulo  string `json:"titulo"`
	Detalhe string `json:"detalhe"`
}

type revisaoDTO struct {
	Disciplina string `json:"disciplina"`
	Questoes   *int   `json:"questoes"`
	Acertos    *int   `json:"acertos"`
	Observacao string `json:"observacao"`
}

type marcoDTO struct {
	ID         uuid.UUID `json:"id"`
	Rotulo     int       `json:"rotulo"`
	DataInicio string    `json:"dataInicio"`
	DataFim    *string   `json:"dataFim"`
	Titulo     string    `json:"titulo"`
	ExigeAcao  bool      `json:"exigeAcao"`
	EProva     bool      `json:"eProva"`
	Cumprido   bool      `json:"cumprido"`
}

type linhaBalanceamento struct {
	Codigo         string  `json:"codigo"`
	Nome           string  `json:"nome"`
	Bloco          string  `json:"bloco"`
	Cor            int     `json:"cor"`
	Questoes       int     `json:"questoes"`
	QuestoesEdital int     `json:"questoesEdital"`
	Delta          int     `json:"delta"`
	Modo           string  `json:"modo"`
	Peso           int     `json:"peso"`
	Pontos         int     `json:"pontos"`
	PctIdeal       float64 `json:"pctIdeal"`
	BlocosConteudo int     `json:"blocosConteudo"`
	BlocosReta     int     `json:"blocosReta"`
	Temas          int     `json:"temas"`
	Passadas       float64 `json:"passadas"`
	Visitas        int     `json:"visitas"`
	RevisoesGerais float64 `json:"revisoesGerais"`
	IntervaloDias  float64 `json:"intervaloDias"`
	HorasPrevisto  float64 `json:"horasPrevisto"`
	HorasLancado   float64 `json:"horasLancado"`
	Desvio         float64 `json:"desvio"`
	AcertoPct      *int    `json:"acertoPct"`
}

type propsDTO struct {
	FaltamDias     int     `json:"faltamDias"`
	Progresso      int     `json:"progresso"`
	HorasTotal     float64 `json:"horasTotal"`
	HorasAlvo      float64 `json:"horasAlvo"`
	AcertoPct      *int    `json:"acertoPct"`
	TotalDias      int     `json:"totalDias"`
	DiasConcluidos int     `json:"diasConcluidos"`
	VoltasRevisao  float64 `json:"voltasRevisao"`
}

type alertaDTO struct {
	Nivel  string `json:"nivel"`
	Titulo string `json:"titulo"`
	Texto  string `json:"texto"`
}

// rotuloDoDia nomeia o dia na tela. Única tabela desses nomes no sistema.
func rotuloDoDia(tipo string) string {
	switch tipo {
	case "sim":
		return "SIMULADO"
	case "disc":
		return "DISCURSIVA"
	case "vespera":
		return "VÉSPERA"
	case "rev":
		return "REVISÃO — RESOLUÇÃO DE QUESTÕES"
	case "revd":
		return "REVISÃO DIRIGIDA"
	default:
		return ""
	}
}

func planoParaDTO(p service.PlanoMontado) planoDTO {
	dias := make([]diaDTO, 0, len(p.Dias))

	for _, d := range p.Dias {
		dd := diaDTO{
			N:         d.N,
			Data:      d.Data,
			Semana:    d.Semana,
			Fase:      d.Fase,
			Tipo:      d.Tipo,
			Tema:      d.Tema,
			Meta:      d.Meta,
			Rotulo:    rotuloDoDia(d.Tipo),
			Concluido: d.Concluido,
			Horas:     d.Horas,
			Questoes:  d.Questoes,
			Acertos:   d.Acertos,
			Nota:      d.Nota,
			Itens:     make([]atividadeDTO, 0, len(d.Itens)),
			Blocos:    make([]blocoDTO, 0, len(d.Blocos)),
		}

		for _, it := range d.Itens {
			dd.Itens = append(dd.Itens, atividadeDTO{
				ID:         it.ID,
				Disciplina: it.Disciplina,
				Tema:       it.Tema,
				Passada:    it.Passada,
				Movida:     it.Movida,
				Horas:      it.Horas,
				Questoes:   it.Questoes,
				Acertos:    it.Acertos,
				Erros:      it.Erros,
				Nota:       it.Nota,
				Concluido:  it.Concluido,
			})
		}

		for _, b := range d.Blocos {
			dd.Blocos = append(dd.Blocos, blocoDTO{
				Minutos: b.Minutos, Titulo: b.Titulo, Detalhe: b.Detalhe,
			})
		}

		if d.Revisao != nil {
			dd.Revisao = &revisaoDTO{
				Disciplina: d.Revisao.Disciplina,
				Questoes:   d.Revisao.Questoes,
				Acertos:    d.Revisao.Acertos,
				Observacao: d.Revisao.Observacao,
			}
		}

		dias = append(dias, dd)
	}

	marcos := make([]marcoDTO, 0, len(p.Marcos))
	for _, m := range p.Marcos {
		marcos = append(marcos, marcoDTO{
			ID:         m.ID,
			Rotulo:     m.Rotulo,
			DataInicio: m.DataInicio,
			DataFim:    m.DataFim,
			Titulo:     m.Titulo,
			ExigeAcao:  m.ExigeAcao,
			EProva:     m.EProva,
			Cumprido:   m.Cumprido,
		})
	}

	alertas := make([]alertaDTO, 0, len(p.Alertas))
	for _, a := range p.Alertas {
		alertas = append(alertas, alertaDTO{Nivel: a.Nivel, Titulo: a.Titulo, Texto: a.Texto})
	}

	return planoDTO{
		Concurso:      concursoDoPlanoParaDTO(p.Concurso),
		Config:        configParaDTO(p.Config),
		Dias:          dias,
		Marcos:        marcos,
		Balanceamento: balanceamentoParaDTO(p.Balanceamento),
		Props: propsDTO{
			FaltamDias:     p.Props.FaltamDias,
			Progresso:      p.Props.Progresso,
			HorasTotal:     p.Props.HorasTotal,
			HorasAlvo:      p.Props.HorasAlvo,
			AcertoPct:      p.Props.AcertoPct,
			TotalDias:      p.Props.TotalDias,
			DiasConcluidos: p.Props.DiasConcluidos,
			VoltasRevisao:  p.Props.VoltasRevisao,
		},
		Alertas:               alertas,
		HojeIndex:             p.HojeIndex,
		TemMovimentacaoManual: p.TemMovimentacaoManual,
		GeradoEm:              p.GeradoEm,
	}
}

func concursoDoPlanoParaDTO(c service.ConcursoDoPlano) concursoDoPlanoDTO {
	discs := make([]disciplinaDTO, 0, len(c.Disciplinas))

	for _, d := range c.Disciplinas {
		fontes := make([]fonteDTO, 0, len(d.Fontes))
		for _, f := range d.Fontes {
			fontes = append(fontes, fonteDTO{Titulo: f.Titulo, URL: f.URL, Tipo: f.Tipo})
		}

		discs = append(discs, disciplinaDTO{
			Codigo:     d.Codigo,
			Nome:       d.Nome,
			Bloco:      d.Bloco,
			Peso:       d.Peso,
			Cor:        d.Cor,
			CadernoURL: d.CadernoURL,
			Temas:      d.Temas,
			Fontes:     fontes,
		})
	}

	conteudo := make([]conteudoItemDTO, 0, len(c.Conteudo))
	for _, it := range c.Conteudo {
		conteudo = append(conteudo, conteudoItemDTO{Tipo: it.Tipo, Texto: it.Texto})
	}

	return concursoDoPlanoDTO{
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

func configParaDTO(c service.ConfigDoPlano) configDTO {
	ciclo := make([]itemCicloDTO, 0, len(c.CicloRevisao))
	for _, it := range c.CicloRevisao {
		ciclo = append(ciclo, itemCicloDTO{Titulo: it.Titulo, Questoes: it.Questoes})
	}

	return configDTO{
		Inicio:         c.Inicio,
		Prova:          c.Prova,
		HorasDia:       c.HorasDia,
		DiasEstudo:     c.DiasEstudo,
		DiaRevisao:     c.DiaRevisao,
		RetaFinalDias:  c.RetaFinalDias,
		TemaUI:         c.TemaUI,
		Questoes:       c.Questoes,
		BlocosPorDia:   c.BlocosPorDia,
		MinutosBloco:   c.MinutosBloco,
		MinutosRevisao: c.MinutosRevisao,
		Reforcos:       c.Reforcos,
		CicloRevisao:   ciclo,
		RevisaoSemanal: c.RevisaoSemanal,
		Simulados:      c.Simulados,
		Discursiva:     c.Discursiva,
		Modos:          c.Modos,
		PctQuestoes:    c.PctQuestoes,
		LimiarFraco:    c.LimiarFraco,
	}
}

func balanceamentoParaDTO(linhas []service.LinhaBalanceamento) []linhaBalanceamento {
	out := make([]linhaBalanceamento, 0, len(linhas))

	for _, l := range linhas {
		out = append(out, linhaBalanceamento{
			Codigo:         l.Codigo,
			Nome:           l.Nome,
			Bloco:          l.Bloco,
			Cor:            l.Cor,
			Questoes:       l.Questoes,
			QuestoesEdital: l.QuestoesEdital,
			Delta:          l.Delta,
			Modo:           l.Modo,
			Peso:           l.Peso,
			Pontos:         l.Pontos,
			PctIdeal:       l.PctIdeal,
			BlocosConteudo: l.BlocosConteudo,
			BlocosReta:     l.BlocosReta,
			Temas:          l.Temas,
			Passadas:       l.Passadas,
			Visitas:        l.Visitas,
			RevisoesGerais: l.RevisoesGerais,
			IntervaloDias:  l.IntervaloDias,
			HorasPrevisto:  l.HorasPrevisto,
			HorasLancado:   l.HorasLancado,
			Desvio:         l.Desvio,
			AcertoPct:      l.AcertoPct,
		})
	}

	return out
}
