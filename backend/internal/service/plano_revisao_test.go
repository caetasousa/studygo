package service

import (
	"testing"
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

func TestAnotacaoDaRevisao(t *testing.T) {
	t.Parallel()

	dt := d(2026, 9, 2)
	outroDia := d(2026, 9, 3)
	revisaoID := uuid.New()

	anotacoes := []plano.Anotacao{
		// Uma anotação manual no mesmo dia não é a observação da revisão.
		{ID: uuid.New(), Data: &dt, Origem: plano.OrigemManual, Texto: "nota manual"},
		// Uma anotação de revisão em outro dia também não é.
		{ID: uuid.New(), Data: &outroDia, Origem: plano.OrigemRevisao, Texto: "de outro dia"},
		{ID: revisaoID, Data: &dt, Origem: plano.OrigemRevisao, Texto: "a observação certa"},
	}

	got := anotacaoDaRevisao(anotacoes, dt)
	if got == nil || got.ID != revisaoID {
		t.Fatalf("anotacaoDaRevisao = %v, queria a de id %s", got, revisaoID)
	}
}

func TestAnotacaoDaRevisao_NenhumaAinda(t *testing.T) {
	t.Parallel()

	if got := anotacaoDaRevisao(nil, d(2026, 9, 2)); got != nil {
		t.Errorf("esperava nil sem nenhuma anotação, veio %v", got)
	}
}

func TestMontarRevisao(t *testing.T) {
	t.Parallel()

	dt := d(2026, 9, 2)

	t.Run("nada registrado ainda", func(t *testing.T) {
		t.Parallel()

		got := montarRevisao("POR", dt, nil, nil)

		if got.Disciplina != "POR" || got.Questoes != nil || got.Acertos != nil || got.AnotacaoID != "" {
			t.Errorf("revisão vazia veio com dado inventado: %+v", got)
		}
	})

	t.Run("junta o registro e a observação do mesmo dia", func(t *testing.T) {
		t.Parallel()

		registros := map[time.Time]plano.RegistroRevisao{
			dt: {Data: dt, Questoes: ptrI(30), Acertos: ptrI(21)},
		}
		id := uuid.New()
		anotacoes := []plano.Anotacao{
			{ID: id, Data: &dt, Origem: plano.OrigemRevisao, Texto: "revisei bem, só errei prazos"},
		}

		got := montarRevisao("POR", dt, registros, anotacoes)

		if *got.Questoes != 30 || *got.Acertos != 21 {
			t.Errorf("questões/acertos = %v/%v, queria 30/21", got.Questoes, got.Acertos)
		}

		if got.AnotacaoID != id.String() || got.Observacao != "revisei bem, só errei prazos" {
			t.Errorf("observação = %+v, não juntou com a anotação certa", got)
		}
	})
}
