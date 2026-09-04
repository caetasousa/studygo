package service

import (
	"context"
	"slices"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// PlanoService cuida do plano em si: lê o plano montado e grava a configuração.
// Mexer no cronograma é com o CronogramaService; registrar estudo, com o
// RegistroService.
type PlanoService struct {
	carregador
}

func NewPlanoService(deps Dependencias) *PlanoService {
	return &PlanoService{deps.carregador()}
}

// Obter devolve o plano montado, criando um padrão no primeiro acesso.
func (s *PlanoService) Obter(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	return s.montar(ctx, c)
}

// ConfigCommand são as mudanças de configuração que chegam de fora. Todo campo
// de método é opcional: um campo ausente deixa aquela escolha intacta, para que
// salvar um controle não redefina os outros.
type ConfigCommand struct {
	Inicio        string
	Prova         string
	HorasDia      *float64
	DiasEstudo    []int
	DiaRevisao    *int
	RetaFinalDias *int
	Questoes      map[string]int

	BlocosPorDia   *int
	MinutosBloco   *int
	MinutosRevisao *int
	Reforcos       map[string]float64
	CicloRevisao   *[]ItemCicloCommand
	RevisaoSemanal *bool
	Simulados      *string
	Discursiva     *bool
	Modos          map[string]string
	PctQuestoes    *float64
	LimiarFraco    *int
}

// ItemCicloCommand é uma semana da rotação de revisão.
type ItemCicloCommand struct {
	Titulo   string
	Questoes int
}

// Salvar valida e grava a configuração nova, replanejando o futuro quando o
// ritmo do dia muda.
func (s *PlanoService) Salvar(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	cmd ConfigCommand,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	anterior := c.Plano.Config

	cfg, err := aplicarConfig(anterior, c.Concurso, cmd)
	if err != nil {
		return PlanoMontado{}, err
	}

	c.Plano.Config = cfg

	if c.Plano, err = s.planos.Salvar(ctx, c.Plano); err != nil {
		return PlanoMontado{}, err
	}

	// O cronograma já está gravado, então mudar o ritmo do dia precisa alcançá-lo
	// — senão a configuração nova só apareceria em dias que ainda não existem.
	// O replanejamento respeita o que já passou, o que está concluído e o que o
	// estudante arrumou à mão.
	if plano.RitmoMudou(anterior, cfg) || datasMudaram(anterior, cfg) {
		if err := s.replanejarFuturo(ctx, &c); err != nil {
			return PlanoMontado{}, err
		}
	}

	return s.montar(ctx, c)
}

// datasMudaram diz se o período do plano mudou. Início, prova, dias de estudo e
// reta final redesenham o calendário inteiro, então também precisam alcançar o
// cronograma gravado.
func datasMudaram(antes, depois plano.Config) bool {
	if !antes.Inicio.Equal(depois.Inicio) ||
		!antes.Prova.Equal(depois.Prova) ||
		antes.RetaFinalDias != depois.RetaFinalDias ||
		antes.RevisaoSemanal != depois.RevisaoSemanal ||
		antes.DiaRevisao != depois.DiaRevisao ||
		antes.Simulados != depois.Simulados ||
		antes.Discursiva != depois.Discursiva ||
		len(antes.DiasEstudo) != len(depois.DiasEstudo) {
		return true
	}

	for i := range antes.DiasEstudo {
		if antes.DiasEstudo[i] != depois.DiasEstudo[i] {
			return true
		}
	}

	// As questões por disciplina mudam a distribuição de blocos, logo o
	// cronograma.
	if len(antes.Questoes) != len(depois.Questoes) {
		return true
	}

	for k, v := range antes.Questoes {
		if depois.Questoes[k] != v {
			return true
		}
	}

	return len(antes.Reforcos) != len(depois.Reforcos)
}

// replanejarFuturo regera os dias a partir de hoje e grava o resultado.
func (s *PlanoService) replanejarFuturo(ctx context.Context, c *contexto) error {
	res := plano.Gerar(c.Plano.Config, &c.Concurso)
	novas := plano.Materializar(res.Dias, idsPorCodigo(c.Concurso))

	atividades := plano.Replanejar(
		c.Atividades,
		novas,
		plano.DayOf(s.relogio.Now()),
		c.Registros.Concluida,
	)

	if err := s.cronograma.SubstituirAtividades(ctx, c.Plano.ID, atividades); err != nil {
		return err
	}

	recarregadas, err := s.cronograma.Atividades(ctx, c.Plano.ID)
	if err != nil {
		return err
	}

	c.Atividades = recarregadas

	return nil
}

// MarcarMarco liga ou desliga um item do cronograma oficial do edital.
func (s *PlanoService) MarcarMarco(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	marcoID uuid.UUID,
	cumprido bool,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	if c.Concurso.MarcoPorID(marcoID) == nil {
		return PlanoMontado{}, concurso.ErrNaoEncontrado
	}

	if err := s.planos.MarcarMarco(ctx, c.Plano.ID, marcoID, cumprido); err != nil {
		return PlanoMontado{}, err
	}

	c.Plano.Marcos[marcoID] = cumprido

	return s.montar(ctx, c)
}

// AtualizarCadernoDisciplina grava o link do caderno de erros de uma matéria,
// para que o bloco de revisão do cronograma leve direto para lá sem exigir uma
// edição completa do concurso.
func (s *PlanoService) AtualizarCadernoDisciplina(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug, codigo, url string,
) (PlanoMontado, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return PlanoMontado{}, err
	}

	if c.Concurso.DisciplinaPorCodigo(codigo) == nil {
		return PlanoMontado{}, erroDeValidacao("matéria não encontrada")
	}

	if err := s.concursos.DefinirCadernoURL(ctx, c.Concurso.ID, codigo, url); err != nil {
		return PlanoMontado{}, err
	}

	if c.Concurso, err = s.concursos.PorSlug(ctx, slug); err != nil {
		return PlanoMontado{}, err
	}

	return s.montar(ctx, c)
}

// aplicarConfig funde o comando com a configuração atual e devolve a nova, já
// normalizada pelo domínio.
func aplicarConfig(
	atual plano.Config,
	cur concurso.Concurso,
	cmd ConfigCommand,
) (plano.Config, error) {
	cfg := atual

	if cmd.Inicio != "" {
		inicio, err := dataISO(cmd.Inicio)
		if err != nil {
			return plano.Config{}, err
		}

		cfg.Inicio = inicio
	}

	if cmd.Prova != "" {
		prova, err := dataISO(cmd.Prova)
		if err != nil {
			return plano.Config{}, err
		}

		cfg.Prova = prova
	}

	if !cfg.Inicio.Before(cfg.Prova) {
		return plano.Config{}, erroDeValidacao("a data da prova precisa ser depois do início")
	}

	if cmd.DiasEstudo != nil {
		dias := diasDaSemanaValidos(cmd.DiasEstudo)
		if len(dias) == 0 {
			return plano.Config{}, erroDeValidacao("escolha ao menos um dia de estudo na semana")
		}

		cfg.DiasEstudo = dias
	}

	if cmd.HorasDia != nil {
		cfg.HorasDia = *cmd.HorasDia
	}

	if cmd.DiaRevisao != nil {
		cfg.DiaRevisao = min(max(*cmd.DiaRevisao, 0), 6)
	}

	if cmd.RetaFinalDias != nil {
		cfg.RetaFinalDias = max(*cmd.RetaFinalDias, 0)
	}

	if cmd.Questoes != nil {
		cfg.Questoes = questoesValidas(cmd.Questoes, cur)
	}

	aplicarMetodo(&cfg, cur, cmd)

	return cfg.Normalizar(), nil
}

func aplicarMetodo(cfg *plano.Config, cur concurso.Concurso, cmd ConfigCommand) {
	if cmd.BlocosPorDia != nil {
		cfg.BlocosPorDia = *cmd.BlocosPorDia
	}

	if cmd.MinutosBloco != nil {
		cfg.MinutosBloco = *cmd.MinutosBloco
	}

	if cmd.MinutosRevisao != nil {
		cfg.MinutosRevisao = *cmd.MinutosRevisao
	}

	if cmd.RevisaoSemanal != nil {
		cfg.RevisaoSemanal = *cmd.RevisaoSemanal
	}

	if cmd.Simulados != nil {
		cfg.Simulados = plano.Frequencia(*cmd.Simulados)
	}

	if cmd.Discursiva != nil {
		cfg.Discursiva = *cmd.Discursiva
	}

	if cmd.PctQuestoes != nil {
		cfg.PctQuestoes = *cmd.PctQuestoes
	}

	if cmd.LimiarFraco != nil {
		cfg.LimiarFraco = *cmd.LimiarFraco
	}

	if cmd.Modos != nil {
		modos := map[string]plano.Modo{}

		for codigo, m := range cmd.Modos {
			if cur.DisciplinaPorCodigo(codigo) != nil {
				modos[codigo] = plano.Modo(m)
			}
		}

		cfg.Modos = modos
	}

	if cmd.Reforcos != nil {
		reforcos := map[string]float64{}

		for codigo, v := range cmd.Reforcos {
			if cur.DisciplinaPorCodigo(codigo) != nil {
				reforcos[codigo] = v
			}
		}

		cfg.Reforcos = reforcos
	}

	if cmd.CicloRevisao != nil {
		ciclo := make([]concurso.ItemRevisao, 0, len(*cmd.CicloRevisao))

		for _, it := range *cmd.CicloRevisao {
			ciclo = append(ciclo, concurso.ItemRevisao{
				Ordem:    len(ciclo),
				Titulo:   it.Titulo,
				Questoes: it.Questoes,
			})
		}

		cfg.CicloRevisao = ciclo
	}
}

// questoesValidas mantém só as disciplinas que o concurso tem, para que uma
// matéria removida não continue pesando na distribuição.
func questoesValidas(entrada map[string]int, cur concurso.Concurso) map[string]int {
	out := map[string]int{}

	for _, d := range cur.Disciplinas {
		if q, ok := entrada[d.Codigo]; ok {
			out[d.Codigo] = max(q, 0)
		} else {
			out[d.Codigo] = d.QuestoesPadrao
		}
	}

	return out
}

// diasDaSemanaValidos ordena, tira repetidos e descarta o que não é dia da
// semana.
func diasDaSemanaValidos(dias []int) []int {
	visto := [7]bool{}
	out := []int{}

	for _, d := range dias {
		if d < 0 || d > 6 || visto[d] {
			continue
		}

		visto[d] = true
		out = append(out, d)
	}

	slices.Sort(out)

	return out
}
