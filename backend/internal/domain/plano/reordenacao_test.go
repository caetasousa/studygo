package plano_test

import (
	"testing"
	"time"

	"annygo/internal/domain/plano"
)

func TestTrocarEAplicarReordenacoes(t *testing.T) {
	t.Parallel()

	c := loadConcurso(t)
	cfg := defaultConfig(c)

	base := plano.Gerar(cfg, &c)

	// Pick the first two distinct content days.
	var dtA, dtB time.Time
	for _, d := range base.Dias {
		if len(d.Itens) == 0 {
			continue
		}

		if dtA.IsZero() {
			dtA = d.Data
			continue
		}

		dtB = d.Data
		break
	}

	origA := findItens(base.Dias, dtA)
	origB := findItens(base.Dias, dtB)

	nadaConcluido := func(time.Time) bool { return false }

	ovA, ovB, err := plano.Trocar(base.Dias, dtA, dtB, nadaConcluido)
	if err != nil {
		t.Fatalf("Trocar: %v", err)
	}

	// The override for A must carry B's original content and vice versa.
	if !sameItens(ovA.Itens, origB) {
		t.Errorf("override A = %v, want B's content %v", ovA.Itens, origB)
	}

	if !sameItens(ovB.Itens, origA) {
		t.Errorf("override B = %v, want A's content %v", ovB.Itens, origA)
	}

	// Regenerating from scratch and reapplying the overrides must reproduce the swap.
	regen := plano.Gerar(cfg, &c)
	overrides := map[time.Time]plano.Reordenacao{
		plano.DayOf(dtA): ovA,
		plano.DayOf(dtB): ovB,
	}

	validas := plano.AplicarReordenacoes(regen.Dias, overrides)
	if len(validas) != 2 {
		t.Fatalf("expected 2 surviving overrides, got %d", len(validas))
	}

	if !sameItens(findItens(regen.Dias, dtA), origB) {
		t.Errorf("after reapply, day A = %v, want %v", findItens(regen.Dias, dtA), origB)
	}

	if !sameItens(findItens(regen.Dias, dtB), origA) {
		t.Errorf("after reapply, day B = %v, want %v", findItens(regen.Dias, dtB), origA)
	}
}

func TestTrocar_RejeitaDiaConcluido(t *testing.T) {
	t.Parallel()

	c := loadConcurso(t)
	res := plano.Gerar(defaultConfig(c), &c)

	var dtA, dtB time.Time
	for _, d := range res.Dias {
		if len(d.Itens) == 0 {
			continue
		}

		if dtA.IsZero() {
			dtA = d.Data
		} else {
			dtB = d.Data
			break
		}
	}

	concluido := func(d time.Time) bool { return plano.DayOf(d).Equal(plano.DayOf(dtA)) }

	if _, _, err := plano.Trocar(res.Dias, dtA, dtB, concluido); err == nil {
		t.Fatal("expected ErrReordenacaoInvalida for a concluded day")
	}
}

func findItens(dias []plano.Dia, dt time.Time) []plano.ItemDia {
	for _, d := range dias {
		if d.Data.Equal(dt) {
			return d.Itens
		}
	}

	return nil
}

func sameItens(a, b []plano.ItemDia) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
