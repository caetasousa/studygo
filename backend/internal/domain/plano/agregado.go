package plano

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNaoEncontrado é devolvido quando o usuário ainda não tem plano para um
// concurso.
var ErrNaoEncontrado = errors.New("plano não encontrado")

// Plano é o plano de estudo de um usuário para um concurso: a configuração
// escolhida mais tudo que ele registrou.
//
// O CRONOGRAMA NÃO ESTÁ AQUI. Ele é uma coleção de Atividade carregada à parte,
// porque é grande e nem todo caso de uso precisa dele — a configuração é lida em
// toda requisição, o cronograma só quando a tela mostra dias.
type Plano struct {
	ID           uuid.UUID
	UsuarioID    uuid.UUID
	ConcursoID   uuid.UUID
	Config       Config
	CriadoEm     time.Time
	AtualizadoEm time.Time

	// Registros é o que foi estudado, por atividade.
	Registros Registros
	// Dias guarda o que pertence ao dia e não a uma atividade.
	Dias map[time.Time]RegistroDia
	// Marcos são os itens do cronograma oficial que o usuário marcou.
	Marcos map[uuid.UUID]bool
}

// NovoPlano devolve um Plano com as coleções já inicializadas.
func NovoPlano() Plano {
	return Plano{
		Registros: Registros{},
		Dias:      map[time.Time]RegistroDia{},
		Marcos:    map[uuid.UUID]bool{},
	}
}

// Origem diz de onde veio uma anotação do caderno. Qualquer coisa diferente de
// OrigemManual foi criada pelo próprio app, a partir de um resultado ruim.
type Origem string

const (
	OrigemManual   Origem = "manual"
	OrigemRevisao  Origem = "revisao"
	OrigemTEC      Origem = "tec"
	OrigemSimulado Origem = "simulado"
)

// OrigemValida normaliza a origem de uma anotação, caindo em manual.
func OrigemValida(o Origem) Origem {
	switch o {
	case OrigemRevisao, OrigemTEC, OrigemSimulado:
		return o
	default:
		return OrigemManual
	}
}

// Anotacao é uma entrada do caderno de erros.
type Anotacao struct {
	ID             uuid.UUID
	Data           *time.Time
	DisciplinaID   *uuid.UUID
	Tema           string
	Texto          string
	Origem         Origem
	URL            string
	ProximaRevisao *time.Time
	Resolvido      bool
	CriadoEm       time.Time
	AtualizadoEm   time.Time
}
