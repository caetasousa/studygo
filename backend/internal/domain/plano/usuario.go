package plano

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a user has no plan yet for a concurso.
var ErrNotFound = errors.New("plano não encontrado")

// TemaUI is the persisted theme choice ("light", "dark", "system").

// Salvo is the persisted state of a user's plan: the config plus everything the
// artifact kept in localStorage.
type Salvo struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	ConcursoID   uuid.UUID
	Config       Config
	TemaUI       string
	CriadoEm     time.Time
	AtualizadoEm time.Time

	Registros    map[time.Time]Registro
	Marcos       map[uuid.UUID]bool
	Reordenacoes map[time.Time]Reordenacao
	Revisoes     []Revisao // the open spaced-review queue
}

// Origem says where a notebook entry came from. Anything but OrigemManual was
// created by the app itself, off a bad result.
type Origem string

const (
	OrigemManual   Origem = "manual"
	OrigemRevisao  Origem = "revisao"
	OrigemTEC      Origem = "tec"
	OrigemSimulado Origem = "simulado"
)

// Anotacao is one entry of the dedicated notebook / error log.
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

// NewSalvo returns a Salvo with initialized (non-nil) collections.
func NewSalvo() Salvo {
	return Salvo{
		Registros:    map[time.Time]Registro{},
		Marcos:       map[uuid.UUID]bool{},
		Reordenacoes: map[time.Time]Reordenacao{},
		Revisoes:     []Revisao{},
	}
}
