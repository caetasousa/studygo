package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/domain/tec"

	"github.com/google/uuid"
)

// ImportacaoTECService traz o desempenho exportado do TEC Concursos para dentro
// do plano.
type ImportacaoTECService struct {
	carregador
}

func NewImportacaoTECService(deps Dependencias) *ImportacaoTECService {
	return &ImportacaoTECService{deps.carregador()}
}

// Preview lê a planilha e mostra o que seria gravado, sem tocar em nada. Dois
// passos, como o assistente de edital: o usuário vê o que casou antes de
// confirmar.
func (s *ImportacaoTECService) Preview(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	planilha io.Reader,
) (tec.Preview, error) {
	c, err := s.concursoDoDono(ctx, usuarioID, slug)
	if err != nil {
		return tec.Preview{}, err
	}

	linhas, err := tec.LerPlanilha(planilha)
	if err != nil {
		return tec.Preview{}, erroDeTEC(err)
	}

	return tec.Casar(c, linhas), nil
}

// ImportarCommand é a confirmação da importação.
type ImportarCommand struct {
	CSV  string
	Data string
}

// Importar aplica a planilha a um dia: cada assunto que casou vira o registro
// da atividade daquela matéria naquele dia, e todo assunto fraco abre uma
// anotação no caderno de erros.
func (s *ImportacaoTECService) Importar(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	cmd ImportarCommand,
) (tec.Preview, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return tec.Preview{}, err
	}

	linhas, err := tec.LerPlanilha(strings.NewReader(cmd.CSV))
	if err != nil {
		return tec.Preview{}, erroDeTEC(err)
	}

	data := plano.DayOf(s.relogio.Now())

	if cmd.Data != "" {
		if data, err = dataISO(cmd.Data); err != nil {
			return tec.Preview{}, err
		}
	}

	prev := tec.Casar(c.Concurso, linhas)
	if len(prev.Casados) == 0 {
		return prev, erroDeValidacao(
			"nenhum assunto da planilha casou com as disciplinas deste concurso",
		)
	}

	// As atividades daquele dia são o alvo do lançamento. Um dia sem atividade
	// nenhuma não recebe registro: a linha ficaria órfã, fora das estatísticas e
	// invisível na tela.
	doDia := plano.AtividadesDoDia(c.Atividades, data)
	if len(doDia) == 0 {
		return prev, erroDeValidacao(diaForaDoPlano(c.Atividades, data))
	}

	if err := s.gravarRegistros(ctx, c, doDia, prev); err != nil {
		return tec.Preview{}, err
	}

	limiar := c.Plano.Config.Normalizar().LimiarFraco

	for _, cas := range prev.Casados {
		if cas.Pct >= limiar {
			continue
		}

		if err := s.anotarErro(ctx, c, data, cas); err != nil {
			return tec.Preview{}, err
		}
	}

	return prev, nil
}

// gravarRegistros soma o que casou por disciplina e lança nas atividades
// daquele dia. Uma disciplina agendada duas vezes no dia recebe o total na
// primeira ocorrência: a planilha do TEC não distingue as duas.
func (s *ImportacaoTECService) gravarRegistros(
	ctx context.Context,
	c contexto,
	doDia []plano.Atividade,
	prev tec.Preview,
) error {
	totais := map[string]struct{ questoes, acertos int }{}

	for _, cas := range prev.Casados {
		t := totais[cas.Disciplina]
		t.questoes += cas.Questoes
		t.acertos += cas.Acertos
		totais[cas.Disciplina] = t
	}

	lancadas := map[string]bool{}

	for _, a := range doDia {
		t, ok := totais[a.Disciplina]
		if !ok || lancadas[a.Disciplina] {
			continue
		}

		lancadas[a.Disciplina] = true

		questoes, acertos := t.questoes, t.acertos

		if err := s.cronograma.SalvarRegistro(ctx, c.Plano.ID, plano.RegistroAtividade{
			AtividadeID: a.ID,
			Questoes:    &questoes,
			Acertos:     plano.AcertosValidos(&questoes, &acertos),
			Concluido:   true,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *ImportacaoTECService) anotarErro(
	ctx context.Context,
	c contexto,
	data time.Time,
	cas tec.Casamento,
) error {
	a := plano.Anotacao{
		Data:   &data,
		Tema:   cas.Tema,
		Origem: plano.OrigemTEC,
		Texto: fmt.Sprintf(
			"%d%% em %q no TEC (%d de %d, %d erros) — anote por que errou, não só a resposta.",
			cas.Pct, cas.Assunto, cas.Acertos, cas.Questoes, cas.Erros,
		),
	}

	if d := c.Concurso.DisciplinaPorCodigo(cas.Disciplina); d != nil {
		id := d.ID
		a.DisciplinaID = &id
		a.URL = urlDeQuestoes(*d, cas.Tema)
	}

	_, err := s.caderno.CriarAnotacao(ctx, c.Plano.ID, a)

	return err
}

// urlDeQuestoes monta o link do banco de questões da disciplina, trocando
// {tema} pelo tópico do dia quando a fonte traz o marcador.
func urlDeQuestoes(d concurso.Disciplina, tema string) string {
	for _, f := range d.Fontes {
		if f.Tipo != "questoes" || f.URL == "" {
			continue
		}

		return strings.ReplaceAll(f.URL, "{tema}", url.QueryEscape(tema))
	}

	return ""
}

// diaForaDoPlano nomeia o intervalo do plano, para que o usuário escolha uma
// data que existe em vez de só ouvir "não".
func diaForaDoPlano(atividades []plano.Atividade, data time.Time) string {
	if len(atividades) == 0 {
		return "este plano ainda não tem dias de estudo"
	}

	primeiro := plano.DayOf(atividades[0].Data)
	ultimo := plano.DayOf(atividades[len(atividades)-1].Data)

	return fmt.Sprintf(
		"%s não é um dia de estudo do plano — escolha uma data entre %s e %s",
		data.Format(formatoISO), primeiro.Format(formatoISO), ultimo.Format(formatoISO),
	)
}

// erroDeTEC traduz a recusa do domínio numa mensagem ao usuário.
func erroDeTEC(err error) error {
	if errors.Is(err, tec.ErrPlanilhaInvalida) {
		return erroDeValidacao(err.Error())
	}

	return err
}
