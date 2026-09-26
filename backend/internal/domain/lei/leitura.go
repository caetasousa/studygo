package lei

import (
	"time"

	"github.com/google/uuid"
)

// Resumo é a lei no catálogo: a versão ativa e quantas questões ela tem.
type Resumo struct {
	Lei         Lei
	Versao      string
	Questoes    int
	ImportadaEm time.Time
}

// Texto é a versão ativa da lei, como o leitor a mostra.
type Texto struct {
	Versao       string
	Dispositivos []Dispositivo
	Unidades     []Unidade
}

// QuestaoPublicada é a questão como está gravada: com id, que é o que prende
// as respostas.
type QuestaoPublicada struct {
	ID      uuid.UUID
	LeiID   uuid.UUID
	Ativa   bool
	Questao Questao
}

// Resposta é uma tentativa do estudante. A mais recente é a que conta no
// progresso; as anteriores ficam como história.
type Resposta struct {
	UsuarioID   uuid.UUID
	QuestaoID   uuid.UUID
	Alternativa string
	Acertou     bool
	Em          time.Time
}

// QuestaoComResposta é a questão com a última resposta do estudante, se houver.
type QuestaoComResposta struct {
	QuestaoPublicada
	Ultima *Resposta
}

// Vinculo é a lei que uma matéria cobra, com o recorte do edital. Recorte
// vazio é a lei inteira.
type Vinculo struct {
	LeiID   uuid.UUID
	Recorte []string
}
