package plano_test

import (
	"strings"
	"testing"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// A planilha do plano de volta para dentro: o que o CSV traz, o que casa com o
// cronograma e o que é recusado com motivo.

func cursoDeTeste() concurso.Concurso {
	return concurso.Concurso{
		Disciplinas: []concurso.Disciplina{
			{ID: uuid.New(), Codigo: "LINPO", Nome: "Língua Portuguesa"},
			{ID: uuid.New(), Codigo: "BANDA", Nome: "Banco de Dados"},
		},
	}
}

func atividade(cur concurso.Concurso, i int, data time.Time, pos int, tema string) plano.Atividade {
	id := cur.Disciplinas[i].ID

	return plano.Atividade{
		ID: uuid.New(), Data: data, Posicao: pos,
		DisciplinaID: &id, Disciplina: cur.Disciplinas[i].Codigo,
		Tema: tema, Passada: 1, Tipo: plano.AtividadeConteudo,
	}
}

const cabecalho = "dia,data,semana,fase,tipo,codigo,disciplina,tema,meta_questoes,minutos,questoes,acertos,concluido,anotacao\n"

func TestLerPlanilha_LeAsColunasDoExport(t *testing.T) {
	t.Parallel()

	// Com o BOM que o Excel espera, como o export escreve.
	csv := "\ufeff" + cabecalho +
		"1,01/09/2026,1,Conteúdo,est,LINPO,Língua Portuguesa,Crase,20,45,10,8,sim,\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	if len(linhas) != 1 {
		t.Fatalf("li %d linhas, quer 1", len(linhas))
	}

	l := linhas[0]

	if !l.Data.Equal(dia(2026, time.September, 1)) {
		t.Errorf("data = %v", l.Data)
	}

	if l.Codigo != "LINPO" || l.Tema != "Crase" {
		t.Errorf("matéria/tema = %q/%q", l.Codigo, l.Tema)
	}

	if l.Minutos == nil || *l.Minutos != 45 {
		t.Errorf("minutos = %v, quer 45", l.Minutos)
	}

	if l.Questoes == nil || *l.Questoes != 10 || l.Acertos == nil || *l.Acertos != 8 {
		t.Errorf("questões/acertos = %v/%v", l.Questoes, l.Acertos)
	}

	if l.Concluido == nil || !*l.Concluido {
		t.Errorf("concluído = %v, quer true", l.Concluido)
	}

	// A linha do arquivo é o que a tela aponta quando algo dá errado.
	if l.Numero != 2 {
		t.Errorf("número da linha = %d, quer 2", l.Numero)
	}
}

// As planilhas antigas trazem horas com vírgula decimal; o tempo entra na mesma
// unidade que a tela usa.
func TestLerPlanilha_AceitaHorasComoVeioAntes(t *testing.T) {
	t.Parallel()

	csv := "data;disciplina;horas;questoes;acertos\n" +
		"01/09/2026;Língua Portuguesa;1,5;10;8\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	if linhas[0].Minutos == nil || *linhas[0].Minutos != 90 {
		t.Errorf("minutos = %v, quer 90", linhas[0].Minutos)
	}
}

// O export tem duas tabelas separadas por uma linha em branco. A segunda é o
// caderno de erros, e a importação não a toca.
func TestLerPlanilha_ParaNoCaderno(t *testing.T) {
	t.Parallel()

	csv := cabecalho +
		"1,01/09/2026,1,Conteúdo,est,LINPO,Língua Portuguesa,Crase,20,45,10,8,sim,\n" +
		"\n" +
		"caderno_data,caderno_disciplina,caderno_tema,caderno_texto,caderno_origem,caderno_resolvido\n" +
		"02/09/2026,Língua Portuguesa,Crase,errei feio,manual,não\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	if len(linhas) != 1 {
		t.Fatalf("li %d linhas, quer 1 — o caderno entrou junto", len(linhas))
	}
}

// Linha de cronograma sem nada lançado não é registro: gravá-la apagaria o que
// já existe na atividade.
//
// O "não" da coluna de conclusão é o caso perigoso: o export o escreve em TODA
// linha ainda não estudada, e tomá-lo por lançamento faria a importação apagar
// o histórico do outro lado.
func TestLerPlanilha_IgnoraLinhaSemLancamento(t *testing.T) {
	t.Parallel()

	csv := cabecalho +
		"1,01/09/2026,1,Conteúdo,est,LINPO,Língua Portuguesa,Crase,20,,,,não,\n" +
		"2,02/09/2026,1,Conteúdo,est,BANDA,Banco de Dados,SQL,20,30,,,não,\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	if len(linhas) != 1 || linhas[0].Codigo != "BANDA" {
		t.Fatalf("li %+v, quer só a linha do BANDA", linhas)
	}
}

// Um dia fixo não tem matéria: a planilha traz o tipo no lugar dela, e o
// simulado que foi feito também volta.
func TestCasarPlanilha_CasaODiaFixoPeloTipo(t *testing.T) {
	t.Parallel()

	cur := cursoDeTeste()
	d1 := dia(2026, time.September, 1)
	simulado := plano.Atividade{
		ID: uuid.New(), Data: d1, Posicao: 0,
		Tema: "Simulado completo", Passada: 1, Tipo: plano.AtividadeSimulado,
	}

	csv := cabecalho +
		"1,01/09/2026,1,Reta final,sim,,simulado,Simulado completo,120,240,120,90,sim,\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	res := plano.CasarPlanilha([]plano.Atividade{simulado}, linhas, cur)

	if len(res.Casadas) != 1 {
		t.Fatalf("casadas = %d (recusadas: %+v), quer 1", len(res.Casadas), res.Recusadas)
	}

	if res.Casadas[0].Registro.AtividadeID != simulado.ID {
		t.Error("a linha do simulado não foi para o simulado")
	}
}

func TestLerPlanilha_RecusaArquivoSemCabecalho(t *testing.T) {
	t.Parallel()

	if _, err := plano.LerPlanilha(strings.NewReader("a,b\n1,2\n")); err == nil {
		t.Fatal("planilha sem data/disciplina devia ser recusada")
	}
}

func TestCasarPlanilha_CasaPorDiaEMateria(t *testing.T) {
	t.Parallel()

	cur := cursoDeTeste()
	d1 := dia(2026, time.September, 1)
	atividades := []plano.Atividade{
		atividade(cur, 0, d1, 0, "Crase"),
		atividade(cur, 1, d1, 1, "SQL"),
	}

	csv := cabecalho +
		"1,01/09/2026,1,Conteúdo,est,BANDA,Banco de Dados,SQL,20,60,20,15,sim,\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	res := plano.CasarPlanilha(atividades, linhas, cur)

	if len(res.Casadas) != 1 || len(res.Recusadas) != 0 {
		t.Fatalf("casadas=%d recusadas=%d", len(res.Casadas), len(res.Recusadas))
	}

	reg := res.Casadas[0].Registro
	if reg.AtividadeID != atividades[1].ID {
		t.Error("a linha entrou na atividade errada")
	}

	if reg.Horas == nil || *reg.Horas != 1 {
		t.Errorf("horas = %v, quer 1 (60 minutos)", reg.Horas)
	}

	if !reg.Concluido {
		t.Error("a linha dizia concluído")
	}
}

// Um dia que agenda a mesma matéria duas vezes tem duas linhas, e o tema diz
// qual é qual.
func TestCasarPlanilha_TemaDesempataOcorrencias(t *testing.T) {
	t.Parallel()

	cur := cursoDeTeste()
	d1 := dia(2026, time.September, 1)
	atividades := []plano.Atividade{
		atividade(cur, 1, d1, 0, "Modelagem"),
		atividade(cur, 1, d1, 1, "SQL"),
	}

	csv := cabecalho +
		"1,01/09/2026,1,Conteúdo,est,BANDA,Banco de Dados,SQL,20,30,10,9,sim,\n" +
		"1,01/09/2026,1,Conteúdo,est,BANDA,Banco de Dados,Modelagem,20,40,5,5,sim,\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	res := plano.CasarPlanilha(atividades, linhas, cur)

	if len(res.Casadas) != 2 {
		t.Fatalf("casadas = %d, quer 2", len(res.Casadas))
	}

	if res.Casadas[0].Registro.AtividadeID != atividades[1].ID {
		t.Error("a linha de SQL não foi para a atividade de SQL")
	}

	if res.Casadas[1].Registro.AtividadeID != atividades[0].ID {
		t.Error("a linha de Modelagem não foi para a atividade de Modelagem")
	}
}

func TestCasarPlanilha_RecusaComMotivo(t *testing.T) {
	t.Parallel()

	cur := cursoDeTeste()
	d1 := dia(2026, time.September, 1)
	atividades := []plano.Atividade{atividade(cur, 0, d1, 0, "Crase")}

	csv := cabecalho +
		// dia que o plano não tem
		"1,20/12/2026,1,Conteúdo,est,LINPO,Língua Portuguesa,Crase,20,30,,,sim,\n" +
		// matéria que não está naquele dia
		"1,01/09/2026,1,Conteúdo,est,BANDA,Banco de Dados,SQL,20,30,,,sim,\n" +
		// segunda linha para a mesma vaga
		"1,01/09/2026,1,Conteúdo,est,LINPO,Língua Portuguesa,Crase,20,30,,,sim,\n" +
		"1,01/09/2026,1,Conteúdo,est,LINPO,Língua Portuguesa,Crase,20,30,,,sim,\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	res := plano.CasarPlanilha(atividades, linhas, cur)

	if len(res.Casadas) != 1 {
		t.Fatalf("casadas = %d, quer 1", len(res.Casadas))
	}

	if len(res.Recusadas) != 3 {
		t.Fatalf("recusadas = %d, quer 3", len(res.Recusadas))
	}

	for _, r := range res.Recusadas {
		if r.Motivo == "" {
			t.Errorf("linha %d recusada sem motivo", r.Linha.Numero)
		}
	}
}

// Sem a coluna de conclusão, uma linha com tempo ou questões é estudo feito —
// é o que ela afirma ao existir.
func TestCasarPlanilha_SemColunaDeConclusao(t *testing.T) {
	t.Parallel()

	cur := cursoDeTeste()
	d1 := dia(2026, time.September, 1)
	atividades := []plano.Atividade{atividade(cur, 0, d1, 0, "Crase")}

	csv := "data,disciplina,minutos\n01/09/2026,Língua Portuguesa,50\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	res := plano.CasarPlanilha(atividades, linhas, cur)

	if len(res.Casadas) != 1 || !res.Casadas[0].Registro.Concluido {
		t.Fatalf("resultado = %+v, quer uma linha concluída", res)
	}
}

func TestHorasDeMinutos_VoltaComoFoiDigitado(t *testing.T) {
	t.Parallel()

	for min := 0; min <= 600; min++ {
		if got := plano.MinutosDeHoras(plano.HorasDeMinutos(min)); got != min {
			t.Fatalf("%d minutos viraram %d na volta", min, got)
		}
	}
}
