package plano_test

import (
	"testing"
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// ArrumarEstudado põe o cronograma de acordo com o que o estudante já estudou.
// Como pode errar — escrito antes do código:
//
//	R1  o tópico já estudado volta adiante com outra caixa ("Pipelines" × "pipelines") e se repete
//	R2  o bloco pendente que junta um tópico já estudado continua com ele
//	R3  a atividade repetida que já tem algo lançado é apagada, e o lançamento se perde
//	R4  marcar num dia que não é de estudo (domingo) deixa a atividade concluída na data futura — "feita lá no final"
//	R5  marcar num dia de estudo não traz a atividade para hoje
//	R6  a 2ª passada e a revisão de um tópico estudado somem (são repetição por desenho)
//	R7  nada a arrumar, e mesmo assim diz que mudou (gravação à toa)

func diaUtil(m time.Month, d int) time.Time {
	return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC)
}

// Dias de estudo de segunda a sexta, de 14/09 a 09/10.
func diasSegASex() []plano.Dia {
	out := []plano.Dia{}
	for d := diaUtil(time.September, 14); d.Before(diaUtil(time.October, 10)); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		out = append(out, plano.Dia{N: len(out) + 1, Data: d, Tipo: plano.TipoEstudo, Fase: plano.FaseBase})
	}

	return out
}

func conteudo(rotulo string, data time.Time, pos int, disc, tema string, passada int) plano.Atividade {
	return plano.Atividade{ID: uid(rotulo), Data: data, Posicao: pos, Disciplina: disc, Tema: tema, Passada: passada, Tipo: plano.AtividadeConteudo}
}

func conjunto(rotulos ...string) func(uuid.UUID) bool {
	m := map[uuid.UUID]bool{}
	for _, r := range rotulos {
		m[uid(r)] = true
	}

	return func(id uuid.UUID) bool { return m[id] }
}

func TestArrumarEstudado(t *testing.T) {
	t.Parallel()

	domingo := diaUtil(time.September, 27)
	atividades := []plano.Atividade{
		conteudo("e1", diaUtil(time.September, 14), 0, "DEVOPS", "Pipelines de desenvolvimento", 1),
		conteudo("e2", diaUtil(time.September, 15), 0, "PT", "Ortografia", 1),
		conteudo("sex", diaUtil(time.September, 25), 0, "RLM", "Frações", 1),
		conteudo("r1", diaUtil(time.September, 29), 0, "DEVOPS", "pipelines  de desenvolvimento", 1), // R1
		conteudo("x1", diaUtil(time.September, 29), 1, "INFRA", "Redes", 1),
		conteudo("b1", diaUtil(time.September, 30), 0, "PT", "ortografia  ·  Crase", 1),          // R2
		conteudo("l1", diaUtil(time.October, 1), 0, "DEVOPS", "Pipelines de Desenvolvimento", 1), // R3
		conteudo("p2", diaUtil(time.October, 2), 0, "DEVOPS", "Pipelines de desenvolvimento", 2), // R6
		conteudo("f1", diaUtil(time.October, 7), 0, "BD", "SQL", 1),                              // R4
	}
	concluida := conjunto("e1", "e2", "sex", "f1")
	lancada := conjunto("e1", "e2", "sex", "f1", "l1")

	out, mudou := plano.ArrumarEstudado(atividades, diasSegASex(), domingo, concluida, lancada)
	if !mudou {
		t.Fatal("havia o que arrumar, e disse que não mudou")
	}

	porID := map[uuid.UUID]plano.Atividade{}
	for _, a := range out {
		porID[a.ID] = a
	}

	if _, ok := porID[uid("r1")]; ok {
		t.Error("R1: a repetição com outra caixa continuou no cronograma")
	}

	if got := porID[uid("b1")].Tema; got != "Crase" {
		t.Errorf("R2: o bloco ficou com %q, quer %q", got, "Crase")
	}

	if _, ok := porID[uid("l1")]; !ok {
		t.Error("R3: a atividade com lançamento foi apagada")
	}

	f1 := porID[uid("f1")]
	if !f1.Data.Equal(diaUtil(time.September, 25)) || f1.Posicao != 1 {
		t.Errorf("R4: a concluída ficou em %v/%d, quer a sexta 25/09 depois do que ela já tinha", f1.Data, f1.Posicao)
	}

	if _, ok := porID[uid("p2")]; !ok {
		t.Error("R6: a 2ª passada do tópico estudado sumiu")
	}

	if _, ok := porID[uid("x1")]; !ok {
		t.Error("o que não foi estudado sumiu")
	}
}

func TestArrumarEstudado_DiaDeEstudoTrazParaHoje(t *testing.T) {
	t.Parallel()

	terca := diaUtil(time.September, 29)
	atividades := []plano.Atividade{
		conteudo("h1", terca, 0, "INFRA", "Redes", 1),
		conteudo("f1", diaUtil(time.October, 7), 0, "BD", "SQL", 1),
	}

	out, mudou := plano.ArrumarEstudado(atividades, diasSegASex(), terca, conjunto("f1"), conjunto("f1"))
	if !mudou {
		t.Fatal("a concluída adiante tinha de vir para hoje")
	}

	for _, a := range out {
		if a.ID == uid("f1") && (!a.Data.Equal(terca) || a.Posicao != 1) {
			t.Errorf("R5: ficou em %v/%d, quer hoje, depois de Redes", a.Data, a.Posicao)
		}
	}
}

func TestArrumarEstudado_NadaAArrumar(t *testing.T) {
	t.Parallel()

	atividades := []plano.Atividade{
		conteudo("e1", diaUtil(time.September, 14), 0, "DEVOPS", "Pipelines", 1),
		conteudo("x1", diaUtil(time.September, 29), 0, "INFRA", "Redes", 1),
	}

	if _, mudou := plano.ArrumarEstudado(atividades, diasSegASex(), diaUtil(time.September, 27), conjunto("e1"), conjunto("e1")); mudou {
		t.Error("R7: nada a arrumar, e disse que mudou")
	}
}

// A mesma comparação vale no replanejamento: o motor regenera o tópico com a
// caixa do edital, e o estudado pode ter outra.
func TestSemConteudoJaConcluido_IgnoraCaixaEEspacos(t *testing.T) {
	t.Parallel()

	estudada := atv("DEVOPS", "Pipelines de desenvolvimento", 1)
	out := plano.SemConteudoJaConcluido(
		[]plano.Atividade{atv("DEVOPS", "pipelines  de desenvolvimento", 1)},
		[]plano.Atividade{estudada},
		concluidas(estudada),
	)

	if len(out) != 0 {
		t.Errorf("o tópico estudado voltou com outra caixa: %+v", out)
	}
}

// A antecipada some do cronograma, mas continua existindo: é o registro de que
// o tópico foi estudado. Como pode errar — escrito antes do código:
//
//	R8  o que foi estudado antes da hora não fica marcado como antecipado, e aparece como linha no dia
//	R9  o estudo do próprio dia (concluído na data) é marcado como antecipado e some
func TestArrumarEstudado_MarcaAntecipada(t *testing.T) {
	t.Parallel()

	domingo := diaUtil(time.September, 27)
	atividades := []plano.Atividade{
		conteudo("sex", diaUtil(time.September, 25), 0, "RLM", "Frações", 1),
		conteudo("f1", diaUtil(time.October, 7), 0, "BD", "SQL", 1),
	}

	out, _ := plano.ArrumarEstudado(atividades, diasSegASex(), domingo, conjunto("sex", "f1"), conjunto("sex", "f1"))
	for _, a := range out {
		switch a.ID {
		case uid("f1"):
			if !a.Antecipada {
				t.Error("R8: o estudado antes da hora não ficou marcado como antecipado")
			}
		case uid("sex"):
			if a.Antecipada {
				t.Error("R9: o estudo do próprio dia virou antecipado")
			}
		}
	}
}

// TopicosEstudados alimenta o balanceamento: quantos tópicos da matéria já
// foram estudados. Como pode errar — escrito antes do código:
//
//	E1  o tópico estudado duas vezes (1ª passada e antecipado) conta duas
//	E2  o bloco que junta dois tópicos conta um só, ou conta tópico de outra matéria
//	E3  a grafia diferente ("pipelines" × "Pipelines") deixa de contar
//	E4  a 2ª passada ou a atividade pendente contam como estudo
func TestTopicosEstudados(t *testing.T) {
	t.Parallel()

	d := diaUtil(time.September, 14)
	atividades := []plano.Atividade{
		conteudo("a", d, 0, "DEVOPS", "Pipelines de desenvolvimento", 1),
		conteudo("b", d, 1, "DEVOPS", "pipelines de desenvolvimento", 1), // E1, E3
		conteudo("c", d, 2, "DEVOPS", "Git  ·  Docker", 1),               // E2
		conteudo("d", d, 3, "INFRA", "Docker", 1),                        // E2: outra matéria
		conteudo("e", d, 4, "DEVOPS", "Kubernetes", 2),                   // E4
		conteudo("f", d, 5, "DEVOPS", "Terraform", 1),                    // E4: pendente
	}
	temas := []string{"Pipelines de desenvolvimento", "Git", "Docker", "Kubernetes", "Terraform"}

	got := plano.TopicosEstudados("DEVOPS", temas, atividades, conjunto("a", "b", "c", "d", "e"))
	if got != 3 {
		t.Errorf("estudados = %d, quer 3 (Pipelines, Git, Docker)", got)
	}
}
