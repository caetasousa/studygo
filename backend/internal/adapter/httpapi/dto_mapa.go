package httpapi

import (
	"errors"
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
	Mapa     mapaResumoDTO      `json:"mapa"`
	Arvore   []itemDoMapaDTO    `json:"arvore"`
	Questoes []questaoDoMapaDTO `json:"questoes"`
}

// questaoDoMapaDTO é a questão como a tela a recebe: sem gabarito nem
// comentário, que só vêm na resposta — a não ser que já respondida, e então
// com a correção da última tentativa. Sem alternativas, é de Certo/Errado.
type questaoDoMapaDTO struct {
	ID           string             `json:"id"`
	Ramo         string             `json:"ramo"`
	Origem       string             `json:"origem"`
	Enunciado    string             `json:"enunciado"`
	Alternativas []string           `json:"alternativas"`
	Resposta     *correcaoDoMapaDTO `json:"resposta"`
}

type correcaoDoMapaDTO struct {
	Escolhida    string    `json:"escolhida"`
	Acertou      bool      `json:"acertou"`
	Gabarito     string    `json:"gabarito"`
	Comentario   string    `json:"comentario"`
	RespondidaEm time.Time `json:"respondidaEm"`
}

// arquivoDeQuestoesDTO é o <slug>.questoes.json que quem escreve as questões
// mantém em conteudo/mapas e importa na página do mapa.
type arquivoDeQuestoesDTO struct {
	Mapa     string                      `json:"mapa"`
	Questoes []questaoDoArquivoDoMapaDTO `json:"questoes"`
}

type questaoDoArquivoDoMapaDTO struct {
	ID           string   `json:"id"`
	Ramo         string   `json:"ramo"`
	Origem       string   `json:"origem"`
	Enunciado    string   `json:"enunciado"`
	Alternativas []string `json:"alternativas"`
	Gabarito     string   `json:"gabarito"`
	Comentario   string   `json:"comentario"`
}

type questoesImportadasDTO struct {
	Novas       int `json:"novas"`
	Atualizadas int `json:"atualizadas"`
	Desativadas int `json:"desativadas"`
	Mantidas    int `json:"mantidas"`
}

type respostaDoMapaRequest struct {
	// Resposta é a letra (A–E) ou, na de julgar, CERTO ou ERRADO.
	Resposta string `json:"resposta"`
}

var errQuestoesDoMapaIlegiveis = errors.New(
	`o arquivo de questões não é um JSON válido: esperava {"mapa": "<slug>", "questoes": [...]}`)

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

func mapaLidoParaDTO(l service.MapaLido) mapaLidoDTO {
	m := l.Mapa
	ramos, itens := m.Contar()

	questoes := make([]questaoDoMapaDTO, 0, len(l.Questoes))
	for _, q := range l.Questoes {
		questoes = append(questoes, questaoDoMapaParaDTO(q))
	}

	return mapaLidoDTO{
		Mapa: mapaResumoDTO{
			Slug: m.Slug, Titulo: m.Titulo, Fonte: m.Fonte, Materia: m.Materia, Ramos: ramos, Itens: itens,
		},
		Arvore:   itensDoMapaParaDTO(m.Ramos),
		Questoes: questoes,
	}
}

func questaoDoMapaParaDTO(q mapa.QuestaoComResposta) questaoDoMapaDTO {
	out := questaoDoMapaDTO{
		ID: q.ID.String(), Ramo: q.Questao.Ramo, Origem: q.Questao.Origem, Enunciado: q.Questao.Enunciado,
		Alternativas: naoNula(q.Questao.Alternativas),
	}

	if q.Ultima != nil {
		c := correcaoDoMapaParaDTO(service.CorrecaoDe(q.Questao, *q.Ultima))
		out.Resposta = &c
	}

	return out
}

func correcaoDoMapaParaDTO(c service.CorrecaoDoMapa) correcaoDoMapaDTO {
	return correcaoDoMapaDTO{
		Escolhida: c.Escolhida, Acertou: c.Acertou, Gabarito: c.Gabarito, Comentario: c.Comentario, RespondidaEm: c.Em,
	}
}

func arquivoDeQuestoesDoDTO(d arquivoDeQuestoesDTO) mapa.ArquivoDeQuestoes {
	qs := make([]mapa.Questao, 0, len(d.Questoes))
	for _, q := range d.Questoes {
		qs = append(qs, mapa.Questao{
			Chave: q.ID, Ramo: q.Ramo, Origem: q.Origem, Enunciado: q.Enunciado,
			Alternativas: q.Alternativas, Gabarito: q.Gabarito, Comentario: q.Comentario,
		})
	}

	return mapa.ArquivoDeQuestoes{Mapa: d.Mapa, Questoes: qs}
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
