package httpapi

import (
	"errors"
	"time"

	"studygo/internal/domain/lei"
	"studygo/internal/service"
)

// leiDTO é a lei no leitor e no resultado da publicação.
type leiDTO struct {
	Slug       string   `json:"slug"`
	Nome       string   `json:"nome"`
	Curto      string   `json:"curto"`
	Reconhecer []string `json:"reconhecer"`
	Fonte      string   `json:"fonte"`
}

type dispositivoDTO struct {
	Ref        string   `json:"ref"`
	Pai        *string  `json:"pai"`
	Tipo       string   `json:"tipo"`
	Rotulo     string   `json:"rotulo"`
	Nome       string   `json:"nome"`
	Texto      string   `json:"texto"`
	Notas      []string `json:"notas"`
	Anteriores []string `json:"anteriores"`
	Revogado   bool     `json:"revogado"`
}

type unidadeDTO struct {
	Ref          string   `json:"ref"`
	Titulo       string   `json:"titulo"`
	Dispositivos []string `json:"dispositivos"`
	// Vazio numa unidade nova: a importação preenche com o do texto ativo.
	Hash string `json:"hash"`
}

type questaoDoArquivoDTO struct {
	ID           string   `json:"id"`
	Unidade      string   `json:"unidade"`
	Enunciado    string   `json:"enunciado"`
	Alternativas []string `json:"alternativas"`
	Gabarito     string   `json:"gabarito"`
	Comentario   string   `json:"comentario"`
	Dispositivos []string `json:"dispositivos"`
	Trecho       string   `json:"trecho"`
}

// questoesDeLeiDTO é o arquivo que quem escreve as questões mantém
// (conteudo/leis/<slug>/questoes.json) e importa na página da lei.
type questoesDeLeiDTO struct {
	Unidades []unidadeDTO          `json:"unidades"`
	Questoes []questaoDoArquivoDTO `json:"questoes"`
}

func dispositivoParaDTO(d lei.Dispositivo) dispositivoDTO {
	var pai *string
	if d.Pai != "" {
		pai = &d.Pai
	}

	return dispositivoDTO{
		Ref: d.Ref, Pai: pai, Tipo: d.Tipo, Rotulo: d.Rotulo, Nome: d.Nome, Texto: d.Texto,
		Notas: naoNula(d.Notas), Anteriores: naoNula(d.Anteriores), Revogado: d.Revogado,
	}
}

func unidadeParaDTO(u lei.Unidade) unidadeDTO {
	return unidadeDTO{Ref: u.Ref, Titulo: u.Titulo, Dispositivos: naoNula(u.Dispositivos), Hash: u.Hash}
}

func questoesDoDTO(d questoesDeLeiDTO) ([]lei.Unidade, []lei.Questao) {
	var (
		us []lei.Unidade
		qs []lei.Questao
	)
	for _, u := range d.Unidades {
		us = append(us, lei.Unidade{Ref: u.Ref, Titulo: u.Titulo, Dispositivos: u.Dispositivos, Hash: u.Hash})
	}
	for _, q := range d.Questoes {
		qs = append(qs, lei.Questao{
			Chave: q.ID, Unidade: q.Unidade, Enunciado: q.Enunciado, Alternativas: q.Alternativas,
			Gabarito: q.Gabarito, Comentario: q.Comentario, Dispositivos: q.Dispositivos, Trecho: q.Trecho,
		})
	}

	return us, qs
}

func naoNula(s []string) []string {
	if s == nil {
		return []string{}
	}

	return s
}

// ---------------------------------------------------------------- captura

type capturaRequest struct {
	Link string `json:"link"`
}

type avisoDTO struct {
	ID     string `json:"id"`
	Texto  string `json:"texto"`
	Trecho string `json:"trecho"`
}

type sumarioDTO struct {
	Ref    string `json:"ref"`
	Tipo   string `json:"tipo"`
	Rotulo string `json:"rotulo"`
	Nome   string `json:"nome"`
}

type resumoCapturaDTO struct {
	Vigentes    int            `json:"vigentes"`
	Anteriores  int            `json:"anteriores"`
	Notas       int            `json:"notas"`
	Revogados   int            `json:"revogados"`
	Tipos       map[string]int `json:"tipos"`
	Descartados []string       `json:"descartados"`
	Riscados    []string       `json:"riscados"`
	Juncoes     []string       `json:"juncoes"`
}

// resultadoCapturaDTO é a prévia: o que revisar e a estrutura da lei. O texto
// inteiro não vai ao navegador — quem publica o busca de novo no processador.
type resultadoCapturaDTO struct {
	Fonte        string           `json:"fonte"`
	Gemini       bool             `json:"gemini"`
	Versao       string           `json:"versao"`
	Publicavel   bool             `json:"publicavel"`
	Bloqueios    []string         `json:"bloqueios"`
	Avisos       []avisoDTO       `json:"avisos"`
	Resumo       resumoCapturaDTO `json:"resumo"`
	Sumario      []sumarioDTO     `json:"sumario"`
	Artigos      int              `json:"artigos"`
	Dispositivos int              `json:"dispositivos"`
}

type sugestaoDoEditalDTO struct {
	DisciplinaID string      `json:"disciplinaId"`
	Materia      string      `json:"materia"`
	Temas        []string    `json:"temas"`
	Recorte      []string    `json:"recorte"`
	Trechos      []trechoDTO `json:"trechos"`
}

type capturaDTO struct {
	ID string `json:"id"`
	// Epigrafe é a primeira linha da lei, para sugerir o nome.
	Epigrafe string `json:"epigrafe"`
	// Edital: as matérias do concurso ativo cujo tópico cita a lei, e o que
	// cada uma pede dela.
	Edital    []sugestaoDoEditalDTO `json:"edital"`
	Estado    string                `json:"estado"`
	Etapa     string                `json:"etapa"`
	Progresso struct {
		Feitos int `json:"feitos"`
		Total  int `json:"total"`
	} `json:"progresso"`
	Erro      string               `json:"erro,omitempty"`
	Resultado *resultadoCapturaDTO `json:"resultado"`
}

// Os agrupamentos que formam o sumário da prévia.
var agrupamentos = map[string]bool{
	"parte": true, "livro": true, "titulo": true, "capitulo": true, "secao": true, "subsecao": true,
}

func capturaParaDTO(ce service.CapturaComEdital) capturaDTO {
	c := ce.Captura
	d := capturaDTO{ID: c.ID, Epigrafe: ce.Epigrafe, Edital: []sugestaoDoEditalDTO{}, Estado: c.Estado, Etapa: c.Etapa, Erro: c.Erro}
	for _, s := range ce.Edital {
		d.Edital = append(d.Edital, sugestaoDoEditalDTO{
			DisciplinaID: s.DisciplinaID.String(), Materia: s.Materia, Temas: s.Temas,
			Recorte: naoNula(s.Recorte), Trechos: trechosParaDTO(s.Trechos),
		})
	}
	d.Progresso.Feitos, d.Progresso.Total = c.Feitos, c.Total
	r := c.Resultado
	if r == nil {
		return d
	}

	res := &resultadoCapturaDTO{
		Fonte: r.Fonte, Gemini: r.Gemini, Versao: r.Versao,
		Publicavel: len(r.Bloqueios) == 0 && len(r.Dispositivos) > 0,
		Bloqueios:  naoNula(r.Bloqueios), Avisos: []avisoDTO{}, Sumario: []sumarioDTO{},
		Dispositivos: len(r.Dispositivos),
		Resumo: resumoCapturaDTO{
			Vigentes: r.Resumo.Vigentes, Anteriores: r.Resumo.Anteriores, Notas: r.Resumo.Notas,
			Revogados: r.Resumo.Revogados, Tipos: r.Resumo.Tipos,
			Descartados: naoNula(r.Resumo.Descartados), Riscados: naoNula(r.Resumo.Riscados),
			Juncoes: naoNula(r.Resumo.Juncoes),
		},
	}
	if res.Resumo.Tipos == nil {
		res.Resumo.Tipos = map[string]int{}
	}
	for _, a := range r.Avisos {
		res.Avisos = append(res.Avisos, avisoDTO{ID: a.ID, Texto: a.Texto, Trecho: a.Trecho})
	}
	for _, x := range r.Dispositivos {
		switch {
		case agrupamentos[x.Tipo]:
			res.Sumario = append(res.Sumario, sumarioDTO{Ref: x.Ref, Tipo: x.Tipo, Rotulo: x.Rotulo, Nome: x.Nome})
		case x.Tipo == "artigo":
			res.Artigos++
		}
	}
	d.Resultado = res

	return d
}

type publicacaoRequest struct {
	// Slug vazio publica uma lei nova; preenchido, atualiza o texto daquela.
	Slug       string   `json:"slug"`
	Nome       string   `json:"nome"`
	Curto      string   `json:"curto"`
	Reconhecer []string `json:"reconhecer"`
	// Aceitos são os ids dos avisos que a pessoa marcou como revisados.
	Aceitos []string `json:"aceitos"`
}

type publicacaoDTO struct {
	Slug                   string `json:"slug"`
	Curto                  string `json:"curto"`
	Versao                 string `json:"versao"`
	NovaVersao             bool   `json:"novaVersao"`
	UnidadesDesatualizadas int    `json:"unidadesDesatualizadas"`
}

func publicacaoParaDTO(r service.ResultadoDaPublicacao) publicacaoDTO {
	return publicacaoDTO{
		Slug: r.Slug, Curto: r.Curto, Versao: r.Versao, NovaVersao: r.NovaVersao,
		UnidadesDesatualizadas: r.UnidadesDesatualizadas,
	}
}

type importacaoQuestoesDTO struct {
	Curto       string `json:"curto"`
	Novas       int    `json:"novas"`
	Atualizadas int    `json:"atualizadas"`
	Desativadas int    `json:"desativadas"`
	Mantidas    int    `json:"mantidas"`
}

func importacaoQuestoesParaDTO(r service.ResultadoDaImportacaoDeQuestoes) importacaoQuestoesDTO {
	return importacaoQuestoesDTO{
		Curto: r.Curto, Novas: r.Novas, Atualizadas: r.Atualizadas,
		Desativadas: r.Desativadas, Mantidas: r.Mantidas,
	}
}

// ---------------------------------------------------------------- respostas

type leiResumoDTO struct {
	Slug        string    `json:"slug"`
	Nome        string    `json:"nome"`
	Curto       string    `json:"curto"`
	Fonte       string    `json:"fonte"`
	Versao      string    `json:"versao"`
	Questoes    int       `json:"questoes"`
	ImportadaEm time.Time `json:"importadaEm"`
}

func resumoLeiParaDTO(r lei.Resumo) leiResumoDTO {
	return leiResumoDTO{
		Slug: r.Lei.Slug, Nome: r.Lei.Nome, Curto: r.Lei.Curto, Fonte: r.Lei.Fonte,
		Versao: r.Versao, Questoes: r.Questoes, ImportadaEm: r.ImportadaEm,
	}
}

type correcaoDTO struct {
	Escolhida    string    `json:"escolhida"`
	Acertou      bool      `json:"acertou"`
	Gabarito     string    `json:"gabarito"`
	Comentario   string    `json:"comentario"`
	Trecho       string    `json:"trecho"`
	RespondidaEm time.Time `json:"respondidaEm"`
}

func correcaoParaDTO(c service.Correcao) correcaoDTO {
	return correcaoDTO{
		Escolhida: c.Escolhida, Acertou: c.Acertou, Gabarito: c.Gabarito,
		Comentario: c.Comentario, Trecho: c.Trecho, RespondidaEm: c.Em,
	}
}

type questaoLeitorDTO struct {
	ID           string       `json:"id"`
	Unidade      string       `json:"unidade"`
	Dispositivos []string     `json:"dispositivos"`
	Enunciado    string       `json:"enunciado"`
	Alternativas []string     `json:"alternativas"`
	Resposta     *correcaoDTO `json:"resposta"`
}

type materiaDoRecorteDTO struct {
	DisciplinaID string `json:"disciplinaId"`
	Nome         string `json:"nome"`
}

// recorteNoConcursoDTO: refs vazias são a lei inteira.
type recorteNoConcursoDTO struct {
	Refs     []string              `json:"refs"`
	Trechos  []trechoDTO           `json:"trechos"`
	Materias []materiaDoRecorteDTO `json:"materias"`
}

type leituraLeiDTO struct {
	Lei          leiDTO             `json:"lei"`
	Versao       string             `json:"versao"`
	Dispositivos []dispositivoDTO   `json:"dispositivos"`
	Unidades     []unidadeDTO       `json:"unidades"`
	Questoes     []questaoLeitorDTO `json:"questoes"`
	// Recorte do concurso ativo; null quando nenhuma matéria dele cobra a lei.
	Recorte *recorteNoConcursoDTO `json:"recorte"`
}

func leituraParaDTO(l service.LeituraDaLei) leituraLeiDTO {
	d := leituraLeiDTO{
		Lei: leiDTO{
			Slug: l.Lei.Slug, Nome: l.Lei.Nome, Curto: l.Lei.Curto,
			Reconhecer: naoNula(l.Lei.Reconhecer), Fonte: l.Lei.Fonte,
		},
		Versao:       l.Texto.Versao,
		Dispositivos: make([]dispositivoDTO, 0, len(l.Texto.Dispositivos)),
		Unidades:     make([]unidadeDTO, 0, len(l.Texto.Unidades)),
		Questoes:     make([]questaoLeitorDTO, 0, len(l.Questoes)),
	}
	for _, x := range l.Texto.Dispositivos {
		d.Dispositivos = append(d.Dispositivos, dispositivoParaDTO(x))
	}
	for _, u := range l.Texto.Unidades {
		d.Unidades = append(d.Unidades, unidadeParaDTO(u))
	}
	for _, q := range l.Questoes {
		x := questaoLeitorDTO{
			ID: q.ID.String(), Unidade: q.Unidade, Dispositivos: naoNula(q.Dispositivos),
			Enunciado: q.Enunciado, Alternativas: naoNula(q.Alternativas),
		}
		if q.Resposta != nil {
			c := correcaoParaDTO(*q.Resposta)
			x.Resposta = &c
		}
		d.Questoes = append(d.Questoes, x)
	}

	if r := l.Recorte; r != nil {
		d.Recorte = &recorteNoConcursoDTO{Refs: naoNula(r.Refs), Trechos: trechosParaDTO(r.Trechos), Materias: []materiaDoRecorteDTO{}}
		for _, m := range r.Materias {
			d.Recorte.Materias = append(d.Recorte.Materias, materiaDoRecorteDTO{DisciplinaID: m.DisciplinaID.String(), Nome: m.Nome})
		}
	}

	return d
}

// trechoDTO é uma raiz do recorte do edital: "Seção IX — DA FISCALIZAÇÃO…
// (arts. 70 a 75)". Lista vazia de trechos é a lei inteira.
type trechoDTO struct {
	Ref     string `json:"ref"`
	Rotulo  string `json:"rotulo"`
	Nome    string `json:"nome"`
	Artigos string `json:"artigos"`
}

func trechosParaDTO(ts []lei.TrechoDoRecorte) []trechoDTO {
	out := make([]trechoDTO, 0, len(ts))
	for _, t := range ts {
		out = append(out, trechoDTO{Ref: t.Ref, Rotulo: t.Rotulo, Nome: t.Nome, Artigos: t.Artigos})
	}

	return out
}

type leiNaMateriaDTO struct {
	leiResumoDTO
	Recorte []trechoDTO `json:"recorte"`
}

type leisDaMateriaDTO struct {
	DisciplinaID string            `json:"disciplinaId"`
	Codigo       string            `json:"codigo"`
	Nome         string            `json:"nome"`
	Vinculadas   []leiNaMateriaDTO `json:"vinculadas"`
	Sugeridas    []leiNaMateriaDTO `json:"sugeridas"`
}

func leisDaMateriaParaDTO(m service.LeisDaMateria) leisDaMateriaDTO {
	d := leisDaMateriaDTO{
		DisciplinaID: m.DisciplinaID.String(), Codigo: m.Codigo, Nome: m.Nome,
		Vinculadas: []leiNaMateriaDTO{}, Sugeridas: []leiNaMateriaDTO{},
	}
	for _, r := range m.Vinculadas {
		d.Vinculadas = append(d.Vinculadas, leiNaMateriaDTO{resumoLeiParaDTO(r.Resumo), trechosParaDTO(r.Recorte)})
	}
	for _, r := range m.Sugeridas {
		d.Sugeridas = append(d.Sugeridas, leiNaMateriaDTO{resumoLeiParaDTO(r.Resumo), trechosParaDTO(r.Recorte)})
	}

	return d
}

// vinculoRequest é opcional: sem corpo, o recorte sai dos tópicos da matéria.
type vinculoRequest struct {
	Recorte *[]string `json:"recorte"`
}

var errQuestoesIlegiveis = errors.New("o arquivo não é um questoes.json de lei: esperava {\"unidades\": [...], \"questoes\": [...]}")
