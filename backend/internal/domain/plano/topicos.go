package plano

import (
	"strings"

	"github.com/google/uuid"
)

// separadorDeTemas junta os tópicos de uma matéria que tem mais tópicos que
// vagas: o motor põe vários numa atividade só (ver reparte).
const separadorDeTemas = "  ·  "

// PartesDoTema são os tópicos que uma atividade junta; um só, na maioria.
func PartesDoTema(tema string) []string {
	out := []string{}

	for _, p := range strings.Split(tema, "·") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}

	return out
}

// SepararTema tira um tópico de uma atividade que junta vários e o põe numa
// atividade própria, logo depois dela no mesmo dia. É o que deixa marcar como
// estudado um tópico sozinho: o registro é por atividade, e sem separar,
// marcar um tópico de "AD · LDAP" marcaria os dois.
//
// A atividade de um tópico só volta como está, com o próprio id. A nova herda
// a matéria, a passada e o tipo da original, e por isso continua contando
// como cobertura do edital.
func SepararTema(
	atividades []Atividade,
	id uuid.UUID,
	tema string,
	novoID uuid.UUID,
) ([]Atividade, uuid.UUID, error) {
	idx := indiceDe(atividades, id)
	if idx < 0 {
		return nil, uuid.Nil, ErrAtividadeNaoEncontrada
	}

	original := atividades[idx]
	partes := PartesDoTema(original.Tema)
	tema = strings.TrimSpace(tema)

	resto := make([]string, 0, len(partes))
	achou := false

	for _, p := range partes {
		if !achou && p == tema {
			achou = true

			continue
		}

		resto = append(resto, p)
	}

	if !achou {
		return nil, uuid.Nil, ErrTemaForaDaAtividade
	}

	if len(resto) == 0 {
		return append([]Atividade(nil), atividades...), id, nil
	}

	saida := make([]Atividade, 0, len(atividades)+1)

	for i, a := range atividades {
		if sameDay(a.Data, original.Data) && a.Posicao > original.Posicao {
			a.Posicao++
		}

		if i == idx {
			a.Tema = strings.Join(resto, separadorDeTemas)
		}

		saida = append(saida, a)
	}

	nova := original
	nova.ID = novoID
	nova.Tema = tema
	nova.Posicao = original.Posicao + 1
	nova.DuracaoMin = 0

	return append(saida, nova), novoID, nil
}
