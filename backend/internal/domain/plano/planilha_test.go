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

// diasDoPlano é um setembro de dias de estudo, de segunda a sexta: é o que
// decide onde a planilha pode reconstruir uma atividade.
func diasDoPlano() []plano.Dia {
	dias := []plano.Dia{}

	for d := 1; d <= 30; d++ {
		data := dia(2026, time.September, d)
		if wd := data.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}

		dias = append(dias, plano.Dia{
			N: len(dias) + 1, Data: data, Semana: (d-1)/7 + 1,
			Fase: plano.FaseBase, Tipo: plano.TipoEstudo,
		})
	}

	return dias
}

// A janela dos testes: o plano começa em 01/09 e hoje é 30/09, então setembro
// inteiro é passado e pode ser reconstruído.
func janela() plano.JanelaDaPlanilha {
	return plano.JanelaDaPlanilha{
		Inicio: dia(2026, time.September, 1),
		Hoje:   dia(2026, time.September, 30),
	}
}

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

	res := plano.CasarPlanilha([]plano.Atividade{simulado}, diasDoPlano(), linhas, cur, janela())

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

	res := plano.CasarPlanilha(atividades, diasDoPlano(), linhas, cur, janela())

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

	res := plano.CasarPlanilha(atividades, diasDoPlano(), linhas, cur, janela())

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

// Uma planilha de OUTRA instalação traz dias que este cronograma não tem: foi
// outro plano que gerou aqueles dias. Dentro da janela — do início do plano até
// hoje — a atividade é reconstruída, porque o registro precisa de uma atividade
// para existir e a planilha é a fonte do que aconteceu.
func TestCasarPlanilha_ReconstroiODiaQueFaltava(t *testing.T) {
	t.Parallel()

	cur := cursoDeTeste()
	d1 := dia(2026, time.September, 1)
	atividades := []plano.Atividade{atividade(cur, 0, d1, 0, "Crase")}

	csv := cabecalho +
		// matéria que não está agendada naquele dia
		"1,01/09/2026,1,Conteúdo,est,BANDA,Banco de Dados,SQL,20,30,10,9,sim,\n" +
		// dia que o plano não tem atividade nenhuma
		"2,03/09/2026,1,Conteúdo,est,LINPO,Língua Portuguesa,Regência,20,45,,,sim,\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	res := plano.CasarPlanilha(atividades, diasDoPlano(), linhas, cur, janela())

	if len(res.Casadas) != 2 || len(res.Recusadas) != 0 {
		t.Fatalf("casadas=%d recusadas=%+v", len(res.Casadas), res.Recusadas)
	}

	if len(res.Novas) != 2 {
		t.Fatalf("novas = %d, quer 2", len(res.Novas))
	}

	for _, c := range res.Casadas {
		if !c.Criada {
			t.Errorf("linha %d devia estar marcada como criada", c.Linha.Numero)
		}
	}

	// A que entra no dia já ocupado vai para a vaga seguinte, não por cima.
	nova := res.Novas[0]
	if !nova.Data.Equal(d1) || nova.Posicao != 1 || nova.Disciplina != "BANDA" {
		t.Errorf("atividade reconstruída = %+v", nova)
	}

	if nova.Tema != "SQL" || nova.DisciplinaID == nil || *nova.DisciplinaID != cur.Disciplinas[1].ID {
		t.Errorf("a atividade reconstruída não descreve a linha: %+v", nova)
	}

	if !nova.Movida {
		t.Error("quem pôs a atividade ali foi o estudante — Movida devia estar ligada")
	}
}

// Fora da janela ninguém reconstrói nada, e o motivo diz o que fazer.
func TestCasarPlanilha_RecusaForaDaJanela(t *testing.T) {
	t.Parallel()

	cur := cursoDeTeste()
	atividades := []plano.Atividade{}

	csv := cabecalho +
		// antes do início do plano
		"1,20/08/2026,1,Conteúdo,est,LINPO,Língua Portuguesa,Crase,20,30,,,sim,\n" +
		// depois de hoje
		"2,20/12/2026,1,Conteúdo,est,LINPO,Língua Portuguesa,Crase,20,30,,,sim,\n" +
		// matéria que não existe neste concurso
		"3,03/09/2026,1,Conteúdo,est,,Astronomia,Estrelas,20,30,,,sim,\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	res := plano.CasarPlanilha(atividades, diasDoPlano(), linhas, cur, janela())

	if len(res.Casadas) != 0 || len(res.Novas) != 0 {
		t.Fatalf("nada devia entrar: casadas=%d novas=%d", len(res.Casadas), len(res.Novas))
	}

	if len(res.Recusadas) != 3 {
		t.Fatalf("recusadas = %d, quer 3", len(res.Recusadas))
	}

	if !strings.Contains(res.Recusadas[0].Motivo, "início do plano") {
		t.Errorf("motivo do dia anterior ao plano = %q", res.Recusadas[0].Motivo)
	}

	if !strings.Contains(res.Recusadas[1].Motivo, "ainda não chegou") {
		t.Errorf("motivo do dia futuro = %q", res.Recusadas[1].Motivo)
	}

	if !strings.Contains(res.Recusadas[2].Motivo, "matéria deste concurso") {
		t.Errorf("motivo da matéria desconhecida = %q", res.Recusadas[2].Motivo)
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

	res := plano.CasarPlanilha(atividades, diasDoPlano(), linhas, cur, janela())

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

// Duas instalações escrevem o mesmo nome de matéria de formas que não são o
// mesmo texto. Comparar byte a byte recusava a planilha inteira com a mensagem
// mais confusa possível — "Língua Portuguesa não é uma matéria deste concurso"
// para uma matéria que está na tela.
func TestCasarPlanilha_NomeDaMateriaComparadoSemAcentoNemPontuacao(t *testing.T) {
	t.Parallel()

	cur := cursoDeTeste()
	d1 := dia(2026, time.September, 1)

	nomes := []struct {
		caso string
		nome string
	}{
		// Acento decomposto: "i" seguido do acento agudo combinante, que é como
		// alguns sistemas gravam o arquivo.
		// "Li" + acento agudo combinante: como alguns sistemas gravam o arquivo.
		{"acento decomposto", "Li\u0301ngua Portuguesa"},
		{"sem acento", "Lingua Portuguesa"},
		{"caixa diferente", "LÍNGUA PORTUGUESA"},
		{"espaço a mais", "Língua  Portuguesa "},
	}

	for _, tt := range nomes {
		t.Run(tt.caso, func(t *testing.T) {
			t.Parallel()

			csv := "data,disciplina,tema,minutos,concluido\n" +
				"01/09/2026," + tt.nome + ",Crase,45,sim\n"

			linhas, err := plano.LerPlanilha(strings.NewReader(csv))
			if err != nil {
				t.Fatalf("LerPlanilha: %v", err)
			}

			atividades := []plano.Atividade{atividade(cur, 0, d1, 0, "Crase")}

			res := plano.CasarPlanilha(atividades, diasDoPlano(), linhas, cur, janela())

			if len(res.Casadas) != 1 {
				t.Fatalf("casadas = %d (recusadas: %+v), quer 1", len(res.Casadas), res.Recusadas)
			}

			if res.Casadas[0].Registro.AtividadeID != atividades[0].ID {
				t.Error("a linha não foi para a atividade daquela matéria")
			}
		})
	}
}

// O caso real: a planilha traz a TAG que o estudante escolheu na instalação de
// origem ("PT"), e aqui o concurso foi recadastrado — os códigos são outros
// ("LINPO"). O nome é o que atravessa entre as duas.
//
// Sem a segunda tentativa, a importação recusava a planilha inteira dizendo que
// "Língua Portuguesa não é uma matéria deste concurso" para uma matéria que
// está na tela.
func TestCasarPlanilha_TagDeOutraInstalacaoCaiNoNome(t *testing.T) {
	t.Parallel()

	cur := cursoDeTeste() // códigos LINPO e BANDA
	d1 := dia(2026, time.September, 1)
	atividades := []plano.Atividade{atividade(cur, 0, d1, 0, "Crase")}

	csv := cabecalho +
		"1,01/09/2026,1,Conteúdo,est,PT,Língua Portuguesa,Crase,20,30,10,8,sim,\n" +
		"1,01/09/2026,1,Conteúdo,est,BD,Banco de Dados,SQL,20,45,20,15,sim,\n"

	linhas, err := plano.LerPlanilha(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	res := plano.CasarPlanilha(atividades, diasDoPlano(), linhas, cur, janela())

	if len(res.Casadas) != 2 || len(res.Recusadas) != 0 {
		t.Fatalf("casadas=%d recusadas=%+v", len(res.Casadas), res.Recusadas)
	}

	// A primeira achou a atividade que já existia; a segunda reconstruiu a dela.
	if res.Casadas[0].Registro.AtividadeID != atividades[0].ID {
		t.Error("a linha de Língua Portuguesa não foi para a atividade do dia")
	}

	if !res.Casadas[1].Criada || len(res.Novas) != 1 {
		t.Fatalf("a linha de Banco de Dados devia reconstruir a atividade: %+v", res.Casadas[1])
	}

	if res.Novas[0].Disciplina != "BANDA" {
		t.Errorf("a atividade nova ficou com o código %q, quer BANDA", res.Novas[0].Disciplina)
	}
}
