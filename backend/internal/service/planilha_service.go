package service

import (
	"context"
	"encoding/csv"
	"errors"
	"strconv"
	"strings"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// PlanilhaService é o CSV do plano nos dois sentidos.
//
// Na saída: o cronograma com o que foi lançado em cada dia, mais o caderno de
// erros como segunda tabela — a exportação leva o raciocínio junto com os
// números.
//
// Na entrada: só os REGISTROS voltam. O cronograma desta instalação é do motor
// e do estudante; a planilha diz o que foi estudado, não quando cada matéria
// deve ser ensinada.
type PlanilhaService struct {
	carregador
}

func NewPlanilhaService(deps Dependencias) *PlanilhaService {
	return &PlanilhaService{deps.carregador()}
}

func (s *PlanilhaService) CSV(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) ([]byte, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return nil, err
	}

	nomes := make(map[string]string, len(c.Concurso.Disciplinas))
	for _, d := range c.Concurso.Disciplinas {
		nomes[d.Codigo] = d.Nome
	}

	res := plano.Gerar(c.Plano.Config, &c.Concurso)
	plano.AplicarNosDias(res.Dias, c.Atividades)

	var b strings.Builder

	// BOM para que o Excel abra o UTF-8 corretamente.
	b.WriteString("\uFEFF")

	w := csv.NewWriter(&b)

	// `codigo` e `minutos` existem para o caminho de volta: o código casa a
	// matéria sem depender de o nome ter sido digitado igual, e o tempo sai na
	// mesma unidade em que é lançado na tela.
	cabecalho := []string{
		"dia", "data", "semana", "fase", "tipo",
		"codigo", "disciplina", "tema", "meta_questoes",
		"minutos", "questoes", "acertos", "concluido", "anotacao",
	}

	if err := w.Write(cabecalho); err != nil {
		return nil, err
	}

	for _, d := range res.Dias {
		dt := plano.DayOf(d.Data)

		fase := "Conteúdo"
		if d.Fase == plano.FaseReta {
			fase = "Reta final"
		}

		nota := ""
		if reg, ok := c.Dias[dt]; ok {
			nota = reg.Nota
		}

		// Uma linha por ATIVIDADE: é a unidade em que o estudo é registrado, e
		// escrevê-la assim tira o limite artificial de dois blocos por dia que a
		// versão anterior tinha.
		doDia := plano.AtividadesDoDia(c.Atividades, dt)
		if len(doDia) == 0 {
			continue
		}

		for _, a := range doDia {
			reg := c.Registros[a.ID]

			disciplina := nomes[a.Disciplina]
			if disciplina == "" {
				disciplina = string(a.Tipo)
			}

			concluido := "não"
			if reg.Concluido {
				concluido = "sim"
			}

			linha := []string{
				strconv.Itoa(d.N),
				dt.Format("02/01/2006"),
				strconv.Itoa(d.Semana),
				fase,
				string(d.Tipo),
				a.Disciplina,
				disciplina,
				a.Tema,
				strconv.Itoa(d.Meta),
				minutosOuVazio(reg.Horas),
				intOuVazio(reg.Questoes),
				intOuVazio(reg.Acertos),
				concluido,
				nota,
			}

			if err := w.Write(linha); err != nil {
				return nil, err
			}
		}
	}

	anotacoes, err := s.caderno.Anotacoes(ctx, c.Plano.ID)
	if err != nil {
		return nil, err
	}

	if err := escreverCadernoCSV(w, c.Concurso, anotacoes); err != nil {
		return nil, err
	}

	if err := escreverMateriasCSV(w, c.Concurso, c.Plano.Config.Normalizar()); err != nil {
		return nil, err
	}

	w.Flush()

	if err := w.Error(); err != nil {
		return nil, err
	}

	return []byte(b.String()), nil
}

// ImportarCommand é o pedido de importação: a planilha e se é para valer.
//
// Confirmar=false é a prévia — nada é gravado, e o resultado diz o que entraria
// e o que ficaria de fora. É o mesmo caminho da importação de verdade, então a
// prévia não pode discordar do que acontece em seguida.
type ImportarPlanilhaCommand struct {
	CSV       string
	Confirmar bool
}

// LinhaImportada é uma linha que encontrou onde entrar.
type LinhaImportada struct {
	Linha      int
	Data       string
	Disciplina string
	Tema       string
	Minutos    *int
	Questoes   *int
	Acertos    *int
	Concluido  bool
	// Criada: a atividade não existia naquele dia e foi reconstruída.
	Criada bool
}

// LinhaRecusada é uma linha que não entrou, e por quê.
type LinhaRecusada struct {
	Linha      int
	Data       string
	Disciplina string
	Motivo     string
}

// ResultadoImportacao é o que a tela mostra depois de ler a planilha.
//
// Criadas são as linhas que caíram num dia onde aquela matéria não estava
// agendada e por isso reconstroem a atividade — o caso de quem traz o histórico
// de outra instalação.
type ResultadoImportacao struct {
	Aplicadas []LinhaImportada
	Recusadas []LinhaRecusada
	Criadas   int
	// Anotacoes são as entradas de caderno que a planilha traz e que ainda não
	// existem aqui.
	Anotacoes int
	// Materias são as matérias cuja personalização a planilha atualiza: a tag, o
	// link do caderno de erros e os ajustes de estudo.
	Materias int
	Gravadas int
}

// ImportarCSV traz os registros de uma planilha do plano.
//
// O que entra é só o que foi ESTUDADO: tempo, questões, acertos e conclusão. O
// cronograma não se mexe — cada linha procura a atividade que já existe no dia
// e na matéria dela, e a linha que não acha nenhuma é devolvida com o motivo em
// vez de inventar uma vaga.
func (s *PlanilhaService) ImportarCSV(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	cmd ImportarPlanilhaCommand,
) (ResultadoImportacao, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return ResultadoImportacao{}, err
	}

	planilha, err := plano.LerPlanilha(strings.NewReader(cmd.CSV))
	if err != nil {
		return ResultadoImportacao{}, erroDePlanilha(err)
	}

	// A janela em que a planilha pode reconstruir o que faltou: do início do
	// plano até hoje, e só nos dias que o plano de fato estuda.
	dias := plano.Gerar(c.Plano.Config, &c.Concurso).Dias

	res := plano.CasarPlanilha(c.Atividades, dias, planilha.Registros, c.Concurso, plano.JanelaDaPlanilha{
		Inicio: plano.DayOf(c.Plano.Config.Inicio),
		Hoje:   plano.DayOf(s.relogio.Now()),
	})

	nomes := make(map[string]string, len(c.Concurso.Disciplinas))
	for _, d := range c.Concurso.Disciplinas {
		nomes[d.Codigo] = d.Nome
	}

	// O caderno de erros é a outra metade do arquivo: sem as anotações, o
	// histórico volta como número e o raciocínio fica para trás.
	anotacoes, err := s.caderno.Anotacoes(ctx, c.Plano.ID)
	if err != nil {
		return ResultadoImportacao{}, err
	}

	novasAnotacoes := plano.AnotacoesDaPlanilha(planilha.Caderno, anotacoes, c.Concurso)

	// A personalização da matéria: a tag, o link do caderno de erros e os ajustes
	// de estudo. É o trabalho que se perdia junto com o concurso.
	ajustes := plano.AjustesDaPlanilha(planilha.Materias, c.Concurso, c.Plano.Config.Normalizar())

	out := ResultadoImportacao{
		Aplicadas: make([]LinhaImportada, 0, len(res.Casadas)),
		Recusadas: make([]LinhaRecusada, 0, len(res.Recusadas)),
		Criadas:   len(res.Novas),
		Anotacoes: len(novasAnotacoes),
		Materias:  len(ajustes),
	}

	registros := make([]plano.RegistroAtividade, 0, len(res.Casadas))

	for _, ca := range res.Casadas {
		reg := ca.Registro

		// A anotação da atividade é do estudante e não vem na planilha: mantê-la
		// é o que impede uma importação de apagar o que ele escreveu.
		if anterior, ok := c.Registros[reg.AtividadeID]; ok {
			reg.Nota = anterior.Nota
		}

		registros = append(registros, reg)
		out.Aplicadas = append(out.Aplicadas, linhaImportada(ca, nomes))
	}

	for _, re := range res.Recusadas {
		out.Recusadas = append(out.Recusadas, LinhaRecusada{
			Linha:      re.Linha.Numero,
			Data:       re.Linha.Data.Format(formatoISO),
			Disciplina: re.Linha.Disciplina,
			Motivo:     re.Motivo,
		})
	}

	if !cmd.Confirmar {
		return out, nil
	}

	if len(registros) == 0 && len(novasAnotacoes) == 0 && len(ajustes) == 0 {
		// Sem nada para gravar há dois casos diferentes, e tratá-los igual seria
		// mentir num deles: a planilha não casou com nada (erro de quem manda), ou
		// o que ela traz já está aqui — reimportar o mesmo arquivo é inofensivo, e
		// dizer "0 linhas" é a resposta certa.
		if len(res.Recusadas) > 0 {
			return out, erroDeValidacao(
				"nenhuma linha da planilha casou com o cronograma deste plano",
			)
		}

		return out, nil
	}

	// As atividades reconstruídas entram ANTES dos registros: um registro é
	// sempre de uma atividade, e a chave estrangeira recusaria a ordem inversa.
	// Vai o cronograma inteiro, porque quem grava substitui — mandar só as novas
	// apagaria todo o resto.
	if len(res.Novas) > 0 {
		completo := append(append([]plano.Atividade{}, c.Atividades...), res.Novas...)

		if err := s.cronograma.SubstituirAtividades(ctx, c.Plano.ID, completo); err != nil {
			return ResultadoImportacao{}, err
		}
	}

	if err := s.cronograma.SalvarRegistros(ctx, c.Plano.ID, registros); err != nil {
		return ResultadoImportacao{}, err
	}

	for _, a := range novasAnotacoes {
		if _, err := s.caderno.CriarAnotacao(ctx, c.Plano.ID, a); err != nil {
			return ResultadoImportacao{}, err
		}
	}

	if err := s.aplicarAjustes(ctx, &c, ajustes); err != nil {
		return ResultadoImportacao{}, err
	}

	out.Gravadas = len(registros)

	return out, nil
}

// aplicarAjustes grava a personalização das matérias.
//
// A tag e o link moram na DISCIPLINA (são do concurso); questões, modo e
// reforço moram no PLANO. São dois agregados e duas gravações — o que o domínio
// já separava.
func (s *PlanilhaService) aplicarAjustes(
	ctx context.Context,
	c *contexto,
	ajustes []plano.AjusteDeMateria,
) error {
	if len(ajustes) == 0 {
		return nil
	}

	cur := c.Concurso
	cfg := c.Plano.Config.Normalizar()
	mexeuNoConcurso, mexeuNoPlano := false, false

	for _, a := range ajustes {
		d := cur.DisciplinaPorID(a.DisciplinaID)
		if d == nil {
			continue
		}

		// Os ajustes do plano são indexados pelo código: eles são lidos com o
		// código ANTIGO e regravados com o novo.
		codigo := d.Codigo

		if a.Codigo != "" {
			d.Codigo = a.Codigo
			mexeuNoConcurso = true
		}

		if a.CadernoURL != "" {
			d.CadernoURL = a.CadernoURL
			mexeuNoConcurso = true
		}

		if a.Questoes != nil {
			cfg.Questoes[d.Codigo] = *a.Questoes
			mexeuNoPlano = true
		} else if d.Codigo != codigo {
			cfg.Questoes[d.Codigo] = cfg.Questoes[codigo]
		}

		if a.Modo != nil {
			cfg.Modos[d.Codigo] = *a.Modo
			mexeuNoPlano = true
		} else if d.Codigo != codigo {
			cfg.Modos[d.Codigo] = cfg.ModoDe(codigo)
		}

		if a.Reforco != nil {
			cfg.Reforcos[d.Codigo] = *a.Reforco
			mexeuNoPlano = true
		} else if d.Codigo != codigo {
			cfg.Reforcos[d.Codigo] = cfg.ReforcoDe(codigo)
		}

		if d.Codigo != codigo {
			delete(cfg.Questoes, codigo)
			delete(cfg.Modos, codigo)
			delete(cfg.Reforcos, codigo)

			mexeuNoPlano = true
		}
	}

	if mexeuNoConcurso {
		cur.Normalizar()

		if err := cur.Validar(); err != nil {
			// A planilha não pode invalidar o cadastro: o histórico já entrou, e a
			// personalização é o acessório. Quem a recusa é o domínio, e o estudante
			// ajusta na tela de editar concurso.
			return nil //nolint:nilerr // recusa da personalização não invalida a importação
		}

		atualizado, err := s.concursos.Atualizar(ctx, cur)
		if err != nil {
			return err
		}

		c.Concurso = atualizado
	}

	if mexeuNoPlano {
		c.Plano.Config = cfg

		p, err := s.planos.Salvar(ctx, c.Plano)
		if err != nil {
			return err
		}

		c.Plano = p
	}

	return nil
}

func linhaImportada(ca plano.LinhaCasada, nomes map[string]string) LinhaImportada {
	nome := ca.Linha.Disciplina
	if n := nomes[ca.Linha.Codigo]; n != "" {
		nome = n
	}

	return LinhaImportada{
		Linha:      ca.Linha.Numero,
		Data:       ca.Linha.Data.Format(formatoISO),
		Disciplina: nome,
		Tema:       ca.Linha.Tema,
		Minutos:    ca.Linha.Minutos,
		Questoes:   ca.Registro.Questoes,
		Acertos:    ca.Registro.Acertos,
		Concluido:  ca.Registro.Concluido,
		Criada:     ca.Criada,
	}
}

// erroDePlanilha traduz a recusa do domínio numa mensagem de validação: uma
// planilha ilegível é erro de quem manda, não do servidor.
func erroDePlanilha(err error) error {
	if errors.Is(err, plano.ErrPlanilhaIlegivel) {
		return erroDeValidacao(err.Error())
	}

	return err
}

// escreverMateriasCSV grava a terceira tabela: o que o estudante personalizou
// em cada matéria.
//
// A tag que ele escolheu, o link do caderno de erros dele (o do TEC, o do
// Qconcursos, um documento) e os ajustes de estudo. Nada disso cabe nas linhas
// do cronograma, e sem isso a planilha leva o histórico mas deixa para trás as
// escolhas — que é o trabalho que ninguém quer refazer numa instalação nova.
func escreverMateriasCSV(w *csv.Writer, cur concurso.Concurso, cfg plano.Config) error {
	if len(cur.Disciplinas) == 0 {
		return nil
	}

	if err := w.Write(nil); err != nil {
		return err
	}

	if err := w.Write([]string{
		"materia_codigo", "materia_nome", "materia_caderno",
		"materia_questoes", "materia_modo", "materia_reforco",
	}); err != nil {
		return err
	}

	for _, d := range cur.Disciplinas {
		if err := w.Write([]string{
			d.Codigo,
			d.Nome,
			d.CadernoURL,
			strconv.Itoa(cfg.Questoes[d.Codigo]),
			string(cfg.ModoDe(d.Codigo)),
			strconv.FormatFloat(cfg.ReforcoDe(d.Codigo), 'f', -1, 64),
		}); err != nil {
			return err
		}
	}

	return nil
}

func escreverCadernoCSV(
	w *csv.Writer,
	cur concurso.Concurso,
	anotacoes []plano.Anotacao,
) error {
	if len(anotacoes) == 0 {
		return nil
	}

	nomePorID := make(map[uuid.UUID]string, len(cur.Disciplinas))
	for _, d := range cur.Disciplinas {
		nomePorID[d.ID] = d.Nome
	}

	if err := w.Write(nil); err != nil {
		return err
	}

	if err := w.Write([]string{
		"caderno_data", "caderno_disciplina", "caderno_tema",
		"caderno_texto", "caderno_origem", "caderno_resolvido",
	}); err != nil {
		return err
	}

	for _, a := range anotacoes {
		data := ""
		if a.Data != nil {
			data = plano.DayOf(*a.Data).Format("02/01/2006")
		}

		disciplina := ""
		if a.DisciplinaID != nil {
			disciplina = nomePorID[*a.DisciplinaID]
		}

		resolvido := "não"
		if a.Resolvido {
			resolvido = "sim"
		}

		if err := w.Write([]string{
			data, disciplina, a.Tema, a.Texto, string(a.Origem), resolvido,
		}); err != nil {
			return err
		}
	}

	return nil
}

// minutosOuVazio escreve o tempo em minutos inteiros, que é como ele é
// digitado. As horas com duas casas ("0,42") só existem porque é assim que o
// registro é guardado.
func minutosOuVazio(p *float64) string {
	if p == nil {
		return ""
	}

	return strconv.Itoa(plano.MinutosDeHoras(*p))
}

func intOuVazio(p *int) string {
	if p == nil {
		return ""
	}

	return strconv.Itoa(*p)
}
