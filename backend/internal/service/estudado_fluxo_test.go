package service

import (
	"context"
	"testing"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// O tópico estudado antes da hora vai para o dia em que foi estudado, e a 1ª
// passada dele não volta adiante (plano.ArrumarEstudado). A conclusão aplica a
// regra; quem mais reescreve o cronograma também precisa aplicá-la, senão o
// estado que uma arrumação interrompida deixou — registro gravado, atividade
// parada no dia futuro — fica para sempre. Como pode falhar, escrito antes do
// código:
//
//	E1  reorganizar a partir de hoje deixa o tópico concluído no dia futuro, riscado
//	E2  restaurar a ordem automática, idem
//	E3  mudar a configuração (o ritmo do dia), idem
//	E4  o cronograma refeito volta a agendar a 1ª passada do tópico já estudado
//	E5  trazer o tópico para hoje perde o registro dele
//
// O estado de partida não se produz pela API — a conclusão já arruma —, então
// o registro entra direto no fake: é o que o banco tinha depois do 500 de
// atividades_plano_data_posicao_key (cenário C21).
func TestFluxo_RefazerOCronogramaPoeEmDiaOEstudadoAntesDaHora(t *testing.T) {
	t.Parallel()

	refazer := map[string]func(ce *cenario) (PlanoMontado, error){
		"E1 reorganizar": func(ce *cenario) (PlanoMontado, error) {
			return NewCronogramaService(ce.deps).ReorganizarDesde(
				context.Background(), ce.usuario, ce.slug, ce.hoje.Format(formatoISO),
			)
		},
		"E2 restaurar a ordem": func(ce *cenario) (PlanoMontado, error) {
			return NewCronogramaService(ce.deps).RestaurarOrdem(context.Background(), ce.usuario, ce.slug)
		},
		"E3 mudar a configuração": func(ce *cenario) (PlanoMontado, error) {
			tres := 3

			return NewPlanoService(ce.deps).Salvar(
				context.Background(), ce.usuario, ce.slug, ConfigCommand{BlocosPorDia: &tres},
			)
		},
	}

	for nome, faz := range refazer {
		t.Run(nome, func(t *testing.T) {
			t.Parallel()

			ce := novoCenario(t)
			p := ce.obter(t)

			alvo := primeiraPassadaAdiante(t, p, ce.hoje.Format(formatoISO))
			ce.cronograma.registros[alvo.ID] = plano.RegistroAtividade{AtividadeID: alvo.ID, Concluido: true}

			p, err := faz(ce)
			if err != nil {
				t.Fatalf("refazer: %v", err)
			}

			hoje := ce.hoje.Format(formatoISO)
			achou := false

			for _, d := range p.Dias {
				for _, it := range d.Itens {
					if it.ID == alvo.ID {
						achou = true

						if d.Data != hoje || !it.Antecipada {
							t.Errorf("o tópico estudado ficou em %s (antecipada=%v), quer %s antecipado", d.Data, it.Antecipada, hoje)
						}

						if !it.Concluido {
							t.Error("E5: o registro do tópico se perdeu")
						}

						continue
					}

					if d.Data > hoje && it.Disciplina == alvo.Disciplina && it.Tema == alvo.Tema &&
						it.Passada == 1 && !it.Concluido {
						t.Errorf("E4: a 1ª passada de %q voltou em %s", alvo.Tema, d.Data)
					}
				}
			}

			if !achou {
				t.Error("E5: o tópico estudado sumiu do cronograma")
			}
		})
	}
}

// primeiraPassadaAdiante é um tópico de 1ª passada agendado depois de hoje, num
// bloco de um tópico só.
func primeiraPassadaAdiante(t *testing.T, p PlanoMontado, hoje string) AtividadeDoDia {
	t.Helper()

	for _, d := range diasDeEstudo(p) {
		if d.Data <= hoje {
			continue
		}

		for _, it := range d.Itens {
			if it.Passada == 1 && it.ID != uuid.Nil && len(plano.PartesDoTema(it.Tema)) == 1 {
				return it
			}
		}
	}

	t.Fatal("o cenário precisa de uma 1ª passada adiante")

	return AtividadeDoDia{}
}
