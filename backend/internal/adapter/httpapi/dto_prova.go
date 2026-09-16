package httpapi

import (
	"time"

	"studygo/internal/domain/prova"
	"studygo/internal/service"
)

// Contrato de transporte do catálogo de provas e da curadoria.
//
// O rascunho vai e volta: o curador recebe, edita no navegador e manda de volta
// inteiro. Por isso os tipos de conteúdo (bloco, questão, apoio) servem tanto à
// resposta quanto ao pedido. Listas saem sempre como [] — o frontend não precisa
// distinguir vazio de ausente.

type origemDTO struct {
	Pagina    int       `json:"pagina"`
	Retangulo []float64 `json:"retangulo"`
	Regiao    string    `json:"regiao"`
}

type blocoProvaDTO struct {
	Tipo      string     `json:"tipo"`
	Texto     string     `json:"texto"`
	Formato   string     `json:"formato"`
	Arquivo   string     `json:"arquivo"`
	Descricao string     `json:"descricao"`
	Origem    *origemDTO `json:"origem"`
	Revisado  bool       `json:"revisado"`
	// Largura da figura na tela, em % da coluna; 0 é o tamanho natural.
	Largura int `json:"largura"`
}

type alternativaDTO struct {
	Letra  string          `json:"letra"`
	Blocos []blocoProvaDTO `json:"blocos"`
}

type questaoDTO struct {
	Numero       int              `json:"numero"`
	Disciplina   string           `json:"disciplina"`
	Blocos       []blocoProvaDTO  `json:"blocos"`
	Alternativas []alternativaDTO `json:"alternativas"`
	Apoios       []string         `json:"apoios"`
	Origens      []origemDTO      `json:"origens"`
	Resposta     string           `json:"resposta"`
	Situacao     string           `json:"situacao"`
	Revisada     bool             `json:"revisada"`
	Completa     bool             `json:"completa"`
	// IgualA: de qual prova publicada o conteúdo foi reaproveitado.
	IgualA string `json:"igualA"`
}

type apoioDTO struct {
	ID       string          `json:"id"`
	Blocos   []blocoProvaDTO `json:"blocos"`
	Questoes []int           `json:"questoes"`
	Aviso    string          `json:"aviso"`
	Origens  []origemDTO     `json:"origens"`
	Revisado bool            `json:"revisado"`
	IgualA   string          `json:"igualA"`
}

type gabaritoDTO struct {
	Cargo     string            `json:"cargo"`
	Caderno   string            `json:"caderno"`
	Tipo      string            `json:"tipo"`
	Respostas map[string]string `json:"respostas"`
	Situacoes map[string]string `json:"situacoes"`
}

type extracaoDTO struct {
	Modelo        string `json:"modelo"`
	TokensEntrada int    `json:"tokensEntrada"`
	TokensSaida   int    `json:"tokensSaida"`
	Regiao        string `json:"regiao"`
	Prompt        string `json:"prompt"`
	Versao        string `json:"versao"`
}

// rascunhoDTO é o conteúdo revisável. `extracoes` só sai: no pedido ela é
// aceita e ignorada, porque o registro das chamadas não é do curador.
type rascunhoDTO struct {
	Banca     string        `json:"banca"`
	Orgao     string        `json:"orgao"`
	Ano       int           `json:"ano"`
	Cargo     string        `json:"cargo"`
	CargoNome string        `json:"cargoNome"`
	Caderno   string        `json:"caderno"`
	Total     int           `json:"total"`
	Questoes  []questaoDTO  `json:"questoes"`
	Apoios    []apoioDTO    `json:"apoios"`
	Gabarito  gabaritoDTO   `json:"gabarito"`
	Alertas   []string      `json:"alertas"`
	Extracoes []extracaoDTO `json:"extracoes"`
	// AnuladasExcluidas são as questões anuladas que o curador tirou da prova.
	AnuladasExcluidas []int `json:"anuladasExcluidas"`
}

type importacaoProvaDTO struct {
	ID          string `json:"id"`
	Estado      string `json:"estado"`
	Versao      int    `json:"versao"`
	Etapa       int    `json:"etapa"`
	TotalEtapas int    `json:"totalEtapas"`
	Erro        string `json:"erro"`
	ProvaID     string `json:"provaId"`
	// Os nomes com que o curador enviou os PDFs; vazios nas anteriores a eles.
	NomeDocumento string      `json:"nomeDocumento"`
	NomeGabarito  string      `json:"nomeGabarito"`
	Regioes       []origemDTO `json:"regioes"`
	Rascunho      rascunhoDTO `json:"rascunho"`
	Pendencias    []string    `json:"pendencias"`
	// ConferenciaObrigatoria diz se a falta de conferência entra nas
	// pendências neste ambiente (PROVAS_EXIGIR_CONFERENCIA).
	ConferenciaObrigatoria bool      `json:"conferenciaObrigatoria"`
	CriadoEm               time.Time `json:"criadoEm"`
	AtualizadoEm           time.Time `json:"atualizadoEm"`
}

type importacaoResumoDTO struct {
	ID            string    `json:"id"`
	Estado        string    `json:"estado"`
	Etapa         int       `json:"etapa"`
	TotalEtapas   int       `json:"totalEtapas"`
	Erro          string    `json:"erro"`
	ProvaID       string    `json:"provaId"`
	NomeDocumento string    `json:"nomeDocumento"`
	NomeGabarito  string    `json:"nomeGabarito"`
	Orgao         string    `json:"orgao"`
	Ano           int       `json:"ano"`
	Cargo         string    `json:"cargo"`
	CargoNome     string    `json:"cargoNome"`
	Caderno       string    `json:"caderno"`
	CriadoEm      time.Time `json:"criadoEm"`
	AtualizadoEm  time.Time `json:"atualizadoEm"`
}

type provaResumoDTO struct {
	ID           string    `json:"id"`
	Revisao      int       `json:"revisao"`
	Banca        string    `json:"banca"`
	Orgao        string    `json:"orgao"`
	Ano          int       `json:"ano"`
	Cargo        string    `json:"cargo"`
	CargoNome    string    `json:"cargoNome"`
	Caderno      string    `json:"caderno"`
	Total        int       `json:"total"`
	GabaritoTipo string    `json:"gabaritoTipo"`
	PublicadoEm  time.Time `json:"publicadoEm"`
}

type provaDTO struct {
	provaResumoDTO
	Questoes []questaoDTO `json:"questoes"`
	Apoios   []apoioDTO   `json:"apoios"`
}

// questaoAvulsaDTO é uma questão do catálogo sem o conteúdo: o bastante para
// filtrar e para saber se o estudante acertou. O conteúdo vem da prova, em
// GET /api/provas/{id}.
type questaoAvulsaDTO struct {
	ProvaID    string `json:"provaId"`
	Numero     int    `json:"numero"`
	Disciplina string `json:"disciplina"`
	Resposta   string `json:"resposta"`
	Orgao      string `json:"orgao"`
	Ano        int    `json:"ano"`
	Cargo      string `json:"cargo"`
	CargoNome  string `json:"cargoNome"`
}

type importacaoEdicaoRequest struct {
	Versao   int         `json:"versao"`
	Rascunho rascunhoDTO `json:"rascunho"`
}

type materiaSugeridaDTO struct {
	Numero  int    `json:"numero"`
	Materia string `json:"materia"`
}

type versaoRequest struct {
	Versao int `json:"versao"`
}

type recorteRequest struct {
	Versao int       `json:"versao"`
	Origem origemDTO `json:"origem"`
}

// trechoRequest é o retângulo que o curador marcou em volta da questão.
type trechoRequest struct {
	Versao  int       `json:"versao"`
	Questao int       `json:"questao"`
	Origem  origemDTO `json:"origem"`
}

// --- domínio → transporte ---------------------------------------------------

func origemParaDTO(o prova.Origem) origemDTO {
	return origemDTO{Pagina: o.Pagina, Retangulo: naoNula(o.Retangulo), Regiao: o.Regiao}
}

func origensParaDTO(os []prova.Origem) []origemDTO {
	out := make([]origemDTO, 0, len(os))
	for _, o := range os {
		out = append(out, origemParaDTO(o))
	}

	return out
}

func blocosParaDTO(bs []prova.Bloco) []blocoProvaDTO {
	out := make([]blocoProvaDTO, 0, len(bs))
	for _, b := range bs {
		d := blocoProvaDTO{
			Tipo: b.Tipo, Texto: b.Texto, Formato: b.Formato, Arquivo: b.Arquivo,
			Descricao: b.Descricao, Revisado: b.Revisado, Largura: b.Largura,
		}
		if b.Origem != nil {
			o := origemParaDTO(*b.Origem)
			d.Origem = &o
		}
		out = append(out, d)
	}

	return out
}

func questaoParaDTO(q prova.Questao) questaoDTO {
	alternativas := make([]alternativaDTO, 0, len(q.Alternativas))
	for _, a := range q.Alternativas {
		alternativas = append(alternativas, alternativaDTO{Letra: a.Letra, Blocos: blocosParaDTO(a.Blocos)})
	}

	return questaoDTO{
		Numero: q.Numero, Disciplina: q.Disciplina, Blocos: blocosParaDTO(q.Blocos),
		Alternativas: alternativas, Apoios: naoNula(q.Apoios), Origens: origensParaDTO(q.Origens),
		Resposta: q.Resposta, Situacao: q.Situacao, Revisada: q.Revisada, Completa: q.Completa,
		IgualA: q.IgualA,
	}
}

func questoesParaDTO(qs []prova.Questao) []questaoDTO {
	out := make([]questaoDTO, 0, len(qs))
	for _, q := range qs {
		out = append(out, questaoParaDTO(q))
	}

	return out
}

func apoiosParaDTO(as []prova.Apoio) []apoioDTO {
	out := make([]apoioDTO, 0, len(as))
	for _, a := range as {
		out = append(out, apoioDTO{
			ID: a.ID, Blocos: blocosParaDTO(a.Blocos), Questoes: naoNula(a.Questoes),
			Aviso: a.Aviso, Origens: origensParaDTO(a.Origens), Revisado: a.Revisado, IgualA: a.IgualA,
		})
	}

	return out
}

func gabaritoParaDTO(g prova.Gabarito) gabaritoDTO {
	return gabaritoDTO{
		Cargo: g.Cargo, Caderno: g.Caderno, Tipo: g.Tipo,
		Respostas: mapaNaoNulo(g.Respostas), Situacoes: mapaNaoNulo(g.Situacoes),
	}
}

func rascunhoParaDTO(r prova.Rascunho) rascunhoDTO {
	extracoes := make([]extracaoDTO, 0, len(r.Extracoes))
	for _, e := range r.Extracoes {
		extracoes = append(extracoes, extracaoDTO{
			Modelo: e.Modelo, TokensEntrada: e.TokensEntrada, TokensSaida: e.TokensSaida,
			Regiao: e.Regiao, Prompt: e.Prompt, Versao: e.Versao,
		})
	}

	return rascunhoDTO{
		Banca: r.Banca, Orgao: r.Orgao, Ano: r.Ano, Cargo: r.Cargo, CargoNome: r.CargoNome, Caderno: r.Caderno,
		Total: r.Total, Questoes: questoesParaDTO(r.Questoes), Apoios: apoiosParaDTO(r.Apoios),
		Gabarito: gabaritoParaDTO(r.Gabarito), Alertas: naoNula(r.Alertas), Extracoes: extracoes,
		AnuladasExcluidas: naoNula(r.AnuladasExcluidas),
	}
}

func importacaoProvaParaDTO(i service.ImportacaoDeProva) importacaoProvaDTO {
	return importacaoProvaDTO{
		ID: i.ID, Estado: i.Estado, Versao: i.Versao, Etapa: i.Etapa, TotalEtapas: i.TotalEtapas,
		Erro: i.Erro, ProvaID: i.ProvaID, NomeDocumento: i.NomeDocumento, NomeGabarito: i.NomeGabarito,
		Regioes:  origensParaDTO(i.Regioes),
		Rascunho: rascunhoParaDTO(i.Rascunho), Pendencias: naoNula(i.Pendencias),
		ConferenciaObrigatoria: i.ConferenciaObrigatoria,
		CriadoEm:               i.CriadoEm, AtualizadoEm: i.AtualizadoEm,
	}
}

func importacaoResumoParaDTO(i prova.Importacao) importacaoResumoDTO {
	return importacaoResumoDTO{
		ID: i.ID, Estado: i.Estado, Etapa: i.Etapa, TotalEtapas: prova.TotalEtapas(len(i.Regioes)),
		Erro: i.Erro, ProvaID: i.ProvaID, NomeDocumento: i.NomeDocumento, NomeGabarito: i.NomeGabarito,
		Orgao: i.Rascunho.Orgao, Ano: i.Rascunho.Ano,
		Cargo: i.Rascunho.Cargo, CargoNome: i.Rascunho.CargoNome, Caderno: i.Rascunho.Caderno,
		CriadoEm: i.CriadoEm, AtualizadoEm: i.AtualizadoEm,
	}
}

func provaResumoParaDTO(p prova.Publicacao) provaResumoDTO {
	c := p.Conteudo

	return provaResumoDTO{
		ID: p.ID, Revisao: p.Revisao, Banca: c.Banca, Orgao: c.Orgao, Ano: c.Ano, Cargo: c.Cargo,
		CargoNome: c.CargoNome, Caderno: c.Caderno, Total: c.QuestoesNaProva(), GabaritoTipo: c.Gabarito.Tipo,
		PublicadoEm: p.PublicadoEm,
	}
}

func questaoAvulsaParaDTO(q prova.QuestaoAvulsa) questaoAvulsaDTO {
	return questaoAvulsaDTO{
		ProvaID: q.ProvaID, Numero: q.Numero, Disciplina: q.Disciplina, Resposta: q.Resposta,
		Orgao: q.Orgao, Ano: q.Ano, Cargo: q.Cargo, CargoNome: q.CargoNome,
	}
}

func provaParaDTO(p prova.Publicacao) provaDTO {
	return provaDTO{
		provaResumoDTO: provaResumoParaDTO(p),
		Questoes:       questoesParaDTO(p.Conteudo.Questoes),
		Apoios:         apoiosParaDTO(p.Conteudo.Apoios),
	}
}

// --- transporte → domínio ---------------------------------------------------

func origemDoDTO(d origemDTO) prova.Origem {
	return prova.Origem{Pagina: d.Pagina, Retangulo: d.Retangulo, Regiao: d.Regiao}
}

func blocosDoDTO(ds []blocoProvaDTO) []prova.Bloco {
	out := make([]prova.Bloco, 0, len(ds))
	for _, d := range ds {
		b := prova.Bloco{
			Tipo: d.Tipo, Texto: d.Texto, Formato: d.Formato, Arquivo: d.Arquivo,
			Descricao: d.Descricao, Revisado: d.Revisado, Largura: d.Largura,
		}
		if d.Origem != nil {
			o := origemDoDTO(*d.Origem)
			b.Origem = &o
		}
		out = append(out, b)
	}

	return out
}

func rascunhoDoDTO(d rascunhoDTO) prova.Rascunho {
	r := prova.Rascunho{
		Banca: d.Banca, Orgao: d.Orgao, Ano: d.Ano, Cargo: d.Cargo, CargoNome: d.CargoNome, Caderno: d.Caderno,
		Total: d.Total,
		Gabarito: prova.Gabarito{
			Cargo: d.Gabarito.Cargo, Caderno: d.Gabarito.Caderno, Tipo: d.Gabarito.Tipo,
			Respostas: d.Gabarito.Respostas, Situacoes: d.Gabarito.Situacoes,
		},
		Alertas:           d.Alertas,
		AnuladasExcluidas: d.AnuladasExcluidas,
	}
	for _, q := range d.Questoes {
		alternativas := make([]prova.Alternativa, 0, len(q.Alternativas))
		for _, a := range q.Alternativas {
			alternativas = append(alternativas, prova.Alternativa{Letra: a.Letra, Blocos: blocosDoDTO(a.Blocos)})
		}
		origens := make([]prova.Origem, 0, len(q.Origens))
		for _, o := range q.Origens {
			origens = append(origens, origemDoDTO(o))
		}
		r.Questoes = append(r.Questoes, prova.Questao{
			Numero: q.Numero, Disciplina: q.Disciplina, Blocos: blocosDoDTO(q.Blocos),
			Alternativas: alternativas, Apoios: q.Apoios, Origens: origens,
			Resposta: q.Resposta, Situacao: q.Situacao, Revisada: q.Revisada, Completa: q.Completa,
			IgualA: q.IgualA,
		})
	}
	for _, a := range d.Apoios {
		origens := make([]prova.Origem, 0, len(a.Origens))
		for _, o := range a.Origens {
			origens = append(origens, origemDoDTO(o))
		}
		r.Apoios = append(r.Apoios, prova.Apoio{
			ID: a.ID, Blocos: blocosDoDTO(a.Blocos), Questoes: a.Questoes,
			Aviso: a.Aviso, Origens: origens, Revisado: a.Revisado, IgualA: a.IgualA,
		})
	}

	return r
}

func naoNula[T any](s []T) []T {
	if s == nil {
		return []T{}
	}

	return s
}

func mapaNaoNulo(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}

	return m
}

// anotacaoDTO é a anotação do estudante numa questão, em markdown.
type anotacaoDeQuestaoDTO struct {
	Numero       int       `json:"numero"`
	Texto        string    `json:"texto"`
	AtualizadaEm time.Time `json:"atualizadaEm"`
}

type anotacaoDeQuestaoRequest struct {
	Texto string `json:"texto"`
}

func anotacaoDeQuestaoParaDTO(a prova.Anotacao) anotacaoDeQuestaoDTO {
	return anotacaoDeQuestaoDTO{Numero: a.Numero, Texto: a.Texto, AtualizadaEm: a.AtualizadaEm}
}
