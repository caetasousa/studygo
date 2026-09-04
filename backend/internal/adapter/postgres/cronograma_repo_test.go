//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// O cronograma materializado e os registros: as constraints que sustentam as
// invariantes do produto.

func TestCronogramaRepo_RoundTrip(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	data := dia(2026, time.September, 1)

	lidas := r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(c.Disciplinas[0], data, 0, "Crase"),
		{
			Data: data, Posicao: 1, Tema: "Simulado",
			Passada: 1, Tipo: plano.AtividadeSimulado,
		},
	})

	if len(lidas) != 2 {
		t.Fatalf("atividades = %d, quer 2", len(lidas))
	}

	// O código da disciplina vem por JOIN; é assim que o domínio a identifica.
	if lidas[0].Disciplina != "LINPO" || lidas[0].DisciplinaID == nil {
		t.Errorf("atividade de conteúdo sem disciplina: %+v", lidas[0])
	}

	if !lidas[0].Data.Equal(data) {
		t.Errorf("data = %v, quer %v", lidas[0].Data, data)
	}

	// Atividade de dia inteiro não aponta para matéria nenhuma — o CHECK do
	// schema garante isso, e a leitura precisa refletir.
	if lidas[1].DisciplinaID != nil || lidas[1].Disciplina != "" {
		t.Errorf("atividade de dia inteiro com disciplina: %+v", lidas[1])
	}
}

// ORDER BY (data, posicao): a aplicação conta com essa ordem para desenhar o dia.
func TestCronogramaRepo_OrdenaPorDataEPosicao(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	d1 := dia(2026, time.September, 1)
	d2 := dia(2026, time.September, 2)

	// Gravadas fora de ordem de propósito.
	lidas := r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(c.Disciplinas[0], d2, 1, "quarto"),
		umDia(c.Disciplinas[0], d1, 1, "segundo"),
		umDia(c.Disciplinas[0], d2, 0, "terceiro"),
		umDia(c.Disciplinas[0], d1, 0, "primeiro"),
	})

	querem := []string{"primeiro", "segundo", "terceiro", "quarto"}

	for i, quer := range querem {
		if lidas[i].Tema != quer {
			t.Errorf("posição %d = %q, quer %q", i, lidas[i].Tema, quer)
		}
	}
}

// A UNIQUE (plano_id, data, posicao) é DEFERRABLE justamente para que um
// remanejamento passe por estados intermediários dentro da transação.
func TestCronogramaRepo_TrocaDePosicoesNaoTropecaNaUnique(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	data := dia(2026, time.September, 1)

	lidas := r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(c.Disciplinas[0], data, 0, "A"),
		umDia(c.Disciplinas[1], data, 1, "B"),
	})

	lidas[0].Posicao, lidas[1].Posicao = 1, 0

	if err := r.cronograma.SubstituirAtividades(t.Context(), p.ID, lidas); err != nil {
		t.Fatalf("trocando posições: %v", err)
	}

	depois, err := r.cronograma.Atividades(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("relendo: %v", err)
	}

	if depois[0].Tema != "B" {
		t.Errorf("na posição 0 ficou %q, quer B", depois[0].Tema)
	}
}

// Duas atividades nunca ocupam a mesma vaga do mesmo dia.
func TestCronogramaRepo_RecusaVagaDuplicada(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	data := dia(2026, time.September, 1)

	err := r.cronograma.SubstituirAtividades(t.Context(), p.ID, []plano.Atividade{
		umDia(c.Disciplinas[0], data, 0, "A"),
		umDia(c.Disciplinas[1], data, 0, "B"),
	})

	if err == nil {
		t.Fatal("duas atividades na mesma vaga deviam ser recusadas")
	}
}

// O registro é história: a FK RESTRICT impede que uma atividade já estudada
// simplesmente saia do cronograma.
func TestCronogramaRepo_NaoApagaAtividadeComRegistro(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	lidas := r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(c.Disciplinas[0], dia(2026, time.September, 1), 0, "A"),
	})

	questoes, acertos := 10, 8

	if err := r.cronograma.SalvarRegistro(t.Context(), p.ID, plano.RegistroAtividade{
		AtividadeID: lidas[0].ID,
		Questoes:    &questoes,
		Acertos:     &acertos,
		Concluido:   true,
	}); err != nil {
		t.Fatalf("SalvarRegistro: %v", err)
	}

	if err := r.cronograma.SubstituirAtividades(
		t.Context(), p.ID, []plano.Atividade{},
	); err == nil {
		t.Fatal("apagar atividade com registro devia ser recusado")
	}

	// E a recusa não pode deixar estado parcial: a transação inteira volta.
	registros, err := r.cronograma.Registros(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("Registros: %v", err)
	}

	if !registros.Concluida(lidas[0].ID) {
		t.Error("o registro sumiu depois de uma gravação recusada")
	}

	restantes, err := r.cronograma.Atividades(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("Atividades: %v", err)
	}

	if len(restantes) != 1 {
		t.Errorf("atividades = %d depois do rollback, quer 1", len(restantes))
	}
}

// Isolamento entre PLANOS, de verdade: dois planos, e uma atividade que existe
// mas pertence ao outro.
//
// A versão anterior deste teste passava um uuid aleatório, o que só provava
// "atividade inexistente" — nada dizia sobre pertencimento.
func TestCronogramaRepo_RegistroRecusaAtividadeDeOutroPlano(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")

	cursoA := r.criarConcurso(t, u, "concurso-a")
	cursoB := r.criarConcurso(t, u, "concurso-b")

	planoA := r.criarPlano(t, u, cursoA)
	planoB := r.criarPlano(t, u, cursoB)

	// A atividade EXISTE — só que no plano B.
	doB := r.criarAtividades(t, planoB, cursoB, []plano.Atividade{
		umDia(cursoB.Disciplinas[0], dia(2026, time.September, 1), 0, "do B"),
	})

	err := r.cronograma.SalvarRegistro(t.Context(), planoA.ID, plano.RegistroAtividade{
		AtividadeID: doB[0].ID,
		Concluido:   true,
	})

	if err == nil {
		t.Fatal("o plano A não pode registrar numa atividade do plano B")
	}

	// E nada foi gravado no plano B por tabela.
	registrosB, err := r.cronograma.Registros(t.Context(), planoB.ID)
	if err != nil {
		t.Fatalf("Registros do B: %v", err)
	}

	if len(registrosB) != 0 {
		t.Errorf("o plano B ganhou %d registros que ninguém pediu", len(registrosB))
	}
}

func TestCronogramaRepo_RegistroRecusaAtividadeInexistente(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	err := r.cronograma.SalvarRegistro(t.Context(), p.ID, plano.RegistroAtividade{
		AtividadeID: uuid.New(),
		Concluido:   true,
	})

	if err == nil {
		t.Fatal("registrar em atividade inexistente devia falhar")
	}
}

// Nulo e zero são coisas diferentes: "não lancei" contra "lancei zero".
func TestCronogramaRepo_RegistroPreservaNulos(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	lidas := r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(c.Disciplinas[0], dia(2026, time.September, 1), 0, "A"),
		umDia(c.Disciplinas[1], dia(2026, time.September, 1), 1, "B"),
	})

	zero := 0
	horas := 1.5

	// A primeira lança zero acertos; a segunda não lança nada.
	if err := r.cronograma.SalvarRegistro(t.Context(), p.ID, plano.RegistroAtividade{
		AtividadeID: lidas[0].ID, Horas: &horas, Questoes: &zero, Acertos: &zero,
	}); err != nil {
		t.Fatalf("SalvarRegistro: %v", err)
	}

	if err := r.cronograma.SalvarRegistro(t.Context(), p.ID, plano.RegistroAtividade{
		AtividadeID: lidas[1].ID, Concluido: true,
	}); err != nil {
		t.Fatalf("SalvarRegistro: %v", err)
	}

	registros, err := r.cronograma.Registros(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("Registros: %v", err)
	}

	comZero := registros[lidas[0].ID]
	if comZero.Questoes == nil || *comZero.Questoes != 0 {
		t.Errorf("questões = %v, quer 0 (não nil)", comZero.Questoes)
	}

	semNada := registros[lidas[1].ID]
	if semNada.Questoes != nil || semNada.Horas != nil {
		t.Errorf("o que não foi lançado devia continuar nil: %+v", semNada)
	}
}

// O registro é por atividade: gravar de novo ATUALIZA, não duplica.
func TestCronogramaRepo_SalvarRegistroEUpsert(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	lidas := r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(c.Disciplinas[0], dia(2026, time.September, 1), 0, "A"),
	})

	dez, vinte := 10, 20

	for _, q := range []*int{&dez, &vinte} {
		if err := r.cronograma.SalvarRegistro(t.Context(), p.ID, plano.RegistroAtividade{
			AtividadeID: lidas[0].ID, Questoes: q,
		}); err != nil {
			t.Fatalf("SalvarRegistro: %v", err)
		}
	}

	registros, err := r.cronograma.Registros(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("Registros: %v", err)
	}

	if len(registros) != 1 {
		t.Fatalf("registros = %d, quer 1 (upsert, não insert)", len(registros))
	}

	if got := registros[lidas[0].ID].Questoes; got == nil || *got != 20 {
		t.Errorf("questões = %v, quer 20 (o valor mais recente)", got)
	}
}

func TestCronogramaRepo_RegistroDoDia(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	data := dia(2026, time.September, 1)
	questoes, acertos := 30, 25

	if err := r.cronograma.SalvarRegistroDia(t.Context(), p.ID, plano.RegistroDia{
		Data: data, Nota: "dia puxado",
		RevisaoQuestoes: &questoes, RevisaoAcertos: &acertos,
	}); err != nil {
		t.Fatalf("SalvarRegistroDia: %v", err)
	}

	dias, err := r.cronograma.RegistrosDia(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("RegistrosDia: %v", err)
	}

	reg, ok := dias[data]
	if !ok {
		t.Fatalf("o registro do dia não voltou; vieram %d dias", len(dias))
	}

	if reg.Nota != "dia puxado" || reg.RevisaoQuestoes == nil || *reg.RevisaoQuestoes != 30 {
		t.Errorf("registro do dia = %+v", reg)
	}
}

// Limpar o histórico apaga registros de atividade E do dia, mas NÃO o
// cronograma: o plano continua de pé, só sem o que foi estudado.
func TestCronogramaRepo_ApagarRegistrosPreservaOCronograma(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	data := dia(2026, time.September, 1)

	lidas := r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(c.Disciplinas[0], data, 0, "A"),
	})

	if err := r.cronograma.SalvarRegistro(t.Context(), p.ID, plano.RegistroAtividade{
		AtividadeID: lidas[0].ID, Concluido: true,
	}); err != nil {
		t.Fatalf("SalvarRegistro: %v", err)
	}

	if err := r.cronograma.SalvarRegistroDia(t.Context(), p.ID, plano.RegistroDia{
		Data: data, Nota: "nota",
	}); err != nil {
		t.Fatalf("SalvarRegistroDia: %v", err)
	}

	if err := r.cronograma.ApagarRegistros(t.Context(), p.ID); err != nil {
		t.Fatalf("ApagarRegistros: %v", err)
	}

	registros, _ := r.cronograma.Registros(t.Context(), p.ID)
	if len(registros) != 0 {
		t.Errorf("sobraram %d registros de atividade", len(registros))
	}

	dias, _ := r.cronograma.RegistrosDia(t.Context(), p.ID)
	if len(dias) != 0 {
		t.Errorf("sobraram %d registros de dia", len(dias))
	}

	atividades, _ := r.cronograma.Atividades(t.Context(), p.ID)
	if len(atividades) != 1 {
		t.Errorf("o cronograma foi junto: %d atividades, quer 1", len(atividades))
	}
}

// Um plano só enxerga o próprio cronograma.
func TestCronogramaRepo_AtividadesIsolamPorPlano(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")

	cursoA := r.criarConcurso(t, u, "concurso-a")
	cursoB := r.criarConcurso(t, u, "concurso-b")
	planoA := r.criarPlano(t, u, cursoA)
	planoB := r.criarPlano(t, u, cursoB)

	r.criarAtividades(t, planoA, cursoA, []plano.Atividade{
		umDia(cursoA.Disciplinas[0], dia(2026, time.September, 1), 0, "do A"),
	})
	r.criarAtividades(t, planoB, cursoB, []plano.Atividade{
		umDia(cursoB.Disciplinas[0], dia(2026, time.September, 1), 0, "do B"),
		umDia(cursoB.Disciplinas[1], dia(2026, time.September, 2), 0, "do B 2"),
	})

	doA, err := r.cronograma.Atividades(t.Context(), planoA.ID)
	if err != nil {
		t.Fatalf("Atividades: %v", err)
	}

	if len(doA) != 1 || doA[0].Tema != "do A" {
		t.Errorf("o plano A enxergou %d atividades: %+v", len(doA), doA)
	}
}
