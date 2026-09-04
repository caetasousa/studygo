//go:build integration

package service_test

import (
	"context"
	"os"
	"testing"
	"time"

	"studygo/internal/adapter/postgres"
	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/domain/usuario"
	"studygo/internal/platform/pgtest"
	"studygo/internal/port"
	"studygo/internal/service"

	"github.com/google/uuid"
)

// Fluxos verticais: services REAIS ligados aos repositories REAIS.
//
// Esta suíte é pequena de propósito. Ela não repete o que os testes unitários de
// service já cobrem (decisões, ordem de chamadas, tratamento de erro) nem o que
// os testes de repository já cobrem (constraints, joins, transações). Ela existe
// para os poucos fluxos em que o erro só aparece quando as duas metades se
// encontram — e todos eles já quebraram em produção alguma vez.
//
// Relógio continua sendo dublê: um fluxo que depende de "hoje" precisa de uma
// data fixa para ser determinístico.

func TestMain(m *testing.M) {
	codigo := m.Run()
	pgtest.Encerrar()
	os.Exit(codigo)
}

// relogioFixo prende o "agora" para que a numeração dos dias não dependa de
// quando a suíte roda.
type relogioFixo struct{ t time.Time }

func (r relogioFixo) Now() time.Time { return r.t }

const hojeDoTeste = "2026-09-01"

type ambiente struct {
	deps    service.Dependencias
	usuario uuid.UUID
	slug    string
}

func novoAmbiente(t *testing.T) *ambiente {
	t.Helper()

	pool := pgtest.Novo(t)
	ctx := t.Context()

	usuarios := postgres.NewUsuarioRepo(pool)
	concursos := postgres.NewConcursoRepo(pool)

	u, err := usuarios.Criar(ctx, usuario.Usuario{
		Email: "a@b.c", Nome: "A", SenhaHash: "x", TemaUI: usuario.TemaPadrao,
	})
	if err != nil {
		t.Fatalf("criando usuário: %v", err)
	}

	c := concurso.Concurso{
		DonoID: u.ID, Slug: "tce-go", Nome: "TCE-GO",
		ProvaPadrao:    time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC),
		RetaPadraoDias: 30,
		Disciplinas: []concurso.Disciplina{
			{
				Codigo: "LINPO", Nome: "Língua Portuguesa",
				Bloco: concurso.BlocoGeral, Peso: 1, QuestoesPadrao: 15, Ordem: 0,
				Temas: []string{"Crase", "Concordância", "Regência", "Pontuação"},
			},
			{
				Codigo: "BANDA", Nome: "Banco de Dados",
				Bloco: concurso.BlocoEspecifico, Peso: 2, QuestoesPadrao: 20, Ordem: 1,
				Temas: []string{"Modelagem", "SQL", "Índices", "Transações"},
			},
		},
	}

	if _, err := concursos.Criar(ctx, c); err != nil {
		t.Fatalf("criando concurso: %v", err)
	}

	hoje, err := time.Parse("2006-01-02", hojeDoTeste)
	if err != nil {
		t.Fatalf("data do teste: %v", err)
	}

	return &ambiente{
		deps: service.Dependencias{
			Planos:     postgres.NewPlanoRepo(pool),
			Cronograma: postgres.NewCronogramaRepo(pool),
			Concursos:  concursos,
			Caderno:    postgres.NewCadernoRepo(pool),
			Usuarios:   usuarios,
			Relogio:    relogioFixo{t: hoje},
		},
		usuario: u.ID,
		slug:    "tce-go",
	}
}

func (a *ambiente) obter(t *testing.T) service.PlanoMontado {
	t.Helper()

	p, err := service.NewPlanoService(a.deps).Obter(context.Background(), a.usuario, a.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	return p
}

func diasDeEstudo(p service.PlanoMontado) []service.DiaDoPlano {
	out := []service.DiaDoPlano{}

	for _, d := range p.Dias {
		if d.Tipo == string(plano.TipoEstudo) {
			out = append(out, d)
		}
	}

	return out
}

// O primeiro acesso cria o plano e GRAVA o cronograma inteiro. É o momento em
// que o motor puro encontra o banco, e onde um erro de mapeamento entre código
// de disciplina e id apareceria.
func TestFluxo_PrimeiroAcessoMaterializaOCronograma(t *testing.T) {
	t.Parallel()

	a := novoAmbiente(t)
	p := a.obter(t)

	estudo := diasDeEstudo(p)
	if len(estudo) == 0 {
		t.Fatal("o plano nasceu sem dias de estudo")
	}

	for _, d := range estudo {
		for _, it := range d.Itens {
			if it.ID == uuid.Nil {
				t.Fatalf("bloco sem id em %s — o cronograma não foi gravado", d.Data)
			}

			if it.Disciplina == "" {
				t.Fatalf("bloco sem disciplina em %s", d.Data)
			}
		}
	}

	// Ler de novo devolve o MESMO cronograma: os ids são estáveis porque estão
	// no banco, não sintetizados a cada requisição.
	depois := a.obter(t)

	if depois.Dias[0].Itens[0].ID != p.Dias[0].Itens[0].ID {
		t.Error("os ids mudaram entre duas leituras")
	}
}

// A invariante central do produto, do service ao banco: o dia só conclui quando
// TODAS as suas matérias concluem, e a conclusão é DERIVADA — nunca uma coluna.
func TestFluxo_ConclusaoDoDiaEDerivada(t *testing.T) {
	t.Parallel()

	a := novoAmbiente(t)
	p := a.obter(t)

	primeiro := diasDeEstudo(p)[0]
	if len(primeiro.Itens) < 2 {
		t.Fatalf("o cenário precisa de um dia com duas matérias, veio %d", len(primeiro.Itens))
	}

	registros := service.NewRegistroService(a.deps)
	ctx := context.Background()

	p, err := registros.Registrar(ctx, a.usuario, a.slug, service.RegistroCommand{
		AtividadeID: primeiro.Itens[0].ID, Concluido: true,
	})
	if err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	if diaPorData(p, primeiro.Data).Concluido {
		t.Error("o dia concluiu com só uma das duas matérias")
	}

	p, err = registros.Registrar(ctx, a.usuario, a.slug, service.RegistroCommand{
		AtividadeID: primeiro.Itens[1].ID, Concluido: true,
	})
	if err != nil {
		t.Fatalf("Registrar a segunda: %v", err)
	}

	if !diaPorData(p, primeiro.Data).Concluido {
		t.Error("com as duas concluídas o dia devia concluir")
	}
}

// Editar o concurso não pode desligar o cronograma nem o histórico. Este é o
// fluxo que só quebra com banco de verdade: o bug era o repository recriar as
// disciplinas com ids novos, e nenhum fake reproduzia isso.
func TestFluxo_RenomearDisciplinaPreservaHistorico(t *testing.T) {
	t.Parallel()

	a := novoAmbiente(t)
	p := a.obter(t)

	alvo := diasDeEstudo(p)[0].Itens[0]
	ctx := context.Background()

	if _, err := service.NewRegistroService(a.deps).Registrar(
		ctx, a.usuario, a.slug,
		service.RegistroCommand{AtividadeID: alvo.ID, Concluido: true},
	); err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	concursos := service.NewConcursoService(a.deps.Concursos, nil)

	detalhe, err := concursos.Detalhe(ctx, a.usuario, a.slug)
	if err != nil {
		t.Fatalf("Detalhe: %v", err)
	}

	cmd := detalhe.Dados
	cmd.Disciplinas[0].Nome = "Português e Redação"

	if _, _, err := concursos.Atualizar(ctx, a.usuario, a.slug, cmd); err != nil {
		t.Fatalf("Atualizar: %v", err)
	}

	depois := a.obter(t)
	diaDepois := diaPorData(depois, diasDeEstudo(p)[0].Data)

	if !contemAtividade(diaDepois, alvo.ID) {
		t.Fatal("renomear a disciplina desligou a atividade do cronograma")
	}

	// O registro segue a ATIVIDADE, não o dia: só uma das duas matérias foi
	// concluída, então o dia continua aberto — o que precisa sobreviver é a
	// conclusão daquela atividade.
	if !atividadeConcluida(diaDepois, alvo.ID) {
		t.Error("renomear a disciplina apagou o registro de estudo da matéria")
	}

	// E o nome novo chegou à tela.
	if depois.Concurso.Disciplinas[0].Nome != "Português e Redação" {
		t.Errorf("nome = %q, quer o novo", depois.Concurso.Disciplinas[0].Nome)
	}
}

func atividadeConcluida(d service.DiaDoPlano, id uuid.UUID) bool {
	for _, it := range d.Itens {
		if it.ID == id {
			return it.Concluido
		}
	}

	return false
}

// Concluir uma matéria agendada para a frente a traz para hoje e fecha o buraco,
// sem duplicá-la. O remanejamento reescreve o cronograma inteiro numa transação,
// então é aqui que a FK RESTRICT e a unique diferível são exercitadas de fato.
func TestFluxo_AntecipacaoNaoDuplicaNemPerdeRegistro(t *testing.T) {
	t.Parallel()

	a := novoAmbiente(t)
	p := a.obter(t)

	estudo := diasDeEstudo(p)
	futura := estudo[4].Itens[0]

	p, err := service.NewRegistroService(a.deps).Registrar(
		context.Background(), a.usuario, a.slug,
		service.RegistroCommand{AtividadeID: futura.ID, Concluido: true},
	)
	if err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	vezes := 0

	for _, d := range p.Dias {
		for _, it := range d.Itens {
			if it.ID == futura.ID {
				vezes++

				if d.Data != hojeDoTeste {
					t.Errorf("a atividade ficou em %s, quer %s", d.Data, hojeDoTeste)
				}

				if !it.Concluido {
					t.Error("a atividade antecipada perdeu o registro")
				}
			}
		}
	}

	if vezes != 1 {
		t.Errorf("a atividade aparece %d vezes no cronograma, quer 1", vezes)
	}
}

// Mudar o ritmo alcança os dias à frente sem tocar no que já foi estudado.
func TestFluxo_MudarRitmoPreservaODiaEstudado(t *testing.T) {
	t.Parallel()

	a := novoAmbiente(t)
	p := a.obter(t)

	primeiro := diasDeEstudo(p)[0]
	ctx := context.Background()

	for _, it := range primeiro.Itens {
		if _, err := service.NewRegistroService(a.deps).Registrar(
			ctx, a.usuario, a.slug,
			service.RegistroCommand{AtividadeID: it.ID, Concluido: true},
		); err != nil {
			t.Fatalf("Registrar: %v", err)
		}
	}

	antes := len(primeiro.Itens)
	tres := 3

	p, err := service.NewPlanoService(a.deps).Salvar(
		ctx, a.usuario, a.slug, service.ConfigCommand{BlocosPorDia: &tres},
	)
	if err != nil {
		t.Fatalf("Salvar: %v", err)
	}

	estudado := diaPorData(p, primeiro.Data)

	if len(estudado.Itens) != antes {
		t.Errorf("o dia estudado ficou com %d matérias, quer %d — o replanejamento "+
			"não pode acrescentar trabalho a um dia concluído",
			len(estudado.Itens), antes)
	}

	if !estudado.Concluido {
		t.Error("o dia estudado deixou de estar concluído")
	}

	if futuro := diasDeEstudo(p)[10]; len(futuro.Itens) != 3 {
		t.Errorf("dia futuro = %d matérias, quer 3 (o ritmo novo)", len(futuro.Itens))
	}
}

// O worker carrega os planos com o contato do dono e monta os lembretes. É o
// único caminho que junta a consulta em lote ao motor.
func TestFluxo_LembretesDoDia(t *testing.T) {
	t.Parallel()

	a := novoAmbiente(t)
	a.obter(t) // materializa o cronograma

	entregues := &notifierEspiao{}

	svc := service.NewNotificacaoService(
		a.deps.Planos, a.deps.Cronograma, a.deps.Concursos,
		entregues, a.deps.Relogio,
	)

	if _, err := svc.EnviarLembretesDoDia(context.Background()); err != nil {
		t.Fatalf("EnviarLembretesDoDia: %v", err)
	}

	// Sem nada estudado ainda, o caderno está vazio e ninguém é notificado —
	// o lembrete persegue o que deu errado, não o calendário.
	if len(entregues.lembretes) != 0 {
		t.Errorf("mandou %d lembretes sem nada registrado", len(entregues.lembretes))
	}
}

// notifierEspiao registra o que seria entregue. Serviço externo continua sendo
// dublê: o teste é sobre o que a aplicação decide enviar.
type notifierEspiao struct {
	lembretes []port.Lembrete
}

func (n *notifierEspiao) EnviarLembrete(_ context.Context, l port.Lembrete) error {
	n.lembretes = append(n.lembretes, l)

	return nil
}

func diaPorData(p service.PlanoMontado, data string) service.DiaDoPlano {
	for _, d := range p.Dias {
		if d.Data == data {
			return d
		}
	}

	return service.DiaDoPlano{}
}

func contemAtividade(d service.DiaDoPlano, id uuid.UUID) bool {
	for _, it := range d.Itens {
		if it.ID == id {
			return true
		}
	}

	return false
}
