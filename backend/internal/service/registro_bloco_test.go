package service

import (
	"testing"

	"annygo/internal/domain/concurso"
	"annygo/internal/domain/plano"
)

func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int         { return &v }

// A day is finished only when every activity scheduled for it is — counting the
// blocks that happened to arrive would finish a two-subject day on the first
// save.
func TestConcluidasNoDia(t *testing.T) {
	t.Parallel()

	tests := []struct {
		nome     string
		blocos   []plano.RegistroBloco
		esperado int
	}{
		{"nenhum", nil, 0},
		{
			"uma de duas",
			[]plano.RegistroBloco{
				{AtividadeID: "a", Concluido: true},
				{AtividadeID: "b", Concluido: false},
			},
			1,
		},
		{
			"todas",
			[]plano.RegistroBloco{
				{AtividadeID: "a", Concluido: true},
				{AtividadeID: "b", Concluido: true},
			},
			2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			t.Parallel()

			if got := concluidasNoDia(tt.blocos); got != tt.esperado {
				t.Errorf("concluidasNoDia = %d, quer %d", got, tt.esperado)
			}
		})
	}
}

// Two activities of the same discipline in one day must survive as two rows:
// deduping by discipline is what silently merged them before.
func TestBlocosDoInput_MesmaDisciplinaDuasVezes(t *testing.T) {
	t.Parallel()

	c := concurso.Concurso{
		Disciplinas: []concurso.Disciplina{{Codigo: "POR", Nome: "Português"}},
	}

	in := []RegistroBlocoInput{
		{Disciplina: "POR", AtividadeID: "atv-1", Horas: ptrF(2), Concluido: true},
		{Disciplina: "POR", AtividadeID: "atv-2", Horas: ptrF(1)},
	}

	got := blocosDoInput(c, in)
	if len(got) != 2 {
		t.Fatalf("blocos = %d, quer 2 (uma por atividade)", len(got))
	}

	if got[0].AtividadeID != "atv-1" || got[1].AtividadeID != "atv-2" {
		t.Errorf("ids = %q, %q", got[0].AtividadeID, got[1].AtividadeID)
	}

	if !got[0].Concluido || got[1].Concluido {
		t.Error("a conclusão de uma atividade vazou para a outra")
	}
}

// Without an activity id the old (day, discipline) key still applies, so a
// legacy client cannot write two conflicting rows for one discipline.
func TestBlocosDoInput_SemAtividadeDedupePorDisciplina(t *testing.T) {
	t.Parallel()

	c := concurso.Concurso{
		Disciplinas: []concurso.Disciplina{{Codigo: "POR", Nome: "Português"}},
	}

	got := blocosDoInput(c, []RegistroBlocoInput{
		{Disciplina: "POR", Horas: ptrF(2)},
		{Disciplina: "POR", Horas: ptrF(9)},
	})

	if len(got) != 1 {
		t.Fatalf("blocos = %d, quer 1", len(got))
	}
}

// Ticking an activity with nothing else filled in is a real state and must not
// be dropped as "empty".
func TestBlocosDoInput_ConcluidoSozinhoPersiste(t *testing.T) {
	t.Parallel()

	c := concurso.Concurso{
		Disciplinas: []concurso.Disciplina{{Codigo: "POR", Nome: "Português"}},
	}

	got := blocosDoInput(c, []RegistroBlocoInput{
		{Disciplina: "POR", AtividadeID: "atv-1", Concluido: true},
	})

	if len(got) != 1 || !got[0].Concluido {
		t.Fatalf("bloco só com concluido foi descartado: %+v", got)
	}

	// ...but a genuinely empty one still is.
	if vazio := blocosDoInput(c, []RegistroBlocoInput{
		{Disciplina: "POR", AtividadeID: "atv-2"},
	}); len(vazio) != 0 {
		t.Errorf("bloco vazio devia ser descartado, veio %+v", vazio)
	}
}

// Unchecking must not erase what was recorded.
func TestBlocosDoInput_DesmarcarPreservaNumeros(t *testing.T) {
	t.Parallel()

	c := concurso.Concurso{
		Disciplinas: []concurso.Disciplina{{Codigo: "POR", Nome: "Português"}},
	}

	got := blocosDoInput(c, []RegistroBlocoInput{
		{
			Disciplina: "POR", AtividadeID: "atv-1",
			Horas: ptrF(2), Questoes: ptrI(20), Acertos: ptrI(15),
			Nota: "revisar crase", Concluido: false,
		},
	})

	if len(got) != 1 {
		t.Fatalf("blocos = %d, quer 1", len(got))
	}

	b := got[0]
	if b.Horas == nil || *b.Horas != 2 || b.Questoes == nil || *b.Questoes != 20 ||
		b.Acertos == nil || *b.Acertos != 15 || b.Nota != "revisar crase" {
		t.Errorf("desmarcar apagou dados: %+v", b)
	}
}
