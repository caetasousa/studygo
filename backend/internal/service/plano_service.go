package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"annygo/internal/domain/concurso"
	"annygo/internal/domain/plano"
	"annygo/internal/port"

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

	return s.montar(ctx, c, salvo)
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

	reg := plano.Registro{
		Data:      data,
		Horas:     in.Horas,
		Concluido: in.Concluido,
		Questoes:  in.Questoes,
		Acertos:   in.Acertos,
		Nota:      in.Nota,
	}

	// Mirror the artifact: logging hours auto-marks the day done.
	if reg.Horas != nil && *reg.Horas != 0 && !reg.Concluido {
		reg.Concluido = true
	}

	if err := s.planos.UpsertRegistro(ctx, salvo.ID, reg); err != nil {
		return PlanoResposta{}, err
	}

	salvo.Registros[data] = reg

	return s.montar(ctx, c, salvo)
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
	novo.TemaUI = "system"
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

	return plano.Config{
		Inicio:        inicio,
		Prova:         plano.DayOf(c.ProvaPadrao),
		HorasDia:      2,
		DiasEstudo:    []int{1, 2, 3, 4, 5},
		DiaRevisao:    5,
		RetaFinalDias: c.RetaPadraoDias,
		Questoes:      questoes,
	}
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

	if in.HorasDia > 0 {
		cfg.HorasDia = clampFloat(in.HorasDia, 0.5, 14)
	}

	if len(in.DiasEstudo) >= 2 {
		cfg.DiasEstudo = normalizarDias(in.DiasEstudo)
	}

	if in.DiaRevisao >= 0 && in.DiaRevisao <= 6 {
		cfg.DiaRevisao = in.DiaRevisao
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

	tema := salvo.TemaUI
	switch in.TemaUI {
	case "light", "dark", "system":
		tema = in.TemaUI
	}

	return cfg, tema, nil
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
