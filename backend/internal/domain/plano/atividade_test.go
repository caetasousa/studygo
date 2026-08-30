package plano_test

import (
	"errors"
	"testing"
	"time"

	"annygo/internal/domain/plano"
)

// `dia` lives in revisao_test.go — same package, same helper.

// Three consecutive content days, which is what a move needs to have somewhere
// to go.
func diasBase() []plano.Dia {
	return []plano.Dia{
		{N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo},
		{N: 2, Data: dia(2026, 9, 2), Tipo: plano.TipoEstudo},
		{N: 3, Data: dia(2026, 9, 3), Tipo: plano.TipoSimulado},
	}
}

func atividadesBase() []plano.Atividade {
	return []plano.Atividade{
		{ID: "a", Data: dia(2026, 9, 1), Posicao: 0, Disciplina: "POR", Tema: "Crase"},
		{ID: "b", Data: dia(2026, 9, 1), Posicao: 1, Disciplina: "MAT", Tema: "Frações"},
		{ID: "c", Data: dia(2026, 9, 1), Posicao: 2, Disciplina: "DIR", Tema: "Atos"},
		{ID: "d", Data: dia(2026, 9, 2), Posicao: 0, Disciplina: "INF", Tema: "Redes"},
	}
}

func nadaConcluido(time.Time) bool { return false }

// posicoes reports the id order of one day, which is what every move assertion
// is really about.
func posicoes(t *testing.T, as []plano.Atividade, dt time.Time) []string {
	t.Helper()

	out := []string{}
	for _, a := range plano.AtividadesDoDia(as, dt) {
		out = append(out, a.ID)
	}

	return out
}

func TestMoverAtividade(t *testing.T) {
	t.Parallel()

	tests := []struct {
		nome     string
		id       string
		destino  time.Time
		posicao  int
		esperado map[time.Time][]string
	}{
		{
			nome:    "sobe dentro do mesmo dia",
			id:      "c",
			destino: dia(2026, 9, 1),
			posicao: 0,
			esperado: map[time.Time][]string{
				dia(2026, 9, 1): {"c", "a", "b"},
			},
		},
		{
			nome:    "desce dentro do mesmo dia",
			id:      "a",
			destino: dia(2026, 9, 1),
			posicao: 2,
			esperado: map[time.Time][]string{
				dia(2026, 9, 1): {"b", "c", "a"},
			},
		},
		{
			nome:    "move para outro dia sem levar o resto junto",
			id:      "b",
			destino: dia(2026, 9, 2),
			posicao: 0,
			esperado: map[time.Time][]string{
				dia(2026, 9, 1): {"a", "c"},
				dia(2026, 9, 2): {"b", "d"},
			},
		},
		{
			nome:    "posição além do fim vai para o fim",
			id:      "a",
			destino: dia(2026, 9, 2),
			posicao: 99,
			esperado: map[time.Time][]string{
				dia(2026, 9, 1): {"b", "c"},
				dia(2026, 9, 2): {"d", "a"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			t.Parallel()

			out, err := plano.MoverAtividade(
				atividadesBase(), diasBase(), tt.id, tt.destino, tt.posicao, nadaConcluido,
			)
			if err != nil {
				t.Fatalf("MoverAtividade: %v", err)
			}

			for dt, want := range tt.esperado {
				got := posicoes(t, out, dt)
				if len(got) != len(want) {
					t.Fatalf("dia %s: ordem = %v, quer %v", dt.Format("02/01"), got, want)
				}

				for i := range want {
					if got[i] != want[i] {
						t.Fatalf("dia %s: ordem = %v, quer %v", dt.Format("02/01"), got, want)
					}
				}
			}
		})
	}
}

func TestMoverAtividade_PosicoesFicamDensasEUnicas(t *testing.T) {
	t.Parallel()

	out, err := plano.MoverAtividade(
		atividadesBase(), diasBase(), "b", dia(2026, 9, 2), 1, nadaConcluido,
	)
	if err != nil {
		t.Fatalf("MoverAtividade: %v", err)
	}

	for _, dt := range []time.Time{dia(2026, 9, 1), dia(2026, 9, 2)} {
		vistas := map[int]bool{}
		for i, a := range plano.AtividadesDoDia(out, dt) {
			if a.Posicao != i {
				t.Errorf("dia %s: posição %d na ordem %d — deveria ser densa", dt.Format("02/01"), a.Posicao, i)
			}

			if vistas[a.Posicao] {
				t.Errorf("dia %s: posição %d duplicada", dt.Format("02/01"), a.Posicao)
			}

			vistas[a.Posicao] = true
		}
	}
}

func TestMoverAtividade_Recusas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		nome      string
		id        string
		destino   time.Time
		concluido func(time.Time) bool
		querErro  error
	}{
		{
			nome:      "atividade inexistente",
			id:        "zzz",
			destino:   dia(2026, 9, 2),
			concluido: nadaConcluido,
			querErro:  plano.ErrAtividadeNaoEncontrada,
		},
		{
			nome:      "dia fora do plano",
			id:        "a",
			destino:   dia(2027, 1, 1),
			concluido: nadaConcluido,
			querErro:  plano.ErrDestinoInvalido,
		},
		{
			nome:      "dia fixo não aceita atividade",
			id:        "a",
			destino:   dia(2026, 9, 3), // simulado
			concluido: nadaConcluido,
			querErro:  plano.ErrDestinoInvalido,
		},
		{
			nome:      "não move para fora de um dia concluído",
			id:        "a",
			destino:   dia(2026, 9, 2),
			concluido: func(t time.Time) bool { return t.Equal(dia(2026, 9, 1)) },
			querErro:  plano.ErrDiaConcluido,
		},
		{
			nome:      "não move para dentro de um dia concluído",
			id:        "a",
			destino:   dia(2026, 9, 2),
			concluido: func(t time.Time) bool { return t.Equal(dia(2026, 9, 2)) },
			querErro:  plano.ErrDiaConcluido,
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			t.Parallel()

			_, err := plano.MoverAtividade(
				atividadesBase(), diasBase(), tt.id, tt.destino, 0, tt.concluido,
			)
			if !errors.Is(err, tt.querErro) {
				t.Fatalf("erro = %v, quer %v", err, tt.querErro)
			}
		})
	}
}

func TestMoverAtividade_NaoAlteraEntrada(t *testing.T) {
	t.Parallel()

	entrada := atividadesBase()

	if _, err := plano.MoverAtividade(
		entrada, diasBase(), "a", dia(2026, 9, 2), 0, nadaConcluido,
	); err != nil {
		t.Fatalf("MoverAtividade: %v", err)
	}

	// The caller's slice must be untouched, so a failed persist can simply
	// discard the result instead of having to undo a mutation.
	if !entrada[0].Data.Equal(dia(2026, 9, 1)) || entrada[0].Posicao != 0 {
		t.Fatalf("entrada foi mutada: %+v", entrada[0])
	}
}

func TestDerivarAtividades_PreservaOrdemEOrigem(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{{
		N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo,
		Itens: []plano.ItemDia{
			{Disciplina: "POR", Tema: "Crase", Passada: 1},
			{Disciplina: "MAT", Tema: "Frações", Passada: 1},
		},
	}}

	as := plano.DerivarAtividades(dias)
	if len(as) != 2 {
		t.Fatalf("derivou %d atividades, quer 2", len(as))
	}

	if as[0].Disciplina != "POR" || as[0].Posicao != 0 || as[1].Posicao != 1 {
		t.Fatalf("ordem incorreta: %+v", as)
	}

	// Freshly derived activities sit exactly where the engine put them.
	for _, a := range as {
		if a.Movida() {
			t.Errorf("atividade recém-derivada não deveria contar como movida: %+v", a)
		}
	}
}

func TestAtividade_MovidaDetectaAjusteManual(t *testing.T) {
	t.Parallel()

	origemDia := dia(2026, 9, 1)
	origemPos := 0

	a := plano.Atividade{
		ID: "a", Data: dia(2026, 9, 1), Posicao: 0,
		OrigemDia: &origemDia, OrigemPos: &origemPos,
	}

	if a.Movida() {
		t.Error("no lugar de origem não é movida")
	}

	outroDia := a
	outroDia.Data = dia(2026, 9, 5)

	if !outroDia.Movida() {
		t.Error("data diferente da origem deveria contar como movida")
	}

	outraPos := a
	outraPos.Posicao = 2

	if !outraPos.Movida() {
		t.Error("posição diferente da origem deveria contar como movida")
	}
}

func TestMinutosPlanejados_UsaPadraoQuandoNaoDefinido(t *testing.T) {
	t.Parallel()

	as := []plano.Atividade{
		{ID: "a", Data: dia(2026, 9, 1), Posicao: 0},                 // usa o padrão
		{ID: "b", Data: dia(2026, 9, 1), Posicao: 1, DuracaoMin: 90}, // valor próprio
	}

	if got := plano.MinutosPlanejados(as, dia(2026, 9, 1), 50); got != 140 {
		t.Fatalf("minutos = %d, quer 140", got)
	}
}

func TestTrocarAtividades(t *testing.T) {
	t.Parallel()

	tests := []struct {
		nome     string
		id       string
		destino  time.Time
		posicao  int
		esperado map[time.Time][]string
	}{
		{
			// The case this exists for: the target day is occupied, and the two
			// subjects exchange places instead of one day growing.
			nome:    "troca com quem ocupa o lugar no outro dia",
			id:      "a",
			destino: dia(2026, 9, 2),
			posicao: 0,
			esperado: map[time.Time][]string{
				dia(2026, 9, 1): {"d", "b", "c"},
				dia(2026, 9, 2): {"a"},
			},
		},
		{
			nome:    "troca dentro do mesmo dia",
			id:      "a",
			destino: dia(2026, 9, 1),
			posicao: 2,
			esperado: map[time.Time][]string{
				dia(2026, 9, 1): {"c", "b", "a"},
			},
		},
		{
			// Nothing to swap with, so it must behave like a plain move rather
			// than refusing or silently doing nothing.
			nome:    "sem ocupante vira movimento simples",
			id:      "a",
			destino: dia(2026, 9, 2),
			posicao: 5,
			esperado: map[time.Time][]string{
				dia(2026, 9, 1): {"b", "c"},
				dia(2026, 9, 2): {"d", "a"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			t.Parallel()

			out, err := plano.TrocarAtividades(
				atividadesBase(), diasBase(), tt.id, tt.destino, tt.posicao, nadaConcluido,
			)
			if err != nil {
				t.Fatalf("TrocarAtividades: %v", err)
			}

			for dt, quer := range tt.esperado {
				tem := posicoes(t, out, dt)
				if len(tem) != len(quer) {
					t.Fatalf("dia %s: ordem = %v, quer %v", dt.Format(time.DateOnly), tem, quer)
				}

				for i := range quer {
					if tem[i] != quer[i] {
						t.Fatalf("dia %s: ordem = %v, quer %v", dt.Format(time.DateOnly), tem, quer)
					}
				}
			}
		})
	}
}

func TestTrocarAtividades_RecusaDiaConcluido(t *testing.T) {
	t.Parallel()

	concluido := func(dt time.Time) bool { return dt.Equal(dia(2026, 9, 2)) }

	_, err := plano.TrocarAtividades(
		atividadesBase(), diasBase(), "a", dia(2026, 9, 2), 0, concluido,
	)
	if !errors.Is(err, plano.ErrDiaConcluido) {
		t.Fatalf("erro = %v, quer ErrDiaConcluido", err)
	}
}
