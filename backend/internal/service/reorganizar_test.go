package service

import (
	"testing"
	"time"

	"studygo/internal/domain/plano"
)

func d(a int, m time.Month, dd int) time.Time {
	return time.Date(a, m, dd, 0, 0, 0, 0, time.UTC)
}

// A completion is recorded per activity. When the activity moves to the day it
// was really finished on, the day row stays behind describing a day that now
// holds nothing — and that row is enough to anchor the hole open forever.
func TestRegistrosOrfaos(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{
		{N: 1, Data: d(2026, 9, 1), Tipo: plano.TipoEstudo},
		{N: 2, Data: d(2026, 9, 2), Tipo: plano.TipoEstudo},
		{N: 3, Data: d(2026, 9, 3), Tipo: plano.TipoEstudo},
		{N: 4, Data: d(2026, 9, 4), Tipo: plano.TipoSimulado},
	}

	atividades := []plano.Atividade{
		{ID: "a1", Data: d(2026, 9, 3), Disciplina: "POR", Tema: "p1"},
	}

	tests := []struct {
		nome      string
		registros map[time.Time]plano.Registro
		quer      []time.Time
	}{
		{
			"o dia esvaziado perde o registro vazio",
			map[time.Time]plano.Registro{
				d(2026, 9, 1): {Data: d(2026, 9, 1), Concluido: true},
			},
			[]time.Time{d(2026, 9, 1)},
		},
		{
			"o dia que ainda tem atividade fica",
			map[time.Time]plano.Registro{
				d(2026, 9, 3): {Data: d(2026, 9, 3), Concluido: true},
			},
			nil,
		},
		{
			"horas lançadas são do estudante, não sobra",
			map[time.Time]plano.Registro{
				d(2026, 9, 1): {Data: d(2026, 9, 1), Concluido: true, Horas: ptrF(2)},
			},
			nil,
		},
		{
			"uma bateria respondida também fica",
			map[time.Time]plano.Registro{
				d(2026, 9, 2): {Data: d(2026, 9, 2), Questoes: ptrI(30), Acertos: ptrI(20)},
			},
			nil,
		},
		{
			"blocos gravados também ficam",
			map[time.Time]plano.Registro{
				d(2026, 9, 2): {
					Data:   d(2026, 9, 2),
					Blocos: []plano.RegistroBloco{{Disciplina: "POR", Concluido: true}},
				},
			},
			nil,
		},
		{
			"o simulado não guarda atividades e por isso não é órfão",
			map[time.Time]plano.Registro{
				d(2026, 9, 4): {Data: d(2026, 9, 4), Concluido: true},
			},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			t.Parallel()

			got := registrosOrfaos(dias, atividades, tt.registros)

			if len(got) != len(tt.quer) {
				t.Fatalf("órfãos = %v, quer %v", got, tt.quer)
			}

			for i := range got {
				if !got[i].Equal(tt.quer[i]) {
					t.Errorf("órfão[%d] = %s, quer %s", i, got[i], tt.quer[i])
				}
			}
		})
	}
}

// The orphan rows are dropped only after the new layout is written, so the
// compaction has to read them as gone while they are still in the database.
func TestDiaConcluidoSem(t *testing.T) {
	t.Parallel()

	registros := map[time.Time]plano.Registro{
		d(2026, 9, 1): {Data: d(2026, 9, 1), Concluido: true},
		d(2026, 9, 2): {Data: d(2026, 9, 2), Concluido: true},
	}

	concluido := diaConcluidoSem(registros, []time.Time{d(2026, 9, 1)})

	if concluido(d(2026, 9, 1)) {
		t.Error("o dia órfão ainda ancora a compactação")
	}

	if !concluido(d(2026, 9, 2)) {
		t.Error("um dia realmente concluído deixou de ancorar")
	}
}
