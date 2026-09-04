package plano_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
)

// fixtureConcurso mirrors the JS constants dumped by scratchpad/gen_golden.mjs.
type fixtureConcurso struct {
	Disciplinas []struct {
		Codigo         string   `json:"codigo"`
		Nome           string   `json:"nome"`
		Bloco          string   `json:"bloco"`
		Peso           int      `json:"peso"`
		QuestoesPadrao int      `json:"questoesPadrao"`
		Ordem          int      `json:"ordem"`
		Temas          []string `json:"temas"`
	} `json:"disciplinas"`
	RevCiclo []struct {
		Ordem    int    `json:"ordem"`
		Titulo   string `json:"titulo"`
		Questoes int    `json:"questoes"`
	} `json:"revCiclo"`
}

type golden struct {
	SomaPontos int            `json:"somaPontos"`
	Pontos     map[string]int `json:"pontos"`
	Slots      map[string]int `json:"slots"`
	SlotsReta  map[string]int `json:"slotsReta"`
	Dias       []struct {
		N      int    `json:"n"`
		Data   string `json:"data"`
		Semana int    `json:"semana"`
		Fase   string `json:"fase"`
		Tipo   string `json:"tipo"`
		Tema   string `json:"tema"`
		Meta   int    `json:"meta"`
		Itens  []struct {
			Disciplina string `json:"disciplina"`
			Tema       string `json:"tema"`
			Passada    int    `json:"passada"`
		} `json:"itens"`
	} `json:"dias"`
}

func loadConcurso(t *testing.T) concurso.Concurso {
	t.Helper()

	raw, err := os.ReadFile("testdata/concurso_tcego.json")
	if err != nil {
		t.Fatalf("reading concurso fixture: %v", err)
	}

	var fx fixtureConcurso
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("decoding concurso fixture: %v", err)
	}

	c := concurso.Concurso{
		Slug:        "tce-go-b02",
		ProvaPadrao: date(2027, 1, 17),
	}

	for _, d := range fx.Disciplinas {
		c.Disciplinas = append(c.Disciplinas, concurso.Disciplina{
			Codigo:         d.Codigo,
			Nome:           d.Nome,
			Bloco:          concurso.Bloco(d.Bloco),
			Peso:           d.Peso,
			QuestoesPadrao: d.QuestoesPadrao,
			Ordem:          d.Ordem,
			Temas:          d.Temas,
		})
	}

	for _, r := range fx.RevCiclo {
		c.RevCiclo = append(c.RevCiclo, concurso.ItemRevisao{
			Ordem:    r.Ordem,
			Titulo:   r.Titulo,
			Questoes: r.Questoes,
		})
	}

	return c
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func defaultConfig(c concurso.Concurso) plano.Config {
	q := map[string]int{}
	for _, d := range c.Disciplinas {
		q[d.Codigo] = d.QuestoesPadrao
	}

	return plano.Config{
		Inicio:        date(2026, 9, 1),
		Prova:         date(2027, 1, 17),
		HorasDia:      2,
		DiasEstudo:    []int{1, 2, 3, 4, 5},
		DiaRevisao:    5,
		RetaFinalDias: 28,
		Questoes:      q,
		// The artifact reserved a whole day of each week for review. That is no
		// longer the default — review is a daily tail now — so the golden test
		// asks for it explicitly, which is what keeps it a faithful comparison.
		RevisaoSemanal: true,
	}
}

// TestGerar_MatchesArtifact is the golden test: the Go engine must reproduce the
// artifact's construir() output byte-for-byte for the default configuration.
func TestGerar_MatchesArtifact(t *testing.T) {
	t.Parallel()

	c := loadConcurso(t)

	raw, err := os.ReadFile("testdata/golden_tcego_default.json")
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}

	var want golden
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("decoding golden: %v", err)
	}

	got := plano.Gerar(defaultConfig(c), &c)

	if got.SomaPontos != want.SomaPontos {
		t.Errorf("somaPontos = %d, want %d", got.SomaPontos, want.SomaPontos)
	}

	assertIntMap(t, "pontos", got.Pontos, want.Pontos)
	assertIntMap(t, "slots", got.Slots, want.Slots)
	assertIntMap(t, "slotsReta", got.SlotsReta, want.SlotsReta)

	if len(got.Dias) != len(want.Dias) {
		t.Fatalf("len(dias) = %d, want %d", len(got.Dias), len(want.Dias))
	}

	for i, wd := range want.Dias {
		gd := got.Dias[i]

		gotData := gd.Data.Format("2006-01-02")
		if gd.N != wd.N || gotData != wd.Data || gd.Semana != wd.Semana ||
			string(gd.Fase) != wd.Fase || string(gd.Tipo) != wd.Tipo ||
			gd.Tema != wd.Tema || gd.Meta != wd.Meta {
			t.Errorf("dia[%d] = {n:%d data:%s sem:%d fase:%s tipo:%s meta:%d tema:%q}, want {n:%d data:%s sem:%d fase:%s tipo:%s meta:%d tema:%q}",
				i, gd.N, gotData, gd.Semana, gd.Fase, gd.Tipo, gd.Meta, gd.Tema,
				wd.N, wd.Data, wd.Semana, wd.Fase, wd.Tipo, wd.Meta, wd.Tema)

			continue
		}

		if len(gd.Itens) != len(wd.Itens) {
			t.Errorf("dia[%d] itens count = %d, want %d", i, len(gd.Itens), len(wd.Itens))

			continue
		}

		for j, wi := range wd.Itens {
			gi := gd.Itens[j]
			if gi.Disciplina != wi.Disciplina || gi.Tema != wi.Tema || gi.Passada != wi.Passada {
				t.Errorf("dia[%d].itens[%d] = %+v, want %+v", i, j, gi, wi)
			}
		}
	}
}

// TestGerar_Propriedades checks invariants that must hold for any configuration.
func TestGerar_Propriedades(t *testing.T) {
	t.Parallel()

	c := loadConcurso(t)

	tests := []struct {
		name string
		cfg  plano.Config
	}{
		{
			name: "default",
			cfg:  defaultConfig(c),
		},
		{
			name: "seis dias, reta longa",
			cfg: func() plano.Config {
				cfg := defaultConfig(c)
				cfg.HorasDia = 3
				cfg.DiasEstudo = []int{1, 2, 3, 4, 5, 6}
				cfg.RetaFinalDias = 49
				return cfg
			}(),
		},
		{
			name: "início tardio",
			cfg: func() plano.Config {
				cfg := defaultConfig(c)
				cfg.Inicio = date(2026, 12, 1)
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res := plano.Gerar(tt.cfg, &c)

			if len(res.Dias) == 0 {
				t.Fatal("plano vazio")
			}

			last := res.Dias[len(res.Dias)-1]
			if last.Tipo != plano.TipoVespera {
				t.Errorf("último dia é %s, esperava vespera", last.Tipo)
			}

			if last.Fase != plano.FaseReta {
				t.Errorf("véspera está na fase %s, esperava reta", last.Fase)
			}

			semanaAtual := 0
			for i, d := range res.Dias {
				if d.N != i+1 {
					t.Errorf("dia[%d].N = %d, esperava %d", i, d.N, i+1)
				}

				if d.Semana != semanaAtual && d.Semana != semanaAtual+1 {
					t.Errorf("semana pulou de %d para %d no dia %d", semanaAtual, d.Semana, d.N)
				}

				if d.Semana > semanaAtual {
					semanaAtual = d.Semana
				}

				if len(d.Itens) == 2 && d.Itens[0].Disciplina == d.Itens[1].Disciplina {
					t.Errorf("dia %d tem os dois blocos da mesma disciplina (%s)", d.N, d.Itens[0].Disciplina)
				}
			}
		})
	}
}

// TestGerar_ConcursoMinimo covers a user-registered concurso with the bare
// minimum: a couple of disciplines, no topics, no review cycle. It must still
// produce a plan without panicking.
func TestGerar_ConcursoMinimo(t *testing.T) {
	t.Parallel()

	c := concurso.Concurso{
		Slug:        "tj-xx",
		ProvaPadrao: date(2026, 6, 1),
		Disciplinas: []concurso.Disciplina{
			{Codigo: "D01", Nome: "Direito Constitucional", Bloco: concurso.BlocoEspecifico, Peso: 2, Ordem: 0},
			{Codigo: "D02", Nome: "Língua Portuguesa", Bloco: concurso.BlocoGeral, Peso: 1, Ordem: 1},
		},
	}

	cfg := plano.Config{
		Inicio:         date(2026, 2, 2),
		Prova:          date(2026, 6, 1),
		HorasDia:       2,
		DiasEstudo:     []int{1, 2, 3, 4, 5},
		DiaRevisao:     5,
		RevisaoSemanal: true,
		RetaFinalDias:  28,
		Questoes:       map[string]int{"D01": 15, "D02": 10},
	}

	res := plano.Gerar(cfg, &c)

	if len(res.Dias) == 0 {
		t.Fatal("plano vazio")
	}

	viuNomeDisciplina := false

	for _, d := range res.Dias {
		if d.Tipo != plano.TipoEstudo {
			continue
		}

		if len(d.Itens) == 0 {
			t.Errorf("dia de estudo %d sem itens", d.N)
			continue
		}

		for _, it := range d.Itens {
			if it.Tema == "" {
				t.Errorf("dia %d: item com tema vazio", d.N)
			}

			if it.Tema == "Direito Constitucional" || it.Tema == "Língua Portuguesa" {
				viuNomeDisciplina = true
			}
		}
	}

	if !viuNomeDisciplina {
		t.Error("esperava que dias de estudo usassem o nome da disciplina como tema")
	}

	var temRev bool
	for _, d := range res.Dias {
		if d.Tipo == plano.TipoRevisaoSemanal && d.Tema != "" {
			temRev = true
		}
	}

	if !temRev {
		t.Error("esperava dias de revisão semanal com título do ciclo padrão")
	}
}

func assertIntMap(t *testing.T, label string, got, want map[string]int) {
	t.Helper()

	for k, wv := range want {
		if got[k] != wv {
			t.Errorf("%s[%s] = %d, want %d", label, k, got[k], wv)
		}
	}
}
