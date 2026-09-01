package service

import (
	"context"
	"testing"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// End-to-end tests of PlanoService against in-memory repositories.
//
// These cover the orchestration — materialisation, reconciliation, what is
// persisted — which is where every reported regression in the schedule has
// actually lived. A pure-domain test cannot see any of it: the domain functions
// were correct while the schedule on screen was wrong.

func diaT(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

// cenario is a service wired to fresh fakes, plus the plan it starts from.
type cenario struct {
	svc     *PlanoService
	planos  *fakePlanos
	usuario uuid.UUID
	slug    string
	hoje    time.Time
}

// novoCenario builds a two-discipline concurso with a materialised plan —
// the state a user is in after any move or record.
func novoCenario(t *testing.T) *cenario {
	t.Helper()

	hoje := diaT(2026, 9, 1)
	userID := uuid.New()
	concursoID := uuid.New()

	c := concurso.Concurso{
		ID: concursoID, OwnerID: userID, Slug: "x", Nome: "X",
		ProvaPadrao: diaT(2026, 12, 15), RetaPadraoDias: 30,
		Disciplinas: []concurso.Disciplina{
			{
				ID: uuid.New(), Codigo: "LP", Nome: "Português",
				Bloco: concurso.BlocoGeral, Peso: 1, QuestoesPadrao: 15, Ordem: 0,
				Temas: []string{"Crase", "Concordância", "Regência", "Pontuação"},
			},
			{
				ID: uuid.New(), Codigo: "BD", Nome: "Banco de Dados",
				Bloco: concurso.BlocoEspecifico, Peso: 2, QuestoesPadrao: 20, Ordem: 1,
				Temas: []string{"Modelagem", "SQL", "Índices", "Transações"},
			},
		},
	}

	cfg := plano.ConfigPadrao()
	cfg.Inicio = hoje
	cfg.Prova = diaT(2026, 12, 15)
	cfg.HorasDia = 2
	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.DiaRevisao = 5
	cfg.RetaFinalDias = 30
	cfg.Questoes = map[string]int{"LP": 15, "BD": 20}
	cfg.BlocosPorDia = 2
	cfg.MinutosBloco = 30

	planos := &fakePlanos{
		salvo: plano.Salvo{
			ID: uuid.New(), UserID: userID, ConcursoID: concursoID,
			TemaUI: "dark", Config: cfg.Normalizar(),
			Registros:    map[time.Time]plano.Registro{},
			Reordenacoes: map[time.Time]plano.Reordenacao{},
			Marcos:       map[uuid.UUID]bool{},
			Revisoes:     map[time.Time]plano.RegistroRevisao{},
		},
	}

	svc := NewPlanoService(planos, &fakeConcursos{c: c}, relogioFixo{t: hoje})

	return &cenario{svc: svc, planos: planos, usuario: userID, slug: "x", hoje: hoje}
}

// diasDeEstudo returns the response's study days, in order.
func diasDeEstudo(r PlanoResposta) []DiaResposta {
	out := []DiaResposta{}

	for _, d := range r.Dias {
		if d.Tipo == string(plano.TipoEstudo) {
			out = append(out, d)
		}
	}

	return out
}

func ocorrencias(d DiaResposta, disciplina, tema string) int {
	n := 0

	for _, it := range d.Itens {
		if it.Disciplina == disciplina && it.Tema == tema {
			n++
		}
	}

	return n
}

func rotulos(d DiaResposta) []string {
	out := make([]string, 0, len(d.Itens))
	for _, it := range d.Itens {
		out = append(out, it.Disciplina+"/"+it.Tema)
	}

	return out
}

// Antecipar uma matéria de amanhã: ela sobe para hoje e NÃO fica embaixo.
// Este é o relato do usuário — "subiu para o dia de hoje mas continuou no de
// baixo" — e a razão de existir todo este arquivo.
func TestFluxo_AnteciparNaoDeixaCopiaNoDiaDeOrigem(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	est := diasDeEstudo(inicial)
	if len(est) < 2 {
		t.Fatalf("plano com %d dias de estudo", len(est))
	}

	hoje, amanha := est[0], est[1]
	if len(amanha.Itens) == 0 {
		t.Fatal("amanhã sem itens")
	}

	alvo := amanha.Itens[0]
	t.Logf("hoje=%s %v", hoje.Data, rotulos(hoje))
	t.Logf("amanhã=%s %v", amanha.Data, rotulos(amanha))
	t.Logf("antecipando %s/%s (id=%q)", alvo.Disciplina, alvo.Tema, alvo.ID)

	depois, err := ce.svc.AntecipouAtividade(ctx, ce.usuario, ce.slug, AnteciparInput{
		ID:   alvo.ID,
		Data: hoje.Data,
	})
	if err != nil {
		t.Fatalf("AntecipouAtividade: %v", err)
	}

	est2 := diasDeEstudo(depois)
	novoHoje, novoAmanha := est2[0], est2[1]

	t.Logf("depois hoje=%s %v", novoHoje.Data, rotulos(novoHoje))
	t.Logf("depois amanhã=%s %v", novoAmanha.Data, rotulos(novoAmanha))

	if n := ocorrencias(novoHoje, alvo.Disciplina, alvo.Tema); n != 1 {
		t.Errorf("em hoje = %d ocorrências, quer 1", n)
	}

	if n := ocorrencias(novoAmanha, alvo.Disciplina, alvo.Tema); n != 0 {
		t.Errorf("em amanhã = %d ocorrências, quer 0 — subiu mas continuou embaixo", n)
	}
}

// A mesma coisa pelo caminho que o usuário realmente usa ao marcar "Concluí
// esta matéria" num bloco agendado para amanhã: RegistrarDia move a atividade
// para o dia em que foi de fato estudada.
func TestFluxo_ConcluirMateriaDeAmanhaTrazParaHojeSemDuplicar(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	est := diasDeEstudo(inicial)
	hoje, amanha := est[0], est[1]
	alvo := amanha.Itens[0]

	t.Logf("concluindo %s/%s de %s no dia %s", alvo.Disciplina, alvo.Tema, amanha.Data, hoje.Data)

	horas := 1.0
	depois, err := ce.svc.RegistrarDia(ctx, ce.usuario, ce.slug, hoje.Data, RegistroInput{
		Blocos: []RegistroBlocoInput{{
			AtividadeID: alvo.ID,
			Disciplina:  alvo.Disciplina,
			Horas:       &horas,
			Concluido:   true,
		}},
	})
	if err != nil {
		t.Fatalf("RegistrarDia: %v", err)
	}

	est2 := diasDeEstudo(depois)
	novoHoje, novoAmanha := est2[0], est2[1]

	t.Logf("depois hoje=%s %v", novoHoje.Data, rotulos(novoHoje))
	t.Logf("depois amanhã=%s %v", novoAmanha.Data, rotulos(novoAmanha))

	if n := ocorrencias(novoHoje, alvo.Disciplina, alvo.Tema); n != 1 {
		t.Errorf("em hoje = %d ocorrências, quer 1", n)
	}

	if n := ocorrencias(novoAmanha, alvo.Disciplina, alvo.Tema); n != 0 {
		t.Errorf("em amanhã = %d ocorrências, quer 0 — subiu mas continuou embaixo", n)
	}
}

// Nenhum dia pode mostrar a mesma disciplina+tema duas vezes, em nenhum ponto
// do plano — a "pilha" que o usuário viu espalhada por todo o cronograma.
func TestFluxo_NenhumDiaRepeteOMesmoTema(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()

	resp, err := ce.svc.Obter(context.Background(), ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	_ = ctx

	for _, d := range resp.Dias {
		visto := map[string]bool{}

		for _, it := range d.Itens {
			k := it.Disciplina + "\x00" + it.Tema
			if visto[k] {
				t.Errorf("dia %s repete %s/%s: %v", d.Data, it.Disciplina, it.Tema, rotulos(d))
			}

			visto[k] = true
		}
	}
}

// Obter é um GET: não pode gravar nada.
func TestFluxo_ObterNaoGrava(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)

	if _, err := ce.svc.Obter(context.Background(), ce.usuario, ce.slug); err != nil {
		t.Fatalf("Obter: %v", err)
	}

	if ce.planos.gravacoes != 0 {
		t.Errorf("Obter gravou atividades %d vez(es); um GET não pode escrever", ce.planos.gravacoes)
	}
}

// Todo bloco de estudo dura exatamente os minutos configurados.
func TestFluxo_BlocoUsaOsMinutosConfigurados(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)

	resp, err := ce.svc.Obter(context.Background(), ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	for _, d := range diasDeEstudo(resp) {
		if len(d.Itens) != resp.Config.BlocosPorDia {
			continue // dia fora do padrão; coberto por outro teste
		}

		for i, b := range d.Blocos {
			if i >= len(d.Itens) {
				break // a cauda de revisão tem duração própria
			}

			if b.Minutos != resp.Config.MinutosBloco {
				t.Errorf("dia %s bloco %d = %d min, quer %d",
					d.Data, i, b.Minutos, resp.Config.MinutosBloco)
			}
		}
	}
}

// materializar força o plano ao estado real do usuário: atividades gravadas com
// uuid, como ficam depois do primeiro movimento ou registro. Os testes acima
// partiam de um plano nunca materializado (ids sintéticos "gen:"), que é um
// caminho DIFERENTE dentro de AplicarAtividades — e o bug relatado vive no
// caminho materializado.
func (ce *cenario) materializar(t *testing.T) {
	t.Helper()

	res := plano.Gerar(ce.planos.salvo.Config, concursoDoCenario(ce))
	plano.AplicarAtividades(res.Dias, nil)

	if err := ce.planos.ReplaceAtividades(
		context.Background(), ce.planos.salvo.ID, plano.AtividadesFaltantes(res.Dias, nil),
	); err != nil {
		t.Fatalf("materializar: %v", err)
	}

	ce.planos.gravacoes = 0
}

func TestFluxo_AnteciparComPlanoMaterializado(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	est := diasDeEstudo(inicial)
	hoje, amanha := est[0], est[1]
	alvo := amanha.Itens[0]

	t.Logf("hoje=%s %v", hoje.Data, rotulos(hoje))
	t.Logf("amanhã=%s %v", amanha.Data, rotulos(amanha))
	t.Logf("antecipando %s/%s (id=%q)", alvo.Disciplina, alvo.Tema, alvo.ID)

	depois, err := ce.svc.AntecipouAtividade(ctx, ce.usuario, ce.slug, AnteciparInput{
		ID:   alvo.ID,
		Data: hoje.Data,
	})
	if err != nil {
		t.Fatalf("AntecipouAtividade: %v", err)
	}

	est2 := diasDeEstudo(depois)
	novoHoje, novoAmanha := est2[0], est2[1]

	t.Logf("depois hoje=%s %v", novoHoje.Data, rotulos(novoHoje))
	t.Logf("depois amanhã=%s %v", novoAmanha.Data, rotulos(novoAmanha))

	if n := ocorrencias(novoHoje, alvo.Disciplina, alvo.Tema); n != 1 {
		t.Errorf("em hoje = %d ocorrências, quer 1", n)
	}

	if n := ocorrencias(novoAmanha, alvo.Disciplina, alvo.Tema); n != 0 {
		t.Errorf("em amanhã = %d ocorrências, quer 0 — subiu mas continuou embaixo", n)
	}
}

func TestFluxo_ConcluirDeAmanhaComPlanoMaterializado(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	est := diasDeEstudo(inicial)
	hoje, amanha := est[0], est[1]
	alvo := amanha.Itens[0]

	t.Logf("concluindo %s/%s (de %s) no dia %s", alvo.Disciplina, alvo.Tema, amanha.Data, hoje.Data)

	horas := 1.0
	depois, err := ce.svc.RegistrarDia(ctx, ce.usuario, ce.slug, hoje.Data, RegistroInput{
		Blocos: []RegistroBlocoInput{{
			AtividadeID: alvo.ID,
			Disciplina:  alvo.Disciplina,
			Horas:       &horas,
			Concluido:   true,
		}},
	})
	if err != nil {
		t.Fatalf("RegistrarDia: %v", err)
	}

	est2 := diasDeEstudo(depois)
	novoHoje, novoAmanha := est2[0], est2[1]

	t.Logf("depois hoje=%s %v", novoHoje.Data, rotulos(novoHoje))
	t.Logf("depois amanhã=%s %v", novoAmanha.Data, rotulos(novoAmanha))

	if n := ocorrencias(novoHoje, alvo.Disciplina, alvo.Tema); n != 1 {
		t.Errorf("em hoje = %d ocorrências, quer 1", n)
	}

	if n := ocorrencias(novoAmanha, alvo.Disciplina, alvo.Tema); n != 0 {
		t.Errorf("em amanhã = %d ocorrências, quer 0 — subiu mas continuou embaixo", n)
	}
}

// O plano do usuário NÃO é saudável: uma disciplina de 1 tópico recebeu
// dezenas de vagas e virou uma pilha de linhas idênticas. Este cenário
// reproduz exatamente isso — é onde o "subiu mas continuou embaixo" vive.
func novoCenarioUmTopico(t *testing.T) *cenario {
	t.Helper()

	hoje := diaT(2026, 9, 1)
	userID := uuid.New()
	concursoID := uuid.New()

	c := concurso.Concurso{
		ID: concursoID, OwnerID: userID, Slug: "x", Nome: "X",
		ProvaPadrao: diaT(2026, 12, 15), RetaPadraoDias: 30,
		Disciplinas: []concurso.Disciplina{
			{
				ID: uuid.New(), Codigo: "INTAR", Nome: "Inteligência Artificial",
				Bloco: concurso.BlocoEspecifico, Peso: 2, QuestoesPadrao: 20, Ordem: 0,
				Temas: []string{"Fundamentos da inteligência artificial"},
			},
			{
				ID: uuid.New(), Codigo: "LP", Nome: "Português",
				Bloco: concurso.BlocoGeral, Peso: 1, QuestoesPadrao: 15, Ordem: 1,
				Temas: []string{"Crase", "Concordância"},
			},
		},
	}

	cfg := plano.ConfigPadrao()
	cfg.Inicio = hoje
	cfg.Prova = diaT(2026, 12, 15)
	cfg.HorasDia = 2
	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.DiaRevisao = 5
	cfg.RetaFinalDias = 30
	cfg.Questoes = map[string]int{"INTAR": 20, "LP": 15}
	cfg.BlocosPorDia = 2
	cfg.MinutosBloco = 30

	planos := &fakePlanos{
		salvo: plano.Salvo{
			ID: uuid.New(), UserID: userID, ConcursoID: concursoID,
			TemaUI: "dark", Config: cfg.Normalizar(),
			Registros:    map[time.Time]plano.Registro{},
			Reordenacoes: map[time.Time]plano.Reordenacao{},
			Marcos:       map[uuid.UUID]bool{},
			Revisoes:     map[time.Time]plano.RegistroRevisao{},
		},
	}

	svc := NewPlanoService(planos, &fakeConcursos{c: c}, relogioFixo{t: hoje})

	return &cenario{svc: svc, planos: planos, usuario: userID, slug: "x", hoje: hoje}
}

func TestFluxo_AnteciparComDisciplinaDeUmTopico(t *testing.T) {
	t.Parallel()

	ce := novoCenarioUmTopico(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	est := diasDeEstudo(inicial)
	for i := 0; i < 4 && i < len(est); i++ {
		t.Logf("inicial dia %s: %v", est[i].Data, rotulos(est[i]))
	}

	hoje, amanha := est[0], est[1]
	alvo := amanha.Itens[0]
	t.Logf("antecipando %s/%s (id=%q) de %s para %s",
		alvo.Disciplina, alvo.Tema, alvo.ID, amanha.Data, hoje.Data)

	depois, err := ce.svc.AntecipouAtividade(ctx, ce.usuario, ce.slug, AnteciparInput{
		ID:   alvo.ID,
		Data: hoje.Data,
	})
	if err != nil {
		t.Fatalf("AntecipouAtividade: %v", err)
	}

	est2 := diasDeEstudo(depois)
	for i := 0; i < 4 && i < len(est2); i++ {
		t.Logf("depois dia %s: %v", est2[i].Data, rotulos(est2[i]))
	}

	// Uma disciplina de um tema só é AGENDADA nesse mesmo tema em vários dias —
	// são visitas distintas, não cópias. Antecipar uma delas não pode apagar as
	// outras nem esvaziar o dia de origem. O que NÃO pode acontecer é o dia de
	// destino passar a mostrar o tema duas vezes.
	if n := ocorrencias(est2[0], alvo.Disciplina, alvo.Tema); n != 1 {
		t.Errorf("em hoje = %d ocorrências, quer exatamente 1 (nunca duplicado)", n)
	}

	for _, d := range est2 {
		if len(d.Itens) == 0 {
			t.Errorf("dia %s ficou vazio", d.Data)
		}

		if n := ocorrencias(d, alvo.Disciplina, alvo.Tema); n > 1 {
			t.Errorf("dia %s mostra %s/%s %d vezes: %v", d.Data, alvo.Disciplina, alvo.Tema, n, rotulos(d))
		}
	}
}

// Antecipar não pode deixar o dia de origem em branco: a matéria subiu, o dia
// que a perdeu tem de receber o conteúdo seguinte, não virar um buraco.
func TestFluxo_AnteciparNaoDeixaDiaVazio(t *testing.T) {
	t.Parallel()

	for _, caso := range []struct {
		nome string
		novo func(*testing.T) *cenario
	}{
		{"disciplinas com vários temas", novoCenario},
		{"disciplina de um tema só", novoCenarioUmTopico},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			t.Parallel()

			ce := caso.novo(t)
			ce.materializar(t)
			ctx := context.Background()

			inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
			if err != nil {
				t.Fatalf("Obter: %v", err)
			}

			est := diasDeEstudo(inicial)
			hoje, amanha := est[0], est[1]
			alvo := amanha.Itens[0]

			depois, err := ce.svc.AntecipouAtividade(ctx, ce.usuario, ce.slug, AnteciparInput{
				ID:   alvo.ID,
				Data: hoje.Data,
			})
			if err != nil {
				t.Fatalf("AntecipouAtividade: %v", err)
			}

			for _, d := range diasDeEstudo(depois) {
				if len(d.Itens) == 0 {
					t.Errorf("dia de estudo %s ficou sem nenhuma matéria", d.Data)
				}
			}
		})
	}
}

// A deduplicação em prepararAtividades cura um plano cuja tabela de atividades
// foi semeada de uma pilha do reparte — e PERSISTE a limpeza, senão ela volta
// no próximo carregamento.
func TestFluxo_DeduplicacaoCuraEPersiste(t *testing.T) {
	t.Parallel()

	ce := novoCenarioUmTopico(t)
	ctx := context.Background()

	// Semeia a pilha: 8 linhas idênticas, todas vindas do MESMO slot do motor,
	// espalhadas por dias diferentes como uma compactação as deixaria.
	origem := diaT(2026, 9, 2)
	pos := 0
	pilha := []plano.Atividade{}

	for i := range 8 {
		od, op := origem, pos
		pilha = append(pilha, plano.Atividade{
			ID:         uuid.NewString(),
			Data:       diaT(2026, 9, 2+i),
			Posicao:    0,
			Disciplina: "INTAR",
			Tema:       "Fundamentos da inteligência artificial",
			Tipo:       plano.AtividadeConteudo,
			OrigemDia:  &od,
			OrigemPos:  &op,
		})
	}

	if err := ce.planos.ReplaceAtividades(ctx, ce.planos.salvo.ID, pilha); err != nil {
		t.Fatalf("semear: %v", err)
	}

	antes, _ := ce.planos.ListAtividades(ctx, ce.planos.salvo.ID)
	t.Logf("pilha semeada: %d linhas", len(antes))

	// Compactar passa por prepararAtividades, que é onde a cura roda.
	if _, err := ce.svc.CompactarPlano(ctx, ce.usuario, ce.slug); err != nil {
		t.Fatalf("CompactarPlano: %v", err)
	}

	depois, _ := ce.planos.ListAtividades(ctx, ce.planos.salvo.ID)

	n := 0
	for _, a := range depois {
		if a.OrigemDia != nil && a.OrigemDia.Equal(origem) &&
			a.Disciplina == "INTAR" && a.Tema == "Fundamentos da inteligência artificial" {
			n++
		}
	}

	t.Logf("linhas da pilha depois: %d", n)

	if n > 1 {
		t.Errorf("a pilha continua com %d linhas gravadas, quer no máximo 1", n)
	}
}

// AplicarAtividades tem de fundir sozinha: é ela que alimenta
// AtividadesFaltantes, e uma pilha não fundida ali vira linhas gravadas.
func TestFluxo_AplicarAtividadesNaoMaterializaPilha(t *testing.T) {
	t.Parallel()

	ce := novoCenarioUmTopico(t)
	ctx := context.Background()

	if _, err := ce.svc.CompactarPlano(ctx, ce.usuario, ce.slug); err != nil {
		t.Fatalf("CompactarPlano: %v", err)
	}

	as, _ := ce.planos.ListAtividades(ctx, ce.planos.salvo.ID)

	porDia := map[string]map[string]int{}
	for _, a := range as {
		k := a.Data.Format("2006-01-02")
		if porDia[k] == nil {
			porDia[k] = map[string]int{}
		}

		porDia[k][a.Disciplina+"/"+a.Tema]++
	}

	for d, temas := range porDia {
		for tema, n := range temas {
			if n > 1 {
				t.Errorf("dia %s gravou %s %d vezes", d, tema, n)
			}
		}
	}
}

// O "concluído" do dia é DERIVADO: só é verdade quando TODAS as atividades
// agendadas para ele estão concluídas. Registrar uma matéria de um dia que tem
// duas não pode fechar o dia inteiro.
func TestFluxo_DiaSoFechaQuandoTodasAsMateriasFecham(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	hoje := diasDeEstudo(inicial)[0]
	if len(hoje.Itens) < 2 {
		t.Fatalf("cenário precisa de um dia com 2 matérias, tem %d", len(hoje.Itens))
	}

	horas := 1.0

	// Primeira das duas.
	parcial, err := ce.svc.RegistrarDia(ctx, ce.usuario, ce.slug, hoje.Data, RegistroInput{
		Blocos: []RegistroBlocoInput{{
			AtividadeID: hoje.Itens[0].ID,
			Disciplina:  hoje.Itens[0].Disciplina,
			Horas:       &horas,
			Concluido:   true,
		}},
	})
	if err != nil {
		t.Fatalf("RegistrarDia: %v", err)
	}

	d := diasDeEstudo(parcial)[0]
	if d.Registro != nil && d.Registro.Concluido {
		t.Error("o dia fechou com só uma das duas matérias concluídas")
	}

	// Agora as duas.
	completo, err := ce.svc.RegistrarDia(ctx, ce.usuario, ce.slug, hoje.Data, RegistroInput{
		Blocos: []RegistroBlocoInput{
			{
				AtividadeID: hoje.Itens[0].ID,
				Disciplina:  hoje.Itens[0].Disciplina,
				Horas:       &horas,
				Concluido:   true,
			},
			{
				AtividadeID: hoje.Itens[1].ID,
				Disciplina:  hoje.Itens[1].Disciplina,
				Horas:       &horas,
				Concluido:   true,
			},
		},
	})
	if err != nil {
		t.Fatalf("RegistrarDia: %v", err)
	}

	d2 := diasDeEstudo(completo)[0]
	if d2.Registro == nil || !d2.Registro.Concluido {
		t.Error("o dia não fechou mesmo com as duas matérias concluídas")
	}
}

// A revisão do dia só pode puxar matéria que o aluno JÁ ESTUDOU. Hoje a fila é
// montada do que estava AGENDADO, então ela oferece assunto nunca estudado.
func TestFluxo_RevisaoSoOfereceOQueFoiEstudado(t *testing.T) {
	ce := novoCenario(t)
	ce.materializar(t)
	ctx := context.Background()

	resp, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	// Nada foi registrado: nenhum dia tem estudo lançado.
	est := diasDeEstudo(resp)

	for i, d := range est {
		if i > 6 {
			break
		}

		if d.Revisao == nil {
			t.Logf("dia %s: sem revisão nomeada", d.Data)
			continue
		}

		t.Logf("dia %s: REV oferece %q (registro do dia: %v)",
			d.Data, d.Revisao.Disciplina, d.Registro != nil)

		// Nada foi estudado ainda, então NENHUM dia deveria ter revisão nomeada.
		if d.Revisao.Disciplina != "" {
			t.Errorf("dia %s revisa %q sem nada ter sido estudado",
				d.Data, d.Revisao.Disciplina)
		}
	}
}

// E o outro lado: depois de estudar, a revisão TEM de aparecer — e só com o
// que foi estudado.
func TestFluxo_RevisaoApareceDepoisDeEstudar(t *testing.T) {
	ce := novoCenario(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	est := diasDeEstudo(inicial)
	hoje := est[0]
	horas := 1.0

	blocos := make([]RegistroBlocoInput, 0, len(hoje.Itens))
	estudadas := map[string]bool{}
	for _, it := range hoje.Itens {
		blocos = append(blocos, RegistroBlocoInput{
			AtividadeID: it.ID, Disciplina: it.Disciplina,
			Horas: &horas, Concluido: true,
		})
		estudadas[it.Disciplina] = true
	}
	t.Logf("estudando hoje: %v", rotulos(hoje))

	depois, err := ce.svc.RegistrarDia(ctx, ce.usuario, ce.slug, hoje.Data, RegistroInput{Blocos: blocos})
	if err != nil {
		t.Fatal(err)
	}

	est2 := diasDeEstudo(depois)
	amanha := est2[1]

	if amanha.Revisao == nil || amanha.Revisao.Disciplina == "" {
		t.Fatalf("dia %s não tem revisão, mas hoje foi estudado", amanha.Data)
	}

	t.Logf("dia %s revisa %q", amanha.Data, amanha.Revisao.Disciplina)

	if !estudadas[amanha.Revisao.Disciplina] {
		t.Errorf("revisa %q, que NÃO foi estudada (estudadas: %v)",
			amanha.Revisao.Disciplina, estudadas)
	}
}

// Abrir o registro do dia sem concluir nada (0 horas, sem check) NÃO conta como
// estudado: a revisão não pode puxar dali.
func TestFluxo_RegistroVazioNaoEntraNaRevisao(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	est := diasDeEstudo(inicial)
	hoje := est[0]

	// Registra o dia com uma anotação, sem horas e sem concluir.
	blocos := make([]RegistroBlocoInput, 0, len(hoje.Itens))
	for _, it := range hoje.Itens {
		blocos = append(blocos, RegistroBlocoInput{
			AtividadeID: it.ID,
			Disciplina:  it.Disciplina,
			Nota:        "comecei e parei",
			Concluido:   false,
		})
	}

	depois, err := ce.svc.RegistrarDia(ctx, ce.usuario, ce.slug, hoje.Data, RegistroInput{Blocos: blocos})
	if err != nil {
		t.Fatal(err)
	}

	for i, d := range diasDeEstudo(depois) {
		if i > 3 {
			break
		}

		if d.Revisao != nil && d.Revisao.Disciplina != "" {
			t.Errorf("dia %s revisa %q, mas nada foi de fato estudado",
				d.Data, d.Revisao.Disciplina)
		}
	}
}

// Registros antigos não têm atividade_id: guardam só "disciplina X neste dia".
// A revisão precisa reconhecê-los, senão quem já usava o app perde a revisão
// de tudo que estudou antes das atividades passarem a ser endereçáveis.
func TestFluxo_RegistroLegadoContaParaRevisao(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	est := diasDeEstudo(inicial)
	hoje := est[0]
	horas := 1.0

	// Sem AtividadeID — exatamente a forma dos registros legados.
	blocos := make([]RegistroBlocoInput, 0, len(hoje.Itens))
	for _, it := range hoje.Itens {
		blocos = append(blocos, RegistroBlocoInput{
			Disciplina: it.Disciplina,
			Horas:      &horas,
			Concluido:  true,
		})
	}

	depois, err := ce.svc.RegistrarDia(ctx, ce.usuario, ce.slug, hoje.Data, RegistroInput{Blocos: blocos})
	if err != nil {
		t.Fatal(err)
	}

	amanha := diasDeEstudo(depois)[1]
	if amanha.Revisao == nil || amanha.Revisao.Disciplina == "" {
		t.Errorf("dia %s sem revisão: um registro legado concluído tem de contar", amanha.Data)
	} else {
		t.Logf("dia %s revisa %q", amanha.Data, amanha.Revisao.Disciplina)
	}
}

// Adiantar para um dia que JÁ tem a mesma matéria não pode gravar uma segunda
// linha da mesma disciplina+tema nesse dia. A tela funde e esconde, mas o banco
// acumula — foi assim que a pilha reapareceu.
func TestFluxo_AnteciparNaoGravaDuplicataNoBanco(t *testing.T) {
	ce := novoCenarioUmTopico(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	est := diasDeEstudo(inicial)
	hoje := est[0]

	// Procura, num dia à frente, a MESMA disciplina que já está em hoje.
	disc := hoje.Itens[0].Disciplina
	var alvoID, alvoDia string
	for _, d := range est[1:] {
		for _, it := range d.Itens {
			if it.Disciplina == disc {
				alvoID, alvoDia = it.ID, d.Data
				break
			}
		}
		if alvoID != "" {
			break
		}
	}

	if alvoID == "" {
		t.Skip("cenário sem a mesma disciplina em outro dia")
	}

	t.Logf("hoje já tem %s; adiantando outra %s de %s", disc, disc, alvoDia)

	if _, err := ce.svc.AntecipouAtividade(ctx, ce.usuario, ce.slug, AnteciparInput{
		ID: alvoID, Data: hoje.Data,
	}); err != nil {
		t.Fatal(err)
	}

	as, _ := ce.planos.ListAtividades(ctx, ce.planos.salvo.ID)

	contagem := map[string]int{}
	for _, a := range as {
		if a.Data.Format("2006-01-02") == hoje.Data {
			contagem[a.Disciplina+"/"+a.Tema]++
		}
	}

	for k, n := range contagem {
		t.Logf("gravado em hoje: %s x%d", k, n)
		if n > 1 {
			t.Errorf("o banco tem %d linhas de %s no mesmo dia", n, k)
		}
	}
}

// A revisão tem de olhar o bloco DA ATIVIDADE, não só a disciplina: num dia com
// duas matérias, concluir uma não pode fazer a outra entrar na fila.
func TestFluxo_RevisaoDistingueAtividadeDentroDoDia(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	est := diasDeEstudo(inicial)
	hoje := est[0]

	if len(hoje.Itens) < 2 {
		t.Skip("precisa de um dia com 2 matérias")
	}

	feita := hoje.Itens[0]
	naoFeita := hoje.Itens[1]
	horas := 1.0

	// Só a primeira é concluída; a segunda é enviada em branco.
	if _, err := ce.svc.RegistrarDia(ctx, ce.usuario, ce.slug, hoje.Data, RegistroInput{
		Blocos: []RegistroBlocoInput{
			{AtividadeID: feita.ID, Disciplina: feita.Disciplina, Horas: &horas, Concluido: true},
			{AtividadeID: naoFeita.ID, Disciplina: naoFeita.Disciplina, Nota: "não deu tempo"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	depois, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("concluída: %s | pendente: %s", feita.Disciplina, naoFeita.Disciplina)

	for i, d := range diasDeEstudo(depois) {
		if i == 0 || i > 3 {
			continue
		}

		if d.Revisao == nil || d.Revisao.Disciplina == "" {
			continue
		}

		t.Logf("dia %s revisa %q", d.Data, d.Revisao.Disciplina)

		if d.Revisao.Disciplina == naoFeita.Disciplina {
			t.Errorf("dia %s revisa %q, que ficou pendente hoje", d.Data, naoFeita.Disciplina)
		}
	}
}

// Adiantar tira a matéria da PRÓXIMA passagem: ela sobe para hoje e o dia de
// onde saiu deixa de oferecê-la. Foi o relato "a riscada de hoje aparece de
// novo amanhã".
func TestFluxo_AdiantadaSaiDaProximaPassagem(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	est := diasDeEstudo(inicial)
	hoje, amanha := est[0], est[1]
	alvo := amanha.Itens[0]

	t.Logf("hoje:   %v", rotulos(hoje))
	t.Logf("amanhã: %v", rotulos(amanha))
	t.Logf("adiantando %s/%s", alvo.Disciplina, alvo.Tema)

	depois, err := ce.svc.AntecipouAtividade(ctx, ce.usuario, ce.slug, AnteciparInput{
		ID: alvo.ID, Data: hoje.Data,
	})
	if err != nil {
		t.Fatal(err)
	}

	est2 := diasDeEstudo(depois)
	t.Logf("DEPOIS hoje:   %v", rotulos(est2[0]))
	t.Logf("DEPOIS amanhã: %v", rotulos(est2[1]))

	if n := ocorrencias(est2[0], alvo.Disciplina, alvo.Tema); n != 1 {
		t.Errorf("hoje mostra %d vezes, quer exatamente 1", n)
	}

	if n := ocorrencias(est2[1], alvo.Disciplina, alvo.Tema); n != 0 {
		t.Errorf("amanhã ainda mostra %s/%s — não saiu da próxima passagem",
			alvo.Disciplina, alvo.Tema)
	}

	for _, d := range est2 {
		if len(d.Itens) == 0 {
			t.Errorf("dia %s ficou vazio", d.Data)
		}
	}
}

// O mesmo, num concurso do tamanho real do usuário: várias disciplinas de 6
// temas, 2 blocos por dia.
func novoCenarioRealista(t *testing.T) *cenario {
	t.Helper()

	hoje := diaT(2026, 9, 1)
	userID, concursoID := uuid.New(), uuid.New()

	temas := func(p string) []string {
		out := make([]string, 6)
		for i := range out {
			out[i] = p + " tema " + string(rune('A'+i))
		}

		return out
	}

	codigos := []string{"LINPO", "BANDA", "INTAR", "DESSI", "DEVPL", "ENGSO", "MATRA", "SEGIN"}
	discs := make([]concurso.Disciplina, 0, len(codigos))
	questoes := map[string]int{}

	for i, c := range codigos {
		bloco, peso := concurso.BlocoEspecifico, 2
		if i == 0 || i == 6 {
			bloco, peso = concurso.BlocoGeral, 1
		}

		discs = append(discs, concurso.Disciplina{
			ID: uuid.New(), Codigo: c, Nome: c, Bloco: bloco,
			Peso: peso, QuestoesPadrao: 10, Ordem: i, Temas: temas(c),
		})
		questoes[c] = 10
	}

	cfg := plano.ConfigPadrao()
	cfg.Inicio = hoje
	cfg.Prova = diaT(2026, 12, 15)
	cfg.HorasDia = 1.67
	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.DiaRevisao = 5
	cfg.RetaFinalDias = 30
	cfg.Questoes = questoes
	cfg.BlocosPorDia = 2
	cfg.MinutosBloco = 30
	cfg.MinutosRevisao = 40

	planos := &fakePlanos{salvo: plano.Salvo{
		ID: uuid.New(), UserID: userID, ConcursoID: concursoID,
		TemaUI: "dark", Config: cfg.Normalizar(),
		Registros:    map[time.Time]plano.Registro{},
		Reordenacoes: map[time.Time]plano.Reordenacao{},
		Marcos:       map[uuid.UUID]bool{},
		Revisoes:     map[time.Time]plano.RegistroRevisao{},
	}}

	c := concurso.Concurso{
		ID: concursoID, OwnerID: userID, Slug: "x", Nome: "X",
		ProvaPadrao: diaT(2026, 12, 15), RetaPadraoDias: 30, Disciplinas: discs,
	}

	return &cenario{
		svc:     NewPlanoService(planos, &fakeConcursos{c: c}, relogioFixo{t: hoje}),
		planos:  planos,
		usuario: userID,
		slug:    "x",
		hoje:    hoje,
	}
}

func TestFluxo_Realista_AdiantadaSaiDaProximaPassagem(t *testing.T) {
	t.Parallel()

	ce := novoCenarioRealista(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	est := diasDeEstudo(inicial)
	hoje, amanha := est[0], est[1]
	alvo := amanha.Itens[0]

	t.Logf("hoje:   %v", rotulos(hoje))
	t.Logf("amanhã: %v", rotulos(amanha))

	depois, err := ce.svc.AntecipouAtividade(ctx, ce.usuario, ce.slug, AnteciparInput{
		ID: alvo.ID, Data: hoje.Data,
	})
	if err != nil {
		t.Fatal(err)
	}

	est2 := diasDeEstudo(depois)
	t.Logf("DEPOIS hoje:   %v", rotulos(est2[0]))
	t.Logf("DEPOIS amanhã: %v", rotulos(est2[1]))

	if n := ocorrencias(est2[0], alvo.Disciplina, alvo.Tema); n != 1 {
		t.Errorf("hoje mostra %d, quer 1", n)
	}

	if n := ocorrencias(est2[1], alvo.Disciplina, alvo.Tema); n != 0 {
		t.Errorf("amanhã ainda mostra %s/%s", alvo.Disciplina, alvo.Tema)
	}

	for _, d := range est2 {
		if len(d.Itens) == 0 {
			t.Errorf("dia %s ficou vazio", d.Data)
		}
	}
}

// Uma disciplina de UM tema só, sozinha no dia seguinte, é o único caso em que a
// regra não pode ser cumprida: tirar o tema deixaria o dia em branco. O que
// continua valendo é o dia não ficar vazio nem mostrar o tema duas vezes.
func TestFluxo_AdiantarComUmTopicoNaoEsvaziaODia(t *testing.T) {
	t.Parallel()

	ce := novoCenarioUmTopico(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	est := diasDeEstudo(inicial)
	hoje, amanha := est[0], est[1]
	alvo := amanha.Itens[0]

	depois, err := ce.svc.AntecipouAtividade(ctx, ce.usuario, ce.slug, AnteciparInput{
		ID: alvo.ID, Data: hoje.Data,
	})
	if err != nil {
		t.Fatal(err)
	}

	est2 := diasDeEstudo(depois)

	if len(est2[1].Itens) == 0 {
		t.Error("o dia de origem ficou vazio")
	}

	for _, d := range est2 {
		if n := ocorrencias(d, alvo.Disciplina, alvo.Tema); n > 1 {
			t.Errorf("dia %s mostra %s/%s %d vezes", d.Data, alvo.Disciplina, alvo.Tema, n)
		}
	}
}

// blocoRegistrado acha o bloco de uma atividade no registro de um dia.
func blocoRegistrado(d DiaResposta, atividadeID string) *RegistroBlocoResposta {
	if d.Registro == nil {
		return nil
	}

	for i := range d.Registro.Blocos {
		if d.Registro.Blocos[i].AtividadeID == atividadeID {
			return &d.Registro.Blocos[i]
		}
	}

	return nil
}

// Concluir uma matéria e DEPOIS mover/adiantar qualquer coisa não pode apagar o
// "concluído" já gravado. Era o efeito de ReplaceAtividades fazer DELETE FROM
// atividades: o FK ON DELETE SET NULL tirava o atividade_id de TODO registro do
// plano, e a matéria voltava a aparecer sem o risquinho.
func TestFluxo_ConclusaoSobreviveAMovimento(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	est := diasDeEstudo(inicial)
	hoje, amanha := est[0], est[1]
	feita := hoje.Itens[0]
	horas := 2.0

	// Conclui a primeira matéria de hoje.
	depois, err := ce.svc.RegistrarDia(ctx, ce.usuario, ce.slug, hoje.Data, RegistroInput{
		Blocos: []RegistroBlocoInput{{
			AtividadeID: feita.ID,
			Disciplina:  feita.Disciplina,
			Horas:       &horas,
			Concluido:   true,
		}},
	})
	if err != nil {
		t.Fatalf("RegistrarDia: %v", err)
	}

	if b := blocoRegistrado(diasDeEstudo(depois)[0], feita.ID); b == nil || !b.Concluido {
		t.Fatalf("conclusão não gravou")
	}

	// Adianta uma matéria de amanhã — o que dispara ReplaceAtividades.
	depois, err = ce.svc.AntecipouAtividade(ctx, ce.usuario, ce.slug, AnteciparInput{
		ID:   amanha.Itens[0].ID,
		Data: hoje.Data,
	})
	if err != nil {
		t.Fatalf("AntecipouAtividade: %v", err)
	}

	// O risquinho da matéria concluída tem de continuar lá.
	b := blocoRegistrado(diasDeEstudo(depois)[0], feita.ID)
	if b == nil {
		t.Fatalf("a matéria concluída perdeu o vínculo com o registro após o adiantamento")
	}

	if !b.Concluido {
		t.Errorf("a matéria voltou a aparecer sem o 'concluído' após o adiantamento")
	}
}

// O relato do usuário: concluir matérias de hoje, adiantar 3 assuntos, e a
// partir daí NENHUM registro salva mais (500 — o FK ON DELETE SET NULL colapsava
// dois registros da mesma disciplina no mesmo dia sobre a chave legada, travando
// todo ReplaceAtividades seguinte).
func TestFluxo_SalvarContinuaFuncionandoAposAdiantarVarias(t *testing.T) {
	t.Parallel()

	ce := novoCenarioRealista(t)
	ce.materializar(t)
	ctx := context.Background()

	horas := 2.0

	// Conclui tudo o que hoje agenda.
	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	hoje := diasDeEstudo(inicial)[0]
	blocos := make([]RegistroBlocoInput, 0, len(hoje.Itens))
	for _, it := range hoje.Itens {
		blocos = append(blocos, RegistroBlocoInput{
			AtividadeID: it.ID, Disciplina: it.Disciplina, Horas: &horas, Concluido: true,
		})
	}

	if _, err := ce.svc.RegistrarDia(ctx, ce.usuario, ce.slug, hoje.Data, RegistroInput{Blocos: blocos}); err != nil {
		t.Fatalf("RegistrarDia inicial: %v", err)
	}

	// Adianta 3 assuntos de dias à frente, um a um.
	for i := 0; i < 3; i++ {
		atual, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
		if err != nil {
			t.Fatalf("Obter rodada %d: %v", i, err)
		}

		est := diasDeEstudo(atual)
		diaHoje := est[0]

		var alvo *ItemResposta
		for _, d := range est[1:] {
			for j := range d.Itens {
				if blocoRegistrado(diaHoje, d.Itens[j].ID) == nil {
					alvo = &d.Itens[j]

					break
				}
			}

			if alvo != nil {
				break
			}
		}

		if alvo == nil {
			t.Fatalf("rodada %d: sem matéria futura para adiantar", i)
		}

		if _, err := ce.svc.AntecipouAtividade(ctx, ce.usuario, ce.slug, AnteciparInput{
			ID:   alvo.ID,
			Data: diaHoje.Data,
		}); err != nil {
			t.Fatalf("AntecipouAtividade rodada %d: %v", i, err)
		}
	}

	// A partir daqui: salvar QUALQUER matéria de hoje tem de continuar funcionando.
	atual, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter final: %v", err)
	}

	diaHoje := diasDeEstudo(atual)[0]

	for _, it := range diaHoje.Itens {
		reenvio := make([]RegistroBlocoInput, 0, len(diaHoje.Itens))
		for _, x := range diaHoje.Itens {
			b := blocoRegistrado(diaHoje, x.ID)
			entry := RegistroBlocoInput{AtividadeID: x.ID, Disciplina: x.Disciplina}
			if b != nil {
				entry.Horas = b.Horas
				entry.Concluido = b.Concluido
			}

			if x.ID == it.ID {
				entry.Horas = &horas
				entry.Concluido = true
			}

			reenvio = append(reenvio, entry)
		}

		resp, err := ce.svc.RegistrarDia(ctx, ce.usuario, ce.slug, diaHoje.Data, RegistroInput{Blocos: reenvio})
		if err != nil {
			t.Fatalf("RegistrarDia para %s/%s após adiantar 3: %v", it.Disciplina, it.Tema, err)
		}

		if b := blocoRegistrado(diasDeEstudo(resp)[0], it.ID); b == nil || !b.Concluido {
			t.Errorf("a conclusão de %s/%s não persistiu", it.Disciplina, it.Tema)
		}
	}
}

// A última matéria do dia, ao descer, atravessa a fronteira e vai para o TOPO
// do próximo dia útil. A primeira do dia, ao subir, vai para o FUNDO do dia
// útil anterior. É o mesmo MoverAtividade — só a data e a posição mudam.
func TestFluxo_MoverEntreDiasVizinhos(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.materializar(t)
	ctx := context.Background()

	inicial, err := ce.svc.Obter(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	est := diasDeEstudo(inicial)
	dia1, dia2 := est[0], est[1]
	if len(dia1.Itens) < 2 || len(dia2.Itens) < 2 {
		t.Fatalf("cenário precisa de 2 itens em cada dia inicial")
	}

	// Desce a última matéria do dia1 → topo do dia2.
	ultima := dia1.Itens[len(dia1.Itens)-1]
	depois, err := ce.svc.MoverAtividade(ctx, ce.usuario, ce.slug, MoverAtividadeInput{
		ID: ultima.ID, Data: dia2.Data, Posicao: 0,
	})
	if err != nil {
		t.Fatalf("descer para o próximo dia: %v", err)
	}

	est = diasDeEstudo(depois)
	if est[0].Itens[len(est[0].Itens)-1].ID == ultima.ID {
		t.Errorf("a matéria ficou no dia1: %v", rotulos(est[0]))
	}
	if est[1].Itens[0].ID != ultima.ID {
		t.Errorf("a matéria não chegou no topo do dia2: %v", rotulos(est[1]))
	}

	// Sobe a agora-primeira do dia2 (a mesma) → fundo do dia1. Ida e volta.
	primeira := est[1].Itens[0]
	dia1Data, dia2Data := est[0].Data, est[1].Data
	posFundo := len(est[0].Itens)

	depois, err = ce.svc.MoverAtividade(ctx, ce.usuario, ce.slug, MoverAtividadeInput{
		ID: primeira.ID, Data: dia1Data, Posicao: posFundo,
	})
	if err != nil {
		t.Fatalf("subir para o dia anterior: %v", err)
	}

	est = diasDeEstudo(depois)
	// achar o dia1 e o dia2 pelas datas (posição na lista pode mudar)
	var d1, d2 DiaResposta
	for _, d := range est {
		switch d.Data {
		case dia1Data:
			d1 = d
		case dia2Data:
			d2 = d
		}
	}

	if d1.Itens[len(d1.Itens)-1].ID != primeira.ID {
		t.Errorf("a matéria não voltou ao fundo do dia1: %v", rotulos(d1))
	}
	for _, it := range d2.Itens {
		if it.ID == primeira.ID {
			t.Errorf("a matéria ficou também no dia2: %v", rotulos(d2))
			break
		}
	}
}
