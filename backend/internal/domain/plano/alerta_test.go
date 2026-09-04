package plano_test

import (
	"testing"

	"studygo/internal/domain/plano"
)

// Uma matéria que o plano não percorre nenhuma vez tem que avisar — inclusive
// quando ela não tem temas cadastrados.
//
// Um concurso importado do edital pode chegar só com os nomes das disciplinas.
// Se a prova está perto demais e o plano vira reta final inteira, nenhuma delas
// é ensinada; calar nesse caso esconde exatamente o buraco que o aviso existe
// para mostrar.
func TestCoberturaDoPlano_AvisaMateriaSemTemaQueNaoEntra(t *testing.T) {
	t.Parallel()

	linhas := []plano.LinhaCobertura{
		{Codigo: "LINPO", Nome: "Língua Portuguesa", Temas: 0, Passadas: 0},
		{Codigo: "BANDA", Nome: "Banco de Dados", Temas: 0, Passadas: 0},
	}

	a := plano.CoberturaDoPlano(linhas)
	if a == nil {
		t.Fatal("um plano que não ensina matéria nenhuma tem que avisar")
	}

	if a.SemNenhuma != 2 {
		t.Errorf("SemNenhuma = %d, quer 2", a.SemNenhuma)
	}

	if a.Severidade != plano.SeveridadePerigo {
		t.Errorf("severidade = %q, quer perigo", a.Severidade)
	}
}

// Uma matéria sem temas que o plano PERCORRE está coberta: o motor usa o nome
// dela como manchete, e uma passada é uma passada.
func TestCoberturaDoPlano_MateriaSemTemaQueEntraNaoAvisa(t *testing.T) {
	t.Parallel()

	linhas := []plano.LinhaCobertura{
		{Codigo: "LINPO", Nome: "Língua Portuguesa", Temas: 0, Passadas: 3},
	}

	if a := plano.CoberturaDoPlano(linhas); a != nil {
		t.Errorf("não devia avisar sobre matéria coberta: %+v", a.Incompletas)
	}
}
