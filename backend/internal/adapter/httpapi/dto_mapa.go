package httpapi

import (
	"time"

	"studygo/internal/domain/mapa"
	"studygo/internal/service"
)

// mapaResumoDTO é o mapa sem a árvore: o que a lista e o vínculo mostram.
type mapaResumoDTO struct {
	Slug        string    `json:"slug"`
	Titulo      string    `json:"titulo"`
	Fonte       string    `json:"fonte"`
	Materia     string    `json:"materia"`
	Ramos       int       `json:"ramos"`
	Itens       int       `json:"itens"`
	ImportadoEm time.Time `json:"importadoEm"`
}

// itemDoMapaDTO é um ponto do mapa e tudo o que está abaixo dele.
type itemDoMapaDTO struct {
	Texto  string          `json:"texto"`
	Marca  string          `json:"marca"`
	Filhos []itemDoMapaDTO `json:"filhos"`
}

// mapaLidoDTO é o mapa aberto: o resumo e a árvore, cujos itens de cima são os
// ramos principais.
type mapaLidoDTO struct {
	Mapa   mapaResumoDTO   `json:"mapa"`
	Arvore []itemDoMapaDTO `json:"arvore"`
}

type importarMapaRequest struct {
	// Texto é o outline do mapa (conteudo/mapas/README.md).
	Texto string `json:"texto"`
	// Concurso, se vier, é o concurso aberto: o mapa é vinculado às matérias
	// dele que o texto indica.
	Concurso string `json:"concurso"`
}

type materiaDoMapaDTO struct {
	Codigo string `json:"codigo"`
	Nome   string `json:"nome"`
}

type mapaImportadoDTO struct {
	Mapa       mapaResumoDTO      `json:"mapa"`
	Novo       bool               `json:"novo"`
	Vinculadas []materiaDoMapaDTO `json:"vinculadas"`
}

// mapasDaMateriaDTO é uma matéria do concurso com os mapas vinculados a ela.
type mapasDaMateriaDTO struct {
	DisciplinaID string          `json:"disciplinaId"`
	Codigo       string          `json:"codigo"`
	Nome         string          `json:"nome"`
	Mapas        []mapaResumoDTO `json:"mapas"`
}

func mapaResumoParaDTO(r mapa.Resumo) mapaResumoDTO {
	return mapaResumoDTO{
		Slug: r.Slug, Titulo: r.Titulo, Fonte: r.Fonte, Materia: r.Materia,
		Ramos: r.Ramos, Itens: r.Itens, ImportadoEm: r.ImportadoEm,
	}
}

func itensDoMapaParaDTO(itens []mapa.Item) []itemDoMapaDTO {
	out := make([]itemDoMapaDTO, 0, len(itens))

	for _, it := range itens {
		out = append(out, itemDoMapaDTO{Texto: it.Texto, Marca: string(it.Marca), Filhos: itensDoMapaParaDTO(it.Filhos)})
	}

	return out
}

func mapaLidoParaDTO(m mapa.Mapa) mapaLidoDTO {
	ramos, itens := m.Contar()

	return mapaLidoDTO{
		Mapa: mapaResumoDTO{
			Slug: m.Slug, Titulo: m.Titulo, Fonte: m.Fonte, Materia: m.Materia, Ramos: ramos, Itens: itens,
		},
		Arvore: itensDoMapaParaDTO(m.Ramos),
	}
}

func mapaImportadoParaDTO(r service.MapaImportado) mapaImportadoDTO {
	vinculadas := make([]materiaDoMapaDTO, 0, len(r.Vinculadas))
	for _, v := range r.Vinculadas {
		vinculadas = append(vinculadas, materiaDoMapaDTO{Codigo: v.Codigo, Nome: v.Nome})
	}

	return mapaImportadoDTO{Mapa: mapaResumoParaDTO(r.Mapa), Novo: r.Novo, Vinculadas: vinculadas}
}

func mapasDaMateriaParaDTO(m service.MapasDaMateria) mapasDaMateriaDTO {
	mapas := make([]mapaResumoDTO, 0, len(m.Mapas))
	for _, r := range m.Mapas {
		mapas = append(mapas, mapaResumoParaDTO(r))
	}

	return mapasDaMateriaDTO{DisciplinaID: m.DisciplinaID.String(), Codigo: m.Codigo, Nome: m.Nome, Mapas: mapas}
}
