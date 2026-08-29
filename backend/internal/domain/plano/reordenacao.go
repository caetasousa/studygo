package plano

import (
	"errors"
	"time"
)

// ErrReordenacaoInvalida is returned when a swap breaks the rules: a day that is
// not a content day, or one already marked concluded.
var ErrReordenacaoInvalida = errors.New("dia não pode ser reordenado")

// Reordenacao is a manual override of a day's content, persisted per date and
// reapplied on every regeneration (as long as that date stays a content day).
type Reordenacao struct {
	Tipo  Tipo      `json:"tipo"`
	Itens []ItemDia `json:"itens"`
	Meta  int       `json:"meta"`
}

// AplicarReordenacoes mutates dias in place with each override whose date is
// still a content day, and returns the set of override dates that survived so
// the caller can delete the rest.
func AplicarReordenacoes(dias []Dia, overrides map[time.Time]Reordenacao) map[time.Time]bool {
	validas := map[time.Time]bool{}

	for i := range dias {
		key := day(dias[i].Data)

		ov, ok := overrides[key]
		if !ok {
			continue
		}

		if len(dias[i].Itens) == 0 {
			continue
		}

		dias[i].Tipo = ov.Tipo
		dias[i].Itens = cloneItens(ov.Itens)
		dias[i].Meta = ov.Meta
		validas[key] = true
	}

	return validas
}

// Trocar swaps the content of two days and returns the override to persist for
// each. concluido reports whether a given date is already done.
func Trocar(
	dias []Dia,
	dtA, dtB time.Time,
	concluido func(time.Time) bool,
) (Reordenacao, Reordenacao, error) {
	a := findDia(dias, dtA)
	b := findDia(dias, dtB)

	if a == nil || b == nil {
		return Reordenacao{}, Reordenacao{}, ErrReordenacaoInvalida
	}

	movivelA := len(a.Itens) > 0 && !concluido(day(dtA))
	movivelB := len(b.Itens) > 0 && !concluido(day(dtB))

	if !movivelA || !movivelB {
		return Reordenacao{}, Reordenacao{}, ErrReordenacaoInvalida
	}

	ra := Reordenacao{Tipo: b.Tipo, Itens: cloneItens(b.Itens), Meta: b.Meta}
	rb := Reordenacao{Tipo: a.Tipo, Itens: cloneItens(a.Itens), Meta: a.Meta}

	a.Tipo, a.Itens, a.Meta = ra.Tipo, cloneItens(ra.Itens), ra.Meta
	b.Tipo, b.Itens, b.Meta = rb.Tipo, cloneItens(rb.Itens), rb.Meta

	return ra, rb, nil
}

func findDia(dias []Dia, dt time.Time) *Dia {
	for i := range dias {
		if sameDay(dias[i].Data, dt) {
			return &dias[i]
		}
	}

	return nil
}

func cloneItens(src []ItemDia) []ItemDia {
	out := make([]ItemDia, len(src))
	copy(out, src)

	return out
}
