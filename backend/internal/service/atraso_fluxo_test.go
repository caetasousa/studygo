//go:build integration

package service

import (
	"context"
	"maps"
	"testing"
	"time"

	"studygo/internal/domain/plano"
)

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
	antes := plano.AtividadesDoDia(ce.atividades(t), perdido)

	if len(antes) == 0 {
		t.Fatal("cenário inválido: o primeiro dia precisa ter atividade")
	}

	// Dois dias depois, sem nada registrado.
	ce.avancarPara(perdido.AddDate(0, 0, 2))

	_, dias := ce.absorver(t)

	if dias == 0 {
		t.Fatal("dias atrasados = 0, quer os dias vencidos sem registro")
	}

	if depois := plano.AtividadesDoDia(ce.atividades(t), perdido); len(depois) != 0 {
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

	for _, a := range ce.atividades(t) {
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

	antes := ce.versoesDasLinhas(t)

	_, dias := ce.absorver(t)

	if dias != 0 {
		t.Errorf("dias atrasados = %d, quer 0 no primeiro dia do plano", dias)
	}

	if !maps.Equal(antes, ce.versoesDasLinhas(t)) {
		t.Error("gravou o cronograma sem ter atraso para absorver")
	}
}

// A varredura diária acha no banco quem está atrasado e replaneja.
func TestAbsorverAtrasosDoDia_VarreOsPlanosApontados(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t)

	ce.avancarPara(ce.hoje.AddDate(0, 0, 2))

	n, err := NewCronogramaService(ce.deps).AbsorverAtrasosDoDia(context.Background())
	if err != nil {
		t.Fatalf("AbsorverAtrasosDoDia: %v", err)
	}

	if n != 1 {
		t.Errorf("planos replanejados = %d, quer 1", n)
	}
}

// Reorganizar a partir de uma data é o que serve a quem recadastra dias já
// vividos: registra o que estudou e manda o motor rearrumar dali para frente.
func TestReorganizarDesde_RefazODaDataEmDiante(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t)

	svc := NewCronogramaService(ce.deps)
	desde := ce.hoje.AddDate(0, 0, 3)

	antes := len(ce.atividades(t))

	if _, err := svc.ReorganizarDesde(
		context.Background(), ce.usuario, ce.slug, desde.Format("2006-01-02"),
	); err != nil {
		t.Fatalf("ReorganizarDesde: %v", err)
	}

	if len(ce.atividades(t)) == 0 {
		t.Fatalf("o cronograma ficou vazio (tinha %d)", antes)
	}

	// O que é anterior à data escolhida não se mexe.
	for _, a := range ce.atividades(t) {
		if plano.DayOf(a.Data).Before(desde) && a.Data.IsZero() {
			t.Error("atividade anterior à data perdeu a data")
		}
	}
}

// Data fora do plano não reorganiza nada: depois da prova não há para onde
// distribuir.
func TestReorganizarDesde_RecusaDepoisDaProva(t *testing.T) {
	ce := novoCenario(t)
	p := ce.obter(t)

	svc := NewCronogramaService(ce.deps)

	prova, err := time.Parse("2006-01-02", p.Config.Prova)
	if err != nil {
		t.Fatalf("data da prova %q: %v", p.Config.Prova, err)
	}

	_, err = svc.ReorganizarDesde(
		context.Background(), ce.usuario, ce.slug,
		prova.AddDate(0, 0, 1).Format("2006-01-02"),
	)
	if err == nil {
		t.Error("reorganizou a partir de uma data depois da prova, quer recusa")
	}
}
