package plano_test

import (
	"strings"
	"testing"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
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

// cfgBlocos is a normalized config with the given daily budget, for the Blocos
// tests — which only care about the study method, not the calendar.
func cfgBlocos(horas float64) plano.Config {
	c := plano.ConfigPadrao()
	c.HorasDia = horas

	return c
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
			nome:  "sem bloco de revisão, o dia é só conteúdo",
			itens: itens("D01", "D02"), horas: 2, temRevisao: false,
			querBlocos: 2, querTotal: 120,
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			d := plano.Dia{Tipo: plano.TipoEstudo, Meta: 20, Itens: c.itens}

			// The review block exists when it has a length, not when something is
			// queued: it is a block of the day like the others now.
			cfg := cfgBlocos(c.horas)
			if !c.temRevisao {
				cfg.MinutosRevisao = 0
			}

			got := plano.Blocos(d, plano.BlocoCtx{Cfg: cfg, Nomes: nomesTeste()})

			if len(got) != c.querBlocos {
				t.Fatalf("gerou %d blocos, queria %d", len(got), c.querBlocos)
			}

			if total := somaMinutos(got); total != c.querTotal {
				t.Errorf("total = %d min, queria %d (o dia inteiro)", total, c.querTotal)
			}
		})
	}
}

func TestMesclarItensIguais(t *testing.T) {
	t.Parallel()

	item := func(disc, tema string) plano.ItemDia {
		return plano.ItemDia{Disciplina: disc, Tema: tema}
	}

	casos := []struct {
		nome    string
		entrada []plano.ItemDia
		quer    []plano.ItemDia
	}{
		{
			nome: "run do mesmo tópico colapsa num só",
			entrada: []plano.ItemDia{
				item("IA", "Fundamentos"), item("IA", "Fundamentos"),
				item("IA", "Fundamentos"), item("DS", "Algoritmos"),
			},
			quer: []plano.ItemDia{item("IA", "Fundamentos"), item("DS", "Algoritmos")},
		},
		{
			nome:    "mesma disciplina, tópicos diferentes ficam",
			entrada: []plano.ItemDia{item("IA", "Fundamentos"), item("IA", "Redes")},
			quer:    []plano.ItemDia{item("IA", "Fundamentos"), item("IA", "Redes")},
		},
		{
			// Repetições NÃO adjacentes também colapsam: a reconciliação anexa as
			// atividades movidas depois das geradas, então trazer um tema para um
			// dia que já o ensina deixa outra matéria no meio do par. Comparar só
			// com o item anterior deixava exatamente esse caso passar.
			nome: "repetição não consecutiva também colapsa",
			entrada: []plano.ItemDia{
				item("IA", "F"), item("DS", "A"), item("IA", "F"),
			},
			quer: []plano.ItemDia{item("IA", "F"), item("DS", "A")},
		},
		{
			nome:    "lista de um item passa intacta",
			entrada: []plano.ItemDia{item("IA", "F")},
			quer:    []plano.ItemDia{item("IA", "F")},
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			got := plano.MesclarItensIguais(c.entrada)

			if len(got) != len(c.quer) {
				t.Fatalf("got %d itens, quer %d: %+v", len(got), len(c.quer), got)
			}

			for i := range got {
				if got[i].Disciplina != c.quer[i].Disciplina || got[i].Tema != c.quer[i].Tema {
					t.Errorf("item %d = %+v, quer %+v", i, got[i], c.quer[i])
				}
			}
		})
	}
}

// The first item wins the merge, so its stored id survives and the block stays
// movable.
func TestMesclarItensIguais_PreservaPrimeiroID(t *testing.T) {
	t.Parallel()

	got := plano.MesclarItensIguais([]plano.ItemDia{
		{Disciplina: "IA", Tema: "F", AtividadeID: "primeiro"},
		{Disciplina: "IA", Tema: "F", AtividadeID: "segundo"},
	})

	if len(got) != 1 || got[0].AtividadeID != "primeiro" {
		t.Fatalf("got %+v, quer um item com id 'primeiro'", got)
	}
}

// O reforço faz a matéria aparecer em MAIS dias (ver TestGerar_reforcoDaMaisBlocos),
// não estica um bloco. A tela de config chama minutosBloco de "tamanho de cada
// atividade", então dois blocos no mesmo dia têm a mesma duração, com ou sem
// reforço.
func TestBlocos_blocosDoDiaTemDuracaoIgual(t *testing.T) {
	t.Parallel()

	cfg := cfgBlocos(3)
	cfg.Reforcos = map[string]float64{"D02": 2}

	d := plano.Dia{
		Tipo:  plano.TipoEstudo,
		Meta:  30,
		Itens: itens("D01", "D02"),
	}

	got := plano.Blocos(d, plano.BlocoCtx{Cfg: cfg, Nomes: nomesTeste()})

	if len(got) != 3 {
		t.Fatalf("gerou %d blocos, queria 3", len(got))
	}

	if got[0].Minutos != got[1].Minutos {
		t.Errorf("blocos com %d e %d min — deviam ser iguais apesar do reforço",
			got[0].Minutos, got[1].Minutos)
	}

	if total := somaMinutos(got); total != 180 {
		t.Errorf("total = %d min, queria 180", total)
	}

	// Baterias iguais, já que os minutos são iguais.
	if !strings.Contains(got[0].Detalhe, "15 questões") || !strings.Contains(got[1].Detalhe, "15 questões") {
		t.Errorf("baterias = %q / %q, queria 15 questões em cada", got[0].Detalhe, got[1].Detalhe)
	}
}

func TestBlocos_modoPorDisciplina(t *testing.T) {
	t.Parallel()

	cfg := cfgBlocos(2)
	cfg.Modos = map[string]plano.Modo{"D01": plano.ModoQuestoes, "D02": plano.ModoTeoria}

	got := plano.Blocos(plano.Dia{
		Tipo:  plano.TipoEstudo,
		Meta:  20,
		Itens: itens("D01", "D02"),
	}, plano.BlocoCtx{Cfg: cfg, Nomes: nomesTeste()})

	if strings.Contains(got[0].Detalhe, "teoria com resumo") {
		t.Errorf("bloco só-questões = %q, não devia pedir teoria", got[0].Detalhe)
	}

	if !strings.Contains(got[1].Detalhe, "sem bateria de questões") {
		t.Errorf("bloco só-teoria = %q, não devia pedir questões", got[1].Detalhe)
	}
}

// TestBlocos_revisaoSemanalFocaQuestoes garante que o dia de revisão semanal
// abre explicitamente com resolução de questões, no volume do ciclo.
func TestBlocos_revisaoSemanalFocaQuestoes(t *testing.T) {
	t.Parallel()

	d := plano.Dia{
		Tipo: plano.TipoRevisaoSemanal,
		Tema: "Revisão ativa da semana",
		Meta: 60,
	}

	got := plano.Blocos(d, plano.BlocoCtx{Cfg: cfgBlocos(3), Nomes: nomesTeste()})

	if len(got) == 0 {
		t.Fatal("dia de revisão semanal sem blocos")
	}

	if !strings.Contains(strings.ToLower(got[0].Titulo), "resolução de questões") {
		t.Errorf("primeiro bloco = %q, queria resolução de questões em destaque", got[0].Titulo)
	}

	if !strings.Contains(got[0].Detalhe, "60 questões") {
		t.Errorf("detalhe = %q, queria as 60 questões do ciclo", got[0].Detalhe)
	}

	if got[0].Minutos < got[1].Minutos {
		t.Errorf("o bloco de questões (%d) devia ser o maior do dia", got[0].Minutos)
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

			cfg := plano.ConfigPadrao()
			cfg.Inicio = dia(2026, 3, 2)
			cfg.Prova = c.ProvaPadrao
			cfg.HorasDia = 3
			cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
			cfg.DiaRevisao = 5
			cfg.RetaFinalDias = 28
			cfg.Questoes = map[string]int{"D01": 10, "D02": 10, "D03": 10}
			cfg.BlocosPorDia = n

			res := plano.Gerar(cfg, &c)

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

	cfg := plano.ConfigPadrao()
	cfg.Inicio = dia(2026, 3, 2)
	cfg.Prova = c.ProvaPadrao
	cfg.HorasDia = 3
	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.DiaRevisao = 5
	cfg.RetaFinalDias = 28
	cfg.Questoes = map[string]int{"D01": 5, "D02": 40} // D02 leva 16/17 dos pontos
	cfg.BlocosPorDia = 3

	res := plano.Gerar(cfg, &c)

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

	cfg := plano.ConfigPadrao()
	cfg.Inicio = dia(2026, 3, 2)
	cfg.Prova = c.ProvaPadrao
	cfg.HorasDia = 2
	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.DiaRevisao = 5
	cfg.RetaFinalDias = 28
	cfg.Questoes = map[string]int{"D01": 10, "D02": 10}

	iguais := plano.Gerar(cfg, &c)
	if iguais.Slots["D01"] != iguais.Slots["D02"] {
		t.Fatalf("sem reforço os slots deviam empatar: %v", iguais.Slots)
	}

	cfg.Reforcos = map[string]float64{"D02": 2}

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

	cfg := plano.ConfigPadrao()
	cfg.Inicio = dia(2026, 3, 2)
	cfg.Prova = c.ProvaPadrao
	cfg.HorasDia = 2
	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.DiaRevisao = 5
	cfg.RetaFinalDias = 28
	cfg.Questoes = map[string]int{"D01": 10}

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

	cfg.Simulados = plano.SimuladoNunca

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

	cfg := plano.ConfigPadrao()
	cfg.Inicio = dia(2026, 3, 2)
	cfg.Prova = c.ProvaPadrao
	cfg.HorasDia = 2
	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.DiaRevisao = 5
	cfg.RetaFinalDias = 28
	cfg.Questoes = map[string]int{"D01": 10}
	// This is about the weekly-review DAY's content, which is opt-in now.
	cfg.RevisaoSemanal = true
	cfg.CicloRevisao = []concurso.RevItem{
		{Ordem: 0, Titulo: "Minha revisão semanal", Questoes: 42},
		{Ordem: 1, Titulo: "", Questoes: 10}, // sem título: descartado
	}

	res := plano.Gerar(cfg, &c)

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
