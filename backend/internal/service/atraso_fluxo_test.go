package service

import (
	"context"
	"testing"
	"time"

	"studygo/internal/domain/plano"
	"studygo/internal/port"
)

// avancarPara move o relógio do cenário, simulando os dias passando sem que
// ninguém estude — que é a única forma de produzir atraso.
func (ce *cenario) avancarPara(d time.Time) {
	ce.deps.Relogio = relogioFixo{t: d}
	ce.hoje = d
}

func (ce *cenario) absorver(t *testing.T) (PlanoMontado, int) {
	t.Helper()

	p, dias, err := NewCronogramaService(ce.deps).
		AbsorverAtraso(context.Background(), ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("AbsorverAtraso: %v", err)
	}

	return p, dias
}

// O sintoma que o estudante relatou: o dia que ele não estudou continuava
// segurando as matérias, como se o tempo não tivesse passado.
func TestAbsorverAtraso_EsvaziaODiaPerdido(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t) // materializa o cronograma

	perdido := ce.hoje
	antes := plano.AtividadesDoDia(ce.cronograma.atividades, perdido)

	if len(antes) == 0 {
		t.Fatal("cenário inválido: o primeiro dia precisa ter atividade")
	}

	// Dois dias depois, sem nada registrado.
	ce.avancarPara(perdido.AddDate(0, 0, 2))

	_, dias := ce.absorver(t)

	if dias == 0 {
		t.Fatal("dias atrasados = 0, quer os dias vencidos sem registro")
	}

	if depois := plano.AtividadesDoDia(ce.cronograma.atividades, perdido); len(depois) != 0 {
		t.Errorf("o dia perdido ficou com %d atividades, quer vazio", len(depois))
	}
}

// O conteúdo não some: some do dia perdido e reaparece à frente.
func TestAbsorverAtraso_MantemOCronogramaAdiante(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t)

	hoje := ce.hoje.AddDate(0, 0, 2)
	ce.avancarPara(hoje)

	ce.absorver(t)

	futuro := 0

	for _, a := range ce.cronograma.atividades {
		if !plano.DayOf(a.Data).Before(hoje) {
			futuro++
		}
	}

	if futuro == 0 {
		t.Error("nenhuma atividade de hoje em diante — o replanejamento esvaziou o plano")
	}
}

// Estudar em dia não deve mexer em nada: sem atraso, sem gravação.
func TestAbsorverAtraso_SemAtrasoNaoGrava(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t)

	gravacoes := ce.cronograma.gravacoes

	_, dias := ce.absorver(t)

	if dias != 0 {
		t.Errorf("dias atrasados = %d, quer 0 no primeiro dia do plano", dias)
	}

	if ce.cronograma.gravacoes != gravacoes {
		t.Error("gravou o cronograma sem ter atraso para absorver")
	}
}

// A varredura diária só carrega quem o banco apontou.
func TestAbsorverAtrasosDoDia_VarreOsPlanosApontados(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t)

	ce.avancarPara(ce.hoje.AddDate(0, 0, 2))
	ce.planos.comAtraso = []port.PlanoAtrasado{{UsuarioID: ce.usuario, Slug: ce.slug}}

	n, err := NewCronogramaService(ce.deps).AbsorverAtrasosDoDia(context.Background())
	if err != nil {
		t.Fatalf("AbsorverAtrasosDoDia: %v", err)
	}

	if n != 1 {
		t.Errorf("planos replanejados = %d, quer 1", n)
	}
}
