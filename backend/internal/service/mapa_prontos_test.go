//go:build integration

package service

import (
	"errors"
	"testing"

	"studygo/internal/adapter/mapasprontos"
	"studygo/internal/adapter/postgres"
	"studygo/internal/domain/mapa"
)

// Os mapas que vêm com o app, no banco de verdade e com o conteúdo embutido de
// verdade. Como pode quebrar — escrito antes do código (M40 no E2E):
//
//	R1  a conta nova vê o pronto como já adicionado, ou a que adicionou não vê
//	R2  adicionar não grava o mapa inteiro com todas as questões
//	R3  adicionar de novo duplica o mapa, apaga as respostas ou o vínculo
//	R4  com o concurso aberto, o mapa não vai para a matéria que o texto indica
//	R5  adicionar numa conta põe o mapa em outra
//	R6  um slug que não é de pronto nenhum grava alguma coisa

const slugDoPronto = "lingua-portuguesa-fcc"

func TestMapasProntos_AdicionarDeNovoAtualizaSemPerderRespostaNemVinculo(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)

	prontos, err := mapasprontos.Todos()
	if err != nil {
		t.Fatal(err)
	}

	svc := NewMapaService(postgres.NewMapaRepo(ce.pool), ce.deps.Concursos, prontos...)

	// R1: antes de adicionar, a conta não tem o pronto.
	lista, err := svc.Prontos(t.Context(), ce.usuario)
	if err != nil {
		t.Fatal(err)
	}

	if len(lista) != 1 || lista[0].Slug != slugDoPronto || lista[0].NaConta {
		t.Fatalf("lista dos prontos numa conta nova: %+v", lista)
	}

	if lista[0].Ramos != 22 || lista[0].Questoes != 282 {
		t.Fatalf("o pronto anuncia %d ramos e %d questões", lista[0].Ramos, lista[0].Questoes)
	}

	// R2, R4: adicionar com o concurso aberto grava tudo e vincula a Língua Portuguesa.
	res, err := svc.AdicionarPronto(t.Context(), ce.usuario, slugDoPronto, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	if !res.Novo || res.Questoes != 282 {
		t.Fatalf("adicionar: novo=%v questões=%d", res.Novo, res.Questoes)
	}

	if len(res.Vinculadas) != 1 || res.Vinculadas[0].Codigo != "LINPO" {
		t.Fatalf("vinculado a %+v, esperado só LINPO", res.Vinculadas)
	}

	lido, err := svc.Ler(t.Context(), ce.usuario, slugDoPronto)
	if err != nil {
		t.Fatal(err)
	}

	if ramos, _ := lido.Mapa.Contar(); ramos != 22 || len(lido.Questoes) != 282 {
		t.Fatalf("gravado: %d ramos, %d questões", ramos, len(lido.Questoes))
	}

	// R3: responder, adicionar de novo, e a resposta e o vínculo continuam.
	q := lido.Questoes[0]
	if _, err := svc.Responder(t.Context(), ce.usuario, q.ID, q.Questao.Gabarito); err != nil {
		t.Fatal(err)
	}

	res, err = svc.AdicionarPronto(t.Context(), ce.usuario, slugDoPronto, "")
	if err != nil {
		t.Fatal(err)
	}

	if res.Novo {
		t.Fatal("adicionar de novo criou outro mapa")
	}

	catalogo, err := svc.Catalogo(t.Context(), ce.usuario)
	if err != nil {
		t.Fatal(err)
	}

	if len(catalogo) != 1 {
		t.Fatalf("a conta tem %d mapas depois de adicionar duas vezes", len(catalogo))
	}

	lido, err = svc.Ler(t.Context(), ce.usuario, slugDoPronto)
	if err != nil {
		t.Fatal(err)
	}

	if len(lido.Questoes) != 282 {
		t.Fatalf("depois de atualizar: %d questões", len(lido.Questoes))
	}

	respondida := false
	for _, x := range lido.Questoes {
		if x.ID == q.ID && x.Ultima != nil && x.Ultima.Acertou {
			respondida = true
		}
	}

	if !respondida {
		t.Fatal("a resposta sumiu ao adicionar de novo")
	}

	materias, err := svc.DoConcurso(t.Context(), ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	vinculado := false
	for _, m := range materias {
		for _, x := range m.Mapas {
			if m.Codigo == "LINPO" && x.Resumo.Slug == slugDoPronto {
				vinculado = true
			}
		}
	}

	if !vinculado {
		t.Fatal("o vínculo com Língua Portuguesa sumiu ao adicionar de novo")
	}

	lista, err = svc.Prontos(t.Context(), ce.usuario)
	if err != nil {
		t.Fatal(err)
	}

	if !lista[0].NaConta {
		t.Fatal("a conta que adicionou não vê o pronto como dela")
	}

	// R5: outra conta continua sem ele.
	outra := novoDono(t, ce.pool)

	lista, err = svc.Prontos(t.Context(), outra)
	if err != nil {
		t.Fatal(err)
	}

	if lista[0].NaConta {
		t.Fatal("o pronto aparece como adicionado numa conta que não o adicionou")
	}

	if _, err := svc.Ler(t.Context(), outra, slugDoPronto); !errors.Is(err, mapa.ErrNaoEncontrado) {
		t.Fatalf("outra conta lê o mapa: %v", err)
	}
}

// R6: o slug de pronto nenhum é recusado sem gravar nada.
func TestMapasProntos_SlugDesconhecidoNaoGravaNada(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)

	prontos, err := mapasprontos.Todos()
	if err != nil {
		t.Fatal(err)
	}

	svc := NewMapaService(postgres.NewMapaRepo(ce.pool), ce.deps.Concursos, prontos...)

	if _, err := svc.AdicionarPronto(t.Context(), ce.usuario, "nao-existe", ""); !errors.Is(err, mapa.ErrNaoEncontrado) {
		t.Fatalf("slug desconhecido: %v", err)
	}

	catalogo, err := svc.Catalogo(t.Context(), ce.usuario)
	if err != nil {
		t.Fatal(err)
	}

	if len(catalogo) != 0 {
		t.Fatalf("slug desconhecido gravou %d mapas", len(catalogo))
	}
}
