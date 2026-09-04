package service

import (
	"context"
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// CronogramaService remaneja o que está agendado: mover, trocar, adiar,
// antecipar, compactar e restaurar a ordem.
//
// Nenhuma destas operações toca em registro: o plano muda, a história do que
// foi estudado não.
type CronogramaService struct {
	carregador
}

func NewCronogramaService(deps Dependencias) *CronogramaService {
	return &CronogramaService{deps.carregador()}
}

// MoverCommand é um remanejamento pedido pela tela.
type MoverCommand struct {
	ID      uuid.UUID
	Data    string
	Posicao int
	// Trocar pede a troca com quem já ocupa a vaga, em vez de inserir ao lado.
	Trocar bool
}

// Mover leva uma atividade para outro dia e posição.
func (s *CronogramaService) Mover(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	cmd MoverCommand,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	destino, err := dataISO(cmd.Data)
	if err != nil {
		return PlanoMontado{}, err
	}

	// Só uma atividade CONCLUÍDA é imóvel: movê-la reescreveria o que foi
	// estudado. Um dia não trava por guardar trabalho concluído — as outras
	// matérias dele continuam livres.
	if !plano.AtividadeMovivel(c.Registros, cmd.ID) {
		return PlanoMontado{}, erroDeValidacao("uma matéria já concluída não pode ser movida")
	}

	// A troca move DUAS atividades; a checagem acima protege só o lado de quem
	// pediu. Sem isto, trocar um bloco a estudar por um já concluído deslocaria o
	// concluído sem tocar no registro dele, e a conclusão passaria a apontar para
	// uma matéria que o estudante nunca estudou naquele lugar.
	res := plano.Gerar(c.Plano.Config, &c.Concurso)

	if cmd.Trocar {
		noDestino := plano.AtividadesDoDia(c.Atividades, destino)
		if cmd.Posicao >= 0 && cmd.Posicao < len(noDestino) {
			alvo := noDestino[cmd.Posicao].ID
			if alvo != cmd.ID && !plano.AtividadeMovivel(c.Registros, alvo) {
				return PlanoMontado{}, erroDeValidacao(
					"uma matéria já concluída não pode ser trocada",
				)
			}
		}
	}

	mover := plano.Mover
	if cmd.Trocar {
		mover = plano.Trocar
	}

	movidas, err := mover(
		c.Atividades, res.Dias, cmd.ID, destino, cmd.Posicao, c.DiaConcluido(),
	)
	if err != nil {
		return PlanoMontado{}, erroDeReplanejamento(err)
	}

	return s.gravarEMontar(ctx, c, movidas)
}

// AdiarDia empurra o conteúdo de um dia perdido para a frente, deslocando o
// resto do plano.
func (s *CronogramaService) AdiarDia(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug, dataISOTexto string,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	data, err := dataISO(dataISOTexto)
	if err != nil {
		return PlanoMontado{}, err
	}

	res := plano.Gerar(c.Plano.Config, &c.Concurso)

	movidas, err := plano.AdiarDia(c.Atividades, res.Dias, data, c.DiaConcluido())
	if err != nil {
		return PlanoMontado{}, erroDeReplanejamento(err)
	}

	return s.gravarEMontar(ctx, c, movidas)
}

// Antecipar traz uma atividade para o dia em que ela foi realmente concluída.
//
// Não é o mesmo que subir/descer: mover anda UMA vaga por vez, para um dia
// vizinho; antecipar salta de qualquer dia futuro direto para hoje e fecha o
// buraco de uma vez. Hoje o caminho normal é concluir a matéria — Registrar
// chama isto sozinho —, mas a operação continua existindo porque não há como
// expressá-la com um movimento só.
func (s *CronogramaService) Antecipar(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	id uuid.UUID,
	dataISOTexto string,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	hoje, err := dataISO(dataISOTexto)
	if err != nil {
		return PlanoMontado{}, err
	}

	res := plano.Gerar(c.Plano.Config, &c.Concurso)

	movidas, err := plano.AntecipouAtividade(
		c.Atividades, res.Dias, id, hoje, c.DiaConcluido(),
	)
	if err != nil {
		return PlanoMontado{}, erroDeReplanejamento(err)
	}

	return s.gravarEMontar(ctx, c, movidas)
}

// Compactar puxa o cronograma para cima de qualquer dia vazio a partir de hoje
// e dá o que fazer aos dias que isso libera.
//
// Roda sozinho quando uma matéria termina antes do previsto, e existe também
// como ação própria para arrumar um plano que já ficou com buracos.
func (s *CronogramaService) Compactar(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	return s.gravarEMontar(ctx, c, s.compactar(c, plano.DayOf(s.relogio.Now())))
}

// compactar fecha os buracos a partir de `desde` e preenche com reforço o que
// sobrar no fim da fase de aprendizado.
//
// As duas metades só fazem sentido juntas: a compactação empurra o plano para
// cima e empilha os dias livres no FIM da fase; sem o reforço, um dia em branco
// logo antes da reta final é o mesmo buraco que acabou de ser fechado, só que
// deslocado.
func (s *CronogramaService) compactar(c contexto, desde time.Time) []plano.Atividade {
	res := plano.Gerar(c.Plano.Config, &c.Concurso)
	concluido := c.DiaConcluido()

	atividades := plano.CompactarAtividades(c.Atividades, res.Dias, desde, concluido)

	return plano.PreencherVazios(atividades, res.Dias, plano.Reforco{
		Fila: plano.FilaDeReforco(
			res.Dias,
			plano.Caderno(resultadosDoPlano(res.Dias, c)),
		),
		Desde:     desde,
		Concluido: concluido,
	})
}

// RestaurarOrdem descarta as movimentações manuais e devolve o cronograma ao
// que o motor calcula — dos dias de HOJE em diante.
//
// O que já passou e o que está concluído continua onde está: restaurar a ordem
// é sobre o plano à frente, não sobre reescrever o que foi estudado.
func (s *CronogramaService) RestaurarOrdem(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	hoje := plano.DayOf(s.relogio.Now())

	// Tirar a marca de "movida" é o que faz o replanejamento poder recolocá-las:
	// enquanto ela estiver ligada, a escolha do estudante vale mais que a do
	// motor.
	atuais := make([]plano.Atividade, len(c.Atividades))
	copy(atuais, c.Atividades)

	for i := range atuais {
		if !plano.DayOf(atuais[i].Data).Before(hoje) {
			atuais[i].Movida = false
		}
	}

	res := plano.Gerar(c.Plano.Config, &c.Concurso)
	novas := plano.Materializar(res.Dias, idsPorCodigo(c.Concurso))

	return s.gravarEMontar(
		ctx, c, plano.Replanejar(atuais, novas, hoje, c.Registros.Concluida),
	)
}

// TemMovimentacaoManual diz se o estudante rearranjou alguma coisa — é o que
// habilita o botão de restaurar a ordem.
func TemMovimentacaoManual(atividades []plano.Atividade) bool {
	for _, a := range atividades {
		if a.Movida {
			return true
		}
	}

	return false
}

// gravarEMontar persiste o cronograma novo e devolve o plano remontado.
func (s *CronogramaService) gravarEMontar(
	ctx context.Context,
	c contexto,
	atividades []plano.Atividade,
) (PlanoMontado, error) {
	if err := s.cronograma.SubstituirAtividades(ctx, c.Plano.ID, atividades); err != nil {
		return PlanoMontado{}, err
	}

	recarregadas, err := s.cronograma.Atividades(ctx, c.Plano.ID)
	if err != nil {
		return PlanoMontado{}, err
	}

	c.Atividades = recarregadas

	return s.montar(ctx, c)
}
