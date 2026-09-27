package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"studygo/internal/domain/plano"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// RegistroService grava o que o estudante fez.
//
// A unidade é a ATIVIDADE. A conclusão do DIA nunca é gravada: ela é derivada
// das atividades daquele dia (ver plano.DiaConcluido), que é o que impede um
// dia de duas matérias de se dar por terminado quando só a primeira foi
// lançada.
type RegistroService struct {
	carregador

	caderno port.CadernoRepository
}

func NewRegistroService(deps Dependencias) *RegistroService {
	return &RegistroService{carregador: deps.carregador(), caderno: deps.Caderno}
}

// RegistroCommand é o lançamento de uma atividade.
type RegistroCommand struct {
	AtividadeID uuid.UUID
	Horas       *float64
	Questoes    *int
	Acertos     *int
	Nota        string
	Concluido   bool
}

// Registrar grava o lançamento de uma atividade e, quando ela estava agendada
// para depois, traz a matéria para o dia em que foi realmente concluída.
func (s *RegistroService) Registrar(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	cmd RegistroCommand,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	atividade, ok := plano.PorID(c.Atividades, cmd.AtividadeID)
	if !ok {
		return PlanoMontado{}, erroDeValidacao("atividade não encontrada")
	}

	reg := plano.RegistroAtividade{
		AtividadeID: cmd.AtividadeID,
		Horas:       cmd.Horas,
		Questoes:    cmd.Questoes,
		// Acertos acima das questões é a única combinação que quebra a
		// estatística, e o domínio é quem decide o corte.
		Acertos:   plano.AcertosValidos(cmd.Questoes, cmd.Acertos),
		Nota:      strings.TrimSpace(cmd.Nota),
		Concluido: cmd.Concluido,
	}

	if err := s.cronograma.SalvarRegistro(ctx, c.Plano.ID, reg); err != nil {
		return PlanoMontado{}, err
	}

	c.Registros[reg.AtividadeID] = reg

	// Concluir põe o cronograma de acordo com o que foi estudado: o que foi
	// feito antes da hora vem para hoje (ou para o último dia de estudo, num
	// domingo), a repetição do que já foi estudado sai, e o resto encosta.
	if reg.Concluido {
		if err := s.arrumarEstudado(ctx, &c); err != nil {
			return PlanoMontado{}, err
		}
	} else if atividade.Antecipada {
		// Desmarcar o "já estudei": o tópico volta a ser conteúdo do dia, e a
		// tela volta a mostrá-lo, pendente.
		if err := s.desantecipar(ctx, &c, atividade.ID); err != nil {
			return PlanoMontado{}, err
		}
	}

	return s.montar(ctx, c)
}

func (s *RegistroService) desantecipar(ctx context.Context, c *contexto, id uuid.UUID) error {
	atividades := append([]plano.Atividade(nil), c.Atividades...)
	for i := range atividades {
		if atividades[i].ID == id {
			atividades[i].Antecipada = false
		}
	}

	if err := s.cronograma.SubstituirAtividades(ctx, c.Plano.ID, atividades); err != nil {
		return err
	}

	c.Atividades = atividades

	return nil
}

// arrumarEstudado aplica plano.ArrumarEstudado e encosta o cronograma a partir
// de hoje. Adiantar-se deve comprar tempo, não deixar o tópico estudado
// "feito lá no final" nem repetido adiante.
func (s *RegistroService) arrumarEstudado(ctx context.Context, c *contexto) error {
	hoje := plano.DayOf(s.relogio.Now())
	res := plano.Gerar(c.Plano.Config, &c.Concurso)

	lancada := func(id uuid.UUID) bool {
		_, ok := c.Registros[id]

		return ok
	}

	arrumadas, mudou := plano.ArrumarEstudado(c.Atividades, res.Dias, hoje, c.Registros.Concluida, lancada)
	if !mudou {
		return nil
	}

	// Encosta a partir do dia SEGUINTE ao que recebeu o estudo feito (hoje, ou
	// o dia de estudo mais próximo): compactar esse dia o reempacotaria e
	// empurraria a atividade recém-concluída para a frente de novo.
	c.Atividades = arrumadas
	desde := naoAntesDe(plano.DiaDoEstudoFeito(res.Dias, hoje), hoje).AddDate(0, 0, 1)
	arrumadas = compactarDesde(*c, arrumadas, desde)

	if err := s.cronograma.SubstituirAtividades(ctx, c.Plano.ID, arrumadas); err != nil {
		return err
	}

	recarregadas, err := s.cronograma.Atividades(ctx, c.Plano.ID)
	if err != nil {
		return err
	}

	c.Atividades = recarregadas

	return nil
}

// EstudarTema marca UM tópico de uma atividade como estudado.
//
// Quando a matéria tem mais tópicos que vagas, o motor junta vários numa
// atividade só, e o registro é por atividade: marcar um tópico marcaria todos.
// O tópico sai para uma atividade própria (plano.SepararTema), e é ela que é
// registrada como concluída — pelo mesmo Registrar, que a traz para hoje e
// encosta o resto do cronograma quando ela estava adiante.
func (s *RegistroService) EstudarTema(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	atividadeID uuid.UUID,
	tema string,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	separadas, id, err := plano.SepararTema(c.Atividades, atividadeID, tema, uuid.New())

	switch {
	case errors.Is(err, plano.ErrAtividadeNaoEncontrada):
		return PlanoMontado{}, erroDeValidacao("atividade não encontrada")
	case errors.Is(err, plano.ErrTemaForaDaAtividade):
		return PlanoMontado{}, erroDeValidacao("o tópico não está nessa atividade")
	case err != nil:
		return PlanoMontado{}, err
	}

	cmd := RegistroCommand{AtividadeID: id, Concluido: true}

	if id == atividadeID {
		// Um tópico só: é a atividade inteira, e o que já foi lançado nela fica.
		if reg, ok := c.Registros[id]; ok {
			cmd.Horas, cmd.Questoes, cmd.Acertos, cmd.Nota = reg.Horas, reg.Questoes, reg.Acertos, reg.Nota
		}
	} else if err := s.cronograma.SubstituirAtividades(ctx, c.Plano.ID, separadas); err != nil {
		return PlanoMontado{}, err
	}

	return s.Registrar(ctx, usuarioID, slug, cmd)
}

// RegistroDiaCommand é o que pertence ao dia: a anotação livre e o resultado da
// cauda de revisão.
type RegistroDiaCommand struct {
	Data       string
	Nota       string
	Questoes   *int
	Acertos    *int
	Observacao string
}

// RegistrarDia grava a anotação do dia e o resultado da revisão diária.
//
// A observação vira uma anotação do caderno de erros, com origem "revisao":
// é o que o bloco de revisão do dia seguinte vai reler.
func (s *RegistroService) RegistrarDia(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	cmd RegistroDiaCommand,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	data, err := dataISO(cmd.Data)
	if err != nil {
		return PlanoMontado{}, err
	}

	reg := plano.RegistroDia{
		Data:            data,
		Nota:            strings.TrimSpace(cmd.Nota),
		RevisaoQuestoes: cmd.Questoes,
		RevisaoAcertos:  plano.AcertosValidos(cmd.Questoes, cmd.Acertos),
	}

	if err := s.cronograma.SalvarRegistroDia(ctx, c.Plano.ID, reg); err != nil {
		return PlanoMontado{}, err
	}

	c.Dias[data] = reg

	if err := s.salvarObservacao(ctx, c, data, cmd.Observacao); err != nil {
		return PlanoMontado{}, err
	}

	return s.montar(ctx, c)
}

// salvarObservacao cria ou edita a anotação daquela revisão. Uma observação
// esvaziada apaga a anotação, em vez de deixar o texto antigo para trás.
func (s *RegistroService) salvarObservacao(
	ctx context.Context,
	c contexto,
	data time.Time,
	texto string,
) error {
	texto = strings.TrimSpace(texto)

	anotacoes, err := s.caderno.Anotacoes(ctx, c.Plano.ID)
	if err != nil {
		return err
	}

	atual := anotacaoDaRevisao(anotacoes, data)

	switch {
	case texto == "" && atual != nil:
		return s.caderno.RemoverAnotacao(ctx, c.Plano.ID, atual.ID)

	case texto == "":
		return nil

	case atual != nil:
		atual.Texto = texto
		_, err = s.caderno.AtualizarAnotacao(ctx, c.Plano.ID, *atual)

		return err

	default:
		_, err = s.caderno.CriarAnotacao(ctx, c.Plano.ID, plano.Anotacao{
			Data:   &data,
			Texto:  texto,
			Origem: plano.OrigemRevisao,
		})

		return err
	}
}

// LimparRegistros apaga todo o histórico do plano.
func (s *RegistroService) LimparRegistros(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	if err := s.cronograma.ApagarRegistros(ctx, c.Plano.ID); err != nil {
		return PlanoMontado{}, err
	}

	c.Registros = plano.Registros{}
	c.Dias = map[time.Time]plano.RegistroDia{}
	c.Plano.Marcos = map[uuid.UUID]bool{}
	c.Plano.Registros = c.Registros

	return s.montar(ctx, c)
}

// anotacaoDaRevisao acha a anotação em que a observação de uma revisão mora — a
// escrita a partir daquela revisão, não qualquer nota que caia na mesma data.
func anotacaoDaRevisao(anotacoes []plano.Anotacao, dt time.Time) *plano.Anotacao {
	for i := range anotacoes {
		a := anotacoes[i]
		if a.Origem == plano.OrigemRevisao && a.Data != nil && plano.DayOf(*a.Data).Equal(dt) {
			return &anotacoes[i]
		}
	}

	return nil
}
