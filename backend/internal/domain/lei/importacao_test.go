package lei

import (
	"testing"

	"github.com/google/uuid"
)

// Como a importação pode estragar as questões — escrito antes do código.
//
//	I1  reimportar o mesmo pacote cria ou altera questões (L2)
//	I2  uma questão que mudou de texto perde o id — e com ele as respostas (L3)
//	I3  a questão que saiu do pacote é apagada, levando as respostas junto
//	I4  a questão que volta ao pacote continua desativada

func TestPlanejar_MesmoPacoteNaoMudaNada(t *testing.T) {
	q := pacoteValido().Questoes[0]
	gravadas := []QuestaoGravada{{ID: uuid.New(), Chave: q.Chave, Assinatura: q.Assinatura(), Ativa: true}}

	plano := PlanejarImportacao(gravadas, []Questao{q})
	if len(plano.Novas)+len(plano.Atualizadas)+len(plano.Desativar) != 0 || plano.Mantidas != 1 {
		t.Fatalf("I1: reimportar mexeu nas questões: %+v", plano)
	}
}

func TestPlanejar_QuestaoAlteradaMantemOID(t *testing.T) {
	q := pacoteValido().Questoes[0]
	id := uuid.New()
	gravadas := []QuestaoGravada{{ID: id, Chave: q.Chave, Assinatura: q.Assinatura(), Ativa: true}}
	q.Comentario = "Comentário revisto."

	plano := PlanejarImportacao(gravadas, []Questao{q})
	if len(plano.Atualizadas) != 1 || plano.Atualizadas[0].ID != id || len(plano.Novas) != 0 {
		t.Fatalf("I2: a questão alterada não manteve o id: %+v", plano)
	}
}

func TestPlanejar_QuestaoQueSaiEDesativadaNaoApagada(t *testing.T) {
	q := pacoteValido().Questoes[0]
	id := uuid.New()
	gravadas := []QuestaoGravada{{ID: id, Chave: q.Chave, Assinatura: q.Assinatura(), Ativa: true}}

	plano := PlanejarImportacao(gravadas, nil)
	if len(plano.Desativar) != 1 || plano.Desativar[0] != id {
		t.Fatalf("I3: a questão que saiu não foi desativada: %+v", plano)
	}

	// Já desativada, não conta de novo.
	gravadas[0].Ativa = false
	if plano := PlanejarImportacao(gravadas, nil); len(plano.Desativar) != 0 {
		t.Fatalf("I3: desativou de novo: %+v", plano)
	}
}

func TestPlanejar_QuestaoQueVoltaEReativada(t *testing.T) {
	q := pacoteValido().Questoes[0]
	id := uuid.New()
	gravadas := []QuestaoGravada{{ID: id, Chave: q.Chave, Assinatura: q.Assinatura(), Ativa: false}}

	plano := PlanejarImportacao(gravadas, []Questao{q})
	if len(plano.Atualizadas) != 1 || plano.Atualizadas[0].ID != id {
		t.Fatalf("I4: a questão que voltou não foi reativada: %+v", plano)
	}
}

func TestCitadaEm(t *testing.T) {
	l := Lei{Reconhecer: []string{"16.168", "Constituição Federal"}}
	if !l.CitadaEm([]string{"Lei Orgânica do TCE-GO (Lei Estadual nº 16.168/2007)"}) {
		t.Fatal("tópico com o número não sugeriu a lei")
	}
	if !l.CitadaEm([]string{"CONSTITUICAO FEDERAL: arts. 70 a 75"}) {
		t.Fatal("caixa e acento não deveriam importar")
	}
	if l.CitadaEm([]string{"Lei nº 16.1680"}) || l.CitadaEm(nil) {
		t.Fatal("sugeriu por número que só começa igual")
	}
}
