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
