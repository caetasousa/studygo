package service

import (
	"context"
	"testing"

	"studygo/internal/adapter/editalproc"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// O assistente de edital, ponta a ponta, contra um edital REAL congelado.
//
// A fixture é a extração de um edital da FCC (TCE-GO, 2 cargos), capturada uma
// vez contra a Gemini. Estes testes não chamam a API: são rápidos, gratuitos,
// determinísticos e rodam sem rede — mas exercitam a mesma forma de dados que a
// produção recebe, incluindo os cantos que dados inventados à mão não teriam
// (peso por grupo, contagem só no total, 22 marcos, acentuação).

func processadorDeFixture(t *testing.T) *editalproc.DeFixture {
	t.Helper()

	p, err := editalproc.NovoDeFixture()
	if err != nil {
		t.Fatalf("carregando a fixture do edital: %v", err)
	}

	return p
}

var pdfDeMentira = port.EditalUpload{PDF: []byte("%PDF-1.4 fake"), MIME: "application/pdf"}

// Passo 1: o edital da FCC declara dois cargos, e os dois precisam chegar à
// tela para o usuário escolher.
func TestEdital_AnalisarListaOsDoisCargos(t *testing.T) {
	t.Parallel()

	svc := NewConcursoService(&fakeConcursos{}, processadorDeFixture(t))

	a, err := svc.AnalisarEdital(context.Background(), "dono", pdfDeMentira)
	if err != nil {
		t.Fatalf("AnalisarEdital: %v", err)
	}

	if a.Banca != "Fundação Carlos Chagas" {
		t.Errorf("banca = %q", a.Banca)
	}

	if len(a.Cargos) != 2 {
		t.Fatalf("cargos = %d, quer 2", len(a.Cargos))
	}

	codigos := map[string]bool{}
	for _, c := range a.Cargos {
		codigos[c.Codigo] = true

		if c.Vagas == nil {
			t.Errorf("cargo %s sem número de vagas", c.Codigo)
		}
	}

	if !codigos["A01"] || !codigos["B02"] {
		t.Errorf("códigos = %v, quer A01 e B02", codigos)
	}
}

// Passo 2: a estrutura de cada cargo. Este edital informa o total do GRUPO
// (25 gerais, 45 específicas) sem abrir por disciplina — e é assim que a maioria
// dos editais da FCC vem.
//
// O processor sinaliza isso como blocker em vez de inventar a divisão: o
// assistente faz o usuário estimar ou ratear antes de salvar. Um dado montado à
// mão dificilmente teria esse formato, e é justamente ele que o wizard precisa
// tratar.
func TestEdital_EstruturaTrazTotalPorGrupoEBloqueia(t *testing.T) {
	t.Parallel()

	svc := NewConcursoService(&fakeConcursos{}, processadorDeFixture(t))
	ctx := context.Background()

	a, err := svc.AnalisarEdital(ctx, "dono", pdfDeMentira)
	if err != nil {
		t.Fatalf("AnalisarEdital: %v", err)
	}

	for _, cargo := range []string{"A01", "B02"} {
		t.Run(cargo, func(t *testing.T) {
			t.Parallel()

			e, err := svc.EstruturaDoCargo(ctx, "dono", a.DocumentoID, cargo)
			if err != nil {
				t.Fatalf("EstruturaDoCargo: %v", err)
			}

			if e.Nome == "" || e.Prova == "" {
				t.Errorf("estrutura sem nome/prova: %+v", e)
			}

			if len(e.Gerais) == 0 || len(e.Especificas) == 0 {
				t.Fatal("faltou um dos grupos de conhecimento")
			}

			// O total vem no grupo; as disciplinas vêm sem contagem.
			g := e.Gerais[0]
			if g.Total == nil || *g.Total != 25 {
				t.Errorf("total de gerais = %v, quer 25", g.Total)
			}

			for _, d := range g.Disciplinas {
				if d.Questoes != nil {
					t.Errorf("a disciplina %q veio com questões; o edital não abriu", d.Nome)
				}
			}

			// E o assistente precisa saber que não pode salvar assim.
			temBlocker := false

			for _, al := range e.Alertas {
				if al.Gravidade == "blocker" {
					temBlocker = true
				}
			}

			if !temBlocker {
				t.Error("sem contagem por disciplina, o edital devia gerar um blocker")
			}

			// O cronograma oficial do edital vira marcos.
			if len(e.Marcos) < 20 {
				t.Errorf("marcos = %d, esperava o cronograma completo do edital", len(e.Marcos))
			}
		})
	}
}

// Passo 3: o conteúdo programático. Só as disciplinas pedidas voltam.
func TestEdital_ConteudoSoDasDisciplinasPedidas(t *testing.T) {
	t.Parallel()

	svc := NewConcursoService(&fakeConcursos{}, processadorDeFixture(t))
	ctx := context.Background()

	a, err := svc.AnalisarEdital(ctx, "dono", pdfDeMentira)
	if err != nil {
		t.Fatalf("AnalisarEdital: %v", err)
	}

	pedidas := []string{"Língua Portuguesa", "Banco de Dados"}

	c, err := svc.ConteudoDoEdital(ctx, "dono", a.DocumentoID, "B02", pedidas, port.EditalUpload{})
	if err != nil {
		t.Fatalf("ConteudoDoEdital: %v", err)
	}

	if len(c.Itens) != len(pedidas) {
		t.Fatalf("itens = %d, quer %d", len(c.Itens), len(pedidas))
	}

	for _, it := range c.Itens {
		if len(it.Temas) == 0 {
			t.Errorf("a disciplina %q voltou sem temas", it.Nome)
		}
	}
}

// O caminho completo do assistente: analisar → estrutura → conteúdo → salvar.
//
// É aqui que os dados do edital encontram as invariantes do domínio: o rateio
// das questões, o peso por bloco e a geração dos códigos de disciplina.
func TestEdital_DoPdfAoConcursoSalvo(t *testing.T) {
	t.Parallel()

	repo := &fakeConcursos{}
	svc := NewConcursoService(repo, processadorDeFixture(t))
	ctx := context.Background()

	a, err := svc.AnalisarEdital(ctx, "dono", pdfDeMentira)
	if err != nil {
		t.Fatalf("AnalisarEdital: %v", err)
	}

	e, err := svc.EstruturaDoCargo(ctx, "dono", a.DocumentoID, "B02")
	if err != nil {
		t.Fatalf("EstruturaDoCargo: %v", err)
	}

	nomes := []string{}
	for _, g := range append(e.Gerais, e.Especificas...) {
		for _, d := range g.Disciplinas {
			nomes = append(nomes, d.Nome)
		}
	}

	conteudo, err := svc.ConteudoDoEdital(ctx, "dono", a.DocumentoID, "B02", nomes, port.EditalUpload{})
	if err != nil {
		t.Fatalf("ConteudoDoEdital: %v", err)
	}

	temasDe := map[string][]string{}
	for _, it := range conteudo.Itens {
		temasDe[it.Nome] = it.Temas
	}

	// O usuário rateia o total do grupo entre as disciplinas — o passo que o
	// blocker exige.
	cmd := ConcursoCommand{
		Nome:          e.Nome,
		Banca:         a.Banca,
		Cargo:         "B02",
		Prova:         e.Prova,
		RetaFinalDias: 30,
	}

	for _, g := range append(e.Gerais, e.Especificas...) {
		porDisciplina := 0
		if g.Total != nil && len(g.Disciplinas) > 0 {
			porDisciplina = *g.Total / len(g.Disciplinas)
		}

		for _, d := range g.Disciplinas {
			cmd.Disciplinas = append(cmd.Disciplinas, DisciplinaCommand{
				Nome: d.Nome, Bloco: g.Tipo, Questoes: max(porDisciplina, 1),
				Temas: temasDe[d.Nome],
			})
		}
	}

	for _, m := range e.Marcos {
		cmd.Marcos = append(cmd.Marcos, MarcoCommand{
			Data: m.Data, DataFim: m.DataFim, Titulo: m.Titulo, ExigeAcao: m.ExigeAcao,
		})
	}

	resumo, _, err := svc.Criar(ctx, uuid.New(), cmd)
	if err != nil {
		t.Fatalf("Criar: %v", err)
	}

	if resumo.Slug == "" {
		t.Error("o concurso salvo veio sem slug")
	}

	salvo := repo.c

	if len(salvo.Disciplinas) != len(cmd.Disciplinas) {
		t.Fatalf("disciplinas salvas = %d, quer %d", len(salvo.Disciplinas), len(cmd.Disciplinas))
	}

	// O domínio atribuiu um código único e legível a cada matéria.
	codigos := map[string]bool{}

	for _, d := range salvo.Disciplinas {
		if d.Codigo == "" {
			t.Errorf("a disciplina %q ficou sem código", d.Nome)
		}

		if codigos[d.Codigo] {
			t.Errorf("código repetido: %q", d.Codigo)
		}

		codigos[d.Codigo] = true

		// E o peso saiu do bloco a que ela pertence.
		if d.Peso == 0 {
			t.Errorf("a disciplina %q ficou sem peso", d.Nome)
		}
	}

	// O marco que cai na data da prova é reconhecido como tal.
	temProva := false

	for _, m := range salvo.Marcos {
		if m.EProva {
			temProva = true
		}
	}

	if !temProva && len(salvo.Marcos) > 0 {
		t.Log("nenhum marco coincide com a data da prova — confira o edital se isso surpreender")
	}
}
