package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
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
		Blocos:    blocosDoInput(c, in.Blocos),
	}

	// Com lançamento por disciplina, os totais do dia são a soma dos blocos — o
	// cliente não precisa mandá-los, e se mandar, os blocos ganham.
	if len(reg.Blocos) > 0 {
		reg.Horas, reg.Questoes, reg.Acertos = reg.Totais()
	}

	// Mirror the artifact: logging hours auto-marks the day done.
	if reg.Horas != nil && *reg.Horas != 0 && !reg.Concluido {
		reg.Concluido = true
	}

	if err := s.planos.UpsertRegistro(ctx, salvo.ID, reg); err != nil {
		return PlanoResposta{}, err
	}

	salvo.Registros[data] = reg

	// Concluir um dia enfileira a revisão dos temas que ele cobriu.
	if reg.Concluido {
		if err := s.enfileirarDoDia(ctx, c, &salvo, data); err != nil {
			return PlanoResposta{}, err
		}
	}

	return s.montar(ctx, c, salvo)
}

// enfileirarDoDia queues the spaced review for every topic the given day
// covered. It is idempotent: the repository ignores a topic already queued at
// the same stage, so re-saving a day never duplicates its entries.
func (s *PlanoService) enfileirarDoDia(
	ctx context.Context,
	c concurso.Concurso,
	salvo *plano.Salvo,
	data time.Time,
) error {
	res := plano.Gerar(salvo.Config, &c)
	plano.AplicarReordenacoes(res.Dias, salvo.Reordenacoes)

	for _, d := range res.Dias {
		if !plano.DayOf(d.Data).Equal(plano.DayOf(data)) {
			continue
		}

		novas := plano.Enfileirar(salvo.Config, d)
		if len(novas) == 0 {
			return nil
		}

		if err := s.planos.EnfileirarRevisoes(ctx, salvo.ID, novas); err != nil {
			return err
		}

		revisoes, err := s.planos.ListRevisoes(ctx, salvo.ID)
		if err != nil {
			return err
		}

		salvo.Revisoes = revisoes

		return nil
	}

	return nil
}

// RegistrarRevisao records how a queued review went. The hit rate decides where
// the topic goes next — up an interval when it is solid, back down when it is
// not — and a bad result also opens an entry in the error notebook.
func (s *PlanoService) RegistrarRevisao(
	ctx context.Context,
	userID uuid.UUID,
	slug string,
	in RevisaoInput,
) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	rev, err := s.planos.RevisaoByID(ctx, salvo.ID, in.ID)
	if err != nil {
		return PlanoResposta{}, err
	}

	if in.Questoes < 0 || in.Acertos < 0 || in.Acertos > in.Questoes {
		return PlanoResposta{}, ErrValidacao{Msg: "acertos precisa estar entre 0 e o total de questões"}
	}

	hoje := plano.DayOf(s.clock.Now())

	feita := rev
	feita.FeitaEm = &hoje
	feita.Questoes = &in.Questoes
	feita.Acertos = &in.Acertos

	var proxima *plano.Revisao
	if p, ok := rev.Resultado(salvo.Config, hoje, in.Questoes, in.Acertos); ok {
		proxima = &p
	}

	if err := s.planos.ConcluirRevisao(ctx, salvo.ID, feita, proxima); err != nil {
		return PlanoResposta{}, err
	}

	if plano.Fraca(in.Questoes, in.Acertos) {
		if err := s.anotarErro(ctx, c, salvo, feita); err != nil {
			return PlanoResposta{}, err
		}
	}

	_, salvo, err = s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	return s.montar(ctx, c, salvo)
}

// anotarErro opens a notebook entry for a topic that went badly, already tagged
// with its discipline and topic — the user only has to write down *why*.
func (s *PlanoService) anotarErro(
	ctx context.Context,
	c concurso.Concurso,
	salvo plano.Salvo,
	rev plano.Revisao,
) error {
	pct := plano.Aproveitamento(derefInt(rev.Questoes), derefInt(rev.Acertos))

	a := plano.Anotacao{
		Data:   rev.FeitaEm,
		Tema:   rev.Tema,
		Origem: plano.OrigemRevisao,
		Texto: fmt.Sprintf(
			"%d%% na revisão de %q (%d de %d) — anote por que errou, não só a resposta.",
			pct, rev.Tema, derefInt(rev.Acertos), derefInt(rev.Questoes),
		),
	}

	if d := c.DisciplinaByCodigo(rev.Disciplina); d != nil {
		id := d.ID
		a.DisciplinaID = &id
	}

	_, err := s.planos.CreateAnotacao(ctx, salvo.ID, a)

	return err
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

	m := cfg.HorasDia * 60 * (1 - cfg.PctRevisao) / float64(cfg.BlocosPorDia)

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

	if in.Intervalos != nil {
		cfg.Intervalos = *in.Intervalos
	}

	if in.PctQuestoes != nil {
		cfg.PctQuestoes = *in.PctQuestoes
	}

	if in.RevisaoPorQuestoes != nil {
		cfg.RevisaoPorQuestoes = *in.RevisaoPorQuestoes
	}

	if in.QuestoesPorRevisao != nil {
		cfg.QuestoesPorRevisao = *in.QuestoesPorRevisao
	}

	if in.LimiarFraco != nil {
		cfg.LimiarFraco = *in.LimiarFraco
	}

	if in.BlocosPorDia != nil {
		cfg.BlocosPorDia = *in.BlocosPorDia
	}

	if in.MinutosBloco != nil {
		cfg.MinutosBloco = *in.MinutosBloco
	}

	if in.PctRevisao != nil {
		cfg.PctRevisao = *in.PctRevisao
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
// and that actually carry a value, one row per discipline.
func blocosDoInput(c concurso.Concurso, in []RegistroBlocoInput) []plano.RegistroBloco {
	out := make([]plano.RegistroBloco, 0, len(in))
	visto := map[string]bool{}

	for _, b := range in {
		codigo := strings.TrimSpace(b.Disciplina)
		if codigo == "" || visto[codigo] || c.DisciplinaByCodigo(codigo) == nil {
			continue
		}

		if b.Horas == nil && b.Questoes == nil && b.Acertos == nil && strings.TrimSpace(b.Nota) == "" {
			continue
		}

		visto[codigo] = true

		out = append(out, plano.RegistroBloco{
			Disciplina: codigo,
			Horas:      b.Horas,
			Questoes:   b.Questoes,
			Acertos:    naoMaiorQue(b.Acertos, b.Questoes),
			Nota:       strings.TrimSpace(b.Nota),
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
