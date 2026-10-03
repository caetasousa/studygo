//go:build integration

package service

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"studygo/internal/adapter/postgres"
	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/domain/usuario"
	"studygo/internal/platform/pgtest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Os casos de uso são testados como rodam em produção: ligados aos
// repositories REAIS, num PostgreSQL efêmero (pgtest, Testcontainers). O que
// um teste afirma é o que fica gravado e o que a tela recebe — nunca a ordem
// das chamadas a uma porta.
//
// Não há dublê de repository aqui. Um fake em memória fica mais verde que a
// produção: não tem a FK RESTRICT, a unique diferível nem a transação, e era
// justamente nelas que as regressões do cronograma moravam.
//
// O relógio é o único dublê: um fluxo que depende de "hoje" precisa de uma data
// fixa para ser determinístico. Serviço externo (o processador de edital, o
// envio de lembrete) também é dublê, na fronteira dele.

func TestMain(m *testing.M) {
	codigo := m.Run()
	pgtest.Encerrar()
	os.Exit(codigo)
}

// relogioFixo prende o "agora" para que a numeração dos dias não dependa de
// quando a suíte roda.
type relogioFixo struct{ t time.Time }

func (r relogioFixo) Now() time.Time { return r.t }

func diaT(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

const hojeDoTeste = "2026-09-01"

// cenario é um banco novo com um estudante, o concurso dele e os casos de uso
// prontos para usar.
type cenario struct {
	deps       Dependencias
	pool       *pgxpool.Pool
	usuario    uuid.UUID
	concursoID uuid.UUID
	slug       string
	hoje       time.Time
}

func novoCenario(t *testing.T) *cenario {
	t.Helper()

	pool := pgtest.Novo(t)
	dono := novoDono(t, pool)
	concursos := postgres.NewConcursoRepo(pool)

	c, err := concursos.Criar(t.Context(), concurso.Concurso{
		DonoID: dono, Slug: "tce-go", Nome: "TCE-GO",
		ProvaPadrao: diaT(2026, time.December, 15), RetaPadraoDias: 30,
		Disciplinas: []concurso.Disciplina{
			{
				Codigo: "LINPO", Nome: "Língua Portuguesa",
				Bloco: concurso.BlocoGeral, Peso: 1, QuestoesPadrao: 15, Ordem: 0,
				Temas: []string{"Crase", "Concordância", "Regência", "Pontuação"},
			},
			{
				Codigo: "BANDA", Nome: "Banco de Dados",
				Bloco: concurso.BlocoEspecifico, Peso: 2, QuestoesPadrao: 20, Ordem: 1,
				Temas: []string{"Modelagem", "SQL", "Índices", "Transações"},
			},
		},
	})
	if err != nil {
		t.Fatalf("criando concurso: %v", err)
	}

	hoje := diaT(2026, time.September, 1)

	return &cenario{
		deps: Dependencias{
			Planos:     postgres.NewPlanoRepo(pool),
			Cronograma: postgres.NewCronogramaRepo(pool),
			Concursos:  concursos,
			Caderno:    postgres.NewCadernoRepo(pool),
			Usuarios:   postgres.NewUsuarioRepo(pool),
			Relogio:    relogioFixo{t: hoje},
		},
		pool:       pool,
		usuario:    dono,
		concursoID: c.ID,
		slug:       c.Slug,
		hoje:       hoje,
	}
}

// novoDono grava um estudante: o concurso tem FK para ele.
func novoDono(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	u, err := postgres.NewUsuarioRepo(pool).Criar(t.Context(), usuario.Usuario{
		Email: uuid.NewString() + "@teste.local", Nome: "Estudante", SenhaHash: "x", TemaUI: usuario.TemaPadrao,
	})
	if err != nil {
		t.Fatalf("criando usuário: %v", err)
	}

	return u.ID
}

func (ce *cenario) obter(t *testing.T) PlanoMontado {
	t.Helper()

	p, err := NewPlanoService(ce.deps).Obter(context.Background(), ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("Obter: %v", err)
	}

	return p
}

// avancarPara move o relógio, simulando os dias passando sem que ninguém
// estude — que é a única forma de produzir atraso.
func (ce *cenario) avancarPara(d time.Time) {
	ce.deps.Relogio = relogioFixo{t: d}
	ce.hoje = d
}

// plano é o plano como está gravado.
func (ce *cenario) plano(t *testing.T) plano.Plano {
	t.Helper()

	p, err := ce.deps.Planos.PorUsuario(t.Context(), ce.usuario, ce.concursoID)
	if err != nil {
		t.Fatalf("lendo o plano: %v", err)
	}

	return p
}

// atividades é o cronograma gravado, na ordem (data, posição).
func (ce *cenario) atividades(t *testing.T) []plano.Atividade {
	t.Helper()

	as, err := ce.deps.Cronograma.Atividades(t.Context(), ce.plano(t).ID)
	if err != nil {
		t.Fatalf("lendo o cronograma: %v", err)
	}

	return as
}

func (ce *cenario) registros(t *testing.T) plano.Registros {
	t.Helper()

	rs, err := ce.deps.Cronograma.Registros(t.Context(), ce.plano(t).ID)
	if err != nil {
		t.Fatalf("lendo os registros: %v", err)
	}

	return rs
}

func (ce *cenario) concurso(t *testing.T) concurso.Concurso {
	t.Helper()

	c, err := ce.deps.Concursos.PorID(t.Context(), ce.concursoID)
	if err != nil {
		t.Fatalf("lendo o concurso: %v", err)
	}

	return c
}

// gravarConcurso muda o concurso direto no repository — o que outra instalação
// (ou o cadastro feito de novo) deixaria no banco.
func (ce *cenario) gravarConcurso(t *testing.T, muda func(*concurso.Concurso)) {
	t.Helper()

	c := ce.concurso(t)
	muda(&c)

	if _, err := ce.deps.Concursos.Atualizar(t.Context(), c); err != nil {
		t.Fatalf("gravando o concurso: %v", err)
	}
}

func (ce *cenario) gravarConfig(t *testing.T, muda func(*plano.Config)) {
	t.Helper()

	p := ce.plano(t)
	muda(&p.Config)

	if _, err := ce.deps.Planos.Salvar(t.Context(), p); err != nil {
		t.Fatalf("gravando o plano: %v", err)
	}
}

// esvaziarDia tira do cronograma gravado as atividades de um dia, como a
// varredura de atraso deixa o dia perdido.
func (ce *cenario) esvaziarDia(t *testing.T, dia time.Time) {
	t.Helper()

	sobrando := slices.DeleteFunc(ce.atividades(t), func(a plano.Atividade) bool {
		return plano.DayOf(a.Data).Equal(dia)
	})

	if err := ce.deps.Cronograma.SubstituirAtividades(t.Context(), ce.plano(t).ID, sobrando); err != nil {
		t.Fatalf("esvaziando o dia: %v", err)
	}
}

// versoesDasLinhas é a versão (xmin) de cada linha do cronograma: qualquer
// INSERT, UPDATE ou DELETE a muda. É como o teste prova, no banco, que um
// caminho de leitura não escreveu.
func (ce *cenario) versoesDasLinhas(t *testing.T) map[uuid.UUID]string {
	t.Helper()

	rows, err := ce.pool.Query(t.Context(),
		`SELECT id, xmin::text FROM atividades WHERE plano_id = $1`, ce.plano(t).ID)
	if err != nil {
		t.Fatalf("lendo as versões: %v", err)
	}
	defer rows.Close()

	out := map[uuid.UUID]string{}

	for rows.Next() {
		var (
			id  uuid.UUID
			ver string
		)

		if err := rows.Scan(&id, &ver); err != nil {
			t.Fatalf("lendo as versões: %v", err)
		}

		out[id] = ver
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("lendo as versões: %v", err)
	}

	return out
}

// travarEscrita faz o PostgreSQL recusar toda escrita nova na tabela, deixando
// a leitura intacta: uma CHECK que nada satisfaz, NOT VALID para não exigir
// das linhas que já existem. É uma falha de gravação de verdade, do banco.
func (ce *cenario) travarEscrita(t *testing.T, tabela string) {
	t.Helper()

	// A tabela é uma constante do teste, nunca entrada de fora.
	if _, err := ce.pool.Exec(t.Context(),
		`ALTER TABLE `+tabela+` ADD CONSTRAINT teste_trava CHECK (false) NOT VALID`); err != nil {
		t.Fatalf("travando %s: %v", tabela, err)
	}
}

func diasDeEstudo(p PlanoMontado) []DiaDoPlano {
	out := []DiaDoPlano{}

	for _, d := range p.Dias {
		if d.Tipo == string(plano.TipoEstudo) {
			out = append(out, d)
		}
	}

	return out
}

func diaPorData(p PlanoMontado, data string) DiaDoPlano {
	for _, d := range p.Dias {
		if d.Data == data {
			return d
		}
	}

	return DiaDoPlano{}
}

func contemAtividade(d DiaDoPlano, id uuid.UUID) bool {
	for _, it := range d.Itens {
		if it.ID == id {
			return true
		}
	}

	return false
}

func atividadeConcluida(d DiaDoPlano, id uuid.UUID) bool {
	for _, it := range d.Itens {
		if it.ID == id {
			return it.Concluido
		}
	}

	return false
}

func asErro(err error, alvo *ErrValidacao) bool {
	v, ok := err.(ErrValidacao) //nolint:errorlint // a recusa é devolvida direto
	if ok {
		*alvo = v
	}

	return ok
}

// dataDe converte a data ISO que a tela usa para o time.Time do domínio.
func dataDe(t *testing.T, iso string) time.Time {
	t.Helper()

	d, err := time.Parse(formatoISO, iso)
	if err != nil {
		t.Fatalf("data inválida %q: %v", iso, err)
	}

	return plano.DayOf(d.UTC())
}
