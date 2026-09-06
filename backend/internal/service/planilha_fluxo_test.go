package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"studygo/internal/domain/plano"
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

	if !strings.Contains(validacao.Msg, "exporte o CSV do plano") {
		t.Errorf("mensagem = %q, devia dizer o que fazer em seguida", validacao.Msg)
	}
}

// O caso que motivou a importação: a planilha vem de OUTRA instalação, e os
// dias dela não têm no cronograma daqui a mesma matéria — ou não têm nada,
// porque a varredura de atraso esvaziou o passado não registrado.
//
// A planilha é a fonte do que aconteceu: dentro da janela do plano, a atividade
// é reconstruída para o registro ter onde se apoiar.
func TestPlanilha_ImportarReconstroiODiaQueFaltava(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	perdido := estudo[2]

	// O dia fica vazio, como depois de um AbsorverAtraso.
	sobrando := []plano.Atividade{}

	for _, a := range ce.cronograma.atividades {
		if !plano.DayOf(a.Data).Equal(dataDe(t, perdido.Data)) {
			sobrando = append(sobrando, a)
		}
	}

	if err := ce.cronograma.SubstituirAtividades(ctx, ce.planos.p.ID, sobrando); err != nil {
		t.Fatalf("esvaziando o dia: %v", err)
	}

	// E o tempo passa: aquele dia agora é passado.
	ce.deps.Relogio = relogioFixo{t: diaT(2026, time.September, 30)}

	data := dataDe(t, perdido.Data).Format("02/01/2006")
	csv := "data,codigo,disciplina,tema,minutos,questoes,acertos,concluido\n" +
		data + ",LINPO,Língua Portuguesa,Crase,45,10,8,sim\n" +
		data + ",BANDA,Banco de Dados,SQL,60,20,15,sim\n"

	svc := NewPlanilhaService(ce.deps)

	prev, err := svc.ImportarCSV(ctx, ce.usuario, ce.slug, ImportarPlanilhaCommand{CSV: csv})
	if err != nil {
		t.Fatalf("prévia: %v", err)
	}

	if len(prev.Aplicadas) != 2 || len(prev.Recusadas) != 0 {
		t.Fatalf("prévia: aplicadas=%d recusadas=%+v", len(prev.Aplicadas), prev.Recusadas)
	}

	if prev.Criadas != 2 {
		t.Errorf("criadas = %d, quer 2", prev.Criadas)
	}

	res, err := svc.ImportarCSV(ctx, ce.usuario, ce.slug, ImportarPlanilhaCommand{
		CSV: csv, Confirmar: true,
	})
	if err != nil {
		t.Fatalf("importar: %v", err)
	}

	if res.Gravadas != 2 {
		t.Fatalf("gravadas = %d, quer 2", res.Gravadas)
	}

	// As atividades voltaram ao dia, com o estudo lançado nelas.
	doDia := plano.AtividadesDoDia(ce.cronograma.atividades, dataDe(t, perdido.Data))
	if len(doDia) != 2 {
		t.Fatalf("o dia ficou com %d atividades, quer 2", len(doDia))
	}

	for _, a := range doDia {
		if !ce.cronograma.registros[a.ID].Concluido {
			t.Errorf("a atividade %s de %s ficou sem registro", a.Disciplina, data)
		}
	}
}

// Antes do início do plano não há cronograma para receber nada, e a mensagem
// diz o que fazer em vez de só recusar.
func TestPlanilha_ImportarRecusaAntesDoInicioDoPlano(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ce.obter(t)

	csv := "data,codigo,disciplina,tema,minutos,concluido\n" +
		"20/08/2026,LINPO,Língua Portuguesa,Crase,45,sim\n"

	res, err := NewPlanilhaService(ce.deps).ImportarCSV(
		context.Background(), ce.usuario, ce.slug, ImportarPlanilhaCommand{CSV: csv},
	)
	if err != nil {
		t.Fatalf("prévia: %v", err)
	}

	if len(res.Recusadas) != 1 {
		t.Fatalf("recusadas = %d, quer 1", len(res.Recusadas))
	}

	if !strings.Contains(res.Recusadas[0].Motivo, "início do plano") {
		t.Errorf("motivo = %q, devia mandar ajustar o início do plano", res.Recusadas[0].Motivo)
	}
}

// dataDe converte a data ISO que a tela usa para o time.Time do domínio.
func dataDe(t *testing.T, iso string) time.Time {
	t.Helper()

	d, err := time.Parse("2006-01-02", iso)
	if err != nil {
		t.Fatalf("data inválida %q: %v", iso, err)
	}

	return plano.DayOf(d.UTC())
}

// Mover o início do plano para TRÁS é o primeiro passo de quem recadastra um
// histórico que começa antes de onde o plano nasceu. O trecho que entra nunca
// existiu no cronograma, então ele nasce materializado: sem isso o estudante
// muda a data, vê o plano crescer e não encontra nada nos dias novos.
func TestSalvar_InicioParaTrasMaterializaOsDiasNovos(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	p := ce.obter(t)

	inicio := dataDe(t, p.Config.Inicio)

	depois, err := NewPlanoService(ce.deps).Salvar(ctx, ce.usuario, ce.slug, ConfigCommand{
		Inicio: "2026-08-10",
	})
	if err != nil {
		t.Fatalf("Salvar: %v", err)
	}

	if depois.Config.Inicio != "2026-08-10" {
		t.Fatalf("início = %s, quer 2026-08-10", depois.Config.Inicio)
	}

	novos, comAtividade := 0, 0

	for _, d := range depois.Dias {
		if !dataDe(t, d.Data).Before(inicio) {
			continue
		}

		novos++

		if len(d.Itens) > 0 {
			comAtividade++
		}
	}

	if novos == 0 {
		t.Fatal("o plano não cresceu para trás")
	}

	if comAtividade == 0 {
		t.Errorf("os %d dias novos do passado ficaram sem atividade nenhuma", novos)
	}
}

// A exceção é só para o início andando para trás. Qualquer outra mudança de
// data continua respeitando o passado — inclusive o dia perdido, que fica vazio
// porque é essa a verdade dele.
func TestSalvar_OutraMudancaDeDataNaoRessuscitaODiaPerdido(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	p := ce.obter(t)

	perdido := dataDe(t, diasDeEstudo(p)[2].Data)

	sobrando := []plano.Atividade{}

	for _, a := range ce.cronograma.atividades {
		if !plano.DayOf(a.Data).Equal(perdido) {
			sobrando = append(sobrando, a)
		}
	}

	if err := ce.cronograma.SubstituirAtividades(ctx, ce.planos.p.ID, sobrando); err != nil {
		t.Fatalf("esvaziando o dia: %v", err)
	}

	ce.deps.Relogio = relogioFixo{t: diaT(2026, time.September, 30)}

	if _, err := NewPlanoService(ce.deps).Salvar(ctx, ce.usuario, ce.slug, ConfigCommand{
		Prova: "2026-12-20",
	}); err != nil {
		t.Fatalf("Salvar: %v", err)
	}

	if doDia := plano.AtividadesDoDia(ce.cronograma.atividades, perdido); len(doDia) != 0 {
		t.Errorf("o dia perdido voltou a ter %d atividades", len(doDia))
	}
}

// O caderno de erros é a outra metade do arquivo. Sem ele o histórico volta
// como número, e o raciocínio — o motivo de cada erro — fica para trás.
func TestPlanilha_ImportarTrazOCadernoDeErros(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	p := ce.obter(t)

	// Uma anotação como a de quem estudou: matéria, tema e o que escapou.
	data := dataDe(t, diasDeEstudo(p)[0].Data)
	disciplina := ce.concursos.c.Disciplinas[0]

	if _, err := ce.caderno.CriarAnotacao(ctx, ce.planos.p.ID, plano.Anotacao{
		Data:         &data,
		DisciplinaID: &disciplina.ID,
		Tema:         "Crase",
		Texto:        "errei a crase antes de pronome",
		Origem:       plano.OrigemManual,
	}); err != nil {
		t.Fatalf("CriarAnotacao: %v", err)
	}

	svc := NewPlanilhaService(ce.deps)

	csv, err := svc.CSV(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("CSV: %v", err)
	}

	// A instalação nova: mesmo concurso, caderno em branco.
	ce.caderno.anotacoes = nil

	res, err := svc.ImportarCSV(ctx, ce.usuario, ce.slug, ImportarPlanilhaCommand{
		CSV: string(csv), Confirmar: true,
	})
	if err != nil {
		t.Fatalf("importar: %v", err)
	}

	if res.Anotacoes != 1 {
		t.Fatalf("anotações importadas = %d, quer 1", res.Anotacoes)
	}

	if len(ce.caderno.anotacoes) != 1 {
		t.Fatalf("o caderno ficou com %d anotações", len(ce.caderno.anotacoes))
	}

	volta := ce.caderno.anotacoes[0]

	if volta.Texto != "errei a crase antes de pronome" || volta.Tema != "Crase" {
		t.Errorf("anotação na volta = %+v", volta)
	}

	if volta.DisciplinaID == nil || *volta.DisciplinaID != disciplina.ID {
		t.Error("a anotação perdeu a matéria")
	}

	// Importar o mesmo arquivo de novo não pode duplicar o caderno.
	repetida, err := svc.ImportarCSV(ctx, ce.usuario, ce.slug, ImportarPlanilhaCommand{
		CSV: string(csv), Confirmar: true,
	})
	if err != nil {
		t.Fatalf("segunda importação: %v", err)
	}

	if repetida.Anotacoes != 0 || len(ce.caderno.anotacoes) != 1 {
		t.Errorf("a segunda importação duplicou o caderno: %d anotações", len(ce.caderno.anotacoes))
	}
}

// A personalização da matéria — a tag escolhida, o link do caderno de erros do
// estudante (o do TEC) e os ajustes de estudo — também viaja na planilha.
//
// Ela morava só na instalação de origem: quem exportava, excluía o concurso e
// recadastrava perdia esse trabalho, porque o CSV levava o histórico e deixava
// as escolhas para trás.
func TestPlanilha_ImportarTrazATagEOCadernoDaMateria(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	ce.obter(t)

	link := "https://www.tecconcursos.com.br/questoes/caderno/123"
	original := ce.concursos.c.Disciplinas[0]

	// Na instalação de origem: tag própria, caderno do TEC e só questões.
	ce.concursos.c.Disciplinas[0].Codigo = "PT"
	ce.concursos.c.Disciplinas[0].CadernoURL = link

	cfg := ce.planos.p.Config
	cfg.Modos = map[string]plano.Modo{"PT": plano.ModoQuestoes}
	cfg.Reforcos = map[string]float64{"PT": 2}
	cfg.Questoes = map[string]int{"PT": 42}
	ce.planos.p.Config = cfg

	svc := NewPlanilhaService(ce.deps)

	csv, err := svc.CSV(ctx, ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("CSV: %v", err)
	}

	// A instalação nova: o concurso foi recadastrado, então a matéria voltou ao
	// código automático, sem link e sem ajustes.
	ce.concursos.c.Disciplinas[0].Codigo = original.Codigo
	ce.concursos.c.Disciplinas[0].CadernoURL = ""
	ce.planos.p.Config.Modos = map[string]plano.Modo{}
	ce.planos.p.Config.Reforcos = map[string]float64{}
	ce.planos.p.Config.Questoes = map[string]int{original.Codigo: 15}

	res, err := svc.ImportarCSV(ctx, ce.usuario, ce.slug, ImportarPlanilhaCommand{
		CSV: string(csv), Confirmar: true,
	})
	if err != nil {
		t.Fatalf("importar: %v", err)
	}

	if res.Materias == 0 {
		t.Fatal("a personalização da matéria não foi reconhecida na planilha")
	}

	volta := ce.concursos.c.Disciplinas[0]

	if volta.CadernoURL != link {
		t.Errorf("caderno na volta = %q, quer %q", volta.CadernoURL, link)
	}

	if volta.Codigo != "PT" {
		t.Errorf("tag na volta = %q, quer PT", volta.Codigo)
	}

	if volta.ID != original.ID {
		t.Error("a matéria trocou de identidade")
	}

	if got := ce.planos.p.Config.ModoDe("PT"); got != plano.ModoQuestoes {
		t.Errorf("modo na volta = %q, quer %q", got, plano.ModoQuestoes)
	}

	if got := ce.planos.p.Config.ReforcoDe("PT"); got != 2 {
		t.Errorf("reforço na volta = %v, quer 2", got)
	}

	if got := ce.planos.p.Config.Questoes["PT"]; got != 42 {
		t.Errorf("questões na volta = %d, quer 42", got)
	}
}

// Depois de importar, o passado que ficou sem estudo é atraso: o dia perdido
// esvazia e o conteúdo dele se redistribui pelos dias que restam — a mesma
// redistribuição da varredura diária, feita na hora em vez de na virada do dia.
func TestPlanilha_ImportarRedistribuiODiaQueNaoFoiEstudado(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	p := ce.obter(t)

	estudo := diasDeEstudo(p)
	estudado, perdido := estudo[0], estudo[1]

	// O tempo passa: os dois primeiros dias já são passado.
	ce.deps.Relogio = relogioFixo{t: diaT(2026, time.September, 10)}

	// A planilha traz só o primeiro dia. O segundo ninguém estudou.
	data := dataDe(t, estudado.Data).Format("02/01/2006")
	csv := "data,codigo,disciplina,tema,minutos,questoes,acertos,concluido\n"

	for _, it := range estudado.Itens {
		csv += data + "," + it.Disciplina + ",," + it.Tema + ",60,10,8,sim\n"
	}

	res, err := NewPlanilhaService(ce.deps).ImportarCSV(
		ctx, ce.usuario, ce.slug, ImportarPlanilhaCommand{CSV: csv, Confirmar: true},
	)
	if err != nil {
		t.Fatalf("importar: %v", err)
	}

	if res.DiasVagos == 0 {
		t.Fatal("o dia não estudado devia ter sido redistribuído")
	}

	// O dia perdido ficou vago; o que era dele foi para a frente.
	if doDia := plano.AtividadesDoDia(ce.cronograma.atividades, dataDe(t, perdido.Data)); len(doDia) != 0 {
		t.Errorf("o dia perdido continuou com %d atividades", len(doDia))
	}

	// E o que foi estudado continua onde estava, com o registro dele.
	doDia := plano.AtividadesDoDia(ce.cronograma.atividades, dataDe(t, estudado.Data))
	if len(doDia) != len(estudado.Itens) {
		t.Fatalf("o dia estudado ficou com %d atividades, quer %d", len(doDia), len(estudado.Itens))
	}

	for _, a := range doDia {
		if !ce.cronograma.registros[a.ID].Concluido {
			t.Error("um registro importado se perdeu na redistribuição")
		}
	}
}
