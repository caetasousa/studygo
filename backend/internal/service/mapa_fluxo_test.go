//go:build integration

package service

import (
	"errors"
	"testing"

	"studygo/internal/adapter/postgres"
	"studygo/internal/domain/mapa"

	"github.com/google/uuid"
)

// Excluir um tópico do mapa, no banco de verdade. Como pode falhar — escrito
// antes do código:
//
//	X1  some outro item (o vizinho de cima ou de baixo), ou os filhos do excluído ficam
//	X2  a exclusão não fica gravada: ler de novo traz o item de volta
//	X3  o mapa mudou (outra aba, reimportação) e o caminho agora aponta outro item,
//	    que é apagado no lugar do que a pessoa viu
//	X4  caminho fora do mapa apaga alguma coisa, ou derruba o servidor
//	X5  excluir o ramo deixa as questões dele ativas; ou excluir um tópico de
//	    dentro do ramo desativa as questões do ramo
//	X6  outra conta exclui item de um mapa que não é dela
//	X7  as respostas das questões do ramo excluído somem (são história)

const mapaDoTeste = `# Ciclo da Água
slug: ciclo-da-agua

- Evaporação
  - A evaporação leva a água ao estado gasoso
    - Acontece nos oceanos
    - Aumenta com o calor
  - Uma poça que seca ao sol
- Condensação
  - Forma as nuvens
- Precipitação
  - Chuva
  - Neve
`

type cenarioDoMapa struct {
	svc  *MapaService
	repo *postgres.MapaRepo
	dono uuid.UUID
}

func novoCenarioDoMapa(t *testing.T) *cenarioDoMapa {
	t.Helper()

	ce := novoCenario(t)
	repo := postgres.NewMapaRepo(ce.pool)
	svc := NewMapaService(repo, ce.deps.Concursos)

	if _, err := svc.Importar(t.Context(), ce.usuario, mapaDoTeste, ""); err != nil {
		t.Fatalf("importando o mapa: %v", err)
	}

	if _, err := svc.ImportarQuestoes(t.Context(), ce.usuario, "ciclo-da-agua", mapa.ArquivoDeQuestoes{
		Mapa: "ciclo-da-agua",
		Questoes: []mapa.Questao{
			{Chave: "q1", Ramo: "Evaporação", Origem: "FGV · 2024", Enunciado: "Evaporar é ir ao gasoso.", Gabarito: "Certo", Comentario: "Sim."},
			{Chave: "q2", Ramo: "Condensação", Origem: "FCC · 2023", Enunciado: "Condensar forma nuvens.", Gabarito: "Certo", Comentario: "Sim."},
		},
	}); err != nil {
		t.Fatalf("importando as questões: %v", err)
	}

	return &cenarioDoMapa{svc: svc, repo: repo, dono: ce.usuario}
}

// textos é a árvore achatada, "nível:texto", na ordem de leitura.
func textos(itens []mapa.Item) []string {
	var out []string

	var descer func([]mapa.Item, int)

	descer = func(is []mapa.Item, nivel int) {
		for _, it := range is {
			out = append(out, string(rune('0'+nivel))+":"+it.Texto)
			descer(it.Filhos, nivel+1)
		}
	}

	descer(itens, 0)

	return out
}

func iguais(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// X1, X2: sai o item e tudo o que há dentro dele, e só isso — e fica gravado.
func TestMapa_ExcluirItemLevaOsFilhosEFicaGravado(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDoMapa(t)

	lido, err := ce.svc.ExcluirItem(t.Context(), ce.dono, "ciclo-da-agua",
		[]int{0, 0}, "A evaporação leva a água ao estado gasoso")
	if err != nil {
		t.Fatalf("ExcluirItem: %v", err)
	}

	quer := []string{
		"0:Evaporação", "1:Uma poça que seca ao sol",
		"0:Condensação", "1:Forma as nuvens",
		"0:Precipitação", "1:Chuva", "1:Neve",
	}

	if got := textos(lido.Mapa.Ramos); !iguais(got, quer) {
		t.Errorf("depois de excluir:\n got %q\nquer %q", got, quer)
	}

	relido, err := ce.svc.Ler(t.Context(), ce.dono, "ciclo-da-agua")
	if err != nil {
		t.Fatalf("Ler: %v", err)
	}

	if got := textos(relido.Mapa.Ramos); !iguais(got, quer) {
		t.Errorf("relido do banco:\n got %q\nquer %q", got, quer)
	}

	resumo, err := ce.repo.ResumoPorSlug(t.Context(), ce.dono, "ciclo-da-agua")
	if err != nil {
		t.Fatalf("ResumoPorSlug: %v", err)
	}

	if resumo.Ramos != 3 || resumo.Itens != 7 {
		t.Errorf("resumo = %d ramos, %d itens; quer 3 e 7", resumo.Ramos, resumo.Itens)
	}
}

// X3, X4: o caminho que não aponta mais o item que a pessoa viu é recusado, e
// nada sai do mapa.
func TestMapa_ExcluirItemQueMudouERecusado(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDoMapa(t)

	casos := []struct {
		nome    string
		caminho []int
		texto   string
	}{
		{"texto de outro item", []int{0, 1}, "Acontece nos oceanos"},
		{"índice além do fim", []int{0, 9}, "Uma poça que seca ao sol"},
		{"ramo além do fim", []int{7}, "Evaporação"},
		{"índice negativo", []int{-1}, "Evaporação"},
		{"caminho vazio", []int{}, ""},
	}

	for _, c := range casos {
		_, err := ce.svc.ExcluirItem(t.Context(), ce.dono, "ciclo-da-agua", c.caminho, c.texto)
		if !errors.Is(err, mapa.ErrItemMudou) {
			t.Errorf("%s: erro = %v, quer ErrItemMudou", c.nome, err)
		}
	}

	lido, err := ce.svc.Ler(t.Context(), ce.dono, "ciclo-da-agua")
	if err != nil {
		t.Fatalf("Ler: %v", err)
	}

	if n := len(textos(lido.Mapa.Ramos)); n != 10 {
		t.Errorf("o mapa ficou com %d itens, quer os 10 de antes", n)
	}
}

// X5, X7: o ramo excluído leva as questões dele para fora da página, sem apagar
// as respostas; o tópico de dentro do ramo não mexe em questão nenhuma.
func TestMapa_ExcluirRamoTiraAsQuestoesDele(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDoMapa(t)

	antes, err := ce.svc.Ler(t.Context(), ce.dono, "ciclo-da-agua")
	if err != nil {
		t.Fatalf("Ler: %v", err)
	}

	q1 := antes.Questoes[0]
	if _, err := ce.svc.Responder(t.Context(), ce.dono, q1.ID, "CERTO"); err != nil {
		t.Fatalf("Responder: %v", err)
	}

	lido, err := ce.svc.ExcluirItem(t.Context(), ce.dono, "ciclo-da-agua", []int{1, 0}, "Forma as nuvens")
	if err != nil {
		t.Fatalf("excluindo o tópico: %v", err)
	}

	if len(lido.Questoes) != 2 {
		t.Fatalf("excluir um tópico de dentro do ramo tirou questões: ficaram %d", len(lido.Questoes))
	}

	lido, err = ce.svc.ExcluirItem(t.Context(), ce.dono, "ciclo-da-agua", []int{0}, "Evaporação")
	if err != nil {
		t.Fatalf("excluindo o ramo: %v", err)
	}

	if len(lido.Questoes) != 1 || lido.Questoes[0].Questao.Ramo != "Condensação" {
		t.Fatalf("depois de excluir o ramo, questões = %+v; quer só a da Condensação", lido.Questoes)
	}

	// A resposta é história: reimportar as questões traz a q1 de volta com ela.
	// (O ramo voltou na reimportação do mapa.)
	if _, err := ce.svc.Importar(t.Context(), ce.dono, mapaDoTeste, ""); err != nil {
		t.Fatalf("reimportando o mapa: %v", err)
	}

	if _, err := ce.svc.ImportarQuestoes(t.Context(), ce.dono, "ciclo-da-agua", mapa.ArquivoDeQuestoes{
		Mapa: "ciclo-da-agua",
		Questoes: []mapa.Questao{
			{Chave: "q1", Ramo: "Evaporação", Origem: "FGV · 2024", Enunciado: "Evaporar é ir ao gasoso.", Gabarito: "Certo", Comentario: "Sim."},
			{Chave: "q2", Ramo: "Condensação", Origem: "FCC · 2023", Enunciado: "Condensar forma nuvens.", Gabarito: "Certo", Comentario: "Sim."},
		},
	}); err != nil {
		t.Fatalf("reimportando as questões: %v", err)
	}

	volta, err := ce.svc.Ler(t.Context(), ce.dono, "ciclo-da-agua")
	if err != nil {
		t.Fatalf("Ler: %v", err)
	}

	if volta.Questoes[0].ID != q1.ID || volta.Questoes[0].Ultima == nil {
		t.Error("a questão do ramo excluído voltou sem a resposta que tinha")
	}
}

// X6: o mapa de uma conta não é alcançado por outra.
func TestMapa_ExcluirItemDeOutraContaNaoAcha(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDoMapa(t)
	outra := uuid.New()

	_, err := ce.svc.ExcluirItem(t.Context(), outra, "ciclo-da-agua", []int{0}, "Evaporação")
	if !errors.Is(err, mapa.ErrNaoEncontrado) {
		t.Errorf("erro = %v, quer ErrNaoEncontrado", err)
	}
}
