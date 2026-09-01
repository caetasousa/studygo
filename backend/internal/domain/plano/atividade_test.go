package plano_test

import (
	"errors"
	"testing"
	"time"

	"studygo/internal/domain/plano"
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
			// A day holding finished work does not freeze its other subjects: what
			// blocks a move is the ACTIVITY being done, which the service checks
			// against the records. Here the origin is concluded and the move is
			// still allowed.
			nome:      "move para fora de um dia concluído",
			id:        "a",
			destino:   dia(2026, 9, 2),
			concluido: func(t time.Time) bool { return t.Equal(dia(2026, 9, 1)) },
			querErro:  nil,
		},
		{
			// A content day's own "concluído" only describes the items it happened
			// to hold a moment ago — a stale snapshot, not a lock. This is exactly
			// the case antecipar exists for: today already fully done, and one more
			// subject finished early still has to land on it. See
			// TestMoverAtividade_NaoMoveParaDentroDeRevisaoConcluida for the day
			// type that DOES stay locked.
			nome:      "move para dentro de um dia de conteúdo concluído",
			id:        "a",
			destino:   dia(2026, 9, 2),
			concluido: func(t time.Time) bool { return t.Equal(dia(2026, 9, 2)) },
			querErro:  nil,
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

// A weekly-review day is never given items by the engine, so a "concluído"
// there is the student's own word for the whole day, not a derived snapshot —
// and that word is exactly what a move into it would silently contradict.
func TestMoverAtividade_NaoMoveParaDentroDeRevisaoConcluida(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{
		{N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo},
		{N: 2, Data: dia(2026, 9, 2), Tipo: plano.TipoRevisaoSemanal},
	}

	concluido := func(t time.Time) bool { return t.Equal(dia(2026, 9, 2)) }

	_, err := plano.MoverAtividade(atividadesBase(), dias, "a", dia(2026, 9, 2), 0, concluido)
	if !errors.Is(err, plano.ErrDiaConcluido) {
		t.Fatalf("erro = %v, quer ErrDiaConcluido", err)
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

// See TestMoverAtividade_Recusas: a content day's own "concluído" is a stale
// snapshot, not a lock, so a swap into one is allowed.
func TestTrocarAtividades_PermiteDiaDeConteudoConcluido(t *testing.T) {
	t.Parallel()

	concluido := func(dt time.Time) bool { return dt.Equal(dia(2026, 9, 2)) }

	_, err := plano.TrocarAtividades(
		atividadesBase(), diasBase(), "a", dia(2026, 9, 2), 0, concluido,
	)
	if err != nil {
		t.Fatalf("TrocarAtividades: %v", err)
	}
}

// A weekly-review day's "concluído" is the student's own word for the whole
// day, not a derived snapshot, and stays a lock.
func TestTrocarAtividades_RecusaRevisaoConcluida(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{
		{N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo},
		{N: 2, Data: dia(2026, 9, 2), Tipo: plano.TipoRevisaoSemanal},
	}

	concluido := func(dt time.Time) bool { return dt.Equal(dia(2026, 9, 2)) }

	_, err := plano.TrocarAtividades(atividadesBase(), dias, "a", dia(2026, 9, 2), 0, concluido)
	if !errors.Is(err, plano.ErrDiaConcluido) {
		t.Fatalf("erro = %v, quer ErrDiaConcluido", err)
	}
}

func TestIDDerivado_EstavelEResolvivel(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{
		{
			N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo,
			Itens: []plano.ItemDia{
				{Disciplina: "POR", Tema: "Crase"},
				{Disciplina: "MAT", Tema: "Frações"},
			},
		},
	}

	derivadas := plano.DerivarAtividades(dias)
	if len(derivadas) != 2 {
		t.Fatalf("derivadas = %d, quer 2", len(derivadas))
	}

	// Deterministic: deriving twice yields the same ids, which is what lets the
	// browser address an activity it was served earlier.
	outra := plano.DerivarAtividades(dias)
	for i := range derivadas {
		if derivadas[i].ID != outra[i].ID {
			t.Fatalf("id instável: %q vs %q", derivadas[i].ID, outra[i].ID)
		}

		if !plano.EhIDDerivado(derivadas[i].ID) {
			t.Fatalf("id %q devia ser reconhecido como derivado", derivadas[i].ID)
		}
	}

	// After materialisation the synthetic id must still find its activity.
	armazenadas := []plano.Atividade{
		{ID: "uuid-a", Data: dia(2026, 9, 1), Posicao: 0, Disciplina: "POR"},
		{ID: "uuid-b", Data: dia(2026, 9, 1), Posicao: 1, Disciplina: "MAT"},
	}

	got, ok := plano.ResolverIDDerivado(armazenadas, derivadas[1].ID)
	if !ok || got != "uuid-b" {
		t.Fatalf("ResolverIDDerivado = %q, %v; quer uuid-b, true", got, ok)
	}

	if _, ok := plano.ResolverIDDerivado(armazenadas, plano.IDDerivado(dia(2026, 9, 9), 0)); ok {
		t.Fatal("slot inexistente devia falhar em vez de resolver")
	}
}

// A move between days in different months must not confuse day-of-month with
// the date, which is why every id and lookup carries the full ISO date.
func TestMoverAtividade_EntreMeses(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{
		{N: 1, Data: dia(2026, 8, 31), Tipo: plano.TipoEstudo},
		{N: 2, Data: dia(2026, 9, 2), Tipo: plano.TipoEstudo},
	}

	atividades := []plano.Atividade{
		{ID: "a", Data: dia(2026, 8, 31), Posicao: 0, Disciplina: "POR"},
		{ID: "b", Data: dia(2026, 9, 2), Posicao: 0, Disciplina: "MAT"},
	}

	out, err := plano.TrocarAtividades(atividades, dias, "a", dia(2026, 9, 2), 0, nadaConcluido)
	if err != nil {
		t.Fatalf("TrocarAtividades: %v", err)
	}

	if got := posicoes(t, out, dia(2026, 9, 2)); len(got) != 1 || got[0] != "a" {
		t.Errorf("02/09 = %v, quer [a]", got)
	}

	if got := posicoes(t, out, dia(2026, 8, 31)); len(got) != 1 || got[0] != "b" {
		t.Errorf("31/08 = %v, quer [b]", got)
	}

	// Nothing may be duplicated or dropped by a swap.
	if len(out) != len(atividades) {
		t.Errorf("total = %d, quer %d", len(out), len(atividades))
	}
}

// Raising blocosPorDia after a day was arranged by hand must show the extra
// subject: the stored layout says WHAT sits in the day, not how many fit.
func TestAplicarAtividades_MostraBlocoNovo(t *testing.T) {
	t.Parallel()

	// The engine now generates three items for the day...
	dias := []plano.Dia{
		{
			N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo,
			Itens: []plano.ItemDia{
				{Disciplina: "POR", Tema: "Crase"},
				{Disciplina: "MAT", Tema: "Frações"},
				{Disciplina: "DIR", Tema: "Atos"},
			},
		},
	}

	// ...but only two were materialised, back when the day held two blocks. Each
	// records the generated slot it came from, so reconciliation knows those two
	// are already represented and only the third is new.
	d0, p0, p1 := dia(2026, 9, 1), 0, 1
	armazenadas := []plano.Atividade{
		{
			ID: "b", Data: d0, Posicao: 0, Disciplina: "MAT", Tema: "Frações",
			OrigemDia: &d0, OrigemPos: &p1,
		},
		{
			ID: "a", Data: d0, Posicao: 1, Disciplina: "POR", Tema: "Crase",
			OrigemDia: &d0, OrigemPos: &p0,
		},
	}

	plano.AplicarAtividades(dias, armazenadas)

	if len(dias[0].Itens) != 3 {
		t.Fatalf("itens = %d, quer 3 (os dois arranjados + o bloco novo)", len(dias[0].Itens))
	}

	// The user's arrangement is preserved, and the new block lands after it.
	if dias[0].Itens[0].Disciplina != "MAT" || dias[0].Itens[1].Disciplina != "POR" {
		t.Errorf("a ordem arranjada foi perdida: %v", dias[0].Itens)
	}

	if dias[0].Itens[2].Disciplina != "DIR" {
		t.Errorf("bloco novo = %q, quer DIR", dias[0].Itens[2].Disciplina)
	}

	// Every item must be addressable: the arranged ones by their stored id, the
	// newly generated one by its slot id.
	for _, it := range dias[0].Itens {
		if it.AtividadeID == "" {
			t.Errorf("item sem id: %+v", it)
		}
	}
}

// Lowering the count (or leaving it alone) must not invent items.
func TestAplicarAtividades_NaoInventaItens(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{
		{
			N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo,
			Itens: []plano.ItemDia{{Disciplina: "POR", Tema: "Crase"}},
		},
	}

	d0, p0 := dia(2026, 9, 1), 0
	armazenadas := []plano.Atividade{
		{
			ID: "a", Data: d0, Posicao: 0, Disciplina: "POR", Tema: "Crase",
			OrigemDia: &d0, OrigemPos: &p0,
		},
		// Moved in from another day: it claims no slot here, and must not be
		// mistaken for a generated one.
		{ID: "b", Data: d0, Posicao: 1, Disciplina: "MAT", Tema: "Frações"},
	}

	plano.AplicarAtividades(dias, armazenadas)

	if len(dias[0].Itens) != 2 {
		t.Fatalf("itens = %d, quer 2 (o que está guardado manda)", len(dias[0].Itens))
	}
}

// Raising blocosPorDia adds blocks the store has never seen. Those must become
// real activities on the next move, or dragging the new subject fails — while
// the ones already arranged are left exactly as they are.
func TestAtividadesFaltantes(t *testing.T) {
	t.Parallel()

	dias := []plano.Dia{
		{
			N: 1, Data: dia(2026, 9, 1), Tipo: plano.TipoEstudo,
			Itens: []plano.ItemDia{
				{Disciplina: "POR", Tema: "Crase"},
				{Disciplina: "MAT", Tema: "Frações"},
				{Disciplina: "DIR", Tema: "Atos"},
			},
		},
	}

	d0, p0, p1 := dia(2026, 9, 1), 0, 1
	existentes := []plano.Atividade{
		{
			ID: "a", Data: d0, Posicao: 0, Disciplina: "POR", Tema: "Crase",
			OrigemDia: &d0, OrigemPos: &p0,
		},
		{
			ID: "b", Data: d0, Posicao: 1, Disciplina: "MAT", Tema: "Frações",
			OrigemDia: &d0, OrigemPos: &p1,
		},
	}

	plano.AplicarAtividades(dias, existentes)

	faltantes := plano.AtividadesFaltantes(dias, existentes)

	if len(faltantes) != 1 {
		t.Fatalf("faltantes = %d, quer 1 (só o bloco novo)", len(faltantes))
	}

	if faltantes[0].Disciplina != "DIR" {
		t.Errorf("faltante = %q, quer DIR", faltantes[0].Disciplina)
	}

	// Its position must not collide with what is already stored for that day.
	for _, e := range existentes {
		if faltantes[0].Posicao == e.Posicao {
			t.Errorf("posição %d colide com atividade existente", faltantes[0].Posicao)
		}
	}

	// Nothing is missing once everything is stored. A materialised activity
	// records the generated slot it came from — that is what marks the slot as
	// already represented.
	p2 := 2
	todas := append(append([]plano.Atividade{}, existentes...), plano.Atividade{
		ID: "c", Data: d0, Posicao: 2, Disciplina: "DIR", Tema: "Atos",
		OrigemDia: &d0, OrigemPos: &p2,
	})

	plano.AplicarAtividades(dias, todas)

	if n := len(plano.AtividadesFaltantes(dias, todas)); n != 0 {
		t.Errorf("faltantes = %d, quer 0 quando tudo já está guardado", n)
	}
}

// Changing blocosPorDia after the plan was materialised: the days still ahead
// must be released back to the engine, while history, finished days and
// hand-moved blocks are kept.
func TestReterAoMudarRitmo(t *testing.T) {
	t.Parallel()

	hoje := dia(2026, 9, 10)

	d5, d12, d15 := dia(2026, 9, 5), dia(2026, 9, 12), dia(2026, 9, 15)
	p0, p1 := 0, 1

	atividades := []plano.Atividade{
		// Past day — kept as history.
		{ID: "passado", Data: d5, Posicao: 0, Disciplina: "POR", OrigemDia: &d5, OrigemPos: &p0},
		// Future, untouched — dropped so the engine can regenerate it.
		{ID: "futuro-a", Data: d12, Posicao: 0, Disciplina: "MAT", OrigemDia: &d12, OrigemPos: &p0},
		{ID: "futuro-b", Data: d12, Posicao: 1, Disciplina: "DIR", OrigemDia: &d12, OrigemPos: &p1},
		// Future but hand-moved (origin day differs) — kept.
		{ID: "movida", Data: d15, Posicao: 0, Disciplina: "INF", OrigemDia: &d12, OrigemPos: &p1},
		// Future, on a day the student marked done — kept.
		{ID: "concluida", Data: d15, Posicao: 1, Disciplina: "POR", OrigemDia: &d15, OrigemPos: &p1},
	}

	concluido := func(dt time.Time) bool { return dt.Equal(d15) }

	retidas := plano.ReterAoMudarRitmo(atividades, hoje, concluido)

	got := map[string]bool{}
	for _, a := range retidas {
		got[a.ID] = true
	}

	quer := []string{"passado", "movida", "concluida"}
	for _, id := range quer {
		if !got[id] {
			t.Errorf("%q deveria ter sido retida", id)
		}
	}

	for _, id := range []string{"futuro-a", "futuro-b"} {
		if got[id] {
			t.Errorf("%q deveria ter sido descartada (dia futuro, intocado)", id)
		}
	}
}

// A discipline with one topic but many slots on the same engine day (reparte's
// leftover repeats) materialises one row per slot. A later compaction can
// scatter them across many later days without breaking their shared origin —
// that shared origin is what marks them as one pile.
func TestDeduplicarAtividades_PilhaEspalhada(t *testing.T) {
	t.Parallel()

	dOrigem := dia(2026, 9, 2)
	d3, d4, d7 := dia(2026, 9, 3), dia(2026, 9, 4), dia(2026, 9, 7)
	p1, p2, p3 := 1, 2, 3

	atividades := []plano.Atividade{
		// Still on its original day and slot.
		{ID: "a", Data: dOrigem, Posicao: 1, Disciplina: "INTAR", Tema: "Fundamentos",
			OrigemDia: &dOrigem, OrigemPos: &p1},
		// Scattered by a compaction: different DATA, but same OrigemDia/topic.
		{ID: "b", Data: d3, Posicao: 0, Disciplina: "INTAR", Tema: "Fundamentos",
			OrigemDia: &dOrigem, OrigemPos: &p2},
		{ID: "c", Data: d4, Posicao: 0, Disciplina: "INTAR", Tema: "Fundamentos",
			OrigemDia: &dOrigem, OrigemPos: &p3},
		// A different subject on d7 — untouched.
		{ID: "d", Data: d7, Posicao: 0, Disciplina: "DESSI", Tema: "Algoritmos",
			OrigemDia: &d7, OrigemPos: new(int)},
	}

	got := plano.DeduplicarAtividades(atividades)

	if len(got) != 2 {
		t.Fatalf("got %d atividades, quer 2 (a pilha INTAR colapsada + DESSI): %+v", len(got), got)
	}

	ids := map[string]bool{}
	for _, a := range got {
		ids[a.ID] = true
	}

	if !ids["a"] {
		t.Error("a (menor OrigemPos da pilha) deveria ter sido mantida")
	}

	if ids["b"] || ids["c"] {
		t.Error("b e c são a mesma pilha de a — deveriam ter sido descartadas")
	}

	if !ids["d"] {
		t.Error("d é de outra disciplina/origem — não deveria ter sido tocada")
	}
}

// Two genuinely separate visits to the same topic — a spaced second pass
// generated on a different day — are not a pile, and must survive.
func TestDeduplicarAtividades_SegundaPassadaEmOutroDiaSobrevive(t *testing.T) {
	t.Parallel()

	d1, d2 := dia(2026, 9, 1), dia(2026, 9, 20)
	p0 := 0

	atividades := []plano.Atividade{
		{ID: "a", Data: d1, Posicao: 0, Disciplina: "INTAR", Tema: "Fundamentos",
			OrigemDia: &d1, OrigemPos: &p0},
		{ID: "b", Data: d2, Posicao: 0, Disciplina: "INTAR", Tema: "Fundamentos",
			OrigemDia: &d2, OrigemPos: &p0},
	}

	got := plano.DeduplicarAtividades(atividades)

	if len(got) != 2 {
		t.Fatalf("got %d atividades, quer 2 (origens diferentes, não é pilha)", len(got))
	}
}

// Reordenada DENTRO do mesmo dia: Movida() é true, mas o dia bate com a origem.
// É o único caso que a cláusula a.Movida() cobre sozinha.
func TestReterAoMudarRitmo_ReordenadaNoMesmoDia(t *testing.T) {
	t.Parallel()

	d := dia(2026, 9, 12)
	p0 := 0

	// origem pos=0, agora está na pos=2 do MESMO dia: o aluno reordenou à mão.
	atividades := []plano.Atividade{
		{ID: "reordenada", Data: d, Posicao: 2, Disciplina: "POR", OrigemDia: &d, OrigemPos: &p0},
	}

	got := plano.ReterAoMudarRitmo(atividades, dia(2026, 9, 1), func(time.Time) bool { return false })

	if len(got) != 1 {
		t.Fatalf("got %d, quer 1 — uma reordenação manual no mesmo dia não pode ser descartada", len(got))
	}
}

// O caso que só a supressão por TEMA resolve: o motor agenda o mesmo tema em
// DOIS slots do dia; a atividade de um deles foi levada para outro dia. O slot
// vizinho não pode regenerar o tema — a passagem já foi gasta.
func TestAplicarAtividades_SlotVizinhoNaoRegeneraTemaMovido(t *testing.T) {
	t.Parallel()

	d1, d2 := dia(2026, 9, 1), dia(2026, 9, 2)
	p0, p1 := 0, 1

	dias := []plano.Dia{
		{N: 1, Data: d1, Tipo: plano.TipoEstudo, Itens: []plano.ItemDia{
			{Disciplina: "LP", Tema: "Crase"},
		}},
		// O motor repete "Fundamentos" em dois slots do dia 2.
		{N: 2, Data: d2, Tipo: plano.TipoEstudo, Itens: []plano.ItemDia{
			{Disciplina: "IA", Tema: "Fundamentos"},
			{Disciplina: "IA", Tema: "Fundamentos"},
		}},
	}

	// A atividade do slot 0 do dia 2 foi adiantada para o dia 1.
	od := d2
	armazenadas := []plano.Atividade{
		{ID: "lp", Data: d1, Posicao: 0, Disciplina: "LP", Tema: "Crase",
			OrigemDia: &d1, OrigemPos: &p0},
		{ID: "movida", Data: d1, Posicao: 1, Disciplina: "IA", Tema: "Fundamentos",
			OrigemDia: &od, OrigemPos: &p0},
		{ID: "fica", Data: d2, Posicao: 0, Disciplina: "IA", Tema: "Fundamentos",
			OrigemDia: &od, OrigemPos: &p1},
	}

	plano.AplicarAtividades(dias, armazenadas)

	conta := func(d plano.Dia) int {
		n := 0
		for _, it := range d.Itens {
			if it.Disciplina == "IA" && it.Tema == "Fundamentos" {
				n++
			}
		}
		return n
	}

	if n := conta(dias[0]); n != 1 {
		t.Errorf("dia 1 mostra IA/Fundamentos %d vezes, quer 1", n)
	}

	// O dia 2 mantém UMA (a que ficou), nunca duas.
	if n := conta(dias[1]); n != 1 {
		t.Errorf("dia 2 mostra IA/Fundamentos %d vezes, quer 1", n)
	}

	if len(dias[1].Itens) == 0 {
		t.Error("dia 2 ficou vazio")
	}
}
