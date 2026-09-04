package plano

import (
	"time"

	"github.com/google/uuid"
)

// O que o estudante fez.
//
// A unidade é a ATIVIDADE, não o dia: um dia que agenda a mesma disciplina duas
// vezes tem dois registros independentes, e concluir um não conclui o outro. A
// conclusão do DIA é derivada daqui — nunca informada pelo cliente.

// RegistroAtividade é o lançamento de uma atividade agendada. Um zero é dado
// real ("resolvi 10 e acertei 0"), então os campos numéricos são ponteiros: nil
// é "não lancei nada".
type RegistroAtividade struct {
	AtividadeID uuid.UUID
	Horas       *float64
	Questoes    *int
	Acertos     *int
	Nota        string
	Concluido   bool
}

// RegistroDia é o que pertence ao dia e não a uma atividade: a anotação livre e
// o resultado da cauda de revisão.
//
// A cauda não é uma atividade: o motor a deriva da fila de revisão a cada
// montagem (ver FilaRevisao), então não existe unidade gravada a que prendê-la —
// a data é a única coisa estável.
type RegistroDia struct {
	Data            time.Time
	Nota            string
	RevisaoQuestoes *int
	RevisaoAcertos  *int
}

// Registros é o histórico de um plano, indexado pela atividade.
type Registros map[uuid.UUID]RegistroAtividade

// De devolve o registro de uma atividade, ou nil.
func (r Registros) De(id uuid.UUID) *RegistroAtividade {
	if reg, ok := r[id]; ok {
		return &reg
	}

	return nil
}

// Concluida diz se uma atividade foi marcada como concluída. É o que a torna
// imóvel: mover o que já foi estudado reescreveria a história.
func (r Registros) Concluida(id uuid.UUID) bool {
	reg, ok := r[id]

	return ok && reg.Concluido
}

// DiaConcluido diz se um dia está terminado: quando TODA atividade agendada
// para ele está concluída.
//
// É derivado, e é por isso que não existe coluna para ele. Contar só as
// atividades que têm registro marcaria um dia de duas matérias como completo
// assim que a primeira fosse lançada — exatamente o que a conclusão por
// atividade existe para impedir. Um dia sem atividade nenhuma não está
// concluído: não há o que concluir.
func DiaConcluido(atividades []Atividade, registros Registros, dt time.Time) bool {
	doDia := AtividadesDoDia(atividades, dt)
	if len(doDia) == 0 {
		return false
	}

	for _, a := range doDia {
		if !registros.Concluida(a.ID) {
			return false
		}
	}

	return true
}

// DiaConcluidoFunc devolve o predicado "este dia está concluído?" que o
// replanejamento usa para saber o que não pode tocar.
func DiaConcluidoFunc(atividades []Atividade, registros Registros) func(time.Time) bool {
	return func(dt time.Time) bool {
		return DiaConcluido(atividades, registros, dt)
	}
}

// TotaisDoDia soma o que foi lançado nas atividades de um dia.
func TotaisDoDia(
	atividades []Atividade,
	registros Registros,
	dt time.Time,
) (horas *float64, questoes, acertos *int) {
	for _, a := range AtividadesDoDia(atividades, dt) {
		reg, ok := registros[a.ID]
		if !ok {
			continue
		}

		if reg.Horas != nil {
			h := naoNuloFloat(horas) + *reg.Horas
			horas = &h
		}

		if reg.Questoes != nil {
			q := naoNuloInt(questoes) + *reg.Questoes
			questoes = &q
		}

		if reg.Acertos != nil {
			a := naoNuloInt(acertos) + *reg.Acertos
			acertos = &a
		}
	}

	return horas, questoes, acertos
}

// ConcluidasNoDia conta quantas atividades do dia estão concluídas.
func ConcluidasNoDia(atividades []Atividade, registros Registros, dt time.Time) int {
	n := 0

	for _, a := range AtividadesDoDia(atividades, dt) {
		if registros.Concluida(a.ID) {
			n++
		}
	}

	return n
}

// AcertosValidos corta acertos maiores que as questões, que é a única
// combinação que quebra a estatística. Devolve nil quando não há acertos.
func AcertosValidos(questoes, acertos *int) *int {
	if acertos == nil {
		return nil
	}

	if questoes != nil && *acertos > *questoes {
		q := *questoes

		return &q
	}

	a := *acertos

	return &a
}

func naoNuloFloat(p *float64) float64 {
	if p == nil {
		return 0
	}

	return *p
}

func naoNuloInt(p *int) int {
	if p == nil {
		return 0
	}

	return *p
}
