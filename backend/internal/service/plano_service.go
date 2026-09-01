package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// ErrValidacao wraps a user-facing validation message.
type ErrValidacao struct{ Msg string }

func (e ErrValidacao) Error() string { return e.Msg }

// PlanoService orchestrates the plan engine with the persisted user state.
type PlanoService struct {
	planos    port.PlanoRepository
	concursos port.ConcursoRepository
	clock     port.Clock
}

func NewPlanoService(
	planos port.PlanoRepository,
	concursos port.ConcursoRepository,
	clock port.Clock,
) *PlanoService {
	return &PlanoService{planos: planos, concursos: concursos, clock: clock}
}

// Obter returns the full assembled plan, creating a default one on first access.
func (s *PlanoService) Obter(ctx context.Context, userID uuid.UUID, slug string) (PlanoResposta, error) {
	c, err := s.concursoDoDono(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	salvo, err := s.garantirPlano(ctx, userID, c)
	if err != nil {
		return PlanoResposta{}, err
	}

	return s.montar(ctx, c, salvo)
}

// Salvar validates and persists a new configuration, then returns the plan.
func (s *PlanoService) Salvar(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	in ConfigInput,
) (PlanoResposta, error) {
	c, err := s.concursoDoDono(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	salvo, err := s.garantirPlano(ctx, userID, c)
	if err != nil {
		return PlanoResposta{}, err
	}

	anterior := salvo.Config

	cfg, tema, err := aplicarConfigInput(salvo, c, in)
	if err != nil {
		return PlanoResposta{}, err
	}

	salvo.Config = cfg
	salvo.TemaUI = tema

	// Regenerate so we can drop reorderings that no longer land on a content day.
	res := plano.Gerar(cfg, &c)
	salvo.Reordenacoes = prune(res.Dias, salvo.Reordenacoes)

	salvo, err = s.planos.UpsertPlano(ctx, salvo)
	if err != nil {
		return PlanoResposta{}, err
	}

	// Changing the day's rhythm has to reach the schedule. Once any day has been
	// recorded the whole plan is materialised, and AplicarAtividades then makes
	// the stored layout win for every day — so a new blocosPorDia would show only
	// on days the store has never seen. Drop the materialised activities for the
	// days still ahead (keeping history, finished days and hand-moved blocks), so
	// the engine regenerates those under the new setting.
	if ritmoMudou(anterior, cfg) {
		if err := s.regenerarDiasFuturos(ctx, c, salvo); err != nil {
			return PlanoResposta{}, err
		}
	}

	return s.montar(ctx, c, salvo)
}

// ritmoMudou reports whether a config change touched how a day is filled —
// blocks per day or block length — which is what a materialised plan freezes.
func ritmoMudou(antes, depois plano.Config) bool {
	antes = antes.Normalizar()
	depois = depois.Normalizar()

	return antes.BlocosPorDia != depois.BlocosPorDia ||
		antes.MinutosBloco != depois.MinutosBloco
}

// regenerarDiasFuturos drops the materialised activities for content days from
// today onward that the student has neither finished nor moved by hand, so the
// next montar rebuilds them from the engine under the current config.
func (s *PlanoService) regenerarDiasFuturos(
	ctx context.Context,
	c concurso.Concurso,
	salvo plano.Salvo,
) error {
	atividades, err := s.planos.ListAtividades(ctx, salvo.ID)
	if err != nil {
		return err
	}

	if len(atividades) == 0 {
		return nil
	}

	retidas := plano.ReterAoMudarRitmo(
		atividades,
		plano.DayOf(s.clock.Now()),
		s.diaConcluido(salvo),
	)

	if len(retidas) == len(atividades) {
		return nil
	}

	return s.planos.ReplaceAtividades(ctx, salvo.ID, retidas)
}

// RegistrarDia upserts one day's log.
func (s *PlanoService) RegistrarDia(
	ctx context.Context,
	userID uuid.UUID,
	slug, dataISO string,
	in RegistroInput,
) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	data, ok := parseISODate(dataISO)
	if !ok {
		return PlanoResposta{}, ErrValidacao{Msg: "data inválida"}
	}

	// Materialise the plan's activities before the record is built.
	//
	// Until something is stored the client addresses activities by a synthetic
	// slot id, which cannot be persisted and used to be dropped — so a record
	// arrived with no activity attached and nothing could be moved by it.
	// Resolving them here means every block carries a real id from the start.
	if _, ats, err := s.prepararAtividades(ctx, c, salvo); err == nil {
		in.Blocos = resolverIDsDosBlocos(in.Blocos, ats)
	}

	reg := plano.Registro{
		Data:      data,
		Horas:     in.Horas,
		Concluido: in.Concluido,
		Questoes:  in.Questoes,
		Acertos:   in.Acertos,
		Nota:      in.Nota,
		Blocos:    blocosDoInput(c, in.Blocos),
	}

	// Com lançamento por disciplina, os totais do dia são a soma dos blocos — o
	// cliente não precisa mandá-los, e se mandar, os blocos ganham.
	if len(reg.Blocos) > 0 {
		reg.Horas, reg.Questoes, reg.Acertos = reg.Totais()
	}

	// With per-activity records the day's flag is DERIVED, never asserted: it is
	// done when every activity SCHEDULED for that day is done. Counting only the
	// blocks that arrived would mark a two-subject day complete as soon as the
	// first subject was recorded — exactly what per-activity completion exists to
	// stop.
	//
	// Days that schedule no subjects (simulado, revisão, véspera) keep the
	// artifact's rule: there, logging hours does mean the day is done.
	agendadas := s.atividadesAgendadas(ctx, c, salvo, data)

	switch {
	case agendadas > 0:
		reg.Concluido = concluidasNoDia(reg.Blocos) >= agendadas
	case reg.Horas != nil && *reg.Horas != 0 && !reg.Concluido:
		reg.Concluido = true
	}

	// Finishing a topic that was scheduled further ahead brings it to the day it
	// was actually finished on. Two blocks of one subject in a single sitting is
	// a real week, and the schedule should say what happened rather than keep
	// claiming the topic is still due later. The day it lands on simply holds
	// one more activity than planned, and the days it left slide up.
	//
	// Done BEFORE the record is written: rearranging rewrites the whole activity
	// layout, and a record already stored would come out the other side with its
	// activity link cleared.
	if err := s.trazerConcluidasParaODia(ctx, c, salvo, data, reg); err != nil {
		return PlanoResposta{}, err
	}

	if err := s.planos.UpsertRegistro(ctx, salvo.ID, reg); err != nil {
		return PlanoResposta{}, err
	}

	salvo.Registros[data] = reg

	return s.montar(ctx, c, salvo)
}

// proximaAtividadeDe finds the earliest activity of a discipline still
// scheduled on or after a date — what an id-less completion of that subject was
// most likely referring to.
func proximaAtividadeDe(
	atividades []plano.Atividade,
	disciplina string,
	desde time.Time,
) (string, bool) {
	if disciplina == "" {
		return "", false
	}

	melhor := ""
	var quando time.Time

	for _, a := range atividades {
		if a.Disciplina != disciplina || a.Data.Before(plano.DayOf(desde)) {
			continue
		}

		if melhor == "" || a.Data.Before(quando) {
			melhor, quando = a.ID, a.Data
		}
	}

	return melhor, melhor != ""
}

// resolverIDsDosBlocos turns the synthetic slot ids the client may be holding
// into the real ids of the now-stored activities.
func resolverIDsDosBlocos(
	blocos []RegistroBlocoInput,
	atividades []plano.Atividade,
) []RegistroBlocoInput {
	out := make([]RegistroBlocoInput, len(blocos))
	copy(out, blocos)

	for i := range out {
		id := strings.TrimSpace(out[i].AtividadeID)
		if !plano.EhIDDerivado(id) {
			continue
		}

		if resolvido, ok := plano.ResolverIDDerivado(atividades, id); ok {
			out[i].AtividadeID = resolvido
		} else {
			out[i].AtividadeID = ""
		}
	}

	return out
}

// trazerConcluidasParaODia moves every activity marked done in this record that
// the plan had scheduled for a LATER day onto `data`.
//
// Only forward-dated activities move: one already in the past stays where it
// happened, and one already on this day is where it should be.
func (s *PlanoService) trazerConcluidasParaODia(
	ctx context.Context,
	c concurso.Concurso,
	salvo plano.Salvo,
	data time.Time,
	reg plano.Registro,
) error {
	res, atividades, err := s.prepararAtividades(ctx, c, salvo)
	if err != nil {
		return err
	}

	feitas := make([]string, 0, len(reg.Blocos))

	for _, b := range reg.Blocos {
		if !b.Concluido {
			continue
		}

		if b.AtividadeID != "" {
			feitas = append(feitas, b.AtividadeID)

			continue
		}

		// A record written before activities were addressable carries no id. The
		// discipline plus the day it was recorded on is enough to find what it
		// meant: the next activity of that subject still scheduled ahead. Without
		// this, a completion made before the fix stays stranded forever.
		if id, ok := proximaAtividadeDe(atividades, b.Disciplina, data); ok {
			feitas = append(feitas, id)
		}
	}

	if len(feitas) == 0 {
		return nil
	}

	concluido := s.diaConcluido(salvo)
	mexeu := false

	for _, id := range feitas {
		atual := atividades

		movidas, err := plano.AntecipouAtividade(atual, res.Dias, id, data, concluido)
		if err != nil {
			// A refusal here is not the caller's problem: the record itself was
			// saved. Leave the activity where it is rather than failing the save.
			continue
		}

		atividades = movidas
		mexeu = true
	}

	if !mexeu {
		return nil
	}

	// Getting ahead should buy time, not leave holes.
	//
	// The reorganisation starts on the day AFTER the one just recorded: that day
	// is where the finished topics have only now landed, and its own record has
	// not been written yet, so it would not be recognised as an anchor and the
	// work would be pulled straight back out of it.
	atividades, orfaos := s.reorganizar(salvo, replanejamento{
		res:        res,
		atividades: atividades,
		desde:      plano.DayOf(data).AddDate(0, 0, 1),
	})

	if err := s.planos.ReplaceAtividades(ctx, salvo.ID, atividades); err != nil {
		return err
	}

	return s.descartarRegistros(ctx, salvo, orfaos)
}

// replanejamento is one rearrangement in flight: the generated plan, the
// activity layout as it now stands, and the first day the change may touch.
type replanejamento struct {
	res        plano.Resultado
	atividades []plano.Atividade
	desde      time.Time
}

// reorganizar closes the holes a rearrangement left behind and gives the days
// it frees something to do.
//
// The three parts only make sense together. A record stranded on a day whose
// work has moved elsewhere would freeze that day as an anchor, so it is set
// aside first; the compaction then pulls the rest of the plan back over the
// holes, piling the free days at the END of the learning phase; and those days
// get a reinforcement block each, because a blank day right before the reta
// final is the same hole the compaction just closed, only moved.
//
// Nothing is persisted here. It returns the new layout and the orphan records
// the caller should drop AFTER the layout is safely written: dropping them
// first would, on a failed write, un-finish a day that still holds work.
func (s *PlanoService) reorganizar(
	salvo plano.Salvo,
	r replanejamento,
) ([]plano.Atividade, []time.Time) {
	orfaos := registrosOrfaos(r.res.Dias, r.atividades, salvo.Registros)
	concluido := diaConcluidoSem(salvo.Registros, orfaos)

	atividades := plano.CompactarAtividades(r.atividades, r.res.Dias, r.desde, concluido)

	preenchidas := plano.PreencherVazios(atividades, r.res.Dias, plano.Reforco{
		Fila: plano.FilaDeReforco(
			r.res.Dias,
			plano.Caderno(resultadosDoPlano(r.res.Dias, salvo)),
		),
		Desde:     r.desde,
		Concluido: concluido,
	})

	return preenchidas, orfaos
}

// registrosOrfaos lists the day records that no longer describe anything.
//
// A completion is recorded per ACTIVITY. When a day's activities move to the
// day they were really finished on, the day row stays behind claiming
// "concluído" over a day that now holds nothing and logs nothing. That leftover
// is not harmless: a concluded day anchors a compaction, so the hole it marks
// could never be closed, and a day later refilled under it would render as
// already done.
//
// Only content days qualify — a simulado or a véspera legitimately schedules no
// activities — and only rows carrying no data of their own: logged hours,
// questions or a note are the student's, whatever moved away from the day.
func registrosOrfaos(
	dias []plano.Dia,
	atividades []plano.Atividade,
	registros map[time.Time]plano.Registro,
) []time.Time {
	ocupados := make(map[time.Time]bool, len(atividades))
	for _, a := range atividades {
		ocupados[plano.DayOf(a.Data)] = true
	}

	out := []time.Time{}

	for _, d := range dias {
		dt := plano.DayOf(d.Data)

		if d.Tipo != plano.TipoEstudo && d.Tipo != plano.TipoRevisaoDirigida {
			continue
		}

		r, ok := registros[dt]
		if !ok || ocupados[dt] || !registroSemDados(r) {
			continue
		}

		out = append(out, dt)
	}

	return out
}

// registroSemDados reports whether a day record carries nothing but its own
// flag — no blocks, no hours, no battery, no note.
func registroSemDados(r plano.Registro) bool {
	return len(r.Blocos) == 0 &&
		r.Horas == nil &&
		r.Questoes == nil &&
		r.Acertos == nil &&
		r.Nota == ""
}

// diaConcluidoSem is diaConcluido with a set of days read as never recorded —
// the orphan rows the reorganisation is about to drop. Without this the
// compaction would still see them as anchors and leave the holes open.
func diaConcluidoSem(
	registros map[time.Time]plano.Registro,
	ignorar []time.Time,
) func(time.Time) bool {
	fora := make(map[time.Time]bool, len(ignorar))
	for _, dt := range ignorar {
		fora[dt] = true
	}

	return func(d time.Time) bool {
		dt := plano.DayOf(d)
		if fora[dt] {
			return false
		}

		r, ok := registros[dt]

		return ok && r.Concluido
	}
}

// descartarRegistros drops the orphan day rows and forgets them locally, so the
// response built right after does not show a day as finished that no longer
// holds anything.
func (s *PlanoService) descartarRegistros(
	ctx context.Context,
	salvo plano.Salvo,
	datas []time.Time,
) error {
	for _, dt := range datas {
		if err := s.planos.DeleteRegistro(ctx, salvo.ID, dt); err != nil {
			return err
		}

		delete(salvo.Registros, dt)
	}

	return nil
}

// atividadeConcluida reports whether one activity has been marked done, which
// is the only thing that makes it immovable.
func atividadeConcluida(salvo plano.Salvo, atividades []plano.Atividade, id string) bool {
	for _, a := range atividades {
		if a.ID != id {
			continue
		}

		r, ok := salvo.Registros[plano.DayOf(a.Data)]
		if !ok {
			return false
		}

		if b := r.BlocoDeAtividade(a.ID); b != nil {
			return b.Concluido
		}

		// No per-activity record: fall back to the day's own flag, which is what
		// records written before activities were addressable carry.
		return r.Concluido
	}

	return false
}

// concluidasNoDia counts the recorded activities that are marked done.
func concluidasNoDia(bs []plano.RegistroBloco) int {
	n := 0

	for _, b := range bs {
		if b.Concluido {
			n++
		}
	}

	return n
}

// atividadesAgendadas reports how many subject activities the plan schedules for
// one day — the denominator for "is this day finished". Returns 0 for days that
// carry no subjects, which keep the day-level rule.
func (s *PlanoService) atividadesAgendadas(
	ctx context.Context,
	c concurso.Concurso,
	salvo plano.Salvo,
	data time.Time,
) int {
	res := plano.Gerar(salvo.Config, &c)
	plano.AplicarReordenacoes(res.Dias, salvo.Reordenacoes)

	if as, err := s.planos.ListAtividades(ctx, salvo.ID); err == nil && len(as) > 0 {
		plano.AplicarAtividades(res.Dias, as)
	}

	for _, d := range res.Dias {
		if plano.DayOf(d.Data).Equal(plano.DayOf(data)) {
			return len(d.Itens)
		}
	}

	return 0
}

// MarcarMarco toggles an edital milestone checkbox.
func (s *PlanoService) MarcarMarco(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	marcoID uuid.UUID,
	cumprido bool,
) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	if c.MarcoByID(marcoID) == nil {
		return PlanoResposta{}, concurso.ErrNotFound
	}

	if err := s.planos.SetMarco(ctx, salvo.ID, marcoID, cumprido); err != nil {
		return PlanoResposta{}, err
	}

	salvo.Marcos[marcoID] = cumprido

	return s.montar(ctx, c, salvo)
}

// prepararAtividades generates the plan and makes sure every activity the
// client can see also exists in the store.
//
// Shared by every operation that rearranges the schedule: the plan is generated,
// not stored, so an activity only gets a real id once something touches it. The
// first move of a plan materialises all of them; raising blocosPorDia later
// materialises just the blocks that were added.
func (s *PlanoService) prepararAtividades(
	ctx context.Context,
	c concurso.Concurso,
	salvo plano.Salvo,
) (plano.Resultado, []plano.Atividade, error) {
	res := plano.Gerar(salvo.Config, &c)
	plano.AplicarReordenacoes(res.Dias, salvo.Reordenacoes)

	atividades, err := s.planos.ListAtividades(ctx, salvo.ID)
	if err != nil {
		return plano.Resultado{}, nil, err
	}

	// Heal a table seeded from a reparte pile — a one-topic discipline the engine
	// gave many slots on one day, each stored as its own row. Done before
	// anything reads the layout, and persisted so it does not recur.
	if limpas := plano.DeduplicarAtividades(atividades); len(limpas) != len(atividades) {
		if err := s.planos.ReplaceAtividades(ctx, salvo.ID, limpas); err != nil {
			return plano.Resultado{}, nil, err
		}

		atividades = limpas
	}

	plano.AplicarAtividades(res.Dias, atividades)

	if faltantes := plano.AtividadesFaltantes(res.Dias, atividades); len(faltantes) > 0 {
		if err := s.planos.ReplaceAtividades(
			ctx, salvo.ID, append(append([]plano.Atividade{}, atividades...), faltantes...),
		); err != nil {
			return plano.Resultado{}, nil, err
		}

		if atividades, err = s.planos.ListAtividades(ctx, salvo.ID); err != nil {
			return plano.Resultado{}, nil, err
		}
	}

	return res, atividades, nil
}

// diaConcluido reports whether a date is already recorded as done.
func (s *PlanoService) diaConcluido(salvo plano.Salvo) func(time.Time) bool {
	return func(d time.Time) bool {
		r, ok := salvo.Registros[plano.DayOf(d)]

		return ok && r.Concluido
	}
}

// erroDeReplanejamento turns a domain refusal into a message that says what to
// do next.
func erroDeReplanejamento(err error) error {
	switch {
	case errors.Is(err, plano.ErrAtividadeNaoEncontrada):
		return ErrValidacao{Msg: "atividade não encontrada"}
	case errors.Is(err, plano.ErrDestinoInvalido):
		return ErrValidacao{Msg: "esse dia não recebe atividades"}
	case errors.Is(err, plano.ErrDiaConcluido):
		return ErrValidacao{Msg: "um dia já concluído não pode ser reorganizado"}
	default:
		return err
	}
}

// AdiarDia pushes a lost day's content forward, shifting the rest of the plan.
func (s *PlanoService) AdiarDia(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	dataISO string,
) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	data, ok := parseISODate(dataISO)
	if !ok {
		return PlanoResposta{}, ErrValidacao{Msg: "data inválida"}
	}

	res, atividades, err := s.prepararAtividades(ctx, c, salvo)
	if err != nil {
		return PlanoResposta{}, err
	}

	movidas, err := plano.AdiarDia(atividades, res.Dias, data, s.diaConcluido(salvo))
	if err != nil {
		return PlanoResposta{}, erroDeReplanejamento(err)
	}

	if err := s.planos.ReplaceAtividades(ctx, salvo.ID, movidas); err != nil {
		return PlanoResposta{}, err
	}

	return s.montar(ctx, c, salvo)
}

// AntecipouAtividade brings an activity forward to the day it was finished on.
func (s *PlanoService) AntecipouAtividade(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	in AnteciparInput,
) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	hoje, ok := parseISODate(in.Data)
	if !ok {
		return PlanoResposta{}, ErrValidacao{Msg: "data inválida"}
	}

	res, atividades, err := s.prepararAtividades(ctx, c, salvo)
	if err != nil {
		return PlanoResposta{}, err
	}

	id := in.ID
	if plano.EhIDDerivado(id) {
		resolvido, ok := plano.ResolverIDDerivado(atividades, id)
		if !ok {
			return PlanoResposta{}, ErrValidacao{Msg: "atividade não encontrada"}
		}

		id = resolvido
	}

	movidas, err := plano.AntecipouAtividade(
		atividades, res.Dias, id, hoje, s.diaConcluido(salvo),
	)
	if err != nil {
		return PlanoResposta{}, erroDeReplanejamento(err)
	}

	if err := s.planos.ReplaceAtividades(ctx, salvo.ID, movidas); err != nil {
		return PlanoResposta{}, err
	}

	return s.montar(ctx, c, salvo)
}

// MoverAtividade moves a single scheduled activity to (data, posicao).
//
// This is the fine-grained counterpart to Reordenar, which can only swap two
// whole days. The first move seeds the activity table from the engine's output,
// so what the user sees is exactly what they were already looking at; from then
// on the stored layout wins for the days it covers.
//
// Registros are never touched: moving what is *planned* must not rewrite the
// record of what was actually studied.
func (s *PlanoService) MoverAtividade(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	mov MoverAtividadeInput,
) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	destino, ok := parseISODate(mov.Data)
	if !ok {
		return PlanoResposta{}, ErrValidacao{Msg: "data inválida"}
	}

	res, atividades, err := s.prepararAtividades(ctx, c, salvo)
	if err != nil {
		return PlanoResposta{}, err
	}

	// The client may address an activity by the deterministic slot id it was
	// served before that activity was stored. Resolve it against what now
	// exists, so the drag works instead of failing "não encontrada".
	if plano.EhIDDerivado(mov.ID) {
		resolvido, ok := plano.ResolverIDDerivado(atividades, mov.ID)
		if !ok {
			return PlanoResposta{}, ErrValidacao{Msg: "atividade não encontrada"}
		}

		mov.ID = resolvido
	}

	// Only a CONCLUDED ACTIVITY is locked — moving one would rewrite what was
	// actually studied. A day is not locked merely for holding some finished
	// work: its other subjects stay rearrangeable.
	//
	// The day-level guard is kept for the destination, where an activity-level
	// one has nothing to point at: dropping into a day that is entirely done
	// would still contradict its record.
	concluido := s.diaConcluido(salvo)

	if atividadeConcluida(salvo, atividades, mov.ID) {
		return PlanoResposta{}, ErrValidacao{
			Msg: "uma matéria já concluída não pode ser movida",
		}
	}

	mover := plano.MoverAtividade
	if mov.Trocar {
		mover = plano.TrocarAtividades
	}

	movidas, err := mover(
		atividades, res.Dias, mov.ID, destino, mov.Posicao, concluido,
	)

	if err != nil {
		return PlanoResposta{}, erroDeReplanejamento(err)
	}

	if err := s.planos.ReplaceAtividades(ctx, salvo.ID, movidas); err != nil {
		return PlanoResposta{}, err
	}

	return s.montar(ctx, c, salvo)
}

// Reordenar swaps the content of two days and persists both as overrides.
func (s *PlanoService) Reordenar(
	ctx context.Context,
	userID uuid.UUID,
	slug, dataAISO, dataBISO string,
) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	dtA, okA := parseISODate(dataAISO)
	dtB, okB := parseISODate(dataBISO)

	if !okA || !okB {
		return PlanoResposta{}, ErrValidacao{Msg: "datas inválidas"}
	}

	res := plano.Gerar(salvo.Config, &c)
	plano.AplicarReordenacoes(res.Dias, salvo.Reordenacoes)

	concluido := func(d time.Time) bool {
		r, ok := salvo.Registros[plano.DayOf(d)]

		return ok && r.Concluido
	}

	ovA, ovB, err := plano.Trocar(res.Dias, dtA, dtB, concluido)
	if errors.Is(err, plano.ErrReordenacaoInvalida) {
		return PlanoResposta{}, ErrValidacao{Msg: "esses dias não podem ser trocados"}
	}

	if err != nil {
		return PlanoResposta{}, err
	}

	salvo.Reordenacoes[plano.DayOf(dtA)] = ovA
	salvo.Reordenacoes[plano.DayOf(dtB)] = ovB

	if err := s.planos.ReplaceReordenacoes(ctx, salvo.ID, salvo.Reordenacoes); err != nil {
		return PlanoResposta{}, err
	}

	return s.montar(ctx, c, salvo)
}

// LimparRegistros drops every daily log and milestone check.
func (s *PlanoService) LimparRegistros(ctx context.Context, userID uuid.UUID, slug string) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	if err := s.planos.DeleteRegistros(ctx, salvo.ID); err != nil {
		return PlanoResposta{}, err
	}

	salvo.Registros = map[time.Time]plano.Registro{}
	salvo.Marcos = map[uuid.UUID]bool{}

	return s.montar(ctx, c, salvo)
}

// CompactarPlano pulls the schedule back over any empty study day from today
// onwards, then fills whatever that frees at the end of the learning phase.
//
// Runs by itself whenever a topic is finished early, and is exposed on its own
// so a plan that already has gaps — from before that was automatic — can be
// tidied without waiting for the next completion.
func (s *PlanoService) CompactarPlano(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	res, atividades, err := s.prepararAtividades(ctx, c, salvo)
	if err != nil {
		return PlanoResposta{}, err
	}

	reorganizadas, orfaos := s.reorganizar(salvo, replanejamento{
		res:        res,
		atividades: atividades,
		desde:      plano.DayOf(s.clock.Now()),
	})

	if err := s.planos.ReplaceAtividades(ctx, salvo.ID, reorganizadas); err != nil {
		return PlanoResposta{}, err
	}

	if err := s.descartarRegistros(ctx, salvo, orfaos); err != nil {
		return PlanoResposta{}, err
	}

	return s.montar(ctx, c, salvo)
}

// AtualizarCadernoDisciplina sets one discipline's error-notebook link and
// returns the refreshed plan, so the schedule's review block can pick up the
// new shortcut without a full concurso edit.
func (s *PlanoService) AtualizarCadernoDisciplina(
	ctx context.Context,
	userID uuid.UUID,
	slug, codigo, url string,
) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	if c.DisciplinaByCodigo(codigo) == nil {
		return PlanoResposta{}, ErrValidacao{Msg: "matéria não encontrada"}
	}

	if err := s.concursos.SetCadernoURL(ctx, c.ID, codigo, strings.TrimSpace(url)); err != nil {
		return PlanoResposta{}, err
	}

	c, err = s.concursos.ConcursoBySlug(ctx, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	return s.montar(ctx, c, salvo)
}

// RestaurarOrdem discards every manual reordering.
func (s *PlanoService) RestaurarOrdem(ctx context.Context, userID uuid.UUID, slug string) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	if err := s.planos.ReplaceReordenacoes(ctx, salvo.ID, map[time.Time]plano.Reordenacao{}); err != nil {
		return PlanoResposta{}, err
	}

	salvo.Reordenacoes = map[time.Time]plano.Reordenacao{}

	return s.montar(ctx, c, salvo)
}

func (s *PlanoService) carregar(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
) (concurso.Concurso, plano.Salvo, error) {
	c, err := s.concursoDoDono(ctx, userID, slug)
	if err != nil {
		return concurso.Concurso{}, plano.Salvo{}, err
	}

	salvo, err := s.garantirPlano(ctx, userID, c)
	if err != nil {
		return concurso.Concurso{}, plano.Salvo{}, err
	}

	return c, salvo, nil
}

// concursoDoDono loads a concurso by slug and 404s if it is not the user's.
func (s *PlanoService) concursoDoDono(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
) (concurso.Concurso, error) {
	c, err := s.concursos.ConcursoBySlug(ctx, slug)
	if err != nil {
		return concurso.Concurso{}, err
	}

	if c.OwnerID != userID {
		return concurso.Concurso{}, concurso.ErrNotFound
	}

	return c, nil
}

func (s *PlanoService) garantirPlano(
	ctx context.Context,
	userID uuid.UUID,
	c concurso.Concurso,
) (plano.Salvo, error) {
	salvo, err := s.planos.PlanoByUser(ctx, userID, c.ID)
	if err == nil {
		return salvo, nil
	}

	if !errors.Is(err, plano.ErrNotFound) {
		return plano.Salvo{}, err
	}

	novo := plano.NewSalvo()
	novo.UserID = userID
	novo.ConcursoID = c.ID
	// Dark is the app's default look; "system" and "light" stay available in the
	// sidebar for whoever prefers them.
	novo.TemaUI = "dark"
	novo.Config = defaultConfig(c, s.clock.Now())

	return s.planos.UpsertPlano(ctx, novo)
}

func defaultConfig(c concurso.Concurso, agora time.Time) plano.Config {
	questoes := map[string]int{}
	for _, d := range c.Disciplinas {
		questoes[d.Codigo] = d.QuestoesPadrao
	}

	inicio := plano.DayOf(agora)
	if !inicio.Before(plano.DayOf(c.ProvaPadrao)) {
		inicio = plano.AddDays(c.ProvaPadrao, -30)
	}

	cfg := plano.ConfigPadrao()
	cfg.Inicio = inicio
	cfg.Prova = plano.DayOf(c.ProvaPadrao)
	cfg.HorasDia = 2
	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.DiaRevisao = 5
	cfg.RetaFinalDias = c.RetaPadraoDias
	cfg.Questoes = questoes
	// MinutosBloco stays 0: HorasDia is the artifact's flat 2h until the user
	// sets an explicit block length, at which point it starts driving the day.

	return cfg.Normalizar()
}

// minutosDe is the block length implied by HorasDia / BlocosPorDia / PctRevisao —
// the number the config screen shows and, once saved, the number that drives the
// day.
func minutosDe(cfg plano.Config) int {
	if cfg.BlocosPorDia <= 0 {
		return 0
	}

	m := (cfg.HorasDia*60 - float64(cfg.MinutosRevisao)) / float64(cfg.BlocosPorDia)

	return int(math.Round(m/5)) * 5
}

func aplicarConfigInput(
	salvo plano.Salvo,
	c concurso.Concurso,
	in ConfigInput,
) (plano.Config, string, error) {
	cfg := salvo.Config

	if in.Inicio != "" {
		d, ok := parseISODate(in.Inicio)
		if !ok {
			return plano.Config{}, "", ErrValidacao{Msg: "data de início inválida"}
		}

		cfg.Inicio = d
	}

	if in.Prova != "" {
		d, ok := parseISODate(in.Prova)
		if !ok {
			return plano.Config{}, "", ErrValidacao{Msg: "data da prova inválida"}
		}

		cfg.Prova = d
	}

	if !cfg.Inicio.Before(cfg.Prova) {
		return plano.Config{}, "", ErrValidacao{Msg: "o início precisa ser antes da prova"}
	}

	// HorasDia may still be sent directly by an old client; MinutosBloco (below)
	// takes precedence — Normalizar recomputes HorasDia from it.
	if in.HorasDia > 0 {
		cfg.HorasDia = clampFloat(in.HorasDia, 0.5, 14)
	}

	if len(in.DiasEstudo) >= 2 {
		cfg.DiasEstudo = normalizarDias(in.DiasEstudo)
	}

	if in.DiaRevisao != nil && *in.DiaRevisao >= 0 && *in.DiaRevisao <= 6 {
		cfg.DiaRevisao = *in.DiaRevisao
	}

	if !containsInt(cfg.DiasEstudo, cfg.DiaRevisao) {
		cfg.DiasEstudo = normalizarDias(append(cfg.DiasEstudo, cfg.DiaRevisao))
	}

	if in.RetaFinalDias > 0 {
		cfg.RetaFinalDias = clampInt(in.RetaFinalDias, 7, 120)
	}

	if cfg.Questoes == nil {
		cfg.Questoes = map[string]int{}
	}

	for codigo, q := range in.Questoes {
		if c.DisciplinaByCodigo(codigo) == nil {
			continue
		}

		cfg.Questoes[codigo] = maxZero(q)
	}

	for _, d := range c.Disciplinas {
		if _, ok := cfg.Questoes[d.Codigo]; !ok {
			cfg.Questoes[d.Codigo] = d.QuestoesPadrao
		}
	}

	aplicarMetodoInput(&cfg, c, in)

	tema := salvo.TemaUI
	switch in.TemaUI {
	case "light", "dark", "system":
		tema = in.TemaUI
	}

	// Normalizar clamps every method field and recomputes HorasDia from
	// MinutosBloco when it is set, so the stored horas_dia stays in step.
	return cfg.Normalizar(), tema, nil
}

// aplicarMetodoInput patches the study-method fields of cfg in place. Every
// field is optional — a nil field leaves that setting as it was, so a patch that
// only touches one control never resets the rest.
func aplicarMetodoInput(cfg *plano.Config, c concurso.Concurso, in ConfigInput) {
	if in.Simulados != nil {
		cfg.Simulados = plano.Frequencia(*in.Simulados)
	}

	if in.Discursiva != nil {
		cfg.Discursiva = *in.Discursiva
	}

	if in.PctQuestoes != nil {
		cfg.PctQuestoes = *in.PctQuestoes
	}

	if in.LimiarFraco != nil {
		cfg.LimiarFraco = *in.LimiarFraco
	}

	if in.BlocosPorDia != nil {
		cfg.BlocosPorDia = *in.BlocosPorDia
	}

	if in.MinutosRevisao != nil {
		cfg.MinutosRevisao = *in.MinutosRevisao
	}

	if in.MinutosBloco != nil {
		cfg.MinutosBloco = *in.MinutosBloco
	}

	if in.RevisaoSemanal != nil {
		cfg.RevisaoSemanal = *in.RevisaoSemanal
	}

	if in.CicloRevisao != nil {
		cfg.CicloRevisao = cicloDoInput(*in.CicloRevisao)
	}

	if cfg.Modos == nil {
		cfg.Modos = map[string]plano.Modo{}
	}

	for codigo, modo := range in.Modos {
		if c.DisciplinaByCodigo(codigo) == nil {
			continue
		}

		cfg.Modos[codigo] = plano.Modo(modo)
	}

	if cfg.Reforcos == nil {
		cfg.Reforcos = map[string]float64{}
	}

	for codigo, r := range in.Reforcos {
		if c.DisciplinaByCodigo(codigo) == nil {
			continue
		}

		cfg.Reforcos[codigo] = r
	}
}

// cicloDoInput turns the wire form of the weekly rotation into domain items.
// An empty list means "use the default cycle", not "no weekly review".
func cicloDoInput(in []CicloItemInput) []concurso.RevItem {
	out := make([]concurso.RevItem, 0, len(in))

	for _, it := range in {
		out = append(out, concurso.RevItem{
			Ordem:    len(out),
			Titulo:   strings.TrimSpace(it.Titulo),
			Questoes: maxZero(it.Questoes),
		})
	}

	return out
}

func normalizarDias(dias []int) []int {
	seen := map[int]bool{}
	out := []int{}

	for _, d := range dias {
		if d < 0 || d > 6 || seen[d] {
			continue
		}

		seen[d] = true
		out = append(out, d)
	}

	// Monday-first ordering, matching the artifact's day picker.
	sort.Slice(out, func(i, j int) bool {
		return (out[i]+6)%7 < (out[j]+6)%7
	})

	return out
}

func prune(dias []plano.Dia, overrides map[time.Time]plano.Reordenacao) map[time.Time]plano.Reordenacao {
	// AplicarReordenacoes mutates a copy of the days; we only want the "which
	// survived" result.
	copia := make([]plano.Dia, len(dias))
	copy(copia, dias)

	validas := plano.AplicarReordenacoes(copia, overrides)

	out := map[time.Time]plano.Reordenacao{}
	for data := range validas {
		out[data] = overrides[data]
	}

	return out
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}

func maxZero(v int) int {
	if v < 0 {
		return 0
	}

	return v
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}

	return false
}

// blocosDoInput keeps only the blocks whose discipline exists in the concurso
// and that actually carry a value.
//
// One row per ACTIVITY when the caller addresses activities, so a day that
// schedules the same discipline twice records each occurrence separately;
// otherwise one row per discipline, as before.
func blocosDoInput(c concurso.Concurso, in []RegistroBlocoInput) []plano.RegistroBloco {
	out := make([]plano.RegistroBloco, 0, len(in))
	visto := map[string]bool{}

	for _, b := range in {
		codigo := strings.TrimSpace(b.Disciplina)
		if codigo == "" || c.DisciplinaByCodigo(codigo) == nil {
			continue
		}

		// Dedupe on the activity when there is one, so two occurrences of the
		// same subject in a day no longer collapse into a single row.
		chave := strings.TrimSpace(b.AtividadeID)
		if chave == "" {
			chave = "disc:" + codigo
		}

		if visto[chave] {
			continue
		}

		// Concluido counts as a value: ticking a discipline with no hours yet is
		// a legitimate state, and dropping the row here would make the check
		// silently fail to persist.
		if b.Horas == nil && b.Questoes == nil && b.Acertos == nil &&
			strings.TrimSpace(b.Nota) == "" && !b.Concluido {
			continue
		}

		visto[chave] = true

		// A synthetic slot id ("gen:<date>:<pos>") addresses an activity that was
		// never stored, so it is not a uuid and must not reach the column. The
		// record still lands, keyed by discipline, exactly as before activities
		// were addressable — the next move materialises the activity and later
		// records carry the real id.
		atividadeID := strings.TrimSpace(b.AtividadeID)
		if plano.EhIDDerivado(atividadeID) {
			atividadeID = ""
		}

		out = append(out, plano.RegistroBloco{
			Disciplina:  codigo,
			Horas:       b.Horas,
			Questoes:    b.Questoes,
			Acertos:     naoMaiorQue(b.Acertos, b.Questoes),
			Nota:        strings.TrimSpace(b.Nota),
			Concluido:   b.Concluido,
			AtividadeID: atividadeID,
		})
	}

	return out
}

// naoMaiorQue clamps acertos to questoes, so erros never goes negative.
func naoMaiorQue(acertos, questoes *int) *int {
	if acertos == nil || questoes == nil || *acertos <= *questoes {
		return acertos
	}

	v := *questoes

	return &v
}
