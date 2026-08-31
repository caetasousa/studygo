package plano

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// Errors a move can fail with. They are distinct so the API can turn each into
// a message that says what to do next, rather than a generic refusal.
var (
	// ErrAtividadeNaoEncontrada: no activity with that id in the plan.
	ErrAtividadeNaoEncontrada = errors.New("atividade não encontrada")
	// ErrDestinoInvalido: the target date is not a day that can hold content
	// (outside the plan, or a fixed day such as the simulado or the eve).
	ErrDestinoInvalido = errors.New("dia de destino não aceita atividades")
	// ErrDiaConcluido: the source or target day is already marked done, so
	// moving would rewrite history.
	ErrDiaConcluido = errors.New("dia já concluído")
)

// TipoAtividade is what a scheduled activity asks of the student. It is
// deliberately separate from Tipo (which classifies a whole day): a single day
// can hold a theory block and a question block at once.
type TipoAtividade string

const (
	AtividadeConteudo   TipoAtividade = "conteudo"
	AtividadeRevisao    TipoAtividade = "revisao"
	AtividadeQuestoes   TipoAtividade = "questoes"
	AtividadeSimulado   TipoAtividade = "simulado"
	AtividadeDiscursiva TipoAtividade = "discursiva"
)

// Atividade is one movable unit of the schedule: a subject+topic to study, a
// review, a block of questions. It is the planning side only — what the student
// actually did lives in Registro, so moving an activity never rewrites history.
type Atividade struct {
	ID         string
	Data       time.Time
	Posicao    int
	Disciplina string // discipline codigo; empty for whole-day activities
	Tema       string
	Passada    int
	Tipo       TipoAtividade
	DuracaoMin int // 0 = use the plan's default block length

	// Where the engine had put it, so a regeneration can distinguish an activity
	// the user moved from one it merely regenerated differently.
	OrigemDia *time.Time
	OrigemPos *int
}

// Movida reports whether the user has moved this activity away from where the
// engine originally placed it.
func (a Atividade) Movida() bool {
	if a.OrigemDia == nil || a.OrigemPos == nil {
		return false
	}

	return !sameDay(*a.OrigemDia, a.Data) || *a.OrigemPos != a.Posicao
}

// Duracao resolves the planned length, falling back to the plan's block size.
func (a Atividade) Duracao(padraoMin int) int {
	if a.DuracaoMin > 0 {
		return a.DuracaoMin
	}

	return padraoMin
}

// DestinoValido reports whether a date can receive activities: it must be a day
// the plan actually contains, and one that carries content rather than a fixed
// fixture (simulado, discursiva, véspera).
func DestinoValido(dias []Dia, dt time.Time) bool {
	d := findDia(dias, dt)
	if d == nil {
		return false
	}

	// Only days that actually carry study items can receive one; the fixed
	// fixtures (simulado, discursiva, véspera) are the plan's skeleton.
	return d.Tipo == TipoEstudo || d.Tipo == TipoRevisaoDirigida || d.Tipo == TipoRevisaoSemanal
}

// TrocarAtividades swaps one activity with whatever occupies (destino, posicao),
// instead of inserting alongside it. This is what "move Português from the 31st
// to the 2nd" usually means when the 2nd is already full: the two subjects
// exchange places and neither day changes size.
//
// When the target slot is empty (the day has fewer activities than posicao),
// there is nothing to swap with and this degrades to a plain move.
func TrocarAtividades(
	atividades []Atividade,
	dias []Dia,
	id string,
	destino time.Time,
	posicao int,
	concluido func(time.Time) bool,
) ([]Atividade, error) {
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
	posOrigem := atividades[idx].Posicao
	destino = day(destino)

	if !DestinoValido(dias, destino) {
		return nil, ErrDestinoInvalido
	}

	// Only the destination is guarded here: whether the activity ITSELF is
	// finished is decided by the caller, which can see the records. Refusing
	// every move out of a day that holds finished work would freeze that day's
	// other subjects too. See DestinoBloqueado: a content day's own flag does
	// not lock it either, since it is derived from a snapshot that is about to
	// change anyway.
	if DestinoBloqueado(dias, destino, concluido) {
		return nil, ErrDiaConcluido
	}

	// Same slot: nothing to do, and swapping with itself would be a no-op that
	// still rewrote every position.
	if sameDay(origem, destino) && posOrigem == posicao {
		return append([]Atividade(nil), atividades...), nil
	}

	destinoLista := doDia(atividades, destino)

	// No occupant at that slot — a swap has no counterpart, so this is a move.
	if posicao < 0 || posicao >= len(destinoLista) {
		return MoverAtividade(atividades, dias, id, destino, posicao, concluido)
	}

	alvoID := destinoLista[posicao].ID
	if alvoID == id {
		return append([]Atividade(nil), atividades...), nil
	}

	saida := append([]Atividade(nil), atividades...)

	for i := range saida {
		switch saida[i].ID {
		case id:
			saida[i].Data = destino
			saida[i].Posicao = posicao
		case alvoID:
			saida[i].Data = origem
			saida[i].Posicao = posOrigem
		}
	}

	return saida, nil
}

// prefixoDerivado marks an id the engine synthesised for an activity that has
// never been stored. It is deliberately not a uuid: the repository rejects it,
// which is what stops a synthetic id from reaching the database by accident.
const prefixoDerivado = "gen:"

// IDDerivado is the stable id of a generated (never-moved) activity: its slot
// in the plan. It lets the UI address an activity on the very first load,
// before any move has been persisted, without turning GET into a write.
//
// It is stable only while the plan is not rearranged, which is exactly its
// lifetime: the first move materialises every activity with a real uuid.
func IDDerivado(data time.Time, posicao int) string {
	return prefixoDerivado + day(data).Format("2006-01-02") + ":" + itoa(posicao)
}

// EhIDDerivado reports whether an id is a synthetic slot id rather than a
// stored activity's uuid.
func EhIDDerivado(id string) bool {
	return strings.HasPrefix(id, prefixoDerivado)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}

	return string(b)
}

// AtividadesFaltantes returns activities for the items of a reconciled plan that
// have no stored counterpart yet — the ones still carrying a synthetic slot id.
//
// This is what lets a plan be materialised incrementally: the whole schedule on
// the first move, and afterwards just the blocks a raised blocosPorDia added,
// without disturbing the activities the user has already arranged.
//
// `dias` must have been through AplicarAtividades, so every item carries the id
// it was reconciled to.
func AtividadesFaltantes(dias []Dia, existentes []Atividade) []Atividade {
	ocupadas := map[string]map[int]bool{}

	for _, a := range existentes {
		k := day(a.Data).Format("2006-01-02")
		if ocupadas[k] == nil {
			ocupadas[k] = map[int]bool{}
		}

		ocupadas[k][a.Posicao] = true
	}

	out := []Atividade{}

	for _, d := range dias {
		data := day(d.Data)
		chave := data.Format("2006-01-02")

		for pos, it := range d.Itens {
			if !EhIDDerivado(it.AtividadeID) {
				continue
			}

			// Position must not collide with an activity already stored for that
			// day: the unique (plano, data, posicao) index would reject it.
			livre := pos
			for ocupadas[chave][livre] {
				livre++
			}

			if ocupadas[chave] == nil {
				ocupadas[chave] = map[int]bool{}
			}

			ocupadas[chave][livre] = true

			origemDia, origemPos := data, pos
			out = append(out, Atividade{
				Data:       data,
				Posicao:    livre,
				Disciplina: it.Disciplina,
				Tema:       it.Tema,
				Passada:    it.Passada,
				Tipo:       tipoDaAtividade(d.Tipo),
				OrigemDia:  &origemDia,
				OrigemPos:  &origemPos,
			})
		}
	}

	return out
}

// ResolverIDDerivado maps a synthetic slot id back to the stored activity that
// now occupies that slot, which is how the first move of a never-arranged plan
// finds its target.
func ResolverIDDerivado(atividades []Atividade, id string) (string, bool) {
	if !EhIDDerivado(id) {
		return id, true
	}

	for _, a := range atividades {
		if IDDerivado(a.Data, a.Posicao) == id {
			return a.ID, true
		}
	}

	return "", false
}

// MoverAtividade moves one activity to (data, posicao) and returns the full new
// ordering of every day it touched, so the caller can persist positions
// densely and atomically.
//
// It never touches Registro: the plan changes, the history of what was studied
// does not. concluido reports whether a date is already marked done.
func MoverAtividade(
	atividades []Atividade,
	dias []Dia,
	id string,
	destino time.Time,
	posicao int,
	concluido func(time.Time) bool,
) ([]Atividade, error) {
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

	origem := atividades[idx].Data
	destino = day(destino)

	if !DestinoValido(dias, destino) {
		return nil, ErrDestinoInvalido
	}

	// See TrocarAtividades: the origin is not guarded, only the destination —
	// and see DestinoBloqueado for what actually locks it. A day whose
	// "concluído" only describes the items it happened to hold a moment ago
	// (any content day) does not lock; one whose "concluído" is the student's
	// own word for the whole day (a weekly review with nothing to point at)
	// does.
	if DestinoBloqueado(dias, destino, concluido) {
		return nil, ErrDiaConcluido
	}

	movida := atividades[idx]
	restantes := make([]Atividade, 0, len(atividades))
	restantes = append(restantes, atividades[:idx]...)
	restantes = append(restantes, atividades[idx+1:]...)

	destinoLista := doDia(restantes, destino)
	if posicao < 0 {
		posicao = 0
	}

	if posicao > len(destinoLista) {
		posicao = len(destinoLista)
	}

	movida.Data = destino

	// Rebuild the destination day explicitly, inserting at `posicao`. Bumping the
	// moved item's position alone is not enough: the ones it displaces would keep
	// their old numbers and a stable sort would leave it behind them.
	saida := make([]Atividade, 0, len(atividades))
	for _, a := range restantes {
		if !sameDay(a.Data, destino) {
			saida = append(saida, a)
		}
	}

	novaOrdem := make([]Atividade, 0, len(destinoLista)+1)
	novaOrdem = append(novaOrdem, destinoLista[:posicao]...)
	novaOrdem = append(novaOrdem, movida)
	novaOrdem = append(novaOrdem, destinoLista[posicao:]...)

	for i := range novaOrdem {
		novaOrdem[i].Posicao = i
	}

	saida = append(saida, novaOrdem...)

	// The source day lost one item, so close the gap it left behind.
	if !sameDay(origem, destino) {
		renumerar(saida, map[time.Time]bool{day(origem): true})
	}

	return saida, nil
}

// doDia returns the activities of one day, ordered by position.
func doDia(atividades []Atividade, dt time.Time) []Atividade {
	out := []Atividade{}

	for _, a := range atividades {
		if sameDay(a.Data, dt) {
			out = append(out, a)
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Posicao < out[j].Posicao })

	return out
}

// renumerar rewrites positions 0..n-1 for each affected day, in place.
func renumerar(atividades []Atividade, dias map[time.Time]bool) {
	for dt := range dias {
		ordenadas := doDia(atividades, dt)
		for i, a := range ordenadas {
			for j := range atividades {
				if atividades[j].ID == a.ID {
					atividades[j].Posicao = i

					break
				}
			}
		}
	}
}

// AtividadesDoDia is the ordered view one day's card renders.
func AtividadesDoDia(atividades []Atividade, dt time.Time) []Atividade {
	return doDia(atividades, dt)
}

// MinutosPlanejados totals a day's planned minutes.
func MinutosPlanejados(atividades []Atividade, dt time.Time, padraoMin int) int {
	total := 0
	for _, a := range doDia(atividades, dt) {
		total += a.Duracao(padraoMin)
	}

	return total
}

// DerivarAtividades turns the engine's generated days into activities. Used to
// seed the override table the first time the user moves something, so what they
// see is exactly what they were already looking at.
func DerivarAtividades(dias []Dia) []Atividade {
	out := []Atividade{}

	for _, d := range dias {
		for i, it := range d.Itens {
			dia, pos := d.Data, i
			out = append(out, Atividade{
				ID:         IDDerivado(d.Data, i),
				Data:       day(d.Data),
				Posicao:    i,
				Disciplina: it.Disciplina,
				Tema:       it.Tema,
				Passada:    it.Passada,
				Tipo:       tipoDaAtividade(d.Tipo),
				OrigemDia:  &dia,
				OrigemPos:  &pos,
			})
		}
	}

	return out
}

// MesclarAtividadesGeradas adds slots introduced by a later configuration
// change to an already materialised layout. A stored activity claims its
// original slot even after it has moved elsewhere, which is what prevents the
// engine from recreating it in the source day.
func MesclarAtividadesGeradas(dias []Dia, armazenadas []Atividade) []Atividade {
	saida := append([]Atividade(nil), armazenadas...)
	reivindicadas := origensReivindicadas(armazenadas)
	quantidadePorDia := map[time.Time]int{}

	for _, a := range armazenadas {
		quantidadePorDia[day(a.Data)]++
	}

	for _, d := range dias {
		data := day(d.Data)
		for pos, it := range d.Itens {
			chave := chaveOrigem(data, pos)
			if reivindicadas[chave] {
				continue
			}

			origemDia, origemPos := data, pos
			saida = append(saida, Atividade{
				ID:         IDDerivado(data, pos),
				Data:       data,
				Posicao:    quantidadePorDia[data],
				Disciplina: it.Disciplina,
				Tema:       it.Tema,
				Passada:    it.Passada,
				Tipo:       tipoDaAtividade(d.Tipo),
				OrigemDia:  &origemDia,
				OrigemPos:  &origemPos,
			})
			quantidadePorDia[data]++
			reivindicadas[chave] = true
		}
	}

	return saida
}

// RestaurarAtividades returns every materialised activity to its engine slot.
// IDs stay unchanged, so study records linked to an activity remain intact.
func RestaurarAtividades(atividades []Atividade) []Atividade {
	saida := append([]Atividade(nil), atividades...)

	for i := range saida {
		if saida[i].OrigemDia == nil || saida[i].OrigemPos == nil {
			continue
		}

		saida[i].Data = day(*saida[i].OrigemDia)
		saida[i].Posicao = *saida[i].OrigemPos
	}

	return saida
}

func tipoDaAtividade(t Tipo) TipoAtividade {
	switch t {
	case TipoRevisaoSemanal, TipoRevisaoDirigida:
		return AtividadeRevisao
	case TipoSimulado:
		return AtividadeSimulado
	case TipoDiscursiva:
		return AtividadeDiscursiva
	default:
		return AtividadeConteudo
	}
}

// AplicarAtividades reconciles the generated days with the persisted activity
// layout. Stored activities win. A generated slot is appended only when no
// stored activity claims that original slot, even if its activity currently
// lives on another day.
func AplicarAtividades(dias []Dia, atividades []Atividade) {
	porDia := map[time.Time][]Atividade{}
	for _, a := range atividades {
		k := day(a.Data)
		porDia[k] = append(porDia[k], a)
	}

	reivindicadas := origensReivindicadas(atividades)

	for i := range dias {
		data := day(dias[i].Data)
		lista := porDia[data]

		sort.SliceStable(lista, func(x, y int) bool { return lista[x].Posicao < lista[y].Posicao })

		itensGerados := dias[i].Itens
		itens := make([]ItemDia, 0, len(lista)+len(itensGerados))
		for _, a := range lista {
			itens = append(itens, ItemDia{
				Disciplina:  a.Disciplina,
				Tema:        a.Tema,
				Passada:     a.Passada,
				AtividadeID: a.ID,
			})
		}

		for pos, it := range itensGerados {
			if reivindicadas[chaveOrigem(data, pos)] {
				continue
			}

			it.AtividadeID = IDDerivado(data, pos)
			itens = append(itens, it)
		}

		dias[i].Itens = itens
	}
}

func origensReivindicadas(atividades []Atividade) map[string]bool {
	out := make(map[string]bool, len(atividades))

	for _, a := range atividades {
		if a.OrigemDia == nil || a.OrigemPos == nil {
			continue
		}

		out[chaveOrigem(day(*a.OrigemDia), *a.OrigemPos)] = true
	}

	return out
}

func chaveOrigem(data time.Time, posicao int) string {
	return data.Format(time.DateOnly) + ":" + itoa(posicao)
}
