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
	// Imagens são os nomes das imagens já enviadas para o mapa.
	Imagens []string `json:"imagens"`
}

// imagensEnviadasDTO diz quantas imagens o envio gravou.
type imagensEnviadasDTO struct {
	Gravadas int `json:"gravadas"`
}

// questaoDoMapaDTO é a questão como a tela a recebe: sem gabarito nem
// comentário, que só vêm na resposta — a não ser que já respondida, e então
// com a correção da última tentativa. Sem alternativas, é de Certo/Errado.
type questaoDoMapaDTO struct {
	ID           string             `json:"id"`
	Ramo         string             `json:"ramo"`
	Origem       string             `json:"origem"`
	Banca        string             `json:"banca"`
	Enunciado    string             `json:"enunciado"`
	Alternativas []string           `json:"alternativas"`
	Resposta     *correcaoDoMapaDTO `json:"resposta"`
}

type correcaoDoMapaDTO struct {
	Escolhida  string `json:"escolhida"`
	Acertou    bool   `json:"acertou"`
	Gabarito   string `json:"gabarito"`
	Comentario string `json:"comentario"`
	// Explicacoes traz uma por alternativa, na ordem delas, ou nenhuma: aí o
	// comentário é a explicação inteira.
	Explicacoes  []string  `json:"explicacoes"`
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

// exclusaoDeItemRequest aponta o tópico pelo caminho de índices desde o ramo
// ([3,1,4]) e leva o texto que a tela mostrava, para o servidor recusar se o
// mapa mudou nesse meio-tempo.
type exclusaoDeItemRequest struct {
	Caminho []int  `json:"caminho"`
	Texto   string `json:"texto"`
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
	DisciplinaID string `json:"disciplinaId"`
	Codigo       string `json:"codigo"`
	Nome         string `json:"nome"`
	// Temas é a ementa da matéria: de onde a tela escolhe os tópicos do mapa.
	Temas []string           `json:"temas"`
	Mapas []mapaDaMateriaDTO `json:"mapas"`
}

// mapaDaMateriaDTO é o mapa vinculado e os tópicos da matéria que ele cobre.
type mapaDaMateriaDTO struct {
	mapaResumoDTO
	// MateriaInteira: nenhum tópico escolhido, o mapa vale para todos.
	MateriaInteira bool     `json:"materiaInteira"`
	Temas          []string `json:"temas"`
}

// vinculoDoMapaRequest são os tópicos escolhidos; nenhum é a matéria inteira.
type vinculoDoMapaRequest struct {
	Temas []string `json:"temas"`
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
		Imagens:  naoNula(l.Imagens),
	}
}

func questaoDoMapaParaDTO(q mapa.QuestaoComResposta) questaoDoMapaDTO {
	out := questaoDoMapaDTO{
		ID: q.ID.String(), Ramo: q.Questao.Ramo, Origem: q.Questao.Origem, Banca: q.Questao.Banca(),
		Enunciado:    q.Questao.Enunciado,
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
		Escolhida: c.Escolhida, Acertou: c.Acertou, Gabarito: c.Gabarito, Comentario: c.Comentario,
		Explicacoes: naoNula(c.Explicacoes), RespondidaEm: c.Em,
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
	mapas := make([]mapaDaMateriaDTO, 0, len(m.Mapas))
	for _, r := range m.Mapas {
		mapas = append(mapas, mapaDaMateriaDTO{
			mapaResumoDTO:  mapaResumoParaDTO(r.Resumo),
			MateriaInteira: r.MateriaInteira,
			Temas:          naoNula(r.Temas),
		})
	}

	return mapasDaMateriaDTO{
		DisciplinaID: m.DisciplinaID.String(), Codigo: m.Codigo, Nome: m.Nome, Temas: naoNula(m.Temas), Mapas: mapas,
	}
}

// pedidoDeMapaDTO é um pedido da fila como a tela o vê. O PDF não vem aqui:
// só se ainda está guardado.
type pedidoDeMapaDTO struct {
	ID       string `json:"id"`
	Arquivo  string `json:"arquivo"`
	Situacao string `json:"situacao"`
	// Concurso e Disciplina vêm vazios quando o pedido não escolheu matéria.
	Concurso     string    `json:"concurso"`
	Disciplina   string    `json:"disciplina"`
	Materia      string    `json:"materia"`
	Mapa         string    `json:"mapa"`
	Relatorio    string    `json:"relatorio"`
	TemPDF       bool      `json:"temPdf"`
	CriadoEm     time.Time `json:"criadoEm"`
	AtualizadoEm time.Time `json:"atualizadoEm"`
}

func pedidoDeMapaParaDTO(p mapa.Pedido) pedidoDeMapaDTO {
	d := pedidoDeMapaDTO{
		ID: p.ID.String(), Arquivo: p.Arquivo, Situacao: string(p.Situacao),
		Concurso: p.ConcursoSlug, Materia: p.DisciplinaNome, Mapa: p.Mapa, Relatorio: p.Relatorio,
		TemPDF: p.TemPDF, CriadoEm: p.CriadoEm, AtualizadoEm: p.AtualizadoEm,
	}
	if p.DisciplinaID.Valid {
		d.Disciplina = p.DisciplinaID.UUID.String()
	}

	return d
}
