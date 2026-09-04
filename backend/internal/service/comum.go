package service

import (
	"context"
	"errors"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/domain/usuario"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// ErrValidacao carrega uma mensagem de validação destinada ao usuário. O
// adapter HTTP a traduz em 422; o domínio nunca a conhece.
type ErrValidacao struct{ Msg string }

func (e ErrValidacao) Error() string { return e.Msg }

func erroDeValidacao(msg string) error { return ErrValidacao{Msg: msg} }

// contexto é o estado que quase todo caso de uso do plano precisa carregar: o
// concurso do dono, o plano dele e o cronograma gravado.
//
// Ele existe porque a alternativa — cada serviço recarregando o que precisa —
// foi o que fazia uma única requisição gerar o plano quatro vezes.
type contexto struct {
	Concurso   concurso.Concurso
	Plano      plano.Plano
	Atividades []plano.Atividade
	Registros  plano.Registros
	Dias       map[time.Time]plano.RegistroDia
	// TemaUI é a preferência visual do DONO, não do plano: quem estuda para dois
	// concursos não quer dois temas.
	TemaUI string
}

// DiaConcluido diz se um dia do plano está terminado, derivando das atividades.
func (c contexto) DiaConcluido() func(time.Time) bool {
	return plano.DiaConcluidoFunc(c.Atividades, c.Registros)
}

// carregador reúne os repositórios que os casos de uso do plano compartilham.
// Cada serviço o embute, em vez de repetir os quatro campos e a lógica de
// carga.
type carregador struct {
	planos     port.PlanoRepository
	cronograma port.CronogramaRepository
	concursos  port.ConcursoRepository
	usuarios   port.UsuarioRepository
	caderno    port.CadernoRepository
	relogio    port.Clock
}

// carregar traz concurso, plano e cronograma, criando o plano na primeira vez
// que o usuário abre o concurso.
func (c carregador) carregar(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) (contexto, error) {
	cur, err := c.concursoDoDono(ctx, usuarioID, slug)
	if err != nil {
		return contexto{}, err
	}

	p, err := c.garantirPlano(ctx, usuarioID, cur)
	if err != nil {
		return contexto{}, err
	}

	atividades, err := c.cronograma.Atividades(ctx, p.ID)
	if err != nil {
		return contexto{}, err
	}

	// Um plano recém-criado ainda não tem cronograma gravado. Materializá-lo
	// aqui é o que dá id de verdade a cada bloco desde a primeira tela — antes
	// isso era adiado, e a tela precisava inventar ids sintéticos para ter o que
	// endereçar.
	if len(atividades) == 0 {
		if atividades, err = c.materializar(ctx, cur, p); err != nil {
			return contexto{}, err
		}
	}

	registros, err := c.cronograma.Registros(ctx, p.ID)
	if err != nil {
		return contexto{}, err
	}

	dias, err := c.cronograma.RegistrosDia(ctx, p.ID)
	if err != nil {
		return contexto{}, err
	}

	p.Registros = registros

	tema := string(usuario.TemaPadrao)

	if c.usuarios != nil {
		u, err := c.usuarios.PorID(ctx, usuarioID)
		if err != nil {
			return contexto{}, err
		}

		tema = string(u.TemaUI)
	}

	return contexto{
		Concurso:   cur,
		Plano:      p,
		Atividades: atividades,
		Registros:  registros,
		Dias:       dias,
		TemaUI:     tema,
	}, nil
}

// materializar grava o cronograma que o motor propõe.
func (c carregador) materializar(
	ctx context.Context,
	cur concurso.Concurso,
	p plano.Plano,
) ([]plano.Atividade, error) {
	res := plano.Gerar(p.Config, &cur)
	atividades := plano.Materializar(res.Dias, idsPorCodigo(cur))

	if err := c.cronograma.SubstituirAtividades(ctx, p.ID, atividades); err != nil {
		return nil, err
	}

	// Relê para que os ids gerados pelo banco cheguem a quem chamou.
	return c.cronograma.Atividades(ctx, p.ID)
}

// concursoDoDono carrega um concurso pelo slug e devolve 404 se não for do
// usuário: não existir e não ser seu são a mesma coisa para quem pergunta.
func (c carregador) concursoDoDono(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) (concurso.Concurso, error) {
	cur, err := c.concursos.PorSlug(ctx, slug)
	if err != nil {
		return concurso.Concurso{}, err
	}

	if cur.DonoID != usuarioID {
		return concurso.Concurso{}, concurso.ErrNaoEncontrado
	}

	return cur, nil
}

func (c carregador) garantirPlano(
	ctx context.Context,
	usuarioID uuid.UUID,
	cur concurso.Concurso,
) (plano.Plano, error) {
	p, err := c.planos.PorUsuario(ctx, usuarioID, cur.ID)
	if err == nil {
		return p, nil
	}

	if !errors.Is(err, plano.ErrNaoEncontrado) {
		return plano.Plano{}, err
	}

	novo := plano.NovoPlano()
	novo.UsuarioID = usuarioID
	novo.ConcursoID = cur.ID
	novo.Config = configPadrao(cur, c.relogio.Now())

	return c.planos.Salvar(ctx, novo)
}

// configPadrao é a configuração com que um plano nasce: o edital manda nas
// questões e na reta final, e o estudo começa hoje.
func configPadrao(cur concurso.Concurso, agora time.Time) plano.Config {
	cfg := plano.ConfigPadrao()

	cfg.Questoes = map[string]int{}
	for _, d := range cur.Disciplinas {
		cfg.Questoes[d.Codigo] = d.QuestoesPadrao
	}

	cfg.Inicio = plano.DayOf(agora)
	cfg.Prova = plano.DayOf(cur.ProvaPadrao)

	// Um concurso cuja prova já passou (ou é hoje) não daria plano nenhum. Trinta
	// dias para trás dá ao estudante um cronograma que ele pode ajustar, em vez
	// de uma tela vazia.
	if !cfg.Inicio.Before(cfg.Prova) {
		cfg.Inicio = plano.AddDays(cur.ProvaPadrao, -30)
	}

	cfg.DiasEstudo = []int{1, 2, 3, 4, 5}
	cfg.DiaRevisao = 5
	cfg.RetaFinalDias = cur.RetaPadraoDias
	cfg.HorasDia = 2
	cfg.MinutosBloco = 60

	return cfg.Normalizar()
}

// idsPorCodigo indexa as disciplinas pelo código, que é como o motor as nomeia.
func idsPorCodigo(cur concurso.Concurso) map[string]uuid.UUID {
	out := make(map[string]uuid.UUID, len(cur.Disciplinas))
	for _, d := range cur.Disciplinas {
		out[d.Codigo] = d.ID
	}

	return out
}

// erroDeReplanejamento transforma a recusa do domínio numa mensagem que diz o
// que fazer em seguida.
func erroDeReplanejamento(err error) error {
	switch {
	case errors.Is(err, plano.ErrAtividadeNaoEncontrada):
		return erroDeValidacao("atividade não encontrada")
	case errors.Is(err, plano.ErrDestinoInvalido):
		return erroDeValidacao("esse dia não recebe atividades")
	case errors.Is(err, plano.ErrDiaConcluido):
		return erroDeValidacao("um dia já concluído não pode ser reorganizado")
	case errors.Is(err, plano.ErrAtividadeConcluida):
		return erroDeValidacao("uma matéria já concluída não pode ser movida")
	default:
		return err
	}
}

const formatoISO = "2006-01-02"

// dataISO converte a data que veio pela borda, recusando o que não é data.
func dataISO(s string) (time.Time, error) {
	t, err := time.Parse(formatoISO, s)
	if err != nil {
		return time.Time{}, erroDeValidacao("data inválida")
	}

	return t.UTC(), nil
}

// Dependencias é o que todo caso de uso do plano precisa. Um struct só, porque
// os seis serviços dependem do mesmo conjunto: passá-lo inteiro evita seis
// construtores com a mesma lista de seis parâmetros posicionais, em que trocar
// dois de lugar compila e quebra em produção.
type Dependencias struct {
	Planos     port.PlanoRepository
	Cronograma port.CronogramaRepository
	Concursos  port.ConcursoRepository
	Caderno    port.CadernoRepository
	Usuarios   port.UsuarioRepository
	Relogio    port.Clock
}

func (d Dependencias) carregador() carregador {
	return carregador{
		planos:     d.Planos,
		cronograma: d.Cronograma,
		concursos:  d.Concursos,
		caderno:    d.Caderno,
		usuarios:   d.Usuarios,
		relogio:    d.Relogio,
	}
}
