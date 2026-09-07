package plano_test

import (
	"testing"
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// SemConteudoJaConcluido é a regra que separa o que já foi coberto do que
// repete por desenho.
//
// Ela existe porque `Gerar` é puro: ele devolve o currículo inteiro, sempre do
// começo, e quem replaneja precisa descontar o histórico. Sem o desconto, uma
// redistribuição devolve à fila tudo que o estudante já estudou.

func atv(disciplina, tema string, passada int) plano.Atividade {
	return plano.Atividade{
		ID:         uuid.New(),
		Data:       time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
		Disciplina: disciplina,
		Tema:       tema,
		Passada:    passada,
		Tipo:       plano.AtividadeConteudo,
	}
}

// concluidas devolve o predicado que marca as atividades dadas como concluídas.
func concluidas(as ...plano.Atividade) func(uuid.UUID) bool {
	feito := map[uuid.UUID]bool{}
	for _, a := range as {
		feito[a.ID] = true
	}

	return func(id uuid.UUID) bool { return feito[id] }
}

func TestSemConteudoJaConcluido_DescontaACobertura(t *testing.T) {
	t.Parallel()

	estudada := atv("LINPO", "Crase", 1)
	atuais := []plano.Atividade{estudada}

	novas := []plano.Atividade{
		atv("LINPO", "Crase", 1),        // já coberta: sai
		atv("LINPO", "Concordância", 1), // ainda não: fica
		atv("BANDA", "SQL", 1),          // outra matéria: fica
	}

	out := plano.SemConteudoJaConcluido(novas, atuais, concluidas(estudada))

	if len(out) != 2 {
		t.Fatalf("sobraram %d atividades, quer 2", len(out))
	}

	for _, a := range out {
		if a.Disciplina == "LINPO" && a.Tema == "Crase" {
			t.Error("a cobertura já concluída voltou para a fila")
		}
	}
}

// O outro lado da regra, e o erro que ela precisa não cometer: a passada 2 é
// repetição por desenho — é ela que dá volume de questões até a prova. Estudar
// uma sessão não encerra o tema.
func TestSemConteudoJaConcluido_PreservaARepeticao(t *testing.T) {
	t.Parallel()

	estudada := atv("BANDA", "SQL", 2)
	atuais := []plano.Atividade{estudada}

	novas := []plano.Atividade{
		atv("BANDA", "SQL", 2),
		atv("BANDA", "SQL", 2),
		atv("BANDA", "SQL", 2),
	}

	out := plano.SemConteudoJaConcluido(novas, atuais, concluidas(estudada))

	if len(out) != 3 {
		t.Errorf("sobraram %d repetições, quer as 3: resolver questões uma vez não encerra o tema", len(out))
	}
}

// Concluir a passada 2 não pode descontar a passada 1, nem o contrário: são
// naturezas diferentes, e só a primeira é cobertura.
func TestSemConteudoJaConcluido_NaoCruzaAsPassadas(t *testing.T) {
	t.Parallel()

	repeticao := atv("BANDA", "SQL", 2)

	out := plano.SemConteudoJaConcluido(
		[]plano.Atividade{atv("BANDA", "SQL", 1)},
		[]plano.Atividade{repeticao},
		concluidas(repeticao),
	)

	if len(out) != 1 {
		t.Error("concluir uma repetição não pode descontar a cobertura do tema")
	}
}

// Dia fixo do método não é conteúdo a cobrir: simulado, discursiva, revisão
// semanal e véspera repetem de propósito, e descontá-los apagaria a estrutura
// do plano a cada replanejamento.
func TestSemConteudoJaConcluido_IgnoraOsDiasFixos(t *testing.T) {
	t.Parallel()

	simulado := plano.Atividade{
		ID: uuid.New(), Passada: 1, Tipo: plano.AtividadeSimulado, Tema: "Simulado",
	}

	novas := []plano.Atividade{
		{ID: uuid.New(), Passada: 1, Tipo: plano.AtividadeSimulado, Tema: "Simulado"},
		{ID: uuid.New(), Passada: 1, Tipo: plano.AtividadeDiscursiva, Tema: "Discursiva"},
	}

	out := plano.SemConteudoJaConcluido(novas, []plano.Atividade{simulado}, concluidas(simulado))

	if len(out) != 2 {
		t.Errorf("sobraram %d dias fixos, quer 2", len(out))
	}
}

// Nada concluído: a lista volta como veio, sem alocação nem surpresa.
func TestSemConteudoJaConcluido_SemHistoricoNaoMexe(t *testing.T) {
	t.Parallel()

	novas := []plano.Atividade{atv("LINPO", "Crase", 1), atv("BANDA", "SQL", 1)}

	out := plano.SemConteudoJaConcluido(novas, []plano.Atividade{atv("LINPO", "Crase", 1)}, concluidas())

	if len(out) != len(novas) {
		t.Errorf("sobraram %d, quer %d: nada foi concluído", len(out), len(novas))
	}
}

// Uma atividade que existe no cronograma mas NÃO foi concluída não desconta
// nada: agendar não é estudar.
func TestSemConteudoJaConcluido_AgendarNaoEEstudar(t *testing.T) {
	t.Parallel()

	agendada := atv("LINPO", "Crase", 1)

	out := plano.SemConteudoJaConcluido(
		[]plano.Atividade{atv("LINPO", "Crase", 1)},
		[]plano.Atividade{agendada},
		concluidas(), // nenhuma concluída
	)

	if len(out) != 1 {
		t.Error("uma atividade só agendada não pode descontar a cobertura")
	}
}
