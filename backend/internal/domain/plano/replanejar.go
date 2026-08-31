package plano

import (
	"sort"
	"time"
)

// Rescheduling: the two ways a real week diverges from the plan.
//
// A schedule that only works when it is followed exactly is a schedule nobody
// can follow. Two things happen constantly — a day is lost, or a subject moves
// faster than planned — and both used to leave the student to drag activities
// around by hand.

// AdiarDia pushes a day's activities forward and shifts everything after it.
//
// Nothing is dropped: the lost day's work lands on the next day that accepts
// activities, and every later day slides one study-day along. The exam date does
// not move, so the plan gets tighter at the end — which is the truth of having
// lost a day, and is what the coverage warning already reports.
//
// Days already recorded are left alone: they are history, not schedule.
func AdiarDia(
	atividades []Atividade,
	dias []Dia,
	data time.Time,
	concluido func(time.Time) bool,
) ([]Atividade, error) {
	data = day(data)

	if concluido(data) {
		return nil, ErrDiaConcluido
	}

	// The study days from the postponed one onwards, in order — the chain the
	// content walks along.
	cadeia := make([]time.Time, 0, len(dias))

	for _, d := range dias {
		dt := day(d.Data)
		if dt.Before(data) || !DestinoValido(dias, dt) || concluido(dt) {
			continue
		}

		cadeia = append(cadeia, dt)
	}

	if len(cadeia) < 2 {
		// Nowhere to push to: the day is the last one that can hold content.
		return nil, ErrDestinoInvalido
	}

	// Map each day in the chain to the next one, so a single pass moves
	// everything without an activity overtaking its own destination.
	proximo := make(map[time.Time]time.Time, len(cadeia))
	for i := 0; i < len(cadeia)-1; i++ {
		proximo[cadeia[i]] = cadeia[i+1]
	}

	saida := append([]Atividade(nil), atividades...)
	afetados := map[time.Time]bool{}

	for i := range saida {
		dt := day(saida[i].Data)

		destino, ok := proximo[dt]
		if !ok {
			continue
		}

		saida[i].Data = destino
		afetados[dt] = true
		afetados[destino] = true
	}

	// The last day of the chain now holds two days' worth; positions have to be
	// dense again everywhere the move touched.
	renumerar(saida, afetados)

	return saida, nil
}

// AntecipouAtividade brings an activity forward to the day it was actually
// finished on, closing the gap it leaves behind.
//
// This is the other half of a real week: a subject whose topics connect, where
// two blocks land in one sitting. Marking the later topic done should move it to
// today rather than leave the schedule claiming it is still ahead.
//
// Activities between the old position and today keep their order and simply
// move up one slot, so the sequence of the subject is preserved.
func AntecipouAtividade(
	atividades []Atividade,
	dias []Dia,
	id string,
	hoje time.Time,
	concluido func(time.Time) bool,
) ([]Atividade, error) {
	hoje = day(hoje)

	idx := -1

	for i := range atividades {
		if atividades[i].ID == id {
			idx = i

			break
		}
	}

	if idx < 0 {
		return nil, ErrAtividadeNaoEncontrada
	}

	origem := day(atividades[idx].Data)

	// Already today, or in the past: nothing to bring forward.
	if !origem.After(hoje) {
		return append([]Atividade(nil), atividades...), nil
	}

	if !DestinoValido(dias, hoje) {
		return nil, ErrDestinoInvalido
	}

	if concluido(hoje) {
		return nil, ErrDiaConcluido
	}

	// It joins the end of today, after whatever was already scheduled.
	posicao := len(doDia(atividades, hoje))

	saida := append([]Atividade(nil), atividades...)
	saida[idx].Data = hoje
	saida[idx].Posicao = posicao

	// Close the hole: everything after it in its old day moves up.
	renumerar(saida, map[time.Time]bool{origem: true, hoje: true})

	return saida, nil
}

// CompactarAtividades pulls the schedule back over the days a study day was
// left empty.
//
// Getting ahead should buy time, not leave holes: when a topic is finished
// early the day it came from can end up with nothing, and the plan then reads
// as if the student were idle on a day they had simply moved past. Everything
// after slides back to fill it, so the free days accumulate at the END of the
// plan — which is where they are worth something, as room for more content
// before the exam.
//
// Days already recorded are anchors: what happened on them happened on them,
// and nothing is pulled across one.
func CompactarAtividades(
	atividades []Atividade,
	dias []Dia,
	desde time.Time,
	concluido func(time.Time) bool,
) []Atividade {
	desde = day(desde)

	// The days that can hold content, in order, from `desde` onwards.
	uteis := make([]time.Time, 0, len(dias))

	for _, d := range dias {
		dt := day(d.Data)
		if dt.Before(desde) || !DestinoValido(dias, dt) {
			continue
		}

		uteis = append(uteis, dt)
	}

	if len(uteis) == 0 {
		return atividades
	}

	// What each day currently holds, in order.
	porDia := map[time.Time][]Atividade{}
	for _, a := range atividades {
		k := day(a.Data)
		porDia[k] = append(porDia[k], a)
	}

	for k := range porDia {
		lista := porDia[k]
		sort.SliceStable(lista, func(i, j int) bool { return lista[i].Posicao < lista[j].Posicao })
		porDia[k] = lista
	}

	// Everything from `desde` onward, in schedule order, becomes one queue. It
	// has to be built before any day is filled: a day that is currently empty
	// would otherwise have nothing to pull from at the moment it is its turn.
	fila := []Atividade{}
	ancoras := map[time.Time][]Atividade{}

	for _, dt := range uteis {
		if concluido(dt) {
			// An anchor: what happened on it stays on it, and it feeds nothing to
			// the queue.
			ancoras[dt] = porDia[dt]

			continue
		}

		fila = append(fila, porDia[dt]...)
	}

	carga := cargaTipica(porDia, uteis)

	saida := make([]Atividade, 0, len(atividades))

	// Everything before `desde` is untouched.
	for _, a := range atividades {
		if day(a.Data).Before(desde) {
			saida = append(saida, a)
		}
	}

	for _, dt := range uteis {
		if ancorados, ok := ancoras[dt]; ok {
			for i, a := range ancorados {
				a.Posicao = i
				saida = append(saida, a)
			}

			continue
		}

		if len(fila) == 0 {
			continue
		}

		quer := carga
		if quer > len(fila) {
			quer = len(fila)
		}

		for i := 0; i < quer; i++ {
			a := fila[i]
			a.Data = dt
			a.Posicao = i
			saida = append(saida, a)
		}

		fila = fila[quer:]
	}

	// Anything still queued did not fit before the exam; it stays on the last
	// useful day rather than vanishing.
	if len(fila) > 0 {
		ultimo := uteis[len(uteis)-1]
		base := 0

		for _, a := range saida {
			if sameDay(a.Data, ultimo) {
				base++
			}
		}

		for i, a := range fila {
			a.Data = ultimo
			a.Posicao = base + i
			saida = append(saida, a)
		}
	}

	return saida
}

// cargaTipica is how many activities a day normally holds in this plan — the
// most common non-zero count, so one unusually full day does not set the pace.
func cargaTipica(porDia map[time.Time][]Atividade, uteis []time.Time) int {
	freq := map[int]int{}

	for _, dt := range uteis {
		if n := len(porDia[dt]); n > 0 {
			freq[n]++
		}
	}

	melhor, vezes := 0, 0

	for n, v := range freq {
		if v > vezes || (v == vezes && n > melhor) {
			melhor, vezes = n, v
		}
	}

	if melhor == 0 {
		return 1
	}

	return melhor
}
