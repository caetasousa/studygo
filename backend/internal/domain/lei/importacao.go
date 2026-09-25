package lei

import (
	"strings"

	"github.com/google/uuid"
)

// QuestaoGravada é o que a importação precisa saber das questões que já
// existem: quem é quem, e se mudou.
type QuestaoGravada struct {
	ID         uuid.UUID
	Chave      string
	Assinatura string
	Ativa      bool
}

// QuestaoAtualizada leva o id da questão gravada: é ele que prende as
// respostas do estudante.
type QuestaoAtualizada struct {
	ID      uuid.UUID
	Questao Questao
}

// PlanoDeImportacao diz o que fazer com as questões. Nenhuma questão é
// apagada: a que saiu do pacote é desativada, e as respostas a ela continuam
// sendo história.
type PlanoDeImportacao struct {
	Novas       []Questao
	Atualizadas []QuestaoAtualizada
	Desativar   []uuid.UUID
	Mantidas    int
}

// PlanejarImportacao casa as questões do pacote com as gravadas pela chave.
func PlanejarImportacao(gravadas []QuestaoGravada, doPacote []Questao) PlanoDeImportacao {
	porChave := make(map[string]QuestaoGravada, len(gravadas))
	for _, g := range gravadas {
		porChave[g.Chave] = g
	}

	var plano PlanoDeImportacao
	noPacote := make(map[string]bool, len(doPacote))
	for _, q := range doPacote {
		noPacote[q.Chave] = true
		g, existe := porChave[q.Chave]
		switch {
		case !existe:
			plano.Novas = append(plano.Novas, q)
		case g.Ativa && g.Assinatura == q.Assinatura():
			plano.Mantidas++
		default:
			plano.Atualizadas = append(plano.Atualizadas, QuestaoAtualizada{ID: g.ID, Questao: q})
		}
	}

	for _, g := range gravadas {
		if g.Ativa && !noPacote[g.Chave] {
			plano.Desativar = append(plano.Desativar, g.ID)
		}
	}

	return plano
}

// Curadoria é quem pode importar leis: o catálogo é de todos, então publicar
// nele não pode ser de qualquer conta.
type Curadoria struct {
	emails map[string]bool
	todos  bool
}

// NovaCuradoria lê a lista de e-mails. "*" (qualquer conta) só existe em
// desenvolvimento: em produção, recusar na partida é melhor que abrir o
// catálogo por um erro de configuração.
func NovaCuradoria(emails []string, desenvolvimento bool) (Curadoria, error) {
	c := Curadoria{emails: map[string]bool{}}
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		switch e {
		case "":
		case "*":
			if !desenvolvimento {
				return Curadoria{}, ErrCuradoriaAbertaDemais
			}
			c.todos = true
		default:
			c.emails[e] = true
		}
	}

	return c, nil
}

// Pode diz se a conta com este e-mail importa leis.
func (c Curadoria) Pode(email string) bool {
	return c.todos || c.emails[strings.ToLower(strings.TrimSpace(email))]
}
