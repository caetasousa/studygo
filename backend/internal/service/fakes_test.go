package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/domain/usuario"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// Dublês em memória dos repositórios de que os casos de uso dependem.
//
// Eles existem para que a ORQUESTRAÇÃO seja testável rápido e sem Docker: o que
// a aplicação decide, em que ordem chama as portas, como trata erro. Toda
// regressão que já escapou nesta área morava aí, e nada disso é alcançável por
// um teste de domínio puro.
//
// O que estes fakes NÃO fazem, deliberadamente: reproduzir constraint,
// ordenação ou semântica relacional do PostgreSQL. Uma versão anterior deles
// chegava a devolver erros com nomes internos de constraint
// ("atividades_plano_data_posicao_key") para fingir equivalência com o banco.
// Isso é ilusão de cobertura: sem uma suíte de contrato rodando contra as duas
// implementações, não há paridade nenhuma a afirmar — só um segundo banco de
// dados, mal escrito, que envelhece sozinho.
//
// PK, FK, UNIQUE, CHECK, RESTRICT, transação, join, upsert e ORDER BY são
// testados no PostgreSQL de verdade, em `internal/adapter/postgres`
// (tag `integration`). Quando um teste de aplicação precisa provocar uma falha
// de persistência, ele usa um stub que devolve o erro do CONTRATO DA PORTA —
// ver `erroAoGravar` abaixo.

type fakeConcursos struct {
	c concurso.Concurso
}

func (f *fakeConcursos) ListarPorDono(context.Context, uuid.UUID) ([]concurso.Concurso, error) {
	return []concurso.Concurso{f.c}, nil
}

func (f *fakeConcursos) PorSlug(_ context.Context, slug string) (concurso.Concurso, error) {
	if slug != f.c.Slug {
		return concurso.Concurso{}, concurso.ErrNaoEncontrado
	}

	return f.c, nil
}

func (f *fakeConcursos) PorID(_ context.Context, id uuid.UUID) (concurso.Concurso, error) {
	if id != f.c.ID {
		return concurso.Concurso{}, concurso.ErrNaoEncontrado
	}

	return f.c, nil
}

func (f *fakeConcursos) Criar(_ context.Context, c concurso.Concurso) (concurso.Concurso, error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}

	f.c = c

	return c, nil
}

func (f *fakeConcursos) Atualizar(
	_ context.Context,
	c concurso.Concurso,
) (concurso.Concurso, error) {
	// O repositório de verdade gera id para disciplina nova; sem isso o
	// cronograma não teria a que se ligar.
	for i := range c.Disciplinas {
		if c.Disciplinas[i].ID == uuid.Nil {
			c.Disciplinas[i].ID = uuid.New()
		}
	}

	f.c = c

	return c, nil
}

func (f *fakeConcursos) Remover(context.Context, uuid.UUID) error { return nil }

func (f *fakeConcursos) DefinirCadernoURL(
	_ context.Context,
	_ uuid.UUID,
	codigo, url string,
) error {
	for i := range f.c.Disciplinas {
		if f.c.Disciplinas[i].Codigo == codigo {
			f.c.Disciplinas[i].CadernoURL = url

			return nil
		}
	}

	return concurso.ErrNaoEncontrado
}

type fakePlanos struct {
	p plano.Plano
}

func (f *fakePlanos) PorUsuario(context.Context, uuid.UUID, uuid.UUID) (plano.Plano, error) {
	if f.p.ID == uuid.Nil {
		return plano.Plano{}, plano.ErrNaoEncontrado
	}

	return f.p, nil
}

func (f *fakePlanos) Salvar(_ context.Context, p plano.Plano) (plano.Plano, error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}

	f.p = p

	return p, nil
}

func (f *fakePlanos) MarcarMarco(_ context.Context, _, marcoID uuid.UUID, cumprido bool) error {
	if f.p.Marcos == nil {
		f.p.Marcos = map[uuid.UUID]bool{}
	}

	f.p.Marcos[marcoID] = cumprido

	return nil
}

func (f *fakePlanos) ParaLembrete(context.Context) ([]port.PlanoDoUsuario, error) {
	return nil, nil
}

type fakeCronograma struct {
	atividades []plano.Atividade
	registros  plano.Registros
	dias       map[time.Time]plano.RegistroDia

	// gravacoes conta as chamadas a SubstituirAtividades, para que um teste
	// possa afirmar que um caminho de LEITURA não escreveu. É o spy.
	gravacoes int

	// erroAoGravar, quando definido, faz a próxima gravação falhar. É como um
	// teste de orquestração provoca uma falha de persistência sem fingir
	// conhecer as constraints do banco.
	erroAoGravar error
}

func novoCronograma() *fakeCronograma {
	return &fakeCronograma{
		registros: plano.Registros{},
		dias:      map[time.Time]plano.RegistroDia{},
	}
}

// Atividades devolve o cronograma na ordem (data, posição), que é o contrato da
// porta — a aplicação monta o dia contando com ela. Que o SQL de fato produza
// essa ordem é o que TestCronogramaRepo_OrdenaPorDataEPosicao verifica.
func (f *fakeCronograma) Atividades(
	context.Context,
	uuid.UUID,
) ([]plano.Atividade, error) {
	out := append([]plano.Atividade(nil), f.atividades...)

	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Data.Equal(out[j].Data) {
			return out[i].Data.Before(out[j].Data)
		}

		return out[i].Posicao < out[j].Posicao
	})

	return out, nil
}

// SubstituirAtividades guarda o cronograma novo.
//
// Ele NÃO valida vaga duplicada nem recusa apagar atividade com registro: são
// constraints do banco, e afirmá-las aqui seria reimplementar o PostgreSQL. Um
// teste que precise provocar essa recusa injeta `erroAoGravar`; quem prova que a
// constraint existe de verdade é a suíte de integração.
func (f *fakeCronograma) SubstituirAtividades(
	_ context.Context,
	_ uuid.UUID,
	as []plano.Atividade,
) error {
	f.gravacoes++

	if f.erroAoGravar != nil {
		return f.erroAoGravar
	}

	novas := make([]plano.Atividade, len(as))
	copy(novas, as)

	// O default da coluna: uma linha inserida sem id ganha um. Isto não é
	// constraint — é o contrato de SubstituirAtividades, que a aplicação usa
	// para materializar o cronograma.
	for i := range novas {
		if novas[i].ID == uuid.Nil {
			novas[i].ID = uuid.New()
		}
	}

	f.atividades = novas

	return nil
}

func (f *fakeCronograma) Registros(context.Context, uuid.UUID) (plano.Registros, error) {
	out := plano.Registros{}
	for k, v := range f.registros {
		out[k] = v
	}

	return out, nil
}

// SalvarRegistro grava o lançamento de uma atividade.
//
// Devolver ErrAtividadeNaoEncontrada para um id ausente não é imitação de FK: é
// o CONTRATO da porta, que o service usa para decidir entre 404 e 500. O
// repository real chega ao mesmo erro por outro caminho, e é a suíte de
// integração que prova isso.
func (f *fakeCronograma) SalvarRegistro(
	_ context.Context,
	_ uuid.UUID,
	r plano.RegistroAtividade,
) error {
	for _, a := range f.atividades {
		if a.ID == r.AtividadeID {
			f.registros[r.AtividadeID] = r

			return nil
		}
	}

	return plano.ErrAtividadeNaoEncontrada
}

func (f *fakeCronograma) ApagarRegistros(context.Context, uuid.UUID) error {
	f.registros = plano.Registros{}
	f.dias = map[time.Time]plano.RegistroDia{}

	return nil
}

func (f *fakeCronograma) RegistrosDia(
	context.Context,
	uuid.UUID,
) (map[time.Time]plano.RegistroDia, error) {
	out := map[time.Time]plano.RegistroDia{}
	for k, v := range f.dias {
		out[k] = v
	}

	return out, nil
}

func (f *fakeCronograma) SalvarRegistroDia(
	_ context.Context,
	_ uuid.UUID,
	r plano.RegistroDia,
) error {
	f.dias[plano.DayOf(r.Data)] = r

	return nil
}

type fakeCaderno struct {
	anotacoes []plano.Anotacao
}

func (f *fakeCaderno) Anotacoes(context.Context, uuid.UUID) ([]plano.Anotacao, error) {
	return append([]plano.Anotacao(nil), f.anotacoes...), nil
}

func (f *fakeCaderno) CriarAnotacao(
	_ context.Context,
	_ uuid.UUID,
	a plano.Anotacao,
) (plano.Anotacao, error) {
	a.ID = uuid.New()
	f.anotacoes = append(f.anotacoes, a)

	return a, nil
}

func (f *fakeCaderno) AtualizarAnotacao(
	_ context.Context,
	_ uuid.UUID,
	a plano.Anotacao,
) (plano.Anotacao, error) {
	for i := range f.anotacoes {
		if f.anotacoes[i].ID == a.ID {
			f.anotacoes[i] = a

			return a, nil
		}
	}

	return plano.Anotacao{}, fmt.Errorf("anotação %s não encontrada", a.ID)
}

func (f *fakeCaderno) RemoverAnotacao(_ context.Context, _, id uuid.UUID) error {
	out := f.anotacoes[:0]

	for _, a := range f.anotacoes {
		if a.ID != id {
			out = append(out, a)
		}
	}

	f.anotacoes = out

	return nil
}

type fakeUsuarios struct {
	u usuario.Usuario
}

func (f *fakeUsuarios) Criar(_ context.Context, u usuario.Usuario) (usuario.Usuario, error) {
	u.ID = uuid.New()
	f.u = u

	return u, nil
}

func (f *fakeUsuarios) PorEmail(context.Context, string) (usuario.Usuario, error) {
	return f.u, nil
}

func (f *fakeUsuarios) PorID(context.Context, uuid.UUID) (usuario.Usuario, error) {
	return f.u, nil
}

func (f *fakeUsuarios) DefinirTema(_ context.Context, _ uuid.UUID, tema usuario.Tema) error {
	f.u.TemaUI = tema

	return nil
}

func (f *fakeUsuarios) GuardarRefreshToken(
	context.Context, uuid.UUID, string, time.Time,
) error {
	return nil
}

func (f *fakeUsuarios) RefreshTokenValido(context.Context, string) (uuid.UUID, error) {
	return f.u.ID, nil
}

func (f *fakeUsuarios) RevogarRefreshToken(context.Context, string) error { return nil }

// relogioFixo prende o "agora" para que a numeração dos dias seja determinística.
type relogioFixo struct{ t time.Time }

func (r relogioFixo) Now() time.Time { return r.t }
