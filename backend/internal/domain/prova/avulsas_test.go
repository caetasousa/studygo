package prova

import (
	"slices"
	"testing"
)

func materias(qs []QuestaoAvulsa) []string {
	out := make([]string, 0, len(qs))
	for _, q := range qs {
		out = append(out, q.Disciplina)
	}

	return out
}

func TestAvulsas_GrafiasDaMesmaMateriaViramUmNome(t *testing.T) {
	t.Parallel()

	qs := []QuestaoAvulsa{
		{TemGabarito: true, Numero: 1, Disciplina: "Noções Sobre Direitos das Pessoas com Deficiência"},
		{TemGabarito: true, Numero: 2, Disciplina: "Noções sobre Direitos das Pessoas com Deficiência"},
		{TemGabarito: true, Numero: 3, Disciplina: "Noções Sobre  Direitos das Pessoas com Deficiência "},
		{TemGabarito: true, Numero: 4, Disciplina: "Redes"},
	}

	got := materias(Avulsas(qs, FiltroDeAvulsas{}))
	want := []string{
		"Noções Sobre Direitos das Pessoas com Deficiência",
		"Noções Sobre Direitos das Pessoas com Deficiência",
		"Noções Sobre Direitos das Pessoas com Deficiência",
		"Redes",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("matérias = %q, quer %q", got, want)
	}
}

// Sem uma grafia mais usada, o nome não pode depender da ordem das questões.
func TestAvulsas_EmpateFicaComAPrimeiraEmOrdemAlfabetica(t *testing.T) {
	t.Parallel()

	for _, qs := range [][]QuestaoAvulsa{
		{{TemGabarito: true, Disciplina: "Língua portuguesa"}, {TemGabarito: true, Disciplina: "Língua Portuguesa"}},
		{{TemGabarito: true, Disciplina: "Língua Portuguesa"}, {TemGabarito: true, Disciplina: "Língua portuguesa"}},
	} {
		if got := materias(Avulsas(qs, FiltroDeAvulsas{})); got[0] != "Língua Portuguesa" || got[1] != got[0] {
			t.Fatalf("matérias = %q, quer as duas como Língua Portuguesa", got)
		}
	}
}

// O nome vem do catálogo inteiro: filtrar pelo ano em que só existe a grafia
// minoritária não troca o nome da matéria.
func TestAvulsas_NomeNaoMudaComOFiltro(t *testing.T) {
	t.Parallel()

	qs := []QuestaoAvulsa{
		{TemGabarito: true, Numero: 1, Ano: 2026, Disciplina: "Governança de TI"},
		{TemGabarito: true, Numero: 2, Ano: 2026, Disciplina: "Governança de TI"},
		{TemGabarito: true, Numero: 3, Ano: 2025, Disciplina: "governança de ti"},
	}

	got := Avulsas(qs, FiltroDeAvulsas{Ano: 2025})
	if len(got) != 1 || got[0].Numero != 3 || got[0].Disciplina != "Governança de TI" {
		t.Fatalf("avulsas = %+v", got)
	}
}

// O assunto vem digitado pelo curador; espaço a mais não pode separar duas
// questões do mesmo assunto no filtro.
func TestAvulsas_AssuntoSemEspacosAMais(t *testing.T) {
	t.Parallel()

	qs := []QuestaoAvulsa{
		{TemGabarito: true, Numero: 1, Disciplina: "Língua Portuguesa", Assunto: " Crase"},
		{TemGabarito: true, Numero: 2, Disciplina: "Língua Portuguesa", Assunto: "Concordância  nominal e verbal "},
	}

	got := Avulsas(qs, FiltroDeAvulsas{})
	if got[0].Assunto != "Crase" || got[1].Assunto != "Concordância nominal e verbal" {
		t.Fatalf("assuntos = %q, %q", got[0].Assunto, got[1].Assunto)
	}
}

func TestAvulsas_FiltraMateriasEAno(t *testing.T) {
	t.Parallel()

	qs := []QuestaoAvulsa{
		{TemGabarito: true, Numero: 1, Ano: 2026, Disciplina: "Redes"},
		{TemGabarito: true, Numero: 2, Ano: 2026, Disciplina: "Banco de Dados"},
		{TemGabarito: true, Numero: 3, Ano: 2025, Disciplina: "Redes"},
		{TemGabarito: true, Numero: 4, Ano: 2026, Disciplina: "Língua Portuguesa"},
		{TemGabarito: true, Numero: 5, Ano: 2026, Disciplina: ""},
	}
	numeros := func(f FiltroDeAvulsas) []int {
		var out []int
		for _, q := range Avulsas(qs, f) {
			out = append(out, q.Numero)
		}

		return out
	}

	casos := []struct {
		nome string
		f    FiltroDeAvulsas
		want []int
	}{
		// Questão sem matéria não entra: o treino é por matéria.
		{"sem filtro", FiltroDeAvulsas{}, []int{1, 2, 3, 4}},
		{"uma matéria, qualquer grafia", FiltroDeAvulsas{Materias: []string{" redes"}}, []int{1, 3}},
		{"duas matérias", FiltroDeAvulsas{Materias: []string{"Redes", "Língua Portuguesa"}}, []int{1, 3, 4}},
		{"matéria e ano", FiltroDeAvulsas{Materias: []string{"Redes"}, Ano: 2026}, []int{1}},
		{"só o ano", FiltroDeAvulsas{Ano: 2025}, []int{3}},
		{"matéria em branco não filtra", FiltroDeAvulsas{Materias: []string{""}}, []int{1, 2, 3, 4}},
		{"matéria que não existe", FiltroDeAvulsas{Materias: []string{"Contabilidade"}}, nil},
	}
	for _, c := range casos {
		if got := numeros(c.f); !slices.Equal(got, c.want) {
			t.Errorf("%s: questões %v, quer %v", c.nome, got, c.want)
		}
	}
}

func TestGrupoDaMateria(t *testing.T) {
	t.Parallel()

	for materia, grupo := range map[string]string{
		"Língua Portuguesa":                          GrupoBasicas,
		"Matemática e Raciocínio Lógico":             GrupoBasicas,
		"Raciocínio Lógico-Matemático":               GrupoBasicas,
		"Legislação Institucional":                   GrupoLegislacao,
		"Legislação Aplicada à TI":                   GrupoLegislacao,
		"Noções de Direito Administrativo":           GrupoLegislacao,
		"Direitos das Pessoas com Deficiência":       GrupoLegislacao,
		"Direitos Humanos":                           GrupoLegislacao,
		"Administração Pública":                      GrupoLegislacao,
		"Sustentabilidade":                           GrupoLegislacao,
		"Sistemas Operacionais, Redes e Nuvem":       GrupoEspecificas,
		"Engenharia de Software":                     GrupoEspecificas,
		"Inteligência Artificial e Ciência de Dados": GrupoEspecificas,
		"Governança de TI":                           GrupoEspecificas,
		"Noções de Informática":                      GrupoBasicas,
		"Informática Básica":                         GrupoBasicas,
		"Pacote Office (Word e Excel)":               GrupoBasicas,
		"Informática":                                GrupoEspecificas,
		"Lógica de Programação":                      GrupoEspecificas,
	} {
		if got := GrupoDaMateria(materia); got != grupo {
			t.Errorf("GrupoDaMateria(%q) = %q, quer %q", materia, got, grupo)
		}
	}
}

// Questão sem linha no gabarito não vai ao treino.
func TestAvulsas_SemGabaritoFicaDeFora(t *testing.T) {
	t.Parallel()

	qs := []QuestaoAvulsa{
		{Numero: 1, Disciplina: "Redes", TemGabarito: true, Resposta: "A"},
		{Numero: 2, Disciplina: "Redes", TemGabarito: true},
		{Numero: 3, Disciplina: "Redes"},
	}

	got := Avulsas(qs, FiltroDeAvulsas{})

	// A 2 é anulada: tem gabarito, sem letra.
	if len(got) != 2 || got[0].Numero != 1 || got[1].Numero != 2 {
		t.Fatalf("avulsas = %+v", got)
	}
}
