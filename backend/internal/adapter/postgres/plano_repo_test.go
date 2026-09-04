//go:build integration

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// O plano, sua configuração e o caderno de erros.

func TestPlanoRepo_RoundTripDaConfiguracao(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")

	p := plano.NovoPlano()
	p.UsuarioID = u.ID
	p.ConcursoID = c.ID
	p.Config = plano.ConfigPadrao()
	p.Config.Inicio = dia(2026, time.September, 1)
	p.Config.Prova = dia(2026, time.December, 15)
	p.Config.DiasEstudo = []int{1, 3, 5}
	p.Config.DiaRevisao = 6
	p.Config.RetaFinalDias = 21
	p.Config.BlocosPorDia = 4
	p.Config.MinutosBloco = 45
	p.Config.MinutosRevisao = 30
	p.Config.RevisaoSemanal = true
	p.Config.Simulados = plano.SimuladoQuinzenal
	p.Config.Discursiva = false
	p.Config.PctQuestoes = 0.7
	p.Config.LimiarFraco = 60
	p.Config.Questoes = map[string]int{"LINPO": 15, "BANDA": 20}
	p.Config.Modos = map[string]plano.Modo{"LINPO": plano.ModoQuestoes}
	p.Config.Reforcos = map[string]float64{"BANDA": 2}
	p.Config.CicloRevisao = []concurso.ItemRevisao{
		{Ordem: 0, Titulo: "Revisão ativa", Questoes: 30},
	}
	p.Config = p.Config.Normalizar()

	salvo, err := r.planos.Salvar(t.Context(), p)
	if err != nil {
		t.Fatalf("Salvar: %v", err)
	}

	lido, err := r.planos.PorUsuario(t.Context(), u.ID, c.ID)
	if err != nil {
		t.Fatalf("PorUsuario: %v", err)
	}

	if lido.ID != salvo.ID {
		t.Errorf("id = %s, quer %s", lido.ID, salvo.ID)
	}

	cfg := lido.Config

	// numeric, integer[], boolean e text voltando com o mesmo valor.
	if cfg.RetaFinalDias != 21 || cfg.BlocosPorDia != 4 || cfg.MinutosBloco != 45 {
		t.Errorf("inteiros não voltaram: %+v", cfg)
	}

	if cfg.PctQuestoes != 0.7 {
		t.Errorf("pctQuestoes = %v, quer 0.7 (numeric -> float)", cfg.PctQuestoes)
	}

	if !cfg.RevisaoSemanal || cfg.Discursiva {
		t.Errorf("booleanos trocados: semanal=%v discursiva=%v",
			cfg.RevisaoSemanal, cfg.Discursiva)
	}

	if cfg.Simulados != plano.SimuladoQuinzenal {
		t.Errorf("simulados = %q", cfg.Simulados)
	}

	if len(cfg.DiasEstudo) != 3 || cfg.DiasEstudo[2] != 5 {
		t.Errorf("diasEstudo = %v, quer [1 3 5]", cfg.DiasEstudo)
	}

	// As questões viajam por CÓDIGO no domínio e por id no banco: o join precisa
	// devolver o que entrou.
	if cfg.Questoes["LINPO"] != 15 || cfg.Questoes["BANDA"] != 20 {
		t.Errorf("questões = %v", cfg.Questoes)
	}

	if cfg.Modos["LINPO"] != plano.ModoQuestoes {
		t.Errorf("modo de LINPO = %q", cfg.Modos["LINPO"])
	}

	if cfg.Reforcos["BANDA"] != 2 {
		t.Errorf("reforço de BANDA = %v, quer 2", cfg.Reforcos["BANDA"])
	}

	if len(cfg.CicloRevisao) != 1 || cfg.CicloRevisao[0].Titulo != "Revisão ativa" {
		t.Errorf("ciclo = %+v", cfg.CicloRevisao)
	}
}

// Salvar de novo ATUALIZA o mesmo plano: a UNIQUE (usuario_id, concurso_id)
// garante um plano por usuário por concurso.
func TestPlanoRepo_SalvarEUpsert(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")

	primeiro := r.criarPlano(t, u, c)

	segundo := primeiro
	segundo.Config.BlocosPorDia = 5
	segundo.ID = uuid.Nil // o service nem sempre carrega o id

	gravado, err := r.planos.Salvar(t.Context(), segundo)
	if err != nil {
		t.Fatalf("Salvar de novo: %v", err)
	}

	if gravado.ID != primeiro.ID {
		t.Errorf("criou um plano novo (%s) em vez de atualizar (%s)",
			gravado.ID, primeiro.ID)
	}

	var planos int
	if err := r.pool.QueryRow(t.Context(), `SELECT count(*) FROM planos`).Scan(&planos); err != nil {
		t.Fatalf("contando planos: %v", err)
	}

	if planos != 1 {
		t.Errorf("planos = %d, quer 1", planos)
	}
}

func TestPlanoRepo_NaoEncontrado(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")

	_, err := r.planos.PorUsuario(t.Context(), u.ID, uuid.New())
	if !errors.Is(err, plano.ErrNaoEncontrado) {
		t.Errorf("erro = %v, quer ErrNaoEncontrado", err)
	}
}

// Um plano pertence a UM usuário: o de outro não é encontrado.
func TestPlanoRepo_IsolaEntreUsuarios(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)

	ana := r.criarUsuario(t, "ana@x.com")
	bruno := r.criarUsuario(t, "bruno@x.com")

	c := r.criarConcurso(t, ana, "da-ana")
	r.criarPlano(t, ana, c)

	if _, err := r.planos.PorUsuario(t.Context(), bruno.ID, c.ID); !errors.Is(
		err, plano.ErrNaoEncontrado,
	) {
		t.Errorf("Bruno achou o plano da Ana: %v", err)
	}
}

func TestPlanoRepo_MarcarMarco(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	marco := c.Marcos[0].ID

	if err := r.planos.MarcarMarco(t.Context(), p.ID, marco, true); err != nil {
		t.Fatalf("MarcarMarco: %v", err)
	}

	lido, err := r.planos.PorUsuario(t.Context(), u.ID, c.ID)
	if err != nil {
		t.Fatalf("PorUsuario: %v", err)
	}

	if !lido.Marcos[marco] {
		t.Error("o marco não voltou marcado")
	}

	// Desmarcar é upsert, não uma segunda linha.
	if err := r.planos.MarcarMarco(t.Context(), p.ID, marco, false); err != nil {
		t.Fatalf("MarcarMarco desmarcando: %v", err)
	}

	lido, _ = r.planos.PorUsuario(t.Context(), u.ID, c.ID)
	if lido.Marcos[marco] {
		t.Error("o marco continuou marcado depois de desmarcar")
	}
}

// ParaLembrete carrega todo plano com o contato do dono, num número de consultas
// independente de quantos planos existem. É a consulta em lote do worker.
func TestPlanoRepo_ParaLembrete(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)

	ana := r.criarUsuario(t, "ana@x.com")
	bruno := r.criarUsuario(t, "bruno@x.com")

	cursoAna := r.criarConcurso(t, ana, "da-ana")
	cursoBruno := r.criarConcurso(t, bruno, "do-bruno")

	r.criarPlano(t, ana, cursoAna)
	r.criarPlano(t, bruno, cursoBruno)

	planos, err := r.planos.ParaLembrete(t.Context())
	if err != nil {
		t.Fatalf("ParaLembrete: %v", err)
	}

	if len(planos) != 2 {
		t.Fatalf("planos = %d, quer 2", len(planos))
	}

	// O join com usuarios traz o contato de cada dono.
	emails := map[string]bool{}
	for _, p := range planos {
		emails[p.Email] = true

		if p.Nome == "" {
			t.Errorf("plano sem nome do dono: %+v", p)
		}

		// A configuração vem preenchida: o worker gera o plano a partir dela.
		if len(p.Plano.Config.Questoes) == 0 {
			t.Errorf("plano de %s veio sem questões por disciplina", p.Email)
		}
	}

	if !emails["ana@x.com"] || !emails["bruno@x.com"] {
		t.Errorf("e-mails = %v", emails)
	}
}

func TestCadernoRepo_CriaAtualizaERemove(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	data := dia(2026, time.September, 1)
	disc := c.Disciplinas[0].ID

	a, err := r.caderno.CriarAnotacao(t.Context(), p.ID, plano.Anotacao{
		Data: &data, DisciplinaID: &disc, Tema: "Crase",
		Texto: "errei por pressa", Origem: plano.OrigemTEC,
		URL: "https://tec", Resolvido: false,
	})
	if err != nil {
		t.Fatalf("CriarAnotacao: %v", err)
	}

	if a.ID == uuid.Nil || a.CriadoEm.IsZero() {
		t.Errorf("a anotação criada voltou sem id/criado_em: %+v", a)
	}

	a.Texto = "revisado"
	a.Resolvido = true

	if _, err := r.caderno.AtualizarAnotacao(t.Context(), p.ID, a); err != nil {
		t.Fatalf("AtualizarAnotacao: %v", err)
	}

	lidas, err := r.caderno.Anotacoes(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("Anotacoes: %v", err)
	}

	if len(lidas) != 1 {
		t.Fatalf("anotações = %d, quer 1", len(lidas))
	}

	if lidas[0].Texto != "revisado" || !lidas[0].Resolvido {
		t.Errorf("a atualização não pegou: %+v", lidas[0])
	}

	if lidas[0].Origem != plano.OrigemTEC || lidas[0].DisciplinaID == nil {
		t.Errorf("campos perdidos no round-trip: %+v", lidas[0])
	}

	if err := r.caderno.RemoverAnotacao(t.Context(), p.ID, a.ID); err != nil {
		t.Fatalf("RemoverAnotacao: %v", err)
	}

	if lidas, _ := r.caderno.Anotacoes(t.Context(), p.ID); len(lidas) != 0 {
		t.Errorf("sobraram %d anotações", len(lidas))
	}
}

// Apagar a disciplina não apaga a anotação: ela é do estudante, e o texto
// continua valendo mesmo sem a matéria (FK ON DELETE SET NULL).
func TestCadernoRepo_AnotacaoSobreviveADisciplina(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "a@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	disc := c.Disciplinas[0].ID

	if _, err := r.caderno.CriarAnotacao(t.Context(), p.ID, plano.Anotacao{
		DisciplinaID: &disc, Texto: "anotação", Origem: plano.OrigemManual,
	}); err != nil {
		t.Fatalf("CriarAnotacao: %v", err)
	}

	// Remove a disciplina do concurso.
	c.Disciplinas = c.Disciplinas[1:]
	if _, err := r.concursos.Atualizar(t.Context(), c); err != nil {
		t.Fatalf("Atualizar: %v", err)
	}

	lidas, err := r.caderno.Anotacoes(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("Anotacoes: %v", err)
	}

	if len(lidas) != 1 {
		t.Fatalf("a anotação sumiu junto com a disciplina: %d restantes", len(lidas))
	}

	if lidas[0].DisciplinaID != nil {
		t.Error("a anotação devia ter ficado sem disciplina, não apontando para uma inexistente")
	}
}
