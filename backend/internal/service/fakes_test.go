package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// In-memory doubles for the two repositories PlanoService depends on.
//
// They exist so the SERVICE can be tested, not just the domain functions it
// calls. Every bug that has escaped to production in this area lived in the
// orchestration — which activities get materialised, in what order the
// reconciliation runs, what gets persisted — and none of it is reachable from a
// pure-domain test.
//
// The fakes mimic the two Postgres behaviours the service silently depends on:
// ReplaceAtividades assigns an id to every row that arrives without one (the
// column default), and ListAtividades returns rows ordered by (data, posicao).

type fakeConcursos struct {
	c concurso.Concurso
}

func (f *fakeConcursos) ListByOwner(context.Context, uuid.UUID) ([]concurso.Concurso, error) {
	return []concurso.Concurso{f.c}, nil
}

func (f *fakeConcursos) ConcursoBySlug(_ context.Context, slug string) (concurso.Concurso, error) {
	if slug != f.c.Slug {
		return concurso.Concurso{}, concurso.ErrNotFound
	}

	return f.c, nil
}

func (f *fakeConcursos) ConcursoByID(_ context.Context, id uuid.UUID) (concurso.Concurso, error) {
	if id != f.c.ID {
		return concurso.Concurso{}, concurso.ErrNotFound
	}

	return f.c, nil
}

func (f *fakeConcursos) CreateConcurso(_ context.Context, c concurso.Concurso) (concurso.Concurso, error) {
	f.c = c

	return c, nil
}

func (f *fakeConcursos) UpdateConcurso(_ context.Context, c concurso.Concurso) (concurso.Concurso, error) {
	f.c = c

	return c, nil
}

func (f *fakeConcursos) DeleteConcurso(context.Context, uuid.UUID) error { return nil }

func (f *fakeConcursos) SetCadernoURL(_ context.Context, _ uuid.UUID, codigo, url string) error {
	for i := range f.c.Disciplinas {
		if f.c.Disciplinas[i].Codigo == codigo {
			f.c.Disciplinas[i].CadernoURL = url

			return nil
		}
	}

	return concurso.ErrNotFound
}

type fakePlanos struct {
	salvo      plano.Salvo
	atividades []plano.Atividade

	// gravacoes counts ReplaceAtividades calls, so a test can assert that a
	// read-only path did not write.
	gravacoes int
}

func (f *fakePlanos) PlanoByUser(context.Context, uuid.UUID, uuid.UUID) (plano.Salvo, error) {
	if f.salvo.ID == uuid.Nil {
		return plano.Salvo{}, plano.ErrNotFound
	}

	return f.salvo, nil
}

func (f *fakePlanos) UpsertPlano(_ context.Context, s plano.Salvo) (plano.Salvo, error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}

	f.salvo = s

	return s, nil
}

func (f *fakePlanos) ReplaceReordenacoes(
	_ context.Context,
	_ uuid.UUID,
	r map[time.Time]plano.Reordenacao,
) error {
	f.salvo.Reordenacoes = r

	return nil
}

func (f *fakePlanos) ListAtividades(context.Context, uuid.UUID) ([]plano.Atividade, error) {
	out := append([]plano.Atividade(nil), f.atividades...)

	// Postgres returns them ORDER BY data, posicao — the service leans on that.
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Data.Equal(out[j].Data) {
			return out[i].Data.Before(out[j].Data)
		}

		return out[i].Posicao < out[j].Posicao
	})

	return out, nil
}

func (f *fakePlanos) ReplaceAtividades(_ context.Context, _ uuid.UUID, as []plano.Atividade) error {
	f.gravacoes++

	novas := make([]plano.Atividade, len(as))
	copy(novas, as)

	// The column default: a row inserted without an id gets one.
	for i := range novas {
		if novas[i].ID == "" {
			novas[i].ID = uuid.NewString()
		}
	}

	// atividades_plano_data_posicao_key UNIQUE (plano_id, data, posicao): the real
	// table rejects a layout with two rows on the same day and slot.
	slots := make(map[string]bool, len(novas))
	for _, a := range novas {
		k := plano.DayOf(a.Data).Format(time.DateOnly) + ":" + strconv.Itoa(a.Posicao)
		if slots[k] {
			return fmt.Errorf("atividades_plano_data_posicao_key: duplicate (data, posicao) %s", k)
		}
		slots[k] = true
	}

	// The real repository keeps the surviving rows in place (UPSERT ON CONFLICT
	// (id)) and DELETEs only the ids absent from the new layout. Deletes cascade
	// to registros_bloco.atividade_id as ON DELETE SET NULL, so a record whose
	// activity really disappeared falls back to the legacy (data, disciplina)
	// key; a record whose activity survives keeps its link.
	sobrevive := make(map[string]bool, len(novas))
	for _, a := range novas {
		sobrevive[a.ID] = true
	}

	for data, reg := range f.salvo.Registros {
		mudou := false

		for i := range reg.Blocos {
			if id := reg.Blocos[i].AtividadeID; id != "" && !sobrevive[id] {
				reg.Blocos[i].AtividadeID = ""
				mudou = true
			}
		}

		if mudou {
			f.salvo.Registros[data] = reg
		}
	}

	// registros_bloco_legado_key UNIQUE (plano_id, data, disciplina) WHERE
	// atividade_id IS NULL: the cascade above must never collapse two records
	// onto the same legacy key. A blanket DELETE FROM atividades used to do
	// exactly that — nulling two same-discipline rows in one day — and wedged
	// the plan against every later write.
	for data, reg := range f.salvo.Registros {
		legado := map[string]bool{}
		for _, b := range reg.Blocos {
			if b.AtividadeID != "" {
				continue
			}
			k := plano.DayOf(data).Format(time.DateOnly) + "\x00" + b.Disciplina
			if legado[k] {
				return fmt.Errorf(
					"registros_bloco_legado_key: cascade collapsed two records onto (%s, %s)",
					plano.DayOf(data).Format(time.DateOnly), b.Disciplina,
				)
			}
			legado[k] = true
		}
	}

	f.atividades = novas

	return nil
}

func (f *fakePlanos) UpsertRegistro(_ context.Context, _ uuid.UUID, r plano.Registro) error {
	if f.salvo.Registros == nil {
		f.salvo.Registros = map[time.Time]plano.Registro{}
	}

	// The two partial unique indexes on registros_bloco:
	//   registros_bloco_atividade_key  UNIQUE (atividade_id) WHERE atividade_id IS NOT NULL
	//   registros_bloco_legado_key     UNIQUE (plano_id, data, disciplina) WHERE atividade_id IS NULL
	// UpsertRegistro replaces only THIS day's blocks, so a clash with another
	// day's rows is a real INSERT failure the service must not cause.
	porAtividade := map[string]bool{}
	porLegado := map[string]bool{}

	for data, reg := range f.salvo.Registros {
		if plano.DayOf(data).Equal(plano.DayOf(r.Data)) {
			continue // this day's rows are about to be deleted and re-inserted
		}
		for _, b := range reg.Blocos {
			if b.AtividadeID != "" {
				porAtividade[b.AtividadeID] = true
			} else {
				porLegado[plano.DayOf(data).Format(time.DateOnly)+"\x00"+b.Disciplina] = true
			}
		}
	}

	vistoAtividade := map[string]bool{}
	vistoLegado := map[string]bool{}
	for _, b := range r.Blocos {
		if b.AtividadeID != "" {
			if porAtividade[b.AtividadeID] || vistoAtividade[b.AtividadeID] {
				return fmt.Errorf("registros_bloco_atividade_key: atividade_id %s já tem registro", b.AtividadeID)
			}
			vistoAtividade[b.AtividadeID] = true
		} else {
			k := plano.DayOf(r.Data).Format(time.DateOnly) + "\x00" + b.Disciplina
			if vistoLegado[k] {
				return fmt.Errorf("registros_bloco_legado_key: (data, disciplina) %s duplicado", k)
			}
			vistoLegado[k] = true
		}
	}

	f.salvo.Registros[plano.DayOf(r.Data)] = r

	return nil
}

func (f *fakePlanos) DeleteRegistro(_ context.Context, _ uuid.UUID, data time.Time) error {
	delete(f.salvo.Registros, plano.DayOf(data))

	return nil
}

func (f *fakePlanos) DeleteRegistros(context.Context, uuid.UUID) error {
	f.salvo.Registros = map[time.Time]plano.Registro{}

	return nil
}

func (f *fakePlanos) UpsertRevisaoRegistro(
	_ context.Context,
	_ uuid.UUID,
	r plano.RegistroRevisao,
) error {
	if f.salvo.Revisoes == nil {
		f.salvo.Revisoes = map[time.Time]plano.RegistroRevisao{}
	}

	f.salvo.Revisoes[plano.DayOf(r.Data)] = r

	return nil
}

func (f *fakePlanos) SetMarco(_ context.Context, _, marcoID uuid.UUID, cumprido bool) error {
	if f.salvo.Marcos == nil {
		f.salvo.Marcos = map[uuid.UUID]bool{}
	}

	f.salvo.Marcos[marcoID] = cumprido

	return nil
}

func (f *fakePlanos) ListAnotacoes(context.Context, uuid.UUID) ([]plano.Anotacao, error) {
	return nil, nil
}

func (f *fakePlanos) CreateAnotacao(
	_ context.Context,
	_ uuid.UUID,
	a plano.Anotacao,
) (plano.Anotacao, error) {
	return a, nil
}

func (f *fakePlanos) UpdateAnotacao(
	_ context.Context,
	_ uuid.UUID,
	a plano.Anotacao,
) (plano.Anotacao, error) {
	return a, nil
}

func (f *fakePlanos) DeleteAnotacao(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (f *fakePlanos) ListPlanosParaLembrete(context.Context) ([]port.PlanoComEmail, error) {
	return nil, nil
}

// relogioFixo pins "now" so a plan's day numbering is deterministic.
type relogioFixo struct{ t time.Time }

func (r relogioFixo) Now() time.Time { return r.t }

// concursoDoCenario reaches the concurso the scenario was built with.
func concursoDoCenario(ce *cenario) *concurso.Concurso {
	c, _ := ce.svc.concursos.ConcursoBySlug(context.Background(), ce.slug)

	return &c
}
