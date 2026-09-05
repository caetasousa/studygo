package plano_test

import (
	"errors"
	"testing"
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// Três dias de estudo seguidos, com duas atividades cada.
func diasReplan() []plano.Dia {
	return []plano.Dia{
		{N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo},
		{N: 2, Data: dia(2026, 9, 2), Tipo: plano.TipoEstudo},
		{N: 3, Data: dia(2026, 9, 3), Tipo: plano.TipoEstudo},
		{N: 4, Data: dia(2026, 9, 4), Tipo: plano.TipoEstudo},
	}
}

func atividadesReplan() []plano.Atividade {
	return []plano.Atividade{
		{ID: uid("a1"), Data: dia(2026, 9, 1), Posicao: 0, Disciplina: "POR", Tema: "p1"},
		{ID: uid("a2"), Data: dia(2026, 9, 1), Posicao: 1, Disciplina: "MAT", Tema: "m1"},
		{ID: uid("b1"), Data: dia(2026, 9, 2), Posicao: 0, Disciplina: "POR", Tema: "p2"},
		{ID: uid("b2"), Data: dia(2026, 9, 2), Posicao: 1, Disciplina: "MAT", Tema: "m2"},
		{ID: uid("c1"), Data: dia(2026, 9, 3), Posicao: 0, Disciplina: "POR", Tema: "p3"},
	}
}

func idsDoDia(as []plano.Atividade, dt time.Time) []uuid.UUID {
	out := []uuid.UUID{}
	for _, a := range plano.AtividadesDoDia(as, dt) {
		out = append(out, a.ID)
	}

	return out
}

// uid transforma um rótulo legível ("a1", "c2") num uuid estável, para que os
// testes continuem se lendo por nome enquanto o modelo usa identidade de
// verdade.
func uid(rotulo string) uuid.UUID {
	return uuid.NewSHA1(uuid.Nil, []byte(rotulo))
}

func TestAdiarDia(t *testing.T) {
	t.Parallel()

	t.Run("empurra o conteúdo e desloca o resto", func(t *testing.T) {
		t.Parallel()

		// Perdi o dia 2: o que era dele vai para o 3, e o que era do 3 vai para o 4.
		got, err := plano.AdiarDia(
			atividadesReplan(), diasReplan(), dia(2026, 9, 2), semDiaConcluido,
		)
		if err != nil {
			t.Fatalf("AdiarDia: %v", err)
		}

		if ids := idsDoDia(got, dia(2026, 9, 2)); len(ids) != 0 {
			t.Errorf("o dia adiado ficou com %v, devia esvaziar", ids)
		}

		if ids := idsDoDia(got, dia(2026, 9, 3)); len(ids) != 2 || ids[0] != uid("b1") {
			t.Errorf("dia 3 = %v, quer [b1 b2]", ids)
		}

		if ids := idsDoDia(got, dia(2026, 9, 4)); len(ids) != 1 || ids[0] != uid("c1") {
			t.Errorf("dia 4 = %v, quer [c1]", ids)
		}
	})

	t.Run("não mexe no que veio antes", func(t *testing.T) {
		t.Parallel()

		got, err := plano.AdiarDia(
			atividadesReplan(), diasReplan(), dia(2026, 9, 2), semDiaConcluido,
		)
		if err != nil {
			t.Fatalf("AdiarDia: %v", err)
		}

		if ids := idsDoDia(got, dia(2026, 9, 1)); len(ids) != 2 || ids[0] != uid("a1") {
			t.Errorf("dia 1 = %v, devia seguir intacto", ids)
		}
	})

	t.Run("nada se perde", func(t *testing.T) {
		t.Parallel()

		got, err := plano.AdiarDia(
			atividadesReplan(), diasReplan(), dia(2026, 9, 2), semDiaConcluido,
		)
		if err != nil {
			t.Fatalf("AdiarDia: %v", err)
		}

		if len(got) != len(atividadesReplan()) {
			t.Errorf("total = %d, quer %d", len(got), len(atividadesReplan()))
		}
	})

	t.Run("recusa adiar um dia já concluído", func(t *testing.T) {
		t.Parallel()

		concluido := func(d time.Time) bool { return d.Equal(dia(2026, 9, 2)) }

		_, err := plano.AdiarDia(atividadesReplan(), diasReplan(), dia(2026, 9, 2), concluido)
		if !errors.Is(err, plano.ErrDiaConcluido) {
			t.Errorf("erro = %v, quer ErrDiaConcluido", err)
		}
	})

	t.Run("recusa quando não há para onde empurrar", func(t *testing.T) {
		t.Parallel()

		_, err := plano.AdiarDia(atividadesReplan(), diasReplan(), dia(2026, 9, 4), semDiaConcluido)
		if !errors.Is(err, plano.ErrDestinoInvalido) {
			t.Errorf("erro = %v, quer ErrDestinoInvalido", err)
		}
	})
}

func TestAntecipouAtividade(t *testing.T) {
	t.Parallel()

	t.Run("traz o assunto adiantado para hoje", func(t *testing.T) {
		t.Parallel()

		// Estou no dia 1 e terminei c1, que estava no dia 3.
		got, err := plano.AntecipouAtividade(
			atividadesReplan(), diasReplan(), uid("c1"), dia(2026, 9, 1), semDiaConcluido,
		)
		if err != nil {
			t.Fatalf("AntecipouAtividade: %v", err)
		}

		ids := idsDoDia(got, dia(2026, 9, 1))
		if len(ids) != 3 || ids[2] != uid("c1") {
			t.Errorf("dia 1 = %v, quer c1 no fim", ids)
		}

		if ids := idsDoDia(got, dia(2026, 9, 3)); len(ids) != 0 {
			t.Errorf("dia 3 = %v, devia ficar vazio", ids)
		}
	})

	t.Run("a ordem do que ficou é preservada e densa", func(t *testing.T) {
		t.Parallel()

		// Trago b1 (dia 2, posição 0): b2 sobe para a posição 0.
		got, err := plano.AntecipouAtividade(
			atividadesReplan(), diasReplan(), uid("b1"), dia(2026, 9, 1), semDiaConcluido,
		)
		if err != nil {
			t.Fatalf("AntecipouAtividade: %v", err)
		}

		restantes := plano.AtividadesDoDia(got, dia(2026, 9, 2))
		if len(restantes) != 1 || restantes[0].ID != uid("b2") || restantes[0].Posicao != 0 {
			t.Errorf("dia 2 = %+v, quer só b2 na posição 0", restantes)
		}
	})

	t.Run("nada se perde nem se duplica", func(t *testing.T) {
		t.Parallel()

		got, err := plano.AntecipouAtividade(
			atividadesReplan(), diasReplan(), uid("c1"), dia(2026, 9, 1), semDiaConcluido,
		)
		if err != nil {
			t.Fatalf("AntecipouAtividade: %v", err)
		}

		if len(got) != len(atividadesReplan()) {
			t.Errorf("total = %d, quer %d", len(got), len(atividadesReplan()))
		}
	})

	t.Run("assunto de hoje ou do passado não se move", func(t *testing.T) {
		t.Parallel()

		got, err := plano.AntecipouAtividade(
			atividadesReplan(), diasReplan(), uid("a1"), dia(2026, 9, 2), semDiaConcluido,
		)
		if err != nil {
			t.Fatalf("AntecipouAtividade: %v", err)
		}

		if ids := idsDoDia(got, dia(2026, 9, 1)); len(ids) != 2 {
			t.Errorf("dia 1 = %v, não devia mudar", ids)
		}
	})

	t.Run("id desconhecido é recusado", func(t *testing.T) {
		t.Parallel()

		_, err := plano.AntecipouAtividade(
			atividadesReplan(), diasReplan(), uid("zzz"), dia(2026, 9, 1), semDiaConcluido,
		)
		if !errors.Is(err, plano.ErrAtividadeNaoEncontrada) {
			t.Errorf("erro = %v, quer ErrAtividadeNaoEncontrada", err)
		}
	})
}

func TestCompactarAtividades(t *testing.T) {
	t.Parallel()

	t.Run("puxa tudo para trás e não deixa buraco", func(t *testing.T) {
		t.Parallel()

		// O dia 2 ficou vazio; o conteúdo dos dias 3 e 4 sobe.
		ats := []plano.Atividade{
			{ID: uid("a1"), Data: dia(2026, 9, 1), Posicao: 0, Disciplina: "POR", Tema: "p1"},
			{ID: uid("a2"), Data: dia(2026, 9, 1), Posicao: 1, Disciplina: "MAT", Tema: "m1"},
			{ID: uid("c1"), Data: dia(2026, 9, 3), Posicao: 0, Disciplina: "POR", Tema: "p2"},
			{ID: uid("c2"), Data: dia(2026, 9, 3), Posicao: 1, Disciplina: "MAT", Tema: "m2"},
			{ID: uid("d1"), Data: dia(2026, 9, 4), Posicao: 0, Disciplina: "POR", Tema: "p3"},
		}

		got := plano.CompactarAtividades(ats, diasReplan(), dia(2026, 9, 1), semDiaConcluido)

		if ids := idsDoDia(got, dia(2026, 9, 2)); len(ids) != 2 {
			t.Errorf("dia 2 = %v, devia ter sido preenchido", ids)
		}

		if ids := idsDoDia(got, dia(2026, 9, 4)); len(ids) != 0 {
			t.Errorf("dia 4 = %v, devia ter esvaziado no fim", ids)
		}
	})

	t.Run("nada se perde nem se duplica", func(t *testing.T) {
		t.Parallel()

		got := plano.CompactarAtividades(
			atividadesReplan(), diasReplan(), dia(2026, 9, 1), semDiaConcluido,
		)

		if len(got) != len(atividadesReplan()) {
			t.Fatalf("total = %d, quer %d", len(got), len(atividadesReplan()))
		}

		vistos := map[uuid.UUID]bool{}
		for _, a := range got {
			if vistos[a.ID] {
				t.Fatalf("id duplicado: %s", a.ID)
			}

			vistos[a.ID] = true
		}
	})

	t.Run("a ordem do conteúdo é preservada", func(t *testing.T) {
		t.Parallel()

		ats := []plano.Atividade{
			{ID: uid("x1"), Data: dia(2026, 9, 3), Posicao: 0, Disciplina: "POR", Tema: "p1"},
			{ID: uid("x2"), Data: dia(2026, 9, 3), Posicao: 1, Disciplina: "MAT", Tema: "m1"},
			{ID: uid("x3"), Data: dia(2026, 9, 4), Posicao: 0, Disciplina: "DIR", Tema: "d1"},
		}

		got := plano.CompactarAtividades(ats, diasReplan(), dia(2026, 9, 1), semDiaConcluido)

		// Sobem para o dia 1 na mesma sequência em que estavam.
		if ids := idsDoDia(got, dia(2026, 9, 1)); len(ids) < 2 || ids[0] != uid("x1") || ids[1] != uid("x2") {
			t.Errorf("dia 1 = %v, quer x1 antes de x2", ids)
		}
	})

	t.Run("um dia já registrado é âncora e não se mexe", func(t *testing.T) {
		t.Parallel()

		concluido := func(d time.Time) bool { return d.Equal(dia(2026, 9, 1)) }

		got := plano.CompactarAtividades(
			atividadesReplan(), diasReplan(), dia(2026, 9, 1), concluido,
		)

		if ids := idsDoDia(got, dia(2026, 9, 1)); len(ids) != 2 || ids[0] != uid("a1") {
			t.Errorf("dia registrado = %v, devia seguir intacto", ids)
		}
	})

	t.Run("posições ficam densas", func(t *testing.T) {
		t.Parallel()

		got := plano.CompactarAtividades(
			atividadesReplan(), diasReplan(), dia(2026, 9, 1), semDiaConcluido,
		)

		for _, dt := range []time.Time{dia(2026, 9, 1), dia(2026, 9, 2), dia(2026, 9, 3)} {
			for i, a := range plano.AtividadesDoDia(got, dt) {
				if a.Posicao != i {
					t.Errorf("%s posição %d = %d, quer %d", dt.Format(time.DateOnly), i, a.Posicao, i)
				}
			}
		}
	})

	t.Run("não mexe no que veio antes do corte", func(t *testing.T) {
		t.Parallel()

		got := plano.CompactarAtividades(
			atividadesReplan(), diasReplan(), dia(2026, 9, 2), semDiaConcluido,
		)

		if ids := idsDoDia(got, dia(2026, 9, 1)); len(ids) != 2 || ids[0] != uid("a1") {
			t.Errorf("dia anterior ao corte = %v, não devia mudar", ids)
		}
	})
}

// The learning phase and the reta final are different kinds of work. Compacting
// across the boundary pulled a guided review back among the content days, which
// is the plan losing its shape rather than getting tighter.
func TestCompactarAtividades_NaoAtravessaAFase(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{
		{N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
		{N: 2, Data: dia(2026, 9, 2), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
		{N: 3, Data: dia(2026, 9, 3), Tipo: plano.TipoRevisaoDirigida, Fase: plano.FaseReta},
	}

	ats := []plano.Atividade{
		{ID: uid("r1"), Data: dia(2026, 9, 3), Posicao: 0, Disciplina: "POR", Tema: "revisão"},
	}

	got := plano.CompactarAtividades(ats, dias, dia(2026, 9, 1), semDiaConcluido)

	if ids := idsDoDia(got, dia(2026, 9, 3)); len(ids) != 1 || ids[0] != uid("r1") {
		t.Errorf("a revisão da reta final saiu do lugar: %v", got)
	}

	for _, d := range []time.Time{dia(2026, 9, 1), dia(2026, 9, 2)} {
		if ids := idsDoDia(got, d); len(ids) != 0 {
			t.Errorf("%s recebeu conteúdo da reta final: %v", d.Format(time.DateOnly), ids)
		}
	}
}

// A completion recorded per activity leaves the day row behind when the work
// moves to the day it was really done on. That row used to anchor an EMPTY day,
// so the hole it marked could never be closed again.
func TestCompactarAtividades_DiaVazioNaoAncora(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{
		{N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
		{N: 2, Data: dia(2026, 9, 2), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
		{N: 3, Data: dia(2026, 9, 3), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
	}

	// Os dois primeiros dias estão marcados como concluídos mas não têm nada:
	// o que era deles foi adiantado e já está registrado noutro dia.
	ats := []plano.Atividade{
		{ID: uid("c1"), Data: dia(2026, 9, 3), Posicao: 0, Disciplina: "POR", Tema: "p3"},
		{ID: uid("c2"), Data: dia(2026, 9, 3), Posicao: 1, Disciplina: "MAT", Tema: "m3"},
	}

	concluido := func(d time.Time) bool {
		return d.Equal(dia(2026, 9, 1)) || d.Equal(dia(2026, 9, 2))
	}

	got := plano.CompactarAtividades(ats, dias, dia(2026, 9, 1), concluido)

	if ids := idsDoDia(got, dia(2026, 9, 1)); len(ids) != 2 {
		t.Errorf("dia 1 = %v, o buraco devia ter sido fechado", ids)
	}

	if ids := idsDoDia(got, dia(2026, 9, 3)); len(ids) != 0 {
		t.Errorf("dia 3 = %v, devia ter esvaziado no fim", ids)
	}
}

// Um dia de revisão semanal não guarda atividades: o trabalho dele é o próprio
// dia. Concluído, continua concluído, e não vira destino de conteúdo.
func TestCompactarAtividades_RevisaoSemanalConcluidaAncora(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{
		{N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoRevisaoSemanal, Fase: plano.FaseBase},
		{N: 2, Data: dia(2026, 9, 2), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
	}

	ats := []plano.Atividade{
		{ID: uid("b1"), Data: dia(2026, 9, 2), Posicao: 0, Disciplina: "POR", Tema: "p2"},
	}

	concluido := func(d time.Time) bool { return d.Equal(dia(2026, 9, 1)) }

	got := plano.CompactarAtividades(ats, dias, dia(2026, 9, 1), concluido)

	if ids := idsDoDia(got, dia(2026, 9, 1)); len(ids) != 0 {
		t.Errorf("a revisão semanal concluída recebeu conteúdo: %v", ids)
	}
}

func diasComReta() []plano.Dia {
	return []plano.Dia{
		{N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
		{N: 2, Data: dia(2026, 9, 2), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
		{N: 3, Data: dia(2026, 9, 3), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
		{N: 4, Data: dia(2026, 9, 4), Tipo: plano.TipoRevisaoDirigida, Fase: plano.FaseReta},
	}
}

// Dois dias cheios e o terceiro vazio — o retrato de quem se adiantou e viu a
// compactação empurrar a folga para a véspera da reta final.
func atividadesComFolga() []plano.Atividade {
	return []plano.Atividade{
		{ID: uid("a1"), Data: dia(2026, 9, 1), Posicao: 0, Disciplina: "POR", Tema: "p1"},
		{ID: uid("a2"), Data: dia(2026, 9, 1), Posicao: 1, Disciplina: "MAT", Tema: "m1"},
		{ID: uid("b1"), Data: dia(2026, 9, 2), Posicao: 0, Disciplina: "POR", Tema: "p2"},
		{ID: uid("b2"), Data: dia(2026, 9, 2), Posicao: 1, Disciplina: "MAT", Tema: "m2"},
	}
}

func temasDoDia(as []plano.Atividade, dt time.Time) []string {
	out := []string{}
	for _, a := range plano.AtividadesDoDia(as, dt) {
		out = append(out, a.Tema)
	}

	return out
}

func TestPreencherVazios(t *testing.T) {
	t.Parallel()

	fila := []plano.ItemRevisao{
		{Disciplina: "MAT", Tema: "m1"},
		{Disciplina: "POR", Tema: "p1"},
	}

	t.Run("o dia livre recebe a carga normal do plano", func(t *testing.T) {
		t.Parallel()

		got := plano.PreencherVazios(atividadesComFolga(), diasComReta(), plano.Reforco{
			Fila:      fila,
			Desde:     dia(2026, 9, 1),
			Concluido: semDiaConcluido,
		})

		temas := temasDoDia(got, dia(2026, 9, 3))
		if len(temas) != 2 || temas[0] != "Reforço — m1" || temas[1] != "Reforço — p1" {
			t.Errorf("dia livre = %v, quer os dois primeiros da fila", temas)
		}
	})

	t.Run("não inventa trabalho na reta final", func(t *testing.T) {
		t.Parallel()

		got := plano.PreencherVazios(atividadesComFolga(), diasComReta(), plano.Reforco{
			Fila:      fila,
			Desde:     dia(2026, 9, 1),
			Concluido: semDiaConcluido,
		})

		if ids := idsDoDia(got, dia(2026, 9, 4)); len(ids) != 0 {
			t.Errorf("a reta final recebeu reforço: %v", ids)
		}
	})

	t.Run("não mexe num dia já fechado", func(t *testing.T) {
		t.Parallel()

		got := plano.PreencherVazios(atividadesComFolga(), diasComReta(), plano.Reforco{
			Fila:      fila,
			Desde:     dia(2026, 9, 1),
			Concluido: func(d time.Time) bool { return d.Equal(dia(2026, 9, 3)) },
		})

		if ids := idsDoDia(got, dia(2026, 9, 3)); len(ids) != 0 {
			t.Errorf("dia fechado = %v, devia seguir intacto", ids)
		}
	})

	t.Run("uma fila curta não se repete dentro do dia", func(t *testing.T) {
		t.Parallel()

		got := plano.PreencherVazios(atividadesComFolga(), diasComReta(), plano.Reforco{
			Fila:      fila[:1],
			Desde:     dia(2026, 9, 1),
			Concluido: semDiaConcluido,
		})

		if temas := temasDoDia(got, dia(2026, 9, 3)); len(temas) != 1 {
			t.Errorf("dia livre = %v, quer um único bloco", temas)
		}
	})

	t.Run("sem fila, nada é inventado", func(t *testing.T) {
		t.Parallel()

		got := plano.PreencherVazios(atividadesComFolga(), diasComReta(), plano.Reforco{
			Desde:     dia(2026, 9, 1),
			Concluido: semDiaConcluido,
		})

		if len(got) != len(atividadesComFolga()) {
			t.Errorf("total = %d, quer %d", len(got), len(atividadesComFolga()))
		}
	})

	// PreencherVazios runs again every time a new topic is finished early, each
	// time possibly freeing a day of its own. A second call that restarted the
	// queue at 0 would hand the SECOND free day the exact same topics as the
	// first — the reinforcement would keep circling the same couple of subjects
	// while everything else studied never comes back.
	t.Run("uma segunda chamada continua a fila em vez de recomeçar", func(t *testing.T) {
		t.Parallel()

		dias := []plano.Dia{
			{N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
			{N: 2, Data: dia(2026, 9, 2), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
			{N: 3, Data: dia(2026, 9, 3), Tipo: plano.TipoEstudo, Fase: plano.FaseBase},
			{N: 4, Data: dia(2026, 9, 4), Tipo: plano.TipoRevisaoDirigida, Fase: plano.FaseReta},
		}

		filaLonga := []plano.ItemRevisao{
			{Disciplina: "MAT", Tema: "m1"},
			{Disciplina: "POR", Tema: "p1"},
			{Disciplina: "DIR", Tema: "d1"},
			{Disciplina: "ADM", Tema: "a1"},
		}

		// Só o dia 1 tem conteúdo, na carga normal de 2 blocos: os dias 2 e 3
		// estão livres, como se dois assuntos tivessem sido adiantados em
		// momentos separados.
		base := []plano.Atividade{
			{ID: uid("a1"), Data: dia(2026, 9, 1), Posicao: 0, Disciplina: "POR", Tema: "p1"},
			{ID: uid("a2"), Data: dia(2026, 9, 1), Posicao: 1, Disciplina: "MAT", Tema: "m1"},
		}

		// Primeira chamada: preenche o dia 2 com os dois primeiros da fila.
		primeira := plano.PreencherVazios(base, dias, plano.Reforco{
			Fila:      filaLonga,
			Desde:     dia(2026, 9, 1),
			Concluido: semDiaConcluido,
		})

		// Segunda chamada, como se fosse uma nova conclusão: parte do resultado
		// da primeira, não da entrada original.
		segunda := plano.PreencherVazios(primeira, dias, plano.Reforco{
			Fila:      filaLonga,
			Desde:     dia(2026, 9, 1),
			Concluido: semDiaConcluido,
		})

		temas2 := temasDoDia(segunda, dia(2026, 9, 2))
		temas3 := temasDoDia(segunda, dia(2026, 9, 3))

		if len(temas2) != 2 || len(temas3) != 2 {
			t.Fatalf("dia 2 = %v, dia 3 = %v, os dois deviam ter sido preenchidos", temas2, temas3)
		}

		if temas2[0] == temas3[0] {
			t.Errorf("dia 3 repetiu o tema do dia 2 em vez de continuar a fila: %v e %v", temas2, temas3)
		}
	})
}

func TestFilaDeReforco(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{
		{N: 1, Data: dia(2026, 9, 1), Fase: plano.FaseBase, Itens: []plano.ItemDia{
			{Disciplina: "POR", Tema: "p1"},
			{Disciplina: "MAT", Tema: "m1"},
		}},
		{N: 2, Data: dia(2026, 9, 2), Fase: plano.FaseBase, Itens: []plano.ItemDia{
			{Disciplina: "POR", Tema: "p2"},
			// Já reforçado numa passada anterior: é o mesmo tema, não um novo.
			{Disciplina: "MAT", Tema: "Reforço — m1"},
			// Uma revisão dirigida que a compactação trouxe para a base também
			// nomeia o tema de baixo, não o rótulo.
			{Disciplina: "MAT", Tema: "Revisão dirigida — m2"},
		}},
		{N: 3, Data: dia(2026, 9, 3), Fase: plano.FaseReta, Itens: []plano.ItemDia{
			{Disciplina: "DIR", Tema: "d1"},
		}},
	}

	cadernos := map[string][]plano.ItemCaderno{
		"MAT": {{Disciplina: "MAT", Tema: "m1", Erros: 2, Questoes: 10, Acertos: 2}},
	}

	got := plano.FilaDeReforco(dias, cadernos)

	quer := []string{"m1", "p1", "p2", "m2"}
	if len(got) != len(quer) {
		t.Fatalf("fila = %v, quer %v", got, quer)
	}

	for i, tema := range quer {
		if got[i].Tema != tema {
			t.Errorf("fila[%d] = %q, quer %q", i, got[i].Tema, tema)
		}
	}
}

// Um dia vencido só conta como atrasado se ninguém o estudou: dia cumprido é
// história, e dia de hoje ainda está correndo.
func TestDiasAtrasados(t *testing.T) {
	as := atividadesReplan()
	hoje := dia(2026, 9, 3)

	casos := []struct {
		nome      string
		concluida func(uuid.UUID) bool
		quer      []time.Time
	}{
		{
			nome:      "nada estudado deixa os dois dias vencidos atrasados",
			concluida: func(uuid.UUID) bool { return false },
			quer:      []time.Time{dia(2026, 9, 1), dia(2026, 9, 2)},
		},
		{
			nome: "dia inteiro estudado sai da lista",
			concluida: func(id uuid.UUID) bool {
				return id == uid("a1") || id == uid("a2")
			},
			quer: []time.Time{dia(2026, 9, 2)},
		},
		{
			nome:      "dia pela metade continua atrasado",
			concluida: func(id uuid.UUID) bool { return id == uid("a1") },
			quer:      []time.Time{dia(2026, 9, 1), dia(2026, 9, 2)},
		},
		{
			nome:      "tudo estudado não deixa atraso",
			concluida: func(uuid.UUID) bool { return true },
			quer:      []time.Time{},
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := plano.DiasAtrasados(as, hoje, c.concluida)

			if len(got) != len(c.quer) {
				t.Fatalf("dias atrasados = %v, quer %v", got, c.quer)
			}

			for i := range got {
				if !got[i].Equal(c.quer[i]) {
					t.Errorf("dia[%d] = %s, quer %s", i, got[i], c.quer[i])
				}
			}
		})
	}
}

// O dia perdido fica vazio; hoje, o futuro e o que tem registro continuam.
func TestSemAtrasadas(t *testing.T) {
	as := atividadesReplan()
	hoje := dia(2026, 9, 2)

	// a1 foi estudada no dia 1; a2 não.
	concluida := func(id uuid.UUID) bool { return id == uid("a1") }

	got := plano.SemAtrasadas(as, hoje, concluida)

	if ids := idsDoDia(got, dia(2026, 9, 1)); len(ids) != 1 || ids[0] != uid("a1") {
		t.Errorf("dia perdido = %v, quer só a atividade com registro (a1)", ids)
	}

	if ids := idsDoDia(got, dia(2026, 9, 2)); len(ids) != 2 {
		t.Errorf("hoje = %v, quer as duas intactas", ids)
	}

	if ids := idsDoDia(got, dia(2026, 9, 3)); len(ids) != 1 {
		t.Errorf("futuro = %v, quer intacto", ids)
	}
}
