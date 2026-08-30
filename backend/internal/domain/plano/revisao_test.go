package plano_test

import (
	"testing"
	"time"

	"annygo/internal/domain/concurso"
	"annygo/internal/domain/plano"
)

func dia(ano, mes, d int) time.Time {
	return time.Date(ano, time.Month(mes), d, 0, 0, 0, 0, time.UTC)
}

// cfgRevisao is a plan running Mon-Fri with the exam far enough away that
// scheduling never bumps into it, unless a test moves it. Study-method fields
// come from ConfigPadrao; a caller that needs a different cycle overrides them
// on the returned value.
func cfgRevisao() plano.Config {
	c := plano.ConfigPadrao()
	c.Inicio = dia(2026, 3, 2)
	c.Prova = dia(2026, 12, 13)
	c.HorasDia = 2
	c.DiasEstudo = []int{1, 2, 3, 4, 5}

	return c
}

func TestEnfileirar(t *testing.T) {
	t.Parallel()

	diaEstudo := plano.Dia{
		Data: dia(2026, 3, 5), // quinta
		Itens: []plano.ItemDia{
			{Disciplina: "D01", Tema: "Crase"},
			{Disciplina: "D02", Tema: "Controle externo"},
		},
	}

	casos := []struct {
		nome    string
		cfg     plano.Config
		d       plano.Dia
		quer    int
		venceEm time.Time
	}{
		{
			nome:    "um por tema, 24h depois",
			cfg:     cfgRevisao(),
			d:       diaEstudo,
			quer:    2,
			venceEm: dia(2026, 3, 6), // sexta
		},
		{
			nome: "pula o fim de semana",
			cfg:  cfgRevisao(),
			d: plano.Dia{
				Data:  dia(2026, 3, 6), // sexta
				Itens: []plano.ItemDia{{Disciplina: "D01", Tema: "Crase"}},
			},
			quer:    1,
			venceEm: dia(2026, 3, 9), // segunda
		},
		{
			nome: "dia sem temas não enfileira nada",
			cfg:  cfgRevisao(),
			d:    plano.Dia{Data: dia(2026, 3, 5), Tipo: plano.TipoSimulado},
			quer: 0,
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			got := plano.Enfileirar(c.cfg, c.d)

			if len(got) != c.quer {
				t.Fatalf("enfileirou %d revisões, queria %d", len(got), c.quer)
			}

			for _, r := range got {
				if r.Etapa != 0 {
					t.Errorf("etapa = %d, queria 0", r.Etapa)
				}

				if !r.VenceEm.Equal(c.venceEm) {
					t.Errorf("venceEm = %s, queria %s",
						r.VenceEm.Format(time.DateOnly), c.venceEm.Format(time.DateOnly))
				}
			}
		})
	}
}

func TestEnfileirar_naoPassaDaProva(t *testing.T) {
	t.Parallel()

	cfg := cfgRevisao()
	cfg.Prova = dia(2026, 3, 6)

	got := plano.Enfileirar(cfg, plano.Dia{
		Data:  dia(2026, 3, 5),
		Itens: []plano.ItemDia{{Disciplina: "D01", Tema: "Crase"}},
	})

	if len(got) != 0 {
		t.Fatalf("enfileirou %d revisões para depois da prova, queria 0", len(got))
	}
}

func TestRevisao_Resultado(t *testing.T) {
	t.Parallel()

	base := plano.Revisao{
		Disciplina: "D01",
		Tema:       "Crase",
		OrigemData: dia(2026, 3, 5),
		Etapa:      1,
		VenceEm:    dia(2026, 3, 13),
	}

	hoje := dia(2026, 3, 13) // sexta

	casos := []struct {
		nome      string
		etapa     int
		questoes  int
		acertos   int
		querEtapa int
		querVence time.Time
		querFica  bool
	}{
		{
			nome: "acima de 80% sobe de etapa", etapa: 1, questoes: 10, acertos: 9,
			querEtapa: 2, querVence: dia(2026, 4, 13), querFica: true, // +30d, segunda
		},
		{
			nome: "entre 60 e 79% repete a etapa", etapa: 1, questoes: 10, acertos: 7,
			querEtapa: 1, querVence: dia(2026, 3, 20), querFica: true, // +7d, sexta
		},
		{
			nome: "abaixo de 60% desce de etapa", etapa: 1, questoes: 10, acertos: 4,
			querEtapa: 0, querVence: dia(2026, 3, 16), querFica: true, // +1d, segunda
		},
		{
			nome: "na etapa 0 não desce mais", etapa: 0, questoes: 10, acertos: 0,
			querEtapa: 0, querVence: dia(2026, 3, 16), querFica: true,
		},
		{
			nome: "sair da última etapa consolida", etapa: 2, questoes: 10, acertos: 10,
			querFica: false,
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			r := base
			r.Etapa = c.etapa

			prox, fica := r.Resultado(cfgRevisao(), hoje, c.questoes, c.acertos)

			if fica != c.querFica {
				t.Fatalf("continua na fila = %v, queria %v", fica, c.querFica)
			}

			if !fica {
				return
			}

			if prox.Etapa != c.querEtapa {
				t.Errorf("etapa = %d, queria %d", prox.Etapa, c.querEtapa)
			}

			if !prox.VenceEm.Equal(c.querVence) {
				t.Errorf("venceEm = %s, queria %s",
					prox.VenceEm.Format(time.DateOnly), c.querVence.Format(time.DateOnly))
			}

			if prox.Tema != r.Tema || prox.Disciplina != r.Disciplina {
				t.Errorf("perdeu a identidade do tema: %+v", prox)
			}
		})
	}
}

func TestRevisao_Resultado_intervalosCustomizados(t *testing.T) {
	t.Parallel()

	cfg := cfgRevisao()
	cfg.Intervalos = []int{2, 10}

	r := plano.Revisao{Disciplina: "D01", Tema: "Crase", Etapa: 0}

	prox, fica := r.Resultado(cfg, dia(2026, 3, 4), 10, 10)
	if !fica {
		t.Fatal("deveria continuar na fila")
	}

	if quer := dia(2026, 3, 16); !prox.VenceEm.Equal(quer) {
		t.Errorf("venceEm = %s, queria %s (hoje + 10d, empurrado para segunda)",
			prox.VenceEm.Format(time.DateOnly), quer.Format(time.DateOnly))
	}

	if _, fica := prox.Resultado(cfg, dia(2026, 3, 16), 10, 10); fica {
		t.Error("deveria consolidar ao sair da última etapa")
	}
}

func TestAproveitamentoEFraca(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome      string
		questoes  int
		acertos   int
		querPct   int
		querFraca bool
	}{
		{"metade", 10, 5, 50, true},
		{"limite da faixa fraca", 10, 6, 60, false},
		{"tudo certo", 10, 10, 100, false},
		{"errou todas", 10, 0, 0, true},
		{"sem questões não é fraca", 0, 0, 0, false},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			if got := plano.Aproveitamento(c.questoes, c.acertos); got != c.querPct {
				t.Errorf("aproveitamento = %d, queria %d", got, c.querPct)
			}

			if got := plano.Fraca(c.questoes, c.acertos); got != c.querFraca {
				t.Errorf("fraca = %v, queria %v", got, c.querFraca)
			}
		})
	}
}

func TestVencidasAte(t *testing.T) {
	t.Parallel()

	feita := dia(2026, 3, 10)

	fila := []plano.Revisao{
		{Disciplina: "D02", Tema: "b", VenceEm: dia(2026, 3, 12)},
		{Disciplina: "D01", Tema: "a", VenceEm: dia(2026, 3, 9)},
		{Disciplina: "D03", Tema: "c", VenceEm: dia(2026, 3, 20)},
		{Disciplina: "D04", Tema: "d", VenceEm: dia(2026, 3, 9), FeitaEm: &feita},
	}

	got := plano.VencidasAte(fila, dia(2026, 3, 12))

	if len(got) != 2 {
		t.Fatalf("venceram %d, queria 2 (a atrasada e a do dia)", len(got))
	}

	if got[0].Tema != "a" {
		t.Errorf("primeira = %q, queria a atrasada %q", got[0].Tema, "a")
	}

	if got[1].Tema != "b" {
		t.Errorf("segunda = %q, queria %q", got[1].Tema, "b")
	}
}

func TestGerar_desligaSimuladoEDiscursiva(t *testing.T) {
	t.Parallel()

	c := concurso.Concurso{
		Nome:        "X",
		ProvaPadrao: dia(2026, 5, 15),
		Disciplinas: []concurso.Disciplina{
			{Codigo: "D01", Nome: "Português", Bloco: concurso.BlocoGeral, Peso: 1, Temas: []string{"Crase"}},
			{Codigo: "D02", Nome: "Direito", Bloco: concurso.BlocoEspecifico, Peso: 2, Temas: []string{"Atos"}},
		},
	}

	casos := []struct {
		nome      string
		simulados plano.Frequencia
		disc      bool
		querSim   int
		querDisc  int
	}{
		{"padrão mantém os dois", plano.SimuladoSemanal, true, 4, 4},
		{"sem simulados", plano.SimuladoNunca, true, 0, 4},
		{"sem discursiva", plano.SimuladoSemanal, false, 4, 0},
		{"sem nenhum dos dois", plano.SimuladoNunca, false, 0, 0},
		{"quinzenal corta pela metade", plano.SimuladoQuinzenal, true, 2, 4},
	}

	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			t.Parallel()

			cfg := plano.ConfigPadrao()
			cfg.Inicio = dia(2026, 3, 2)
			cfg.Prova = c.ProvaPadrao
			cfg.HorasDia = 2
			cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
			cfg.DiaRevisao = 5
			cfg.RetaFinalDias = 28
			cfg.Questoes = map[string]int{"D01": 10, "D02": 20}
			cfg.Simulados = tc.simulados
			cfg.Discursiva = tc.disc

			res := plano.Gerar(cfg, &c)

			sims, discs := 0, 0
			for _, d := range res.Dias {
				switch d.Tipo {
				case plano.TipoSimulado:
					sims++
				case plano.TipoDiscursiva:
					discs++
				}
			}

			if sims != tc.querSim {
				t.Errorf("simulados = %d, queria %d", sims, tc.querSim)
			}

			if discs != tc.querDisc {
				t.Errorf("discursivas = %d, queria %d", discs, tc.querDisc)
			}
		})
	}
}
