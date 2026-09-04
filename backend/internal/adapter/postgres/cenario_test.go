//go:build integration

package postgres_test

import (
	"os"
	"testing"
	"time"

	"studygo/internal/adapter/postgres"
	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/domain/usuario"
	"studygo/internal/platform/pgtest"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Builders dos testes de repository.
//
// Cada teste monta só o que precisa e ganha um banco próprio, então nenhum
// depende da ordem de execução nem do estado deixado por outro. Preferimos
// builders em Go a dumps SQL: um dump acopla o teste ao schema e envelhece a
// cada migration.

func TestMain(m *testing.M) {
	codigo := m.Run()
	pgtest.Encerrar()
	os.Exit(codigo)
}

func dia(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// repos reúne os repositories ligados ao mesmo pool.
type repos struct {
	pool       *pgxpool.Pool
	usuarios   *postgres.UsuarioRepo
	concursos  *postgres.ConcursoRepo
	planos     *postgres.PlanoRepo
	cronograma *postgres.CronogramaRepo
	caderno    *postgres.CadernoRepo
}

func novoRepos(t *testing.T) *repos {
	t.Helper()

	pool := pgtest.Novo(t)

	return &repos{
		pool:       pool,
		usuarios:   postgres.NewUsuarioRepo(pool),
		concursos:  postgres.NewConcursoRepo(pool),
		planos:     postgres.NewPlanoRepo(pool),
		cronograma: postgres.NewCronogramaRepo(pool),
		caderno:    postgres.NewCadernoRepo(pool),
	}
}

// criarUsuario grava uma conta. O e-mail é parâmetro porque vários testes
// precisam de duas contas para provar isolamento.
func (r *repos) criarUsuario(t *testing.T, email string) usuario.Usuario {
	t.Helper()

	u, err := r.usuarios.Criar(t.Context(), usuario.Usuario{
		Email:     email,
		Nome:      "Fulano",
		SenhaHash: "$argon2id$fake",
		TemaUI:    usuario.TemaPadrao,
	})
	if err != nil {
		t.Fatalf("criando usuário %s: %v", email, err)
	}

	return u
}

// criarConcurso grava um concurso com duas disciplinas — o suficiente para
// exercitar join por id, ordenação e a distinção entre os dois blocos.
func (r *repos) criarConcurso(t *testing.T, dono usuario.Usuario, slug string) concurso.Concurso {
	t.Helper()

	c, err := r.concursos.Criar(t.Context(), concurso.Concurso{
		DonoID: dono.ID, Slug: slug, Nome: "TCE-GO",
		Banca: "FGV", Cargo: "Analista", Emoji: "🏛",
		ProvaPadrao: dia(2026, time.December, 15), RetaPadraoDias: 30,
		Disciplinas: []concurso.Disciplina{
			{
				Codigo: "LINPO", Nome: "Língua Portuguesa",
				Bloco: concurso.BlocoGeral, Peso: 1, QuestoesPadrao: 15, Ordem: 0,
				Temas: []string{"Crase", "Regência"},
				Fontes: []concurso.Fonte{
					{Ordem: 0, Titulo: "Gramática", URL: "https://x", Tipo: "material"},
				},
			},
			{
				Codigo: "BANDA", Nome: "Banco de Dados",
				Bloco: concurso.BlocoEspecifico, Peso: 2, QuestoesPadrao: 20, Ordem: 1,
				Temas: []string{"SQL"},
			},
		},
		Marcos: []concurso.Marco{
			{
				Ordem: 0, Rotulo: 1, DataInicio: dia(2026, time.October, 1),
				Titulo: "Inscrições", ExigeAcao: true,
			},
		},
		Conteudo: []concurso.ConteudoItem{{Ordem: 0, Tipo: "p", Texto: "Edital"}},
	})
	if err != nil {
		t.Fatalf("criando concurso %s: %v", slug, err)
	}

	return c
}

// criarPlano grava um plano para (usuário, concurso).
func (r *repos) criarPlano(t *testing.T, u usuario.Usuario, c concurso.Concurso) plano.Plano {
	t.Helper()

	p := plano.NovoPlano()
	p.UsuarioID = u.ID
	p.ConcursoID = c.ID
	p.Config = plano.ConfigPadrao()
	p.Config.Inicio = dia(2026, time.September, 1)
	p.Config.Prova = dia(2026, time.December, 15)
	p.Config.DiasEstudo = []int{1, 2, 3, 4, 5}
	p.Config.Questoes = map[string]int{"LINPO": 15, "BANDA": 20}
	p.Config = p.Config.Normalizar()

	salvo, err := r.planos.Salvar(t.Context(), p)
	if err != nil {
		t.Fatalf("salvando plano: %v", err)
	}

	return salvo
}

// criarAtividades grava um cronograma mínimo e devolve o que ficou no banco,
// com os ids atribuídos.
func (r *repos) criarAtividades(
	t *testing.T,
	p plano.Plano,
	c concurso.Concurso,
	as []plano.Atividade,
) []plano.Atividade {
	t.Helper()

	if err := r.cronograma.SubstituirAtividades(t.Context(), p.ID, as); err != nil {
		t.Fatalf("gravando atividades: %v", err)
	}

	lidas, err := r.cronograma.Atividades(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("lendo atividades: %v", err)
	}

	return lidas
}

// umDia devolve uma atividade de conteúdo pronta para gravar.
func umDia(d concurso.Disciplina, data time.Time, pos int, tema string) plano.Atividade {
	id := d.ID

	return plano.Atividade{
		Data: data, Posicao: pos, DisciplinaID: &id,
		Tema: tema, Passada: 1, Tipo: plano.AtividadeConteudo,
	}
}
