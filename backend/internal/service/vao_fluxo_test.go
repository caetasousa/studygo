package service

import (
	"context"
	"testing"
)

// Adiantar-se compra tempo, não abre vão.
//
// Trazer uma matéria para um dia anterior deixa o dia de origem com uma vaga a
// menos. Um vão desses não é neutro: na segunda vez que o estudante se adianta,
// o dia fica vazio, e o cronograma passa a prometer uma folga que ninguém pediu.
// Estes testes fixam que o vão se fecha sozinho — encostando o que vem depois,
// na ordem em que já estava.

// posicoes lista as atividades de um dia, na ordem.
func posicoes(p PlanoMontado, data string) []string {
	d := diaPorData(p, data)

	out := make([]string, 0, len(d.Itens))
	for _, it := range d.Itens {
		out = append(out, it.ID.String())
	}

	return out
}

func TestMover_AdiantarFechaOVaoNoDiaDeOrigem(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	origem, destino, seguinte := estudo[4], estudo[0], estudo[5]
	antes := len(origem.Itens)
	primeiraDoSeguinte := seguinte.Itens[0].ID

	p, err := NewCronogramaService(ce.deps).Mover(
		context.Background(), ce.usuario, ce.slug,
		MoverCommand{
			ID: origem.Itens[0].ID, Data: destino.Data, Posicao: len(destino.Itens),
		},
	)
	if err != nil {
		t.Fatalf("Mover: %v", err)
	}

	if !contemAtividade(diaPorData(p, destino.Data), origem.Itens[0].ID) {
		t.Fatal("a matéria adiantada não chegou ao destino")
	}

	if depois := len(diaPorData(p, origem.Data).Itens); depois != antes {
		t.Errorf("o dia de origem ficou com %d atividades, quer %d", depois, antes)
	}

	// O vão é fechado puxando o que vinha depois, e não recomeçando o plano: a
	// primeira matéria do dia seguinte é a que sobe.
	if !contemAtividade(diaPorData(p, origem.Data), primeiraDoSeguinte) {
		t.Error("o que vinha depois não subiu para fechar o vão")
	}
}

// O dia que recebeu a matéria adiantada fica com uma a mais, e assim continua:
// fechar o vão é sobre o dia de origem, não sobre normalizar o cronograma
// inteiro por cima da escolha do estudante.
func TestMover_AdiantarNaoDesfazOProprioAdiantamento(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	origem, destino := estudo[4], estudo[0]
	antes := len(destino.Itens)

	p, err := NewCronogramaService(ce.deps).Mover(
		context.Background(), ce.usuario, ce.slug,
		MoverCommand{
			ID: origem.Itens[0].ID, Data: destino.Data, Posicao: len(destino.Itens),
		},
	)
	if err != nil {
		t.Fatalf("Mover: %v", err)
	}

	if depois := len(diaPorData(p, destino.Data).Itens); depois != antes+1 {
		t.Errorf("o dia de destino ficou com %d atividades, quer %d", depois, antes+1)
	}
}

// Mandar uma matéria para a FRENTE é uma escolha de data. Encostar o cronograma
// ali a traria de volta, desfazendo o que acabou de ser pedido.
func TestMover_AdiarUmaMateriaMantemOndeFoiPosta(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	alvo := estudo[0].Itens[0].ID
	destino := estudo[3]

	p, err := NewCronogramaService(ce.deps).Mover(
		context.Background(), ce.usuario, ce.slug,
		MoverCommand{ID: alvo, Data: destino.Data, Posicao: 0},
	)
	if err != nil {
		t.Fatalf("Mover: %v", err)
	}

	if !contemAtividade(diaPorData(p, destino.Data), alvo) {
		t.Error("a matéria não ficou no dia para onde foi mandada")
	}
}

// A troca não abre vão: as duas matérias mudam de lugar e cada dia mantém a
// conta. Compactar aí só remexeria o cronograma sem motivo.
func TestMover_TrocaNaoRemexeOResto(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	distante := estudo[6].Data
	antes := posicoes(p, distante)

	p, err := NewCronogramaService(ce.deps).Mover(
		context.Background(), ce.usuario, ce.slug,
		MoverCommand{
			ID: estudo[1].Itens[0].ID, Data: estudo[0].Data, Posicao: 0, Trocar: true,
		},
	)
	if err != nil {
		t.Fatalf("Mover: %v", err)
	}

	depois := posicoes(p, distante)
	if len(antes) != len(depois) {
		t.Fatalf("o dia distante tinha %d atividades e ficou com %d", len(antes), len(depois))
	}

	for i := range antes {
		if antes[i] != depois[i] {
			t.Errorf("a troca remexeu um dia distante: %v -> %v", antes, depois)

			break
		}
	}
}

// O mesmo vale para o caminho automático: concluir hoje uma matéria agendada
// para a frente a traz para hoje e encosta o que ficou para trás dela.
func TestRegistrar_AntecipacaoFechaOVao(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	futuro := estudo[3]
	antes := len(futuro.Itens)

	p, err := NewRegistroService(ce.deps).Registrar(
		context.Background(), ce.usuario, ce.slug,
		RegistroCommand{AtividadeID: futuro.Itens[0].ID, Concluido: true},
	)
	if err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	if depois := len(diaPorData(p, futuro.Data).Itens); depois != antes {
		t.Errorf("o dia de origem ficou com %d atividades, quer %d", depois, antes)
	}
}
