package lei

import "github.com/google/uuid"

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
