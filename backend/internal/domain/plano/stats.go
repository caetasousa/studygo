package plano

import "time"

// Agregação do que foi estudado, por disciplina e no total.
//
// Com o cronograma materializado, cada lançamento pertence a UMA atividade, que
// pertence a UMA disciplina. O rateio que existia aqui — dividir as horas do dia
// igualmente entre os blocos dele — era a aproximação necessária quando o
// registro era do dia inteiro; agora ele seria uma segunda resposta, pior, para
// uma pergunta que os dados já respondem exatamente.

// StatDisciplina são os totais atribuídos a uma disciplina.
type StatDisciplina struct {
	Horas      float64
	Concluido  float64
	Questoes   float64
	Acertos    float64
	Blocos     int
	BlocosReta int
}

// StatExtras agrega os dias especiais (simulado, discursiva, véspera, revisão).
type StatExtras struct {
	Dias      int
	Horas     float64
	Concluido int
	Questoes  int
	Acertos   int
}

// Stats é o agregado completo sobre um plano e seus registros.
type Stats struct {
	Disciplina    map[string]StatDisciplina
	Extras        StatExtras
	HorasTotal    float64
	Feitos        int
	QuestoesTotal int
	AcertosTotal  int
}

// CalcularStats agrega o cronograma gravado com o que foi lançado nele.
func CalcularStats(
	dias []Dia,
	codigos []string,
	atividades []Atividade,
	registros Registros,
) Stats {
	st := Stats{Disciplina: map[string]StatDisciplina{}}

	for _, k := range codigos {
		st.Disciplina[k] = StatDisciplina{}
	}

	fasePorDia := make(map[string]Fase, len(dias))
	for _, d := range dias {
		fasePorDia[chaveDia(d.Data)] = d.Fase
	}

	for _, d := range dias {
		dt := day(d.Data)
		doDia := AtividadesDoDia(atividades, dt)

		if len(doDia) == 0 {
			continue
		}

		diaDeConteudo := false

		for _, a := range doDia {
			if !a.Tipo.DeDiaInteiro() {
				diaDeConteudo = true
			}
		}

		if !diaDeConteudo {
			st.Extras.Dias++
		}

		concluidoODia := DiaConcluido(atividades, registros, dt)
		if concluidoODia {
			st.Feitos++
		}

		for _, a := range doDia {
			reg, temReg := registros[a.ID]

			horas := naoNuloFloat(reg.Horas)
			questoes := naoNuloInt(reg.Questoes)
			acertos := naoNuloInt(reg.Acertos)

			if temReg {
				st.HorasTotal += horas
				st.QuestoesTotal += questoes
				st.AcertosTotal += acertos
			}

			if a.Tipo.DeDiaInteiro() {
				if temReg {
					st.Extras.Horas += horas
					st.Extras.Questoes += questoes
					st.Extras.Acertos += acertos

					if reg.Concluido {
						st.Extras.Concluido++
					}
				}

				continue
			}

			alvo, conhecida := st.Disciplina[a.Disciplina]
			if !conhecida {
				continue
			}

			alvo.Blocos++

			if fasePorDia[chaveDia(a.Data)] == FaseReta {
				alvo.BlocosReta++
			}

			if temReg {
				alvo.Horas += horas
				alvo.Questoes += float64(questoes)
				alvo.Acertos += float64(acertos)

				if reg.Concluido {
					alvo.Concluido++
				}
			}

			st.Disciplina[a.Disciplina] = alvo
		}
	}

	return st
}

func chaveDia(t time.Time) string {
	return day(t).Format(time.DateOnly)
}
