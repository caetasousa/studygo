package plano_test

import (
	"testing"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// cronogramaDeTeste monta um plano curto e o materializa, que é o estado em que
// todo plano nasce agora.
func cronogramaDeTeste(t *testing.T) ([]plano.Dia, []plano.Atividade, concurso.Concurso) {
	t.Helper()

	lp := uuid.New()
	bd := uuid.New()

	c := concurso.Concurso{
		Nome:        "X",
		ProvaPadrao: dia(2026, time.December, 15),
		Disciplinas: []concurso.Disciplina{
			{
				ID: lp, Codigo: "LP", Nome: "Português",
				Bloco: concurso.BlocoGeral, Peso: 1, QuestoesPadrao: 15,
				Temas: []string{"Crase", "Concordância", "Regência", "Pontuação"},
			},
			{
				ID: bd, Codigo: "BD", Nome: "Banco de Dados",
				Bloco: concurso.BlocoEspecifico, Peso: 2, QuestoesPadrao: 20,
				Temas: []string{"Modelagem", "SQL", "Índices", "Transações"},
			},
		},
	}

	cfg := plano.ConfigPadrao()
	cfg.Inicio = dia(2026, time.September, 1)
	cfg.Prova = dia(2026, time.December, 15)
	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.RetaFinalDias = 30
	cfg.Questoes = map[string]int{"LP": 15, "BD": 20}
	cfg.BlocosPorDia = 2
	cfg.MinutosBloco = 30

	res := plano.Gerar(cfg.Normalizar(), &c)

	atividades := plano.Materializar(res.Dias, map[string]uuid.UUID{"LP": lp, "BD": bd})

	return res.Dias, atividades, c
}

// Toda atividade nasce com id e com a disciplina ligada por id, que é o que
// permite o registro ter chave estrangeira de verdade.
func TestMaterializar_TodaAtividadeTemIdentidade(t *testing.T) {
	t.Parallel()

	_, atividades, _ := cronogramaDeTeste(t)

	if len(atividades) == 0 {
		t.Fatal("materializar não produziu atividade nenhuma")
	}

	vistos := map[uuid.UUID]bool{}

	for _, a := range atividades {
		if a.ID == uuid.Nil {
			t.Fatalf("atividade em %s sem id", a.Data.Format(time.DateOnly))
		}

		if vistos[a.ID] {
			t.Fatalf("id repetido: %s", a.ID)
		}

		vistos[a.ID] = true

		if a.Tipo.DeDiaInteiro() {
			if a.DisciplinaID != nil {
				t.Errorf("atividade de dia inteiro (%s) não devia apontar para disciplina", a.Tipo)
			}

			continue
		}

		if a.DisciplinaID == nil {
			t.Errorf("atividade de conteúdo em %s sem disciplina_id", a.Data.Format(time.DateOnly))
		}
	}
}

// Duas atividades do mesmo dia nunca compartilham posição — é a invariante que
// o índice UNIQUE (plano_id, data, posicao) cobra do banco.
func TestMaterializar_PosicoesNaoColidem(t *testing.T) {
	t.Parallel()

	_, atividades, _ := cronogramaDeTeste(t)

	ocupadas := map[string]bool{}

	for _, a := range atividades {
		chave := a.Data.Format(time.DateOnly) + ":" + string(rune('0'+a.Posicao))
		if ocupadas[chave] {
			t.Fatalf("duas atividades na mesma vaga: %s", chave)
		}

		ocupadas[chave] = true
	}
}

func TestMover_LevaParaODiaEPosicaoPedidos(t *testing.T) {
	t.Parallel()

	dias, atividades, _ := cronogramaDeTeste(t)

	origem := plano.AtividadesDoDia(atividades, dia(2026, time.September, 1))
	if len(origem) == 0 {
		t.Fatal("o primeiro dia não tem atividades")
	}

	alvo := origem[0]
	destino := dia(2026, time.September, 3)

	movidas, err := plano.Mover(
		atividades, dias, alvo.ID, destino, 0, semDiaConcluido,
	)
	if err != nil {
		t.Fatalf("Mover: %v", err)
	}

	noDestino := plano.AtividadesDoDia(movidas, destino)
	if len(noDestino) == 0 || noDestino[0].ID != alvo.ID {
		t.Fatalf("a atividade não chegou à posição 0 do destino")
	}

	if !noDestino[0].Movida {
		t.Error("uma atividade movida à mão deve ficar marcada como Movida")
	}

	// O dia de origem perdeu um item: as posições que sobraram têm que ser
	// densas de novo, ou a unique do banco recusa a gravação.
	for i, a := range plano.AtividadesDoDia(movidas, dia(2026, time.September, 1)) {
		if a.Posicao != i {
			t.Errorf("posição %d na origem, esperava %d", a.Posicao, i)
		}
	}
}

func TestMover_RecusaDiaQueNaoAceitaConteudo(t *testing.T) {
	t.Parallel()

	dias, atividades, _ := cronogramaDeTeste(t)

	alvo := plano.AtividadesDoDia(atividades, dia(2026, time.September, 1))[0]

	// Um domingo não é dia de estudo deste plano.
	_, err := plano.Mover(atividades, dias, alvo.ID, dia(2026, time.September, 6), 0, semDiaConcluido)
	if err == nil {
		t.Fatal("mover para um dia fora do plano devia falhar")
	}
}

func TestTrocar_AsDuasTrocamDeLugar(t *testing.T) {
	t.Parallel()

	dias, atividades, _ := cronogramaDeTeste(t)

	a := plano.AtividadesDoDia(atividades, dia(2026, time.September, 1))[0]
	b := plano.AtividadesDoDia(atividades, dia(2026, time.September, 2))[0]

	trocadas, err := plano.Trocar(atividades, dias, a.ID, dia(2026, time.September, 2), 0, semDiaConcluido)
	if err != nil {
		t.Fatalf("Trocar: %v", err)
	}

	if got := plano.AtividadesDoDia(trocadas, dia(2026, time.September, 2))[0].ID; got != a.ID {
		t.Errorf("no dia 2 ficou %s, esperava %s", got, a.ID)
	}

	if got := plano.AtividadesDoDia(trocadas, dia(2026, time.September, 1))[0].ID; got != b.ID {
		t.Errorf("no dia 1 ficou %s, esperava %s", got, b.ID)
	}
}

// Replanejar existe para que mudar a configuração alcance o cronograma já
// gravado sem atropelar o que o estudante fez.
func TestReplanejar_PreservaPassadoConcluidoEMovido(t *testing.T) {
	t.Parallel()

	_, atividades, c := cronogramaDeTeste(t)

	hoje := dia(2026, time.September, 10)

	// Uma atividade no passado, uma concluída no futuro e uma movida à mão.
	passada := plano.AtividadesDoDia(atividades, dia(2026, time.September, 1))[0]
	futuras := plano.AtividadesDoDia(atividades, dia(2026, time.September, 15))

	if len(futuras) < 2 {
		t.Fatal("cenário precisa de dois blocos no dia 15")
	}

	concluida := futuras[0]

	atividades[indiceDe(t, atividades, futuras[1].ID)].Movida = true
	movida := futuras[1]

	cfg := plano.ConfigPadrao()
	cfg.Inicio = dia(2026, time.September, 1)
	cfg.Prova = dia(2026, time.December, 15)
	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.RetaFinalDias = 30
	cfg.Questoes = map[string]int{"LP": 15, "BD": 20}
	cfg.BlocosPorDia = 3 // o ritmo mudou
	cfg.MinutosBloco = 30

	res := plano.Gerar(cfg.Normalizar(), &c)
	novas := plano.Materializar(res.Dias, map[string]uuid.UUID{})

	saida := plano.Replanejar(atividades, novas, hoje, func(id uuid.UUID) bool {
		return id == concluida.ID
	})

	for _, quer := range []uuid.UUID{passada.ID, concluida.ID, movida.ID} {
		if !contemID(saida, quer) {
			t.Errorf("replanejar descartou a atividade %s, que devia ser preservada", quer)
		}
	}

	// E as posições continuam sem colidir.
	ocupadas := map[string]bool{}

	for _, a := range saida {
		chave := a.Data.Format(time.DateOnly) + ":" + string(rune('0'+a.Posicao))
		if ocupadas[chave] {
			t.Fatalf("replanejar deixou duas atividades na vaga %s", chave)
		}

		ocupadas[chave] = true
	}
}

func semDiaConcluido(time.Time) bool { return false }

func indiceDe(t *testing.T, as []plano.Atividade, id uuid.UUID) int {
	t.Helper()

	for i := range as {
		if as[i].ID == id {
			return i
		}
	}

	t.Fatalf("atividade %s não encontrada", id)

	return -1
}

func contemID(as []plano.Atividade, id uuid.UUID) bool {
	for _, a := range as {
		if a.ID == id {
			return true
		}
	}

	return false
}

// Um dia que já tem trabalho preservado não recebe também a leva recém-gerada.
//
// Sem isto, mudar o ritmo duplicava a matéria num dia já estudado: metade das
// linhas concluída, metade não, a mesma matéria aparecendo duas vezes.
func TestReplanejar_NaoDuplicaDiaJaEstudado(t *testing.T) {
	t.Parallel()

	_, atividades, c := cronogramaDeTeste(t)

	hoje := dia(2026, time.September, 1)
	doDia := plano.AtividadesDoDia(atividades, hoje)

	if len(doDia) < 2 {
		t.Fatal("o cenário precisa de duas atividades no primeiro dia")
	}

	concluidas := map[uuid.UUID]bool{doDia[0].ID: true, doDia[1].ID: true}

	cfg := plano.ConfigPadrao()
	cfg.Inicio = hoje
	cfg.Prova = dia(2026, time.December, 15)
	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.RetaFinalDias = 30
	cfg.Questoes = map[string]int{"LP": 15, "BD": 20}
	cfg.BlocosPorDia = 3 // o ritmo mudou
	cfg.MinutosBloco = 30

	res := plano.Gerar(cfg.Normalizar(), &c)
	novas := plano.Materializar(res.Dias, map[string]uuid.UUID{})

	saida := plano.Replanejar(atividades, novas, hoje, func(id uuid.UUID) bool {
		return concluidas[id]
	})

	depois := plano.AtividadesDoDia(saida, hoje)

	if len(depois) != len(doDia) {
		t.Errorf(
			"o dia estudado ficou com %d atividades, quer %d — o replanejamento "+
				"não pode acrescentar trabalho a um dia já concluído",
			len(depois), len(doDia),
		)
	}

	for _, a := range depois {
		if !concluidas[a.ID] {
			t.Errorf("atividade %s apareceu num dia que devia ficar intocado", a.ID)
		}
	}
}
