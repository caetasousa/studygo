package mapa

import (
	"time"

	"github.com/google/uuid"
)

// O que a exportação leva além do texto, das questões e das imagens: o estudo
// da conta em volta do mapa. Sai pelo que se reconhece noutra conta (ou depois
// de recriar o concurso) — nunca por id, que não sobrevive à viagem.

// VinculoExportado é o vínculo do mapa com uma matéria: o concurso pelo slug, a
// matéria pelo código e pelo nome, e os tópicos que o mapa cobre (nenhum é a
// matéria inteira).
type VinculoExportado struct {
	ConcursoSlug string
	ConcursoNome string
	Codigo       string
	Disciplina   string
	Temas        []string
}

// RespostaExportada é uma tentativa, presa à questão pela chave do arquivo de
// questões, com a hora em que foi dada.
type RespostaExportada struct {
	Chave    string
	Resposta string
	Acertou  bool
	Em       time.Time
}

// RespostaRestaurada é a tentativa que volta, já presa à questão gravada.
type RespostaRestaurada struct {
	QuestaoID uuid.UUID
	Resposta  Resposta
}
