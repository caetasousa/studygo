//go:build integration

package service

import (
	"context"
	"maps"
	"slices"
	"testing"

	"studygo/internal/domain/plano"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// O plano, do caso de uso ao banco: materializar, registrar, mover, mudar o
// ritmo. Todo fluxo aqui já quebrou em produção alguma vez — e quase sempre
// onde a aplicação encontra a FK RESTRICT, a unique diferível ou a transação.

// O primeiro acesso cria o plano e GRAVA o cronograma inteiro. É o momento em
// que o motor puro encontra o banco, e onde um erro de mapeamento entre código
// de disciplina e id apareceria.
func TestFluxo_PrimeiroAcessoMaterializaOCronograma(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

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
	depois := ce.obter(t)

	if depois.Dias[0].Itens[0].ID != p.Dias[0].Itens[0].ID {
		t.Error("os ids mudaram entre duas leituras")
	}
}

// A invariante central do produto, do service ao banco: o dia só conclui quando
// TODAS as suas matérias concluem, e a conclusão é DERIVADA — nunca uma coluna.
func TestFluxo_ConclusaoDoDiaEDerivada(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	primeiro := diasDeEstudo(p)[0]
	if len(primeiro.Itens) < 2 {
		t.Fatalf("o cenário precisa de um dia com duas matérias, veio %d", len(primeiro.Itens))
	}

	registros := NewRegistroService(ce.deps)
	ctx := context.Background()

	p, err := registros.Registrar(ctx, ce.usuario, ce.slug, RegistroCommand{
		AtividadeID: primeiro.Itens[0].ID, Concluido: true,
	})
	if err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	if diaPorData(p, primeiro.Data).Concluido {
		t.Error("o dia concluiu com só uma das duas matérias")
	}

	p, err = registros.Registrar(ctx, ce.usuario, ce.slug, RegistroCommand{
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

	ce := novoCenario(t)
	p := ce.obter(t)

	alvo := diasDeEstudo(p)[0].Itens[0]
	ctx := context.Background()

	if _, err := NewRegistroService(ce.deps).Registrar(
		ctx, ce.usuario, ce.slug,
		RegistroCommand{AtividadeID: alvo.ID, Concluido: true},
	); err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	concursos := NewConcursoService(ce.deps.Concursos, nil)

	detalhe, err := concursos.Detalhe(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Detalhe: %v", err)
	}

	cmd := detalhe.Dados
	cmd.Disciplinas[0].Nome = "Português e Redação"

	if _, _, err := concursos.Atualizar(ctx, ce.usuario, ce.slug, cmd); err != nil {
		t.Fatalf("Atualizar: %v", err)
	}

	depois := ce.obter(t)
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

// Concluir uma matéria agendada para a frente a traz para hoje e fecha o buraco,
// sem duplicá-la. O remanejamento reescreve o cronograma inteiro numa transação,
// então é aqui que a FK RESTRICT e a unique diferível são exercitadas de fato.
func TestFluxo_AntecipacaoNaoDuplicaNemPerdeRegistro(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	futura := estudo[4].Itens[0]

	p, err := NewRegistroService(ce.deps).Registrar(
		context.Background(), ce.usuario, ce.slug,
		RegistroCommand{AtividadeID: futura.ID, Concluido: true},
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

	ce := novoCenario(t)
	p := ce.obter(t)

	primeiro := diasDeEstudo(p)[0]
	ctx := context.Background()

	for _, it := range primeiro.Itens {
		if _, err := NewRegistroService(ce.deps).Registrar(
			ctx, ce.usuario, ce.slug,
			RegistroCommand{AtividadeID: it.ID, Concluido: true},
		); err != nil {
			t.Fatalf("Registrar: %v", err)
		}
	}

	antes := len(primeiro.Itens)
	tres := 3

	p, err := NewPlanoService(ce.deps).Salvar(
		ctx, ce.usuario, ce.slug, ConfigCommand{BlocosPorDia: &tres},
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

	ce := novoCenario(t)
	ce.obter(t) // materializa o cronograma

	entregues := &notifierEspiao{}

	svc := NewNotificacaoService(
		ce.deps.Planos, ce.deps.Cronograma, ce.deps.Concursos,
		entregues, ce.deps.Relogio,
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

// Depois de materializado, ler o plano é só leitura: um GET não escreve. A
// versão de cada linha no banco (xmin) mudaria com qualquer escrita.
func TestObter_NaoEscreveDepoisDeMaterializado(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.obter(t)

	antes := ce.versoesDasLinhas(t)

	ce.obter(t)
	ce.obter(t)

	if depois := ce.versoesDasLinhas(t); !maps.Equal(antes, depois) {
		t.Error("ler o plano reescreveu o cronograma — GET não pode escrever")
	}
}

// Uma matéria agendada duas vezes no mesmo dia tem registros independentes —
// era exatamente isso que a chave antiga (data, disciplina) colapsava. O motor
// não repete a matéria no primeiro dia, então o estudante a repete: traz para
// hoje outra ocorrência dela.
func TestRegistrar_OcorrenciasDaMesmaMateriaNaoSeMisturam(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	hoje := estudo[0]
	primeira := hoje.Itens[0]

	var segunda AtividadeDoDia

	for _, d := range estudo[1:] {
		for _, it := range d.Itens {
			if it.Disciplina == primeira.Disciplina && segunda.ID == uuid.Nil {
				segunda = it
			}
		}
	}

	if segunda.ID == uuid.Nil {
		t.Fatalf("o cenário precisa de outra ocorrência de %s adiante", primeira.Disciplina)
	}

	ctx := context.Background()

	if _, err := NewCronogramaService(ce.deps).Mover(ctx, ce.usuario, ce.slug, MoverCommand{
		ID: segunda.ID, Data: hoje.Data, Posicao: len(hoje.Itens),
	}); err != nil {
		t.Fatalf("Mover: %v", err)
	}

	p, err := NewRegistroService(ce.deps).Registrar(ctx, ce.usuario, ce.slug,
		RegistroCommand{AtividadeID: primeira.ID, Concluido: true},
	)
	if err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	d := diaPorData(p, hoje.Data)

	if !contemAtividade(d, segunda.ID) {
		t.Fatal("a segunda ocorrência não ficou no dia")
	}

	if !atividadeConcluida(d, primeira.ID) {
		t.Error("a primeira ocorrência devia estar concluída")
	}

	if atividadeConcluida(d, segunda.ID) {
		t.Error("concluir uma ocorrência concluiu a outra")
	}

	if d.Concluido {
		t.Error("o dia não pode concluir com a segunda ocorrência pendente")
	}
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

	if reg := ce.registros(t)[origem.Itens[0].ID]; !reg.Concluido {
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

// Uma falha de persistência não pode ser engolida: se a gravação do cronograma
// falhar, o caso de uso propaga o erro em vez de devolver um plano que parece
// salvo — e nada fica gravado pela metade. A falha é do próprio PostgreSQL.
func TestMover_PropagaFalhaDeGravacao(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	alvo := estudo[0].Itens[0]
	antes := ce.atividades(t)

	ce.travarEscrita(t, "atividades")

	_, err := NewCronogramaService(ce.deps).Mover(
		context.Background(), ce.usuario, ce.slug,
		MoverCommand{ID: alvo.ID, Data: estudo[2].Data, Posicao: 0},
	)
	if err == nil {
		t.Fatal("o banco recusou a gravação e o Mover respondeu como se tivesse salvo")
	}

	var validacao ErrValidacao
	if asErro(err, &validacao) {
		t.Errorf("erro = %v: falha de gravação não é recusa do usuário", err)
	}

	if depois := ce.atividades(t); !slices.EqualFunc(antes, depois, mesmaVaga) {
		t.Error("a gravação falhou, mas o cronograma mudou pela metade")
	}
}

// mesmaVaga diz se a atividade continua no mesmo lugar do cronograma.
func mesmaVaga(a, b plano.Atividade) bool {
	return a.ID == b.ID && a.Data.Equal(b.Data) && a.Posicao == b.Posicao
}
