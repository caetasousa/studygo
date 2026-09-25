package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"studygo/internal/domain/lei"
	"studygo/internal/service"
)

// O pacote de lei ("studygo.lei/1") é contrato em duas pontas: a captura do
// edital-processor e o cmd/leis escrevem, a importação lê. Os tipos ficam aqui
// porque aqui ficam as tags JSON; o cmd/leis usa LerPacoteLei e companhia em
// vez de repetir o formato.

type pacoteLeiDTO struct {
	Formato string       `json:"formato"`
	Lei     leiPacoteDTO `json:"lei"`
	Versao  string       `json:"versao"`
	// Captura é o relatório de onde o texto veio (hash do original, se o
	// Gemini conferiu). Informativo: a importação não o usa.
	Captura      json.RawMessage    `json:"captura,omitempty"`
	Dispositivos []dispositivoDTO   `json:"dispositivos"`
	Unidades     []unidadeDTO       `json:"unidades"`
	Questoes     []questaoPacoteDTO `json:"questoes"`
}

type leiPacoteDTO struct {
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
	Hash         string   `json:"hash"`
}

type questaoPacoteDTO struct {
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
// (conteudo/leis/<slug>/questoes.json): as unidades e as questões, sem a lei.
type questoesDeLeiDTO struct {
	Unidades []unidadeDTO       `json:"unidades"`
	Questoes []questaoPacoteDTO `json:"questoes"`
}

func (d pacoteLeiDTO) paraDominio() lei.Pacote {
	p := lei.Pacote{
		Formato: d.Formato,
		Lei: lei.Lei{
			Slug: d.Lei.Slug, Nome: d.Lei.Nome, Curto: d.Lei.Curto,
			Fonte: d.Lei.Fonte, Reconhecer: d.Lei.Reconhecer,
		},
		Versao: d.Versao,
	}
	for _, x := range d.Dispositivos {
		p.Dispositivos = append(p.Dispositivos, dispositivoDoDTO(x))
	}
	p.Unidades, p.Questoes = questoesDoDTO(questoesDeLeiDTO{Unidades: d.Unidades, Questoes: d.Questoes})

	return p
}

func dispositivoDoDTO(x dispositivoDTO) lei.Dispositivo {
	pai := ""
	if x.Pai != nil {
		pai = *x.Pai
	}

	return lei.Dispositivo{
		Ref: x.Ref, Pai: pai, Tipo: x.Tipo, Rotulo: x.Rotulo, Nome: x.Nome, Texto: x.Texto,
		Notas: x.Notas, Anteriores: x.Anteriores, Revogado: x.Revogado,
	}
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

func decodificarEstrito(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("lendo JSON: %w", err)
	}

	return nil
}

// LerPacoteLei lê um pacote (ou o lei.json da captura, que é um pacote sem
// questões).
func LerPacoteLei(r io.Reader) (lei.Pacote, error) {
	var d pacoteLeiDTO
	if err := decodificarEstrito(r, &d); err != nil {
		return lei.Pacote{}, err
	}

	return d.paraDominio(), nil
}

// LerQuestoesLei lê o questoes.json de uma lei.
func LerQuestoesLei(r io.Reader) ([]lei.Unidade, []lei.Questao, error) {
	var d questoesDeLeiDTO
	if err := decodificarEstrito(r, &d); err != nil {
		return nil, nil, err
	}
	us, qs := questoesDoDTO(d)

	return us, qs, nil
}

// EscreverQuestoesLei grava o questoes.json de volta (o cmd/leis preenche os
// hashes das unidades novas).
func EscreverQuestoesLei(w io.Writer, us []lei.Unidade, qs []lei.Questao) error {
	d := questoesDeLeiDTO{Unidades: []unidadeDTO{}, Questoes: []questaoPacoteDTO{}}
	for _, u := range us {
		d.Unidades = append(d.Unidades, unidadeParaDTO(u))
	}
	for _, q := range qs {
		d.Questoes = append(d.Questoes, questaoParaPacote(q))
	}

	return escreverJSON(w, d)
}

func questaoParaPacote(q lei.Questao) questaoPacoteDTO {
	return questaoPacoteDTO{
		ID: q.Chave, Unidade: q.Unidade, Enunciado: q.Enunciado, Alternativas: naoNula(q.Alternativas),
		Gabarito: q.Gabarito, Comentario: q.Comentario, Dispositivos: naoNula(q.Dispositivos), Trecho: q.Trecho,
	}
}

// EscreverPacoteLei grava o pacote que a curadoria importa.
func EscreverPacoteLei(w io.Writer, p lei.Pacote) error {
	d := pacoteLeiDTO{
		Formato: p.Formato,
		Lei: leiPacoteDTO{
			Slug: p.Lei.Slug, Nome: p.Lei.Nome, Curto: p.Lei.Curto,
			Reconhecer: naoNula(p.Lei.Reconhecer), Fonte: p.Lei.Fonte,
		},
		Versao:       p.Versao,
		Dispositivos: []dispositivoDTO{},
		Unidades:     []unidadeDTO{},
		Questoes:     []questaoPacoteDTO{},
	}
	for _, x := range p.Dispositivos {
		d.Dispositivos = append(d.Dispositivos, dispositivoParaDTO(x))
	}
	for _, u := range p.Unidades {
		d.Unidades = append(d.Unidades, unidadeParaDTO(u))
	}
	for _, q := range p.Questoes {
		d.Questoes = append(d.Questoes, questaoParaPacote(q))
	}

	return escreverJSON(w, d)
}

func escreverJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")

	return enc.Encode(v)
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

type importacaoLeiDTO struct {
	Slug       string `json:"slug"`
	Curto      string `json:"curto"`
	Versao     string `json:"versao"`
	NovaVersao bool   `json:"novaVersao"`
	Questoes   struct {
		Novas       int `json:"novas"`
		Atualizadas int `json:"atualizadas"`
		Desativadas int `json:"desativadas"`
		Mantidas    int `json:"mantidas"`
	} `json:"questoes"`
}

func importacaoLeiParaDTO(r service.ResultadoDaImportacaoDeLei) importacaoLeiDTO {
	d := importacaoLeiDTO{Slug: r.Slug, Curto: r.Curto, Versao: r.Versao, NovaVersao: r.NovaVersao}
	d.Questoes.Novas = r.Novas
	d.Questoes.Atualizadas = r.Atualizadas
	d.Questoes.Desativadas = r.Desativadas
	d.Questoes.Mantidas = r.Mantidas

	return d
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

type leituraLeiDTO struct {
	Lei          leiPacoteDTO       `json:"lei"`
	Versao       string             `json:"versao"`
	Dispositivos []dispositivoDTO   `json:"dispositivos"`
	Unidades     []unidadeDTO       `json:"unidades"`
	Questoes     []questaoLeitorDTO `json:"questoes"`
}

func leituraParaDTO(l service.LeituraDaLei) leituraLeiDTO {
	d := leituraLeiDTO{
		Lei: leiPacoteDTO{
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

	return d
}

type leisDaMateriaDTO struct {
	DisciplinaID string         `json:"disciplinaId"`
	Codigo       string         `json:"codigo"`
	Nome         string         `json:"nome"`
	Vinculadas   []leiResumoDTO `json:"vinculadas"`
	Sugeridas    []leiResumoDTO `json:"sugeridas"`
}

func leisDaMateriaParaDTO(m service.LeisDaMateria) leisDaMateriaDTO {
	d := leisDaMateriaDTO{
		DisciplinaID: m.DisciplinaID.String(), Codigo: m.Codigo, Nome: m.Nome,
		Vinculadas: []leiResumoDTO{}, Sugeridas: []leiResumoDTO{},
	}
	for _, r := range m.Vinculadas {
		d.Vinculadas = append(d.Vinculadas, resumoLeiParaDTO(r))
	}
	for _, r := range m.Sugeridas {
		d.Sugeridas = append(d.Sugeridas, resumoLeiParaDTO(r))
	}

	return d
}

var errPacoteIlegivel = errors.New("o arquivo não é um pacote de lei (studygo.lei/1)")
