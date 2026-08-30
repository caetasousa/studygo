package plano_test

import (
	"strings"
	"testing"

	"annygo/internal/domain/concurso"
	"annygo/internal/domain/plano"
)

func nomesTeste() map[string]string {
	return map[string]string{"D01": "Português", "D02": "Direito", "D03": "Informática"}
}

func itens(codigos ...string) []plano.ItemDia {
	out := make([]plano.ItemDia, 0, len(codigos))
	for _, c := range codigos {
		out = append(out, plano.ItemDia{Disciplina: c, Tema: "tema de " + c})
	}

	return out
}

func somaMinutos(bs []plano.Bloco) int {
	t := 0
	for _, b := range bs {
		t += b.Minutos
	}

	return t
}

func TestBlocos_repartePorNumeroDeBlocos(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome       string
		itens      []plano.ItemDia
		horas      float64
		temRevisao bool
		querBlocos int
		querTotal  int
	}{
		{
			nome: "dois blocos com revisão", itens: itens("D01", "D02"), horas: 2,
			temRevisao: true, querBlocos: 3, querTotal: 120,
		},
		{
			nome: "cinco blocos com revisão", itens: itens("D01", "D02", "D03", "D01", "D02"),
			horas: 5, temRevisao: true, querBlocos: 6, querTotal: 300,
		},
		{
			nome: "um bloco só", itens: itens("D01"), horas: 2,
			temRevisao: true, querBlocos: 2, querTotal: 120,
		},
		{
			nome:  "sem revisão vencendo, o tempo volta para o conteúdo",
			itens: itens("D01", "D02"), horas: 2, temRevisao: false,
			querBlocos: 2, querTotal: 120,
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			d := plano.Dia{Tipo: plano.TipoEstudo, Meta: 20, Itens: c.itens}
			if c.temRevisao {
				d.Revisoes = []plano.Revisao{{Tema: "x"}}
			}

			got := plano.Blocos(d, plano.BlocoCtx{
				HorasDia: c.horas,
				Nomes:    nomesTeste(),
				Perfil:   plano.PerfilPadrao(),
			})

			if len(got) != c.querBlocos {
				t.Fatalf("gerou %d blocos, queria %d", len(got), c.querBlocos)
			}

			if total := somaMinutos(got); total != c.querTotal {
				t.Errorf("total = %d min, queria %d (o dia inteiro)", total, c.querTotal)
			}
		})
	}
}

func TestBlocos_reforcoAumentaOBloco(t *testing.T) {
	t.Parallel()

	perfil := plano.PerfilPadrao()
	perfil.Reforcos = map[string]float64{"D02": 2}

	d := plano.Dia{
		Tipo:     plano.TipoEstudo,
		Meta:     30,
		Itens:    itens("D01", "D02"),
		Revisoes: []plano.Revisao{{Tema: "x"}},
	}

	got := plano.Blocos(d, plano.BlocoCtx{HorasDia: 3, Nomes: nomesTeste(), Perfil: perfil})

	if len(got) != 3 {
		t.Fatalf("gerou %d blocos, queria 3", len(got))
	}

	// 3h = 180min; 16% de revisão ≈ 30min; sobram ~150 divididos 1:2.
	if got[1].Minutos <= got[0].Minutos {
		t.Errorf("bloco reforçado tem %d min e o normal %d — o reforço não aumentou o tempo",
			got[1].Minutos, got[0].Minutos)
	}

	if total := somaMinutos(got); total != 180 {
		t.Errorf("total = %d min, queria 180", total)
	}

	// A bateria de questões acompanha o tempo.
	if !strings.Contains(got[1].Detalhe, "20 questões") {
		t.Errorf("detalhe do bloco reforçado = %q, queria 2/3 das 30 questões", got[1].Detalhe)
	}
}

func TestBlocos_modoPorDisciplina(t *testing.T) {
	t.Parallel()

	perfil := plano.PerfilPadrao()
	perfil.Modos = map[string]plano.Modo{"D01": plano.ModoQuestoes, "D02": plano.ModoTeoria}

	got := plano.Blocos(plano.Dia{
		Tipo:  plano.TipoEstudo,
		Meta:  20,
		Itens: itens("D01", "D02"),
	}, plano.BlocoCtx{HorasDia: 2, Nomes: nomesTeste(), Perfil: perfil})

	if strings.Contains(got[0].Detalhe, "teoria com resumo") {
		t.Errorf("bloco só-questões = %q, não devia pedir teoria", got[0].Detalhe)
	}

	if !strings.Contains(got[1].Detalhe, "sem bateria de questões") {
		t.Errorf("bloco só-teoria = %q, não devia pedir questões", got[1].Detalhe)
	}
}

func TestGerar_blocosPorDia(t *testing.T) {
	t.Parallel()

	// Pesos iguais de propósito: assim nenhuma disciplina precisa de mais de um
	// bloco por dia, e repetição dentro do dia seria falha do despareia.
	c := concurso.Concurso{
		Nome:        "X",
		ProvaPadrao: dia(2026, 8, 14),
		Disciplinas: []concurso.Disciplina{
			{Codigo: "D01", Nome: "A", Bloco: concurso.BlocoGeral, Peso: 1, Temas: []string{"a1", "a2"}},
			{Codigo: "D02", Nome: "B", Bloco: concurso.BlocoGeral, Peso: 1, Temas: []string{"b1", "b2"}},
			{Codigo: "D03", Nome: "C", Bloco: concurso.BlocoGeral, Peso: 1, Temas: []string{"c1", "c2"}},
		},
	}

	for _, n := range []int{1, 2, 3, 5} {
		t.Run("blocos por dia = "+string(rune('0'+n)), func(t *testing.T) {
			t.Parallel()

			perfil := plano.PerfilPadrao()
			perfil.BlocosPorDia = n

			res := plano.Gerar(plano.Config{
				Inicio:        dia(2026, 3, 2),
				Prova:         c.ProvaPadrao,
				HorasDia:      3,
				DiasEstudo:    []int{1, 2, 3, 4, 5},
				DiaRevisao:    5,
				RetaFinalDias: 28,
				Questoes:      map[string]int{"D01": 10, "D02": 10, "D03": 10},
				Perfil:        perfil,
			}, &c)

			for _, d := range res.Dias {
				if d.Tipo != plano.TipoEstudo {
					continue
				}

				if len(d.Itens) != n {
					t.Fatalf("dia %d tem %d itens, queria %d", d.N, len(d.Itens), n)
				}

				vistos := map[string]bool{}
				for _, it := range d.Itens {
					vistos[it.Disciplina] = true
				}

				// Com mais blocos que disciplinas a repetição é inevitável.
				if n <= len(c.Disciplinas) && len(vistos) != n {
					t.Errorf("dia %d repete disciplina: %v", d.N, d.Itens)
				}
			}
		})
	}
}

// Uma disciplina que precisa de mais de um bloco por dia — porque o peso dela
// passa de 1/n do total — repete dentro do dia, e não há como não repetir.
func TestGerar_disciplinaPesadaRepeteNoDia(t *testing.T) {
	t.Parallel()

	c := concurso.Concurso{
		Nome:        "X",
		ProvaPadrao: dia(2026, 8, 14),
		Disciplinas: []concurso.Disciplina{
			{Codigo: "D01", Nome: "A", Bloco: concurso.BlocoGeral, Peso: 1, Temas: []string{"a1"}},
			{Codigo: "D02", Nome: "B", Bloco: concurso.BlocoEspecifico, Peso: 2, Temas: []string{"b1"}},
		},
	}

	perfil := plano.PerfilPadrao()
	perfil.BlocosPorDia = 3

	res := plano.Gerar(plano.Config{
		Inicio:        dia(2026, 3, 2),
		Prova:         c.ProvaPadrao,
		HorasDia:      3,
		DiasEstudo:    []int{1, 2, 3, 4, 5},
		DiaRevisao:    5,
		RetaFinalDias: 28,
		Questoes:      map[string]int{"D01": 5, "D02": 40}, // D02 leva 16/17 dos pontos
		Perfil:        perfil,
	}, &c)

	dias := 0
	for _, d := range res.Dias {
		if d.Tipo == plano.TipoEstudo {
			dias++
		}
	}

	if res.Slots["D02"] <= dias {
		t.Fatalf("montagem do teste errada: D02 tem %d slots para %d dias — deveria passar",
			res.Slots["D02"], dias)
	}

	for _, d := range res.Dias {
		if d.Tipo != plano.TipoEstudo {
			continue
		}

		if len(d.Itens) != 3 {
			t.Fatalf("dia %d tem %d itens, queria 3", d.N, len(d.Itens))
		}
	}
}

func TestGerar_reforcoDaMaisBlocos(t *testing.T) {
	t.Parallel()

	c := concurso.Concurso{
		Nome:        "X",
		ProvaPadrao: dia(2026, 8, 14),
		Disciplinas: []concurso.Disciplina{
			{Codigo: "D01", Nome: "A", Bloco: concurso.BlocoGeral, Peso: 1, Temas: []string{"a1"}},
			{Codigo: "D02", Nome: "B", Bloco: concurso.BlocoGeral, Peso: 1, Temas: []string{"b1"}},
		},
	}

	cfg := plano.Config{
		Inicio:        dia(2026, 3, 2),
		Prova:         c.ProvaPadrao,
		HorasDia:      2,
		DiasEstudo:    []int{1, 2, 3, 4, 5},
		DiaRevisao:    5,
		RetaFinalDias: 28,
		Questoes:      map[string]int{"D01": 10, "D02": 10},
		Perfil:        plano.PerfilPadrao(),
	}

	iguais := plano.Gerar(cfg, &c)
	if iguais.Slots["D01"] != iguais.Slots["D02"] {
		t.Fatalf("sem reforço os slots deviam empatar: %v", iguais.Slots)
	}

	perfil := plano.PerfilPadrao()
	perfil.Reforcos = map[string]float64{"D02": 2}
	cfg.Perfil = perfil

	comReforco := plano.Gerar(cfg, &c)

	if comReforco.Slots["D02"] <= comReforco.Slots["D01"] {
		t.Errorf("slots = %v, queria D02 com mais blocos que D01", comReforco.Slots)
	}

	// O peso da prova mostrado no balanceamento não muda com o reforço.
	if comReforco.Pontos["D02"] != iguais.Pontos["D02"] {
		t.Errorf("pontos mudaram com o reforço: %d vs %d",
			comReforco.Pontos["D02"], iguais.Pontos["D02"])
	}
}

func TestGerar_simuladoNaoMexeNoCicloDeRevisao(t *testing.T) {
	t.Parallel()

	c := concurso.Concurso{
		Nome:        "X",
		ProvaPadrao: dia(2026, 8, 14),
		Disciplinas: []concurso.Disciplina{
			{Codigo: "D01", Nome: "A", Bloco: concurso.BlocoGeral, Peso: 1, Temas: []string{"a1"}},
		},
	}

	cfg := plano.Config{
		Inicio:        dia(2026, 3, 2),
		Prova:         c.ProvaPadrao,
		HorasDia:      2,
		DiasEstudo:    []int{1, 2, 3, 4, 5},
		DiaRevisao:    5,
		RetaFinalDias: 28,
		Questoes:      map[string]int{"D01": 10},
		Perfil:        plano.PerfilPadrao(),
	}

	temas := func(res plano.Resultado) map[string]bool {
		out := map[string]bool{}
		for _, d := range res.Dias {
			if d.Tipo == plano.TipoRevisaoSemanal {
				out[d.Tema] = true
			}
		}

		return out
	}

	comSim := temas(plano.Gerar(cfg, &c))

	perfil := plano.PerfilPadrao()
	perfil.Simulados = plano.SimuladoNunca
	cfg.Perfil = perfil

	semSim := plano.Gerar(cfg, &c)

	if len(temas(semSim)) != len(comSim) {
		t.Errorf("desligar simulados mudou o ciclo de revisão: %v", temas(semSim))
	}

	for tema := range comSim {
		if !temas(semSim)[tema] {
			t.Errorf("o tema %q sumiu do ciclo ao desligar simulados", tema)
		}
	}

	for _, d := range semSim.Dias {
		if d.Tipo == plano.TipoSimulado {
			t.Errorf("dia %d ainda é simulado", d.N)
		}
	}
}

func TestGerar_cicloDeRevisaoCustomizado(t *testing.T) {
	t.Parallel()

	c := concurso.Concurso{
		Nome:        "X",
		ProvaPadrao: dia(2026, 8, 14),
		Disciplinas: []concurso.Disciplina{
			{Codigo: "D01", Nome: "A", Bloco: concurso.BlocoGeral, Peso: 1, Temas: []string{"a1"}},
		},
	}

	perfil := plano.PerfilPadrao()
	perfil.CicloRevisao = []concurso.RevItem{
		{Ordem: 0, Titulo: "Minha revisão semanal", Questoes: 42},
		{Ordem: 1, Titulo: "", Questoes: 10}, // sem título: descartado
	}

	res := plano.Gerar(plano.Config{
		Inicio:        dia(2026, 3, 2),
		Prova:         c.ProvaPadrao,
		HorasDia:      2,
		DiasEstudo:    []int{1, 2, 3, 4, 5},
		DiaRevisao:    5,
		RetaFinalDias: 28,
		Questoes:      map[string]int{"D01": 10},
		Perfil:        perfil,
	}, &c)

	achou := false

	for _, d := range res.Dias {
		if d.Tipo != plano.TipoRevisaoSemanal {
			continue
		}

		achou = true

		if d.Tema != "Minha revisão semanal" || d.Meta != 42 {
			t.Fatalf("revisão semanal = %q/%d, queria o ciclo customizado", d.Tema, d.Meta)
		}
	}

	if !achou {
		t.Fatal("nenhum dia de revisão semanal no plano")
	}
}
