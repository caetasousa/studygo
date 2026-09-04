package service

import (
	"context"
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

	// Terminar um tema agendado para a frente traz a matéria para o dia em que
	// ela realmente foi concluída: duas passadas de uma matéria numa sentada é
	// uma semana real, e o cronograma deve dizer o que aconteceu em vez de
	// continuar afirmando que o tema ainda está por vir.
	hoje := plano.DayOf(s.relogio.Now())

	if reg.Concluido && plano.DayOf(atividade.Data).After(hoje) {
		if err := s.anteciparEReorganizar(ctx, &c, cmd.AtividadeID, hoje); err != nil {
			return PlanoMontado{}, err
		}
	}

	return s.montar(ctx, c)
}

// anteciparEReorganizar traz a atividade para hoje e fecha o buraco que ela
// deixa. Adiantar-se deve comprar tempo, não abrir vãos no cronograma.
func (s *RegistroService) anteciparEReorganizar(
	ctx context.Context,
	c *contexto,
	id uuid.UUID,
	hoje time.Time,
) error {
	res := plano.Gerar(c.Plano.Config, &c.Concurso)

	movidas, err := plano.AntecipouAtividade(
		c.Atividades, res.Dias, id, hoje, c.DiaConcluido(),
	)
	if err != nil {
		// Uma recusa aqui não é problema de quem chamou: o registro em si foi
		// salvo. A atividade fica onde está em vez de o lançamento falhar.
		return nil //nolint:nilerr // a recusa do remanejamento não invalida o registro
	}

	if err := s.cronograma.SubstituirAtividades(ctx, c.Plano.ID, movidas); err != nil {
		return err
	}

	recarregadas, err := s.cronograma.Atividades(ctx, c.Plano.ID)
	if err != nil {
		return err
	}

	c.Atividades = recarregadas

	return nil
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
