package service

import (
	"context"
	"strings"
	"testing"
)

// A planilha, ida e volta.
//
// O CSV é a cópia de segurança do estudante e o caminho por onde ele traz o
// histórico de outra instalação. O teste que importa é o ciclo fechado: o que o
// export escreve, o import tem de reconhecer — sem isso as duas metades
// envelhecem em direções diferentes.

func TestPlanilha_ExportarEImportarDeVolta(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	primeira, segunda := estudo[0].Itens[0], estudo[1].Itens[0]

	horas := 0.75
	questoes, acertos := 20, 16

	reg := NewRegistroService(ce.deps)
	if _, err := reg.Registrar(ctx, ce.usuario, ce.slug, RegistroCommand{
		AtividadeID: primeira.ID, Horas: &horas, Questoes: &questoes,
		Acertos: &acertos, Concluido: true,
	}); err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	if _, err := reg.Registrar(ctx, ce.usuario, ce.slug, RegistroCommand{
		AtividadeID: segunda.ID, Horas: &horas, Concluido: true,
	}); err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	svc := NewPlanilhaService(ce.deps)

	csv, err := svc.CSV(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("CSV: %v", err)
	}

	// O histórico some — é a instalação nova, com o mesmo concurso e o mesmo
	// cronograma, mas sem nada lançado.
	if err := ce.cronograma.ApagarRegistros(ctx, ce.planos.p.ID); err != nil {
		t.Fatalf("ApagarRegistros: %v", err)
	}

	// Prévia: diz o que entraria e não grava nada.
	prev, err := svc.ImportarCSV(ctx, ce.usuario, ce.slug, ImportarPlanilhaCommand{
		CSV: string(csv),
	})
	if err != nil {
		t.Fatalf("prévia: %v", err)
	}

	if len(prev.Aplicadas) != 2 || len(prev.Recusadas) != 0 {
		t.Fatalf("prévia: aplicadas=%d recusadas=%v", len(prev.Aplicadas), prev.Recusadas)
	}

	if prev.Gravadas != 0 || len(ce.cronograma.registros) != 0 {
		t.Fatal("a prévia gravou registro")
	}

	res, err := svc.ImportarCSV(ctx, ce.usuario, ce.slug, ImportarPlanilhaCommand{
		CSV: string(csv), Confirmar: true,
	})
	if err != nil {
		t.Fatalf("importar: %v", err)
	}

	if res.Gravadas != 2 {
		t.Fatalf("gravadas = %d, quer 2", res.Gravadas)
	}

	volta := ce.cronograma.registros[primeira.ID]

	if volta.Horas == nil || *volta.Horas != horas {
		t.Errorf("horas na volta = %v, quer %v", volta.Horas, horas)
	}

	if volta.Questoes == nil || *volta.Questoes != questoes {
		t.Errorf("questões na volta = %v, quer %d", volta.Questoes, questoes)
	}

	if volta.Acertos == nil || *volta.Acertos != acertos {
		t.Errorf("acertos na volta = %v, quer %d", volta.Acertos, acertos)
	}

	if !volta.Concluido {
		t.Error("a conclusão não voltou")
	}
}

// A anotação da atividade não vai na planilha. Uma importação não pode apagar o
// que o estudante escreveu.
func TestPlanilha_ImportarPreservaAAnotacaoDaAtividade(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	p := ce.obter(t)

	alvo := diasDeEstudo(p)[0].Itens[0]
	horas := 1.0

	if _, err := NewRegistroService(ce.deps).Registrar(ctx, ce.usuario, ce.slug, RegistroCommand{
		AtividadeID: alvo.ID, Horas: &horas, Nota: "revisar crase", Concluido: true,
	}); err != nil {
		t.Fatalf("Registrar: %v", err)
	}

	svc := NewPlanilhaService(ce.deps)

	csv, err := svc.CSV(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("CSV: %v", err)
	}

	if _, err := svc.ImportarCSV(ctx, ce.usuario, ce.slug, ImportarPlanilhaCommand{
		CSV: string(csv), Confirmar: true,
	}); err != nil {
		t.Fatalf("importar: %v", err)
	}

	if nota := ce.cronograma.registros[alvo.ID].Nota; nota != "revisar crase" {
		t.Errorf("nota depois da importação = %q, quer %q", nota, "revisar crase")
	}
}

// Uma planilha de outro plano não casa com nada, e a importação diz isso em vez
// de gravar em silêncio.
func TestPlanilha_ImportarRecusaPlanilhaQueNaoCasa(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.obter(t)

	csv := "data,disciplina,minutos,questoes,acertos\n" +
		"01/01/2030,Astronomia,60,10,9\n"

	_, err := NewPlanilhaService(ce.deps).ImportarCSV(
		context.Background(), ce.usuario, ce.slug,
		ImportarPlanilhaCommand{CSV: csv, Confirmar: true},
	)

	var validacao ErrValidacao
	if !asErro(err, &validacao) {
		t.Fatalf("erro = %v, esperava uma recusa de validação", err)
	}
}

// Arquivo que não é a planilha do plano: a mensagem diz o que fazer.
func TestPlanilha_ImportarRecusaArquivoIlegivel(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.obter(t)

	_, err := NewPlanilhaService(ce.deps).ImportarCSV(
		context.Background(), ce.usuario, ce.slug,
		ImportarPlanilhaCommand{CSV: "isto não é uma planilha\n"},
	)

	var validacao ErrValidacao
	if !asErro(err, &validacao) {
		t.Fatalf("erro = %v, esperava uma recusa de validação", err)
	}

	if !strings.Contains(validacao.Msg, "colunas") {
		t.Errorf("mensagem = %q, devia dizer o que falta na planilha", validacao.Msg)
	}
}
