package plano_test

import (
	"testing"
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// A invariante central do produto: a conclusão do DIA é derivada das atividades
// dele, nunca informada. Um dia de duas matérias não pode se dar por terminado
// quando só a primeira foi lançada.
func TestDiaConcluido_ExigeTodasAsAtividades(t *testing.T) {
	t.Parallel()

	hoje := dia(2026, time.September, 1)
	a1, a2 := uuid.New(), uuid.New()

	atividades := []plano.Atividade{
		{ID: a1, Data: hoje, Posicao: 0, Disciplina: "LP", Tipo: plano.AtividadeConteudo},
		{ID: a2, Data: hoje, Posicao: 1, Disciplina: "BD", Tipo: plano.AtividadeConteudo},
	}

	casos := []struct {
		nome      string
		registros plano.Registros
		quer      bool
	}{
		{
			nome:      "sem registro nenhum",
			registros: plano.Registros{},
			quer:      false,
		},
		{
			nome: "só a primeira concluída",
			registros: plano.Registros{
				a1: {AtividadeID: a1, Concluido: true},
			},
			quer: false,
		},
		{
			nome: "a segunda lançada mas não concluída",
			registros: plano.Registros{
				a1: {AtividadeID: a1, Concluido: true},
				a2: {AtividadeID: a2, Concluido: false},
			},
			quer: false,
		},
		{
			nome: "as duas concluídas",
			registros: plano.Registros{
				a1: {AtividadeID: a1, Concluido: true},
				a2: {AtividadeID: a2, Concluido: true},
			},
			quer: true,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			t.Parallel()

			if got := plano.DiaConcluido(atividades, caso.registros, hoje); got != caso.quer {
				t.Errorf("DiaConcluido = %v, quer %v", got, caso.quer)
			}
		})
	}
}

// Um dia sem atividade nenhuma não está concluído: não há o que concluir.
func TestDiaConcluido_DiaVazioNaoConclui(t *testing.T) {
	t.Parallel()

	if plano.DiaConcluido(nil, plano.Registros{}, dia(2026, time.September, 1)) {
		t.Error("um dia sem atividades não pode contar como concluído")
	}
}

// Duas ocorrências da mesma disciplina num dia são independentes — era
// exatamente isso que a chave antiga (data, disciplina) não conseguia guardar.
func TestRegistros_DuasOcorrenciasDaMesmaDisciplinaSaoIndependentes(t *testing.T) {
	t.Parallel()

	hoje := dia(2026, time.September, 1)
	primeira, segunda := uuid.New(), uuid.New()

	atividades := []plano.Atividade{
		{ID: primeira, Data: hoje, Posicao: 0, Disciplina: "LP", Tema: "Crase"},
		{ID: segunda, Data: hoje, Posicao: 1, Disciplina: "LP", Tema: "Regência"},
	}

	registros := plano.Registros{
		primeira: {AtividadeID: primeira, Concluido: true},
	}

	if !registros.Concluida(primeira) {
		t.Error("a primeira ocorrência devia estar concluída")
	}

	if registros.Concluida(segunda) {
		t.Error("concluir uma ocorrência não pode concluir a outra")
	}

	if plano.DiaConcluido(atividades, registros, hoje) {
		t.Error("o dia não pode concluir com uma das duas ocorrências pendente")
	}
}

func TestTotaisDoDia_SomaSoOQueFoiLancado(t *testing.T) {
	t.Parallel()

	hoje := dia(2026, time.September, 1)
	a1, a2 := uuid.New(), uuid.New()

	atividades := []plano.Atividade{
		{ID: a1, Data: hoje, Posicao: 0, Disciplina: "LP"},
		{ID: a2, Data: hoje, Posicao: 1, Disciplina: "BD"},
	}

	duas := 2.0
	dez := 10
	oito := 8

	registros := plano.Registros{
		a1: {AtividadeID: a1, Horas: &duas, Questoes: &dez, Acertos: &oito},
		a2: {AtividadeID: a2}, // lançada sem números
	}

	horas, questoes, acertos := plano.TotaisDoDia(atividades, registros, hoje)

	if horas == nil || *horas != 2 {
		t.Errorf("horas = %v, quer 2", horas)
	}

	if questoes == nil || *questoes != 10 {
		t.Errorf("questões = %v, quer 10", questoes)
	}

	if acertos == nil || *acertos != 8 {
		t.Errorf("acertos = %v, quer 8", acertos)
	}
}

// Um campo que ninguém preencheu continua nulo, em vez de virar zero: "não
// lancei" e "lancei zero" são coisas diferentes na estatística.
func TestTotaisDoDia_SemLancamentoContinuaNulo(t *testing.T) {
	t.Parallel()

	hoje := dia(2026, time.September, 1)
	a1 := uuid.New()

	atividades := []plano.Atividade{{ID: a1, Data: hoje, Disciplina: "LP"}}

	horas, questoes, acertos := plano.TotaisDoDia(atividades, plano.Registros{}, hoje)

	if horas != nil || questoes != nil || acertos != nil {
		t.Errorf("sem registro tudo devia ser nil, veio (%v, %v, %v)", horas, questoes, acertos)
	}
}

func TestAcertosValidos_NuncaPassaDasQuestoes(t *testing.T) {
	t.Parallel()

	dez, quinze := 10, 15

	if got := plano.AcertosValidos(&dez, &quinze); got == nil || *got != 10 {
		t.Errorf("AcertosValidos(10, 15) = %v, quer 10", got)
	}

	oito := 8
	if got := plano.AcertosValidos(&dez, &oito); got == nil || *got != 8 {
		t.Errorf("AcertosValidos(10, 8) = %v, quer 8", got)
	}

	if got := plano.AcertosValidos(&dez, nil); got != nil {
		t.Errorf("sem acertos o resultado devia ser nil, veio %v", got)
	}
}
