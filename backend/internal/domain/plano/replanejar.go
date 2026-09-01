package plano

import (
	"sort"
	"strings"
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

	if DestinoBloqueado(dias, hoje, concluido) {
		return nil, ErrDiaConcluido
	}

	// Today already teaches this exact topic: bringing a second copy in would
	// store two rows the schedule then merges away on screen, so the pile grows
	// invisibly. The activity is simply dropped from its old day — the student
	// did finish the subject, and it is already represented today.
	for _, a := range doDia(atividades, hoje) {
		if a.ID == atividades[idx].ID {
			continue
		}

		if a.Disciplina == atividades[idx].Disciplina && a.Tema == atividades[idx].Tema {
			saida := make([]Atividade, 0, len(atividades))
			saida = append(saida, atividades[:idx]...)
			saida = append(saida, atividades[idx+1:]...)

			renumerar(saida, map[time.Time]bool{origem: true})

			return saida, nil
		}
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
// learning phase, where PreencherVazios turns them into reinforcement.
//
// A day already recorded is an anchor: what happened on it happened on it, and
// nothing is pulled across one — but only while it still holds the work that
// record describes. See ancorado.
func CompactarAtividades(
	atividades []Atividade,
	dias []Dia,
	desde time.Time,
	concluido func(time.Time) bool,
) []Atividade {
	desde = day(desde)

	uteis := diasQueRecebem(dias, desde, faseDe(dias, desde))
	if len(uteis) == 0 {
		return atividades
	}

	// What each day currently holds, in order.
	porDia := agruparPorDia(atividades)

	// Everything from `desde` onward, in schedule order, becomes one queue. It
	// has to be built before any day is filled: a day that is currently empty
	// would otherwise have nothing to pull from at the moment it is its turn.
	fila := []Atividade{}
	ancoras := map[time.Time][]Atividade{}

	for _, dt := range uteis {
		if ancorado(dias, dt, len(porDia[dt]) > 0, concluido) {
			// An anchor: what happened on it stays on it, and it feeds nothing to
			// the queue.
			ancoras[dt] = porDia[dt]

			continue
		}

		fila = append(fila, porDia[dt]...)
	}

	carga := cargaTipica(porDia, uteis)

	// Anything the compaction does not govern has to be carried through
	// untouched, or it is silently dropped: what comes before the cut, and
	// everything belonging to another phase.
	naFase := make(map[time.Time]bool, len(uteis))
	for _, dt := range uteis {
		naFase[dt] = true
	}

	saida := make([]Atividade, 0, len(atividades))

	for _, a := range atividades {
		dt := day(a.Data)
		if dt.Before(desde) || !naFase[dt] {
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

		// Take the next `carga` items the day does not already teach. A
		// discipline with few topics has the same one queued several times over
		// (reparte fills its extra slots with repeats), and packing blindly put
		// that topic on the day twice — the same subject listed back to back,
		// which is the pile the student sees. Skipped items stay in the queue for
		// a later day, so nothing is dropped.
		noDia := map[string]bool{}
		posicao := 0
		restante := make([]Atividade, 0, len(fila))

		for _, a := range fila {
			chave := a.Disciplina + "\x00" + a.Tema

			if posicao >= carga || noDia[chave] {
				restante = append(restante, a)

				continue
			}

			noDia[chave] = true
			a.Data = dt
			a.Posicao = posicao
			posicao++
			saida = append(saida, a)
		}

		fila = restante
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

// prefixoReforco marks a block a freed day drills rather than teaches.
const prefixoReforco = "Reforço — "

// reforcosJaUsados counts how much of the reinforcement queue earlier calls
// already spent, so a later call can pick up the rotation instead of
// restarting it.
func reforcosJaUsados(atividades []Atividade) int {
	n := 0

	for _, a := range atividades {
		if strings.HasPrefix(a.Tema, prefixoReforco) {
			n++
		}
	}

	return n
}

// temaBase is the topic behind a block's label.
//
// A topic reads the same to the student whether the schedule is teaching it,
// reviewing it in the reta final, or reinforcing it on a freed day; only the
// label in front of it changes. Reading it back off the schedule has to see
// through that, or the same topic counts as three and the labels stack.
func temaBase(tema string) string {
	for _, p := range []string{prefixoReforco, prefixoRevisaoDirigida} {
		tema = strings.TrimPrefix(tema, p)
	}

	return tema
}

// Reforco is what PreencherVazios needs beyond the schedule itself.
type Reforco struct {
	// Fila is the drill order: what the error notebook holds first, then
	// everything else already studied, oldest first. It is consumed in a cycle,
	// so a short list still fills every day.
	Fila []ItemRevisao
	// Desde is the first day that may be filled — before it is history.
	Desde time.Time
	// Concluido reports whether a day is already closed, and so untouchable.
	Concluido func(time.Time) bool
}

// PreencherVazios gives the days a compaction emptied something to do.
//
// Getting ahead pushes the free days to the end of the learning phase, right
// before the reta final. A free day there is not a prize — it is a day the
// student opens and finds blank, which is the same hole compaction just closed,
// only moved. What the time is actually worth is a second pass over what is
// already weak: the error notebook first, then everything else studied, oldest
// first.
//
// Only the learning phase is filled, and only its content days. The reta final
// has its own guided review and its own fixtures; a gap there means something
// else went wrong, and inventing work for it would paper over that.
func PreencherVazios(atividades []Atividade, dias []Dia, r Reforco) []Atividade {
	if len(r.Fila) == 0 {
		return atividades
	}

	desde := day(r.Desde)
	porDia := agruparPorDia(atividades)

	uteis := make([]time.Time, 0, len(dias))

	for _, dt := range diasQueRecebem(dias, desde, FaseBase) {
		if d := findDia(dias, dt); d != nil && d.Tipo == TipoEstudo {
			uteis = append(uteis, dt)
		}
	}

	if len(uteis) == 0 {
		return atividades
	}

	// A filled day carries what a normal day of this plan carries, rather than a
	// number of its own: the point is that it stops reading as a hole.
	carga := minInt(cargaTipica(porDia, uteis), len(r.Fila))

	saida := append([]Atividade(nil), atividades...)

	// PreencherVazios runs again every time another topic is finished early —
	// each completion can free up a fresh day on its own. Starting the queue
	// over at 0 every time would hand the SAME first couple of topics to every
	// call: the second free day of the plan would review the same thing as the
	// first, forever, instead of working through what was actually studied.
	// Picking up after however much reinforcement already exists keeps the
	// rotation moving across calls the same way it moves within one.
	cursor := reforcosJaUsados(atividades) % len(r.Fila)

	for _, dt := range uteis {
		if len(porDia[dt]) > 0 || r.Concluido(dt) {
			continue
		}

		for i := 0; i < carga; i++ {
			it := r.Fila[cursor%len(r.Fila)]
			cursor++

			saida = append(saida, Atividade{
				Data:       dt,
				Posicao:    i,
				Disciplina: it.Disciplina,
				Tema:       prefixoReforco + it.Tema,
				Passada:    2,
				Tipo:       AtividadeRevisao,
			})
		}
	}

	return saida
}

// FilaDeReforco is the drill order PreencherVazios consumes.
//
// The error notebook comes first: a topic that actually went wrong is the
// reason to spend a freed day coming back, and it outranks anything the queue
// would otherwise pick. Behind it comes everything else the learning phase
// taught, in the order it was taught — the oldest material is the one furthest
// from memory.
//
// What the queue holds is the TOPIC, never the label a block wears: a day
// already filled by a previous pass, or a guided review that ended up among the
// content days, would otherwise come back as "Reforço — Reforço — …" and drift
// one prefix further from the edital every time.
func FilaDeReforco(dias []Dia, cadernos map[string][]ItemCaderno) []ItemRevisao {
	type chave struct{ disc, tema string }

	// Everything the learning phase actually teaches, in schedule order.
	porChave := map[chave]ItemRevisao{}
	estudados := []ItemRevisao{}

	for _, d := range dias {
		if d.Fase == FaseReta {
			break
		}

		for _, it := range d.Itens {
			tema := temaBase(it.Tema)
			if it.Disciplina == "" || tema == "" {
				continue
			}

			k := chave{it.Disciplina, tema}
			if _, visto := porChave[k]; visto {
				continue
			}

			item := ItemRevisao{Disciplina: it.Disciplina, Tema: tema, DiaEstudo: d.N}
			porChave[k] = item
			estudados = append(estudados, item)
		}
	}

	naFrente := []ItemRevisao{}
	doCaderno := map[chave]bool{}

	for _, c := range cadernoGeral(cadernos) {
		k := chave{c.Disciplina, c.Tema}

		item, ensinado := porChave[k]
		if !ensinado || doCaderno[k] {
			continue
		}

		doCaderno[k] = true
		naFrente = append(naFrente, item)
	}

	out := make([]ItemRevisao, 0, len(estudados))
	out = append(out, naFrente...)

	for _, it := range estudados {
		if doCaderno[chave{it.Disciplina, it.Tema}] {
			continue
		}

		out = append(out, it)
	}

	return out
}

// diasQueRecebem lists the days that can hold content, in order, from `desde`
// onwards and WITHIN one phase.
//
// The learning phase and the reta final are different kinds of work: the reta
// reviews what the cycle taught. Crossing the boundary would pull a guided
// review back into the middle of the content days, which is not the plan
// getting tighter but the plan losing its shape.
func diasQueRecebem(dias []Dia, desde time.Time, fase Fase) []time.Time {
	desde = day(desde)
	out := make([]time.Time, 0, len(dias))

	for _, d := range dias {
		dt := day(d.Data)
		if dt.Before(desde) || d.Fase != fase || !DestinoValido(dias, dt) {
			continue
		}

		out = append(out, dt)
	}

	return out
}

// agruparPorDia indexes the activities by day, each day in position order.
func agruparPorDia(atividades []Atividade) map[time.Time][]Atividade {
	out := map[time.Time][]Atividade{}

	for _, a := range atividades {
		k := day(a.Data)
		out[k] = append(out[k], a)
	}

	for k := range out {
		lista := out[k]
		sort.SliceStable(lista, func(i, j int) bool { return lista[i].Posicao < lista[j].Posicao })
		out[k] = lista
	}

	return out
}

// ancorado reports whether a day should hold its ground during a compaction.
//
// An anchor exists to protect what happened on a day, so a day holding nothing
// has nothing to protect. On a content day the "concluído" flag is then a
// leftover — the work it described has since moved to the day it was really
// done on — and honouring it would freeze an empty day in the middle of the
// plan for good, which is exactly the hole compaction exists to close. A
// weekly-review day is the exception: its work IS the day, not a list of
// activities, so a finished one stays closed even while it carries none.
func ancorado(dias []Dia, dt time.Time, ocupada bool, concluido func(time.Time) bool) bool {
	if !concluido(dt) {
		return false
	}

	if ocupada {
		return true
	}

	return DestinoBloqueado(dias, dt, concluido)
}

// DestinoBloqueado reports whether a day's OWN record locks it against new
// arrivals — the question MoverAtividade, TrocarAtividades and
// AntecipouAtividade all ask about the day something is about to land on.
//
// Only a day whose "concluído" is an INDEPENDENT assertion — one that is never
// given items by the engine, such as a weekly review — really means "this day
// is closed". A content day's flag is DERIVED from whatever it happens to
// schedule at the moment it was last recorded (see CLAUDE.md: "the day-level
// concluído is derived, never asserted by the client"); it goes stale the
// instant an activity leaves or, as here, is about to arrive. Honouring it
// would block antecipar and every drag-in the moment today's original
// activities were finished — precisely the case antecipar exists for, and
// exactly what used to happen silently.
//
// A day already holding an activity is not protected by this check: bringing
// something new alongside it does not touch that activity's own record. What
// DOES have to stay protected — never relocating an activity that is ITSELF
// already marked done — is the caller's job, checked against the specific
// activity being moved (see atividadeConcluida in the service layer), not
// against the day as a whole.
func DestinoBloqueado(dias []Dia, dt time.Time, concluido func(time.Time) bool) bool {
	if !concluido(dt) {
		return false
	}

	d := findDia(dias, dt)

	return d != nil && d.Tipo != TipoEstudo && d.Tipo != TipoRevisaoDirigida
}

// faseDe is the phase a date belongs to, defaulting to the base phase for a
// date the plan does not contain.
func faseDe(dias []Dia, dt time.Time) Fase {
	if d := findDia(dias, dt); d != nil {
		return d.Fase
	}

	return FaseBase
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
