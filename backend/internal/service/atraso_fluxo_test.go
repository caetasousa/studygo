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

// Reorganizar a partir de uma data que já passou é o atraso com a data na mão
// do estudante (D13). Como pode falhar, escrito antes do código:
//
//	R1  os dias vividos sem registro voltam a receber matéria, em vez de ficar vazios
//	R2  o que foi estudado na janela sai do lugar
//	R3  nada é redistribuído de hoje em diante
//	R4  a 1ª passada do tópico estudado na janela volta a ser agendada adiante
func TestReorganizarDesde_DataPassadaEsvaziaOsDiasSemEstudo(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t)

	desde := ce.hoje
	estudado := plano.AtividadesDoDia(ce.atividades(t), desde)

	if len(estudado) == 0 || estudado[0].Passada != 1 {
		t.Fatal("cenário inválido: o primeiro dia precisa começar por uma 1ª passada")
	}

	alvo := estudado[0]
	if err := ce.deps.Cronograma.SalvarRegistro(t.Context(), ce.plano(t).ID,
		plano.RegistroAtividade{AtividadeID: alvo.ID, Concluido: true}); err != nil {
		t.Fatalf("registrando o estudo: %v", err)
	}

	// Uma semana depois, sem mais nada registrado — e sem a varredura diária
	// ter passado, que é o estado de quem recadastra dias já vividos.
	hoje := desde.AddDate(0, 0, 7)
	ce.avancarPara(hoje)

	if _, err := NewCronogramaService(ce.deps).ReorganizarDesde(
		context.Background(), ce.usuario, ce.slug, desde.Format(formatoISO),
	); err != nil {
		t.Fatalf("ReorganizarDesde: %v", err)
	}

	rs := ce.registros(t)
	adiante := 0
	ficou := false

	for _, a := range ce.atividades(t) {
		dt := plano.DayOf(a.Data)

		switch {
		case a.ID == alvo.ID:
			ficou = dt.Equal(plano.DayOf(alvo.Data))
		case dt.Before(hoje) && !rs.Lancada(a.ID):
			t.Errorf("R1: %s ficou com %q sem registro, quer o dia vazio", dt.Format(formatoISO), a.Tema)
		case !dt.Before(hoje):
			adiante++

			if a.Passada == 1 && a.Disciplina == alvo.Disciplina && a.Tema == alvo.Tema {
				t.Errorf("R4: a 1ª passada de %q voltou em %s", a.Tema, dt.Format(formatoISO))
			}
		}
	}

	if !ficou {
		t.Error("R2: a atividade estudada saiu do dia em que foi estudada")
	}

	if adiante == 0 {
		t.Error("R3: nada agendado de hoje em diante")
	}
}
