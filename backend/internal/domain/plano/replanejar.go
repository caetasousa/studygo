package plano

import "time"

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
