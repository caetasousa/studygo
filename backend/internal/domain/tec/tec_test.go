package tec

import (
	"errors"
	"strings"
	"testing"

	"studygo/internal/domain/concurso"
)

// O leitor da planilha do TEC não tinha teste nenhum, e é o único código do
// projeto que consome um formato de TERCEIRO: ninguém aqui controla o que o
// TEC exporta, e mudanças chegam sem aviso. O que estes testes fixam é
// tolerância — separador, acento, cabeçalho, lixo no meio — e a recusa clara
// quando de fato não dá para ler.

func planilha(linhas ...string) *strings.Reader {
	return strings.NewReader(strings.Join(linhas, "\n"))
}

func TestLerPlanilha_exportacaoBrasileira(t *testing.T) {
	t.Parallel()

	// Separador ';', que é o que o Excel em pt-BR gera.
	linhas, err := LerPlanilha(planilha(
		"Assunto;Questões;Acertos",
		"Crase;40;28",
		"Concordância verbal;25;20",
	))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	if len(linhas) != 2 {
		t.Fatalf("linhas = %d, quer 2", len(linhas))
	}

	if linhas[0] != (Linha{Assunto: "Crase", Questoes: 40, Acertos: 28}) {
		t.Errorf("primeira linha = %+v", linhas[0])
	}
}

func TestLerPlanilha_toleraOFormatoQueOTECManda(t *testing.T) {
	t.Parallel()

	casos := map[string][]string{
		"vírgula como separador": {
			"assunto,questoes,acertos",
			"Crase,40,28",
		},
		"BOM no começo do arquivo": {
			"\ufeffAssunto;Questões;Acertos",
			"Crase;40;28",
		},
		"cabeçalho em outra caixa": {
			"ASSUNTO;QUESTÕES;ACERTOS",
			"Crase;40;28",
		},
		"apelidos alternativos": {
			"Matéria;Resolvidas;Corretas",
			"Crase;40;28",
		},
		"milhar com ponto": {
			"Assunto;Questões;Acertos",
			"Crase;1.040;28",
		},
		"colunas a mais": {
			"Assunto;Questões;Acertos;Aproveitamento;Tempo",
			"Crase;40;28;70%;01:20:00",
		},
		"colunas fora de ordem": {
			"Acertos;Assunto;Questões",
			"28;Crase;40",
		},
	}

	for nome, conteudo := range casos {
		t.Run(nome, func(t *testing.T) {
			t.Parallel()

			linhas, err := LerPlanilha(planilha(conteudo...))
			if err != nil {
				t.Fatalf("LerPlanilha: %v", err)
			}

			if len(linhas) != 1 || linhas[0].Assunto != "Crase" || linhas[0].Acertos != 28 {
				t.Errorf("linhas = %+v", linhas)
			}
		})
	}
}

// O milhar com ponto vira o número inteiro, não 1.
func TestLerPlanilha_leMilharComPonto(t *testing.T) {
	t.Parallel()

	linhas, err := LerPlanilha(planilha(
		"Assunto;Questões;Acertos",
		"Crase;1.040;1.000",
	))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	if linhas[0].Questoes != 1040 || linhas[0].Acertos != 1000 {
		t.Errorf("linha = %+v, quer 1040/1000", linhas[0])
	}
}

// Linha sem assunto ou sem questão resolvida não é erro: é linha de total, de
// subtítulo ou de assunto que a pessoa nunca abriu. Some em silêncio.
func TestLerPlanilha_descartaLinhasSemConteudo(t *testing.T) {
	t.Parallel()

	linhas, err := LerPlanilha(planilha(
		"Assunto;Questões;Acertos",
		";40;28", // sem assunto
		"Crase;40;28",
		"Regência;0;0", // nunca resolvida
		"Sintaxe;;",    // colunas vazias
	))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	if len(linhas) != 1 || linhas[0].Assunto != "Crase" {
		t.Errorf("linhas = %+v, quer só a Crase", linhas)
	}
}

// Acertos maiores que questões é dado impossível: o leitor apara em vez de
// propagar um aproveitamento acima de 100%.
func TestLerPlanilha_aparaAcertosImpossiveis(t *testing.T) {
	t.Parallel()

	linhas, err := LerPlanilha(planilha(
		"Assunto;Questões;Acertos",
		"Crase;10;99",
	))
	if err != nil {
		t.Fatalf("LerPlanilha: %v", err)
	}

	if linhas[0].Acertos != 10 {
		t.Errorf("acertos = %d, quer aparado em 10", linhas[0].Acertos)
	}
}

func TestLerPlanilha_recusaOQueNaoDaParaLer(t *testing.T) {
	t.Parallel()

	casos := map[string][]string{
		"vazia":                 {""},
		"só cabeçalho":          {"Assunto;Questões;Acertos"},
		"sem coluna de acertos": {"Assunto;Questões", "Crase;40"},
		"sem cabeçalho":         {"Crase;40;28", "Regência;25;20"},
		"nenhuma linha útil":    {"Assunto;Questões;Acertos", "Total;0;0"},
	}

	for nome, conteudo := range casos {
		t.Run(nome, func(t *testing.T) {
			t.Parallel()

			_, err := LerPlanilha(planilha(conteudo...))
			if !errors.Is(err, ErrPlanilhaInvalida) {
				t.Errorf("erro = %v, quer ErrPlanilhaInvalida", err)
			}
		})
	}
}

// A mensagem precisa dizer QUAL coluna faltou: "planilha inválida" sozinho
// manda o estudante adivinhar o que exportar de novo.
func TestLerPlanilha_erroNomeiaAColunaQueFaltou(t *testing.T) {
	t.Parallel()

	_, err := LerPlanilha(planilha("Assunto;Questões", "Crase;40"))
	if err == nil {
		t.Fatal("quer erro")
	}

	if !strings.Contains(err.Error(), "acertos") {
		t.Errorf("mensagem = %q, quer nomear a coluna de acertos", err)
	}
}

// --- casamento com o concurso ---------------------------------------------

func concursoDeTeste() concurso.Concurso {
	return concurso.Concurso{
		Disciplinas: []concurso.Disciplina{
			{
				Codigo: "LINPO",
				Nome:   "Língua Portuguesa",
				Temas:  []string{"Crase", "Concordância verbal"},
			},
			{
				Codigo: "DIRAD",
				Nome:   "Direito Administrativo",
				Temas:  []string{"Licitações e contratos"},
			},
		},
	}
}

func TestCasar_casaPeloTemaAntesDoNome(t *testing.T) {
	t.Parallel()

	p := Casar(concursoDeTeste(), []Linha{{Assunto: "Crase", Questoes: 10, Acertos: 5}})

	if len(p.Casados) != 1 {
		t.Fatalf("casados = %d, quer 1", len(p.Casados))
	}

	c := p.Casados[0]
	if c.Disciplina != "LINPO" || c.Tema != "Crase" {
		t.Errorf("casamento = %+v, quer LINPO/Crase", c)
	}
}

// A redação do TEC quase nunca é idêntica à do edital — o casamento ignora
// acento, caixa e sobra de texto dos dois lados.
func TestCasar_toleraADiferencaDeRedacao(t *testing.T) {
	t.Parallel()

	casos := map[string]string{
		"sem acento":       "concordancia verbal",
		"em maiúsculas":    "CONCORDÂNCIA VERBAL",
		"com sobra":        "Concordância verbal e nominal",
		"pedaço do tema":   "Concordância verb",
		"com espaço solto": "  Concordância verbal  ",
	}

	for nome, assunto := range casos {
		t.Run(nome, func(t *testing.T) {
			t.Parallel()

			p := Casar(concursoDeTeste(), []Linha{{Assunto: assunto, Questoes: 10, Acertos: 5}})

			if len(p.Casados) != 1 {
				t.Fatalf("%q não casou: %+v", assunto, p.SemCorrespon)
			}

			if p.Casados[0].Disciplina != "LINPO" {
				t.Errorf("disciplina = %q, quer LINPO", p.Casados[0].Disciplina)
			}
		})
	}
}

func TestCasar_casaPeloNomeDaDisciplinaQuandoNenhumTemaBate(t *testing.T) {
	t.Parallel()

	p := Casar(concursoDeTeste(), []Linha{
		{Assunto: "Direito Administrativo", Questoes: 10, Acertos: 5},
	})

	if len(p.Casados) != 1 {
		t.Fatalf("casados = %d, quer 1", len(p.Casados))
	}

	if p.Casados[0].Disciplina != "DIRAD" || p.Casados[0].Tema != "" {
		t.Errorf("casamento = %+v, quer DIRAD sem tema", p.Casados[0])
	}
}

// O que não casa NÃO some: vai para a lista que a tela mostra, para o estudante
// decidir. Importar em silêncio o que não se entendeu seria pior.
func TestCasar_separaOQueNaoCasou(t *testing.T) {
	t.Parallel()

	p := Casar(concursoDeTeste(), []Linha{
		{Assunto: "Crase", Questoes: 10, Acertos: 5},
		{Assunto: "Astronomia", Questoes: 8, Acertos: 2},
	})

	if len(p.Casados) != 1 || len(p.SemCorrespon) != 1 {
		t.Fatalf("casados = %d, sem correspondência = %d", len(p.Casados), len(p.SemCorrespon))
	}

	if p.SemCorrespon[0].Assunto != "Astronomia" {
		t.Errorf("sem correspondência = %+v", p.SemCorrespon[0])
	}

	// Os totais contam SÓ o que casou: são eles que viram registro no plano.
	if p.Questoes != 10 || p.Acertos != 5 {
		t.Errorf("totais = %d/%d, quer 10/5", p.Questoes, p.Acertos)
	}
}

// A ordem é a do pior aproveitamento primeiro: a tela existe para mostrar onde
// o estudante está mal, e isso precisa estar no topo.
func TestCasar_ordenaDoPiorParaOMelhor(t *testing.T) {
	t.Parallel()

	p := Casar(concursoDeTeste(), []Linha{
		{Assunto: "Crase", Questoes: 10, Acertos: 9},                  // 90%
		{Assunto: "Licitações e contratos", Questoes: 10, Acertos: 2}, // 20%
		{Assunto: "Concordância verbal", Questoes: 10, Acertos: 5},    // 50%
	})

	if len(p.Casados) != 3 {
		t.Fatalf("casados = %d, quer 3", len(p.Casados))
	}

	quer := []int{20, 50, 90}
	for i, pct := range quer {
		if p.Casados[i].Pct != pct {
			t.Errorf("posição %d = %d%%, quer %d%%", i, p.Casados[i].Pct, pct)
		}
	}
}

func TestCasar_calculaErrosEPercentual(t *testing.T) {
	t.Parallel()

	p := Casar(concursoDeTeste(), []Linha{{Assunto: "Crase", Questoes: 40, Acertos: 28}})

	c := p.Casados[0]
	if c.Erros != 12 {
		t.Errorf("erros = %d, quer 12", c.Erros)
	}

	if c.Pct != 70 {
		t.Errorf("pct = %d, quer 70", c.Pct)
	}
}

// Assunto curto não casa por continência: "ADM" está contido em dezenas de
// palavras, e casar por isso ligaria o registro à matéria errada.
func TestCasar_naoCasaTextoCurtoPorContinencia(t *testing.T) {
	t.Parallel()

	c := concurso.Concurso{
		Disciplinas: []concurso.Disciplina{
			{Codigo: "DIRAD", Nome: "Direito Administrativo", Temas: []string{"Atos"}},
		},
	}

	p := Casar(c, []Linha{{Assunto: "Ato", Questoes: 10, Acertos: 5}})

	if len(p.SemCorrespon) != 1 {
		t.Errorf("um assunto de três letras não devia casar: %+v", p.Casados)
	}
}

// Planilha sem nenhuma linha devolve as duas listas vazias — e não nulas, que a
// serialização do adapter transformaria em `null` no JSON.
func TestCasar_semLinhasDevolveListasVazias(t *testing.T) {
	t.Parallel()

	p := Casar(concursoDeTeste(), nil)

	if p.Casados == nil || p.SemCorrespon == nil {
		t.Error("as listas precisam vir vazias, não nulas")
	}

	if len(p.Casados) != 0 || len(p.SemCorrespon) != 0 {
		t.Errorf("preview = %+v, quer vazio", p)
	}
}
