package service

import (
	"testing"

	"studygo/internal/domain/plano"
)

// A redistribuição não pode devolver à fila o que já foi estudado.
//
// O sintoma que o estudante relatou: importado um histórico de duas semanas, o
// cronograma à frente voltava cheio das mesmas matérias, recomeçando do
// primeiro tema. Medido antes da correção: 84 de 84 atividades futuras eram
// conteúdo já concluído.
//
// A causa não estava na importação. `plano.Gerar` é um planejador PURO — dado o
// edital e a configuração, devolve o currículo inteiro, sempre do começo — e a
// redistribuição usava a saída dele sem descontar o que já tinha sido feito. A
// varredura da meia-noite faz a mesma coisa; ao vivo isso passava despercebido
// porque perder um dia repete um dia.

// unidade é como o teste identifica conteúdo, do jeito que o estudante o vê.
type unidade struct {
	disciplina string
	tema       string
	passada    int
}

func unidadeDe(a plano.Atividade) unidade {
	return unidade{a.Disciplina, a.Tema, a.Passada}
}

// estudarPrimeirosDias conclui tudo que está agendado nos primeiros `n` dias e
// devolve as unidades cobertas.
func estudarPrimeirosDias(ce *cenario, n int) map[unidade]bool {
	estudado := map[unidade]bool{}

	for d := range n {
		dia := ce.hoje.AddDate(0, 0, d)

		for _, a := range plano.AtividadesDoDia(ce.cronograma.atividades, dia) {
			if a.Disciplina == "" {
				continue
			}

			ce.cronograma.registros[a.ID] = plano.RegistroAtividade{
				AtividadeID: a.ID, Concluido: true,
			}
			estudado[unidadeDe(a)] = true
		}
	}

	return estudado
}

func TestAbsorverAtraso_NaoReagendaCoberturaJaConcluida(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t)

	estudado := estudarPrimeirosDias(ce, 14)

	hoje := ce.hoje.AddDate(0, 0, 16)
	ce.avancarPara(hoje)
	ce.absorver(t)

	for _, a := range ce.cronograma.atividades {
		if plano.DayOf(a.Data).Before(hoje) || a.Disciplina == "" {
			continue
		}

		// Passada 1 é a COBERTURA do edital: uma vaga por tema. Uma vez estudada,
		// ela não volta.
		if a.Passada == 1 && estudado[unidadeDe(a)] {
			t.Errorf(
				"%s · %s (passada 1) foi concluída e voltou a ser agendada em %s",
				a.Disciplina, a.Tema, a.Data.Format("02/01"),
			)
		}
	}
}

// O outro lado, e a correção que quase saiu errada: a passada 2 é repetição POR
// DESENHO — é ela que dá volume de questões até a prova.
//
// Uma primeira versão do desconto usava (matéria, tema, passada) como chave, e
// com isso resolver questões de um tema UMA vez apagava todas as repetições
// futuras dele: numa medição, 19 sessões programadas caíam para 3. Este teste
// existe para que essa versão não volte.
func TestAbsorverAtraso_PreservaARepeticaoDaSegundaPassada(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t)

	antes := map[unidade]int{}
	for _, a := range ce.cronograma.atividades {
		if a.Disciplina != "" && a.Passada == 2 {
			antes[unidadeDe(a)]++
		}
	}

	if len(antes) == 0 {
		t.Fatal("cenário inválido: o plano precisa ter repetições de segunda passada")
	}

	estudarPrimeirosDias(ce, 14)

	hoje := ce.hoje.AddDate(0, 0, 16)
	ce.avancarPara(hoje)
	ce.absorver(t)

	depois := map[unidade]int{}
	for _, a := range ce.cronograma.atividades {
		if a.Disciplina != "" && a.Passada == 2 {
			depois[unidadeDe(a)]++
		}
	}

	// A contagem cai um pouco: sobram menos dias até a prova. O que não pode é
	// desabar — o sinal de que o desconto comeu a repetição.
	for u, n := range antes {
		if depois[u]*2 < n {
			t.Errorf(
				"%s · %s (passada 2): %d repetições viraram %d — o desconto comeu a prática",
				u.disciplina, u.tema, n, depois[u],
			)
		}
	}
}

// Descontar não pode APAGAR: um tema que nunca foi estudado continua no plano.
func TestAbsorverAtraso_NaoPerdeConteudoNaoEstudado(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t)

	antes := map[unidade]bool{}
	for _, a := range ce.cronograma.atividades {
		if a.Disciplina != "" {
			antes[unidadeDe(a)] = true
		}
	}

	estudado := estudarPrimeirosDias(ce, 14)

	hoje := ce.hoje.AddDate(0, 0, 16)
	ce.avancarPara(hoje)
	ce.absorver(t)

	sobrou := map[unidade]bool{}
	for _, a := range ce.cronograma.atividades {
		if a.Disciplina != "" {
			sobrou[unidadeDe(a)] = true
		}
	}

	for u := range antes {
		if !estudado[u] && !sobrou[u] {
			t.Errorf(
				"%s · %s (passada %d) sumiu do plano sem nunca ter sido estudada",
				u.disciplina, u.tema, u.passada,
			)
		}
	}
}
