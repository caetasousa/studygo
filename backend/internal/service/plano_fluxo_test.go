package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// Testes de ponta a ponta dos casos de uso contra repositórios em memória.
//
// Eles cobrem a orquestração — o que é gravado, em que ordem, o que é relido —
// que é onde todas as regressões do cronograma de fato moraram. Um teste de
// domínio puro não enxerga nada disso: as funções do domínio estavam corretas
// enquanto o cronograma na tela estava errado.

func diaT(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// cenario é o conjunto de serviços ligado a fakes novos, mais o plano de que
// eles partem.
type cenario struct {
	deps       Dependencias
	planos     *fakePlanos
	cronograma *fakeCronograma
	caderno    *fakeCaderno
	concursos  *fakeConcursos
	usuario    uuid.UUID
	slug       string
	hoje       time.Time
}

func novoCenario(t *testing.T) *cenario {
	t.Helper()

	hoje := diaT(2026, time.September, 1)
	usuarioID := uuid.New()
	concursoID := uuid.New()

	c := concurso.Concurso{
		ID: concursoID, DonoID: usuarioID, Slug: "x", Nome: "X",
		ProvaPadrao: diaT(2026, time.December, 15), RetaPadraoDias: 30,
		Disciplinas: []concurso.Disciplina{
			{
				ID: uuid.New(), Codigo: "LINPO", Nome: "Língua Portuguesa",
				Bloco: concurso.BlocoGeral, Peso: 1, QuestoesPadrao: 15, Ordem: 0,
				Temas: []string{"Crase", "Concordância", "Regência", "Pontuação"},
			},
			{
				ID: uuid.New(), Codigo: "BANDA", Nome: "Banco de Dados",
				Bloco: concurso.BlocoEspecifico, Peso: 2, QuestoesPadrao: 20, Ordem: 1,
				Temas: []string{"Modelagem", "SQL", "Índices", "Transações"},
			},
		},
	}

	planos := &fakePlanos{}
	cronograma := novoCronograma()
	caderno := &fakeCaderno{}
	concursos := &fakeConcursos{c: c}

	return &cenario{
		deps: Dependencias{
			Planos:     planos,
			Cronograma: cronograma,
			Concursos:  concursos,
			Caderno:    caderno,
			Usuarios:   &fakeUsuarios{},
			Relogio:    relogioFixo{t: hoje},
		},
		planos:     planos,
		cronograma: cronograma,
		caderno:    caderno,
		concursos:  concursos,
		usuario:    usuarioID,
		slug:       "x",
		hoje:       hoje,
	}
}

func (ce *cenario) obter(t *testing.T) PlanoMontado {
	t.Helper()

	p, err := NewPlanoService(ce.deps).Obter(context.Background(), ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	return p
}

func diasDeEstudo(p PlanoMontado) []DiaDoPlano {
	out := []DiaDoPlano{}

	for _, d := range p.Dias {
		if d.Tipo == string(plano.TipoEstudo) {
			out = append(out, d)
		}
	}

	return out
}

// O plano nasce materializado: já na primeira leitura todo bloco tem id, e a
// tela não precisa inventar identificador nenhum para poder mover algo.
func TestObter_MaterializaOCronogramaNaPrimeiraLeitura(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	if len(estudo) == 0 {
		t.Fatal("o plano não tem dias de estudo")
	}

	for _, d := range estudo {
		for _, it := range d.Itens {
			if it.ID == uuid.Nil {
				t.Fatalf("bloco sem id em %s", d.Data)
			}
		}
	}

	if len(ce.cronograma.atividades) == 0 {
		t.Error("o cronograma devia ter sido gravado")
	}
}

// Depois de materializado, ler o plano é só leitura: um GET não escreve.
func TestObter_NaoEscreveDepoisDeMaterializado(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.obter(t)

	gravacoes := ce.cronograma.gravacoes

	ce.obter(t)
	ce.obter(t)

	if ce.cronograma.gravacoes != gravacoes {
		t.Errorf(
			"ler o plano gravou %d vez(es) a mais — GET não pode escrever",
			ce.cronograma.gravacoes-gravacoes,
		)
	}
}

// A invariante do produto, ponta a ponta: o dia só conclui quando TODAS as suas
// matérias concluem, e o cliente nunca informa isso.
func TestRegistrar_DiaSoConcluiComTodasAsMaterias(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	primeiro := diasDeEstudo(p)[0]
	if len(primeiro.Itens) < 2 {
		t.Fatalf("o cenário precisa de um dia com duas matérias, veio %d", len(primeiro.Itens))
	}

	svc := NewRegistroService(ce.deps)
	ctx := context.Background()

	p, err := svc.Registrar(ctx, ce.usuario, ce.slug, RegistroCommand{
		AtividadeID: primeiro.Itens[0].ID,
		Concluido:   true,
	})
	if err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	if diaPorData(p, primeiro.Data).Concluido {
		t.Error("o dia concluiu com só uma das duas matérias lançadas")
	}

	p, err = svc.Registrar(ctx, ce.usuario, ce.slug, RegistroCommand{
		AtividadeID: primeiro.Itens[1].ID,
		Concluido:   true,
	})
	if err != nil {
		t.Fatalf("Registrar a segunda: %v", err)
	}

	if !diaPorData(p, primeiro.Data).Concluido {
		t.Error("com as duas matérias concluídas o dia devia concluir")
	}
}

// Uma matéria agendada duas vezes no mesmo dia tem registros independentes —
// era exatamente isso que a chave antiga (data, disciplina) colapsava.
func TestRegistrar_OcorrenciasDaMesmaMateriaNaoSeMisturam(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	// O primeiro dia do plano: registrar nele não dispara antecipação, então o
	// teste observa só o efeito do lançamento.
	var alvo DiaDoPlano

	for _, d := range diasDeEstudo(p) {
		if d.Data != p.Dias[*p.HojeIndex].Data {
			continue
		}

		if len(d.Itens) >= 2 && d.Itens[0].Disciplina == d.Itens[1].Disciplina {
			alvo = d
		}

		break
	}

	if alvo.Data == "" {
		// A distribuição do motor decide se o primeiro dia repete a matéria;
		// quando não repete, a invariante é a mesma e já está coberta em
		// TestRegistros_DuasOcorrenciasDaMesmaDisciplinaSaoIndependentes.
		t.Skip("o primeiro dia deste cenário não repete a mesma matéria")
	}

	primeiroID, segundoID := alvo.Itens[0].ID, alvo.Itens[1].ID

	p, err := NewRegistroService(ce.deps).Registrar(
		context.Background(), ce.usuario, ce.slug,
		RegistroCommand{AtividadeID: primeiroID, Concluido: true},
	)
	if err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	d := diaPorData(p, alvo.Data)

	if !atividadeConcluida(d, primeiroID) {
		t.Error("a primeira ocorrência devia estar concluída")
	}

	if atividadeConcluida(d, segundoID) {
		t.Error("concluir uma ocorrência não pode concluir a outra")
	}

	if d.Concluido {
		t.Error("o dia não pode concluir com a segunda ocorrência pendente")
	}
}

func atividadeConcluida(d DiaDoPlano, id uuid.UUID) bool {
	for _, it := range d.Itens {
		if it.ID == id {
			return it.Concluido
		}
	}

	return false
}

// Mover uma matéria não pode reescrever o que foi estudado.
func TestMover_NaoTocaNosRegistros(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	origem := estudo[0]
	destino := estudo[2]

	ctx := context.Background()

	// Registra uma matéria e move OUTRA: o registro tem que sobreviver intacto.
	if _, err := NewRegistroService(ce.deps).Registrar(ctx, ce.usuario, ce.slug, RegistroCommand{
		AtividadeID: origem.Itens[0].ID,
		Concluido:   true,
	}); err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	p, err := NewCronogramaService(ce.deps).Mover(ctx, ce.usuario, ce.slug, MoverCommand{
		ID:      origem.Itens[1].ID,
		Data:    destino.Data,
		Posicao: 0,
	})
	if err != nil {
		t.Fatalf("Mover: %v", err)
	}

	if reg := ce.cronograma.registros[origem.Itens[0].ID]; !reg.Concluido {
		t.Error("mover uma matéria apagou o registro de outra")
	}

	if !contemAtividade(diaPorData(p, destino.Data), origem.Itens[1].ID) {
		t.Error("a matéria movida não chegou ao destino")
	}
}

// Uma matéria já concluída é imóvel: movê-la faria o cronograma mentir sobre o
// que aconteceu.
func TestMover_RecusaMateriaConcluida(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	origem := estudo[0]

	ctx := context.Background()

	if _, err := NewRegistroService(ce.deps).Registrar(ctx, ce.usuario, ce.slug, RegistroCommand{
		AtividadeID: origem.Itens[0].ID,
		Concluido:   true,
	}); err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	_, err := NewCronogramaService(ce.deps).Mover(ctx, ce.usuario, ce.slug, MoverCommand{
		ID:      origem.Itens[0].ID,
		Data:    estudo[2].Data,
		Posicao: 0,
	})

	if err == nil {
		t.Fatal("mover uma matéria concluída devia ser recusado")
	}

	var validacao ErrValidacao
	if !asErro(err, &validacao) {
		t.Errorf("erro = %v, esperava uma recusa de validação", err)
	}
}

// Concluir uma matéria agendada para a frente a traz para o dia em que ela foi
// realmente estudada, sem duplicá-la.
func TestRegistrar_AntecipaMateriaConcluidaAntesDaHora(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	futuro := estudo[3]
	alvo := futuro.Itens[0]

	p, err := NewRegistroService(ce.deps).Registrar(
		context.Background(), ce.usuario, ce.slug,
		RegistroCommand{AtividadeID: alvo.ID, Concluido: true},
	)
	if err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	// A atividade existe uma única vez no cronograma inteiro.
	vezes := 0

	for _, d := range p.Dias {
		for _, it := range d.Itens {
			if it.ID == alvo.ID {
				vezes++
			}
		}
	}

	if vezes != 1 {
		t.Errorf("a atividade aparece %d vezes no cronograma, quer 1", vezes)
	}

	if reg := ce.cronograma.registros[alvo.ID]; !reg.Concluido {
		t.Error("o registro devia ter sido salvo mesmo com o remanejamento")
	}
}

// Editar o concurso não pode desligar o cronograma nem o histórico: era o que
// acontecia quando as disciplinas eram apagadas e recriadas com ids novos.
func TestAtualizarConcurso_PreservaCronogramaEHistorico(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	alvo := diasDeEstudo(p)[0].Itens[0]

	ctx := context.Background()

	if _, err := NewRegistroService(ce.deps).Registrar(ctx, ce.usuario, ce.slug, RegistroCommand{
		AtividadeID: alvo.ID,
		Concluido:   true,
	}); err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	// Renomeia a primeira disciplina, devolvendo o id como o formulário faz.
	detalhe, err := NewConcursoService(ce.concursos, nil).Detalhe(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Detalhe: %v", err)
	}

	cmd := detalhe.Dados
	cmd.Disciplinas[0].Nome = "Português (renomeada)"

	if _, _, err := NewConcursoService(ce.concursos, nil).Atualizar(
		ctx, ce.usuario, ce.slug, cmd,
	); err != nil {
		t.Fatalf("Atualizar: %v", err)
	}

	depois := ce.obter(t)

	if !contemAtividade(diaPorData(depois, diasDeEstudo(p)[0].Data), alvo.ID) {
		t.Error("renomear uma disciplina desligou a atividade do cronograma")
	}

	if reg := ce.cronograma.registros[alvo.ID]; !reg.Concluido {
		t.Error("renomear uma disciplina apagou o registro de estudo")
	}
}

// Mudar o ritmo do dia alcança o cronograma já gravado, sem mexer no que já
// passou nem no que foi concluído.
func TestSalvar_MudarBlocosPorDiaAlcancaOsDiasFuturos(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	antes := len(diasDeEstudo(p)[3].Itens)

	tres := 3

	p, err := NewPlanoService(ce.deps).Salvar(
		context.Background(), ce.usuario, ce.slug,
		ConfigCommand{BlocosPorDia: &tres},
	)
	if err != nil {
		t.Fatalf("Salvar: %v", err)
	}

	depois := len(diasDeEstudo(p)[3].Itens)

	if depois <= antes {
		t.Errorf("blocos por dia no futuro = %d, esperava mais que %d", depois, antes)
	}
}

func diaPorData(p PlanoMontado, data string) DiaDoPlano {
	for _, d := range p.Dias {
		if d.Data == data {
			return d
		}
	}

	return DiaDoPlano{}
}

func contemAtividade(d DiaDoPlano, id uuid.UUID) bool {
	for _, it := range d.Itens {
		if it.ID == id {
			return true
		}
	}

	return false
}

func asErro(err error, alvo *ErrValidacao) bool {
	v, ok := err.(ErrValidacao) //nolint:errorlint // a recusa é devolvida direto
	if ok {
		*alvo = v
	}

	return ok
}

// Uma falha de persistência não pode ser engolida: se a gravação do cronograma
// falhar, o caso de uso precisa propagar o erro em vez de devolver um plano que
// parece salvo.
//
// O stub devolve o erro do CONTRATO DA PORTA — nada aqui finge conhecer as
// constraints do PostgreSQL. Que elas existam de verdade é o que a suíte de
// integração verifica.
func TestMover_PropagaFalhaDeGravacao(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	alvo := estudo[0].Itens[0]

	falha := errors.New("conexão perdida")
	ce.cronograma.erroAoGravar = falha

	_, err := NewCronogramaService(ce.deps).Mover(
		context.Background(), ce.usuario, ce.slug,
		MoverCommand{ID: alvo.ID, Data: estudo[2].Data, Posicao: 0},
	)

	if !errors.Is(err, falha) {
		t.Errorf("erro = %v, quer a falha de gravação propagada", err)
	}
}
