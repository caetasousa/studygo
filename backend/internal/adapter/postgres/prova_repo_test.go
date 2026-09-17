//go:build integration

package postgres_test

import (
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	"studygo/internal/adapter/postgres"
	"studygo/internal/domain/prova"
	"studygo/internal/domain/usuario"
	"studygo/internal/platform/pgtest"
	"studygo/internal/port"

	"github.com/google/uuid"
)

func novoCurador(t *testing.T, repo *postgres.UsuarioRepo) string {
	t.Helper()

	u, err := repo.Criar(t.Context(), usuario.Usuario{
		Email: uuid.NewString() + "@example.com", Nome: "Curador", SenhaHash: "x", TemaUI: usuario.TemaEscuro,
	})
	if err != nil {
		t.Fatal(err)
	}

	return u.ID.String()
}

func novaImportacao(criador, hash string) prova.Importacao {
	return prova.Importacao{
		ID: uuid.NewString(), Criador: criador, Hash: hash, Documento: uuid.NewString(),
		Estado: prova.EstadoNaFila, Rascunho: prova.Rascunho{Banca: "FCC"},
	}
}

// rascunhoPublicavel tem uma questão com recorte, para que a publicação
// carregue arquivos além do PDF.
func rascunhoPublicavel(recorte string) prova.Rascunho {
	q := prova.Questao{
		Numero: 1, Disciplina: "Redes", Completa: true, Revisada: true,
		Blocos: []prova.Bloco{
			{Tipo: "texto", Texto: "Enunciado"},
			{Tipo: "imagem", Arquivo: recorte, Origem: &prova.Origem{Pagina: 1, Retangulo: []float64{0, 0, 10, 10}}},
		},
	}
	for _, l := range []string{"A", "B", "C", "D", "E"} {
		q.Alternativas = append(q.Alternativas, prova.Alternativa{Letra: l, Blocos: []prova.Bloco{{Tipo: "texto", Texto: l}}})
	}

	return prova.Rascunho{
		Banca: "FCC", Orgao: "TJCE", Ano: 2026, Cargo: "E05", CargoNome: "Analista Judiciário – Infraestrutura de TI",
		Caderno: "004", Total: 1,
		Questoes: []prova.Questao{q},
	}
}

// levarARevisao passa a importação pela fila até em_revisao, como o worker.
func levarARevisao(t *testing.T, repo *postgres.ProvaRepo, r prova.Rascunho) prova.Importacao {
	t.Helper()

	i, err := repo.Reservar(t.Context())
	if err != nil {
		t.Fatalf("Reservar: %v", err)
	}
	i.Rascunho = r
	i.Estado = prova.EstadoEmRevisao
	if err := repo.ConcluirEtapa(t.Context(), i, prova.EtapaPreparar, map[string]int{"ok": 1}, time.Second); err != nil {
		t.Fatalf("ConcluirEtapa: %v", err)
	}
	i, err = repo.Obter(t.Context(), i.ID)
	if err != nil {
		t.Fatal(err)
	}

	return i
}

func TestProvas_ReenvioECancelamento(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))

	primeira, err := repo.Criar(ctx, novaImportacao(criador, "h1"), 2)
	if err != nil {
		t.Fatal(err)
	}

	// Os mesmos PDFs acham a importação existente.
	reenvio, err := repo.Criar(ctx, novaImportacao(criador, "h1"), 2)
	if err != nil || reenvio.ID != primeira.ID {
		t.Fatalf("reenvio criou outra importação: %v %v", reenvio.ID, err)
	}

	// Cancelada, ela libera o hash: o curador pode recomeçar do zero.
	primeira.Cancelar()
	if err := repo.Salvar(ctx, primeira, primeira.Versao); err != nil {
		t.Fatal(err)
	}
	recomeco, err := repo.Criar(ctx, novaImportacao(criador, "h1"), 2)
	if err != nil || recomeco.ID == primeira.ID {
		t.Fatalf("hash de importação cancelada não foi liberado: %v", err)
	}

	// O limite conta as pendentes do criador.
	if _, err := repo.Criar(ctx, novaImportacao(criador, "h2"), 2); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Criar(ctx, novaImportacao(criador, "h3"), 2); !errors.Is(err, prova.ErrLimite) {
		t.Fatalf("err = %v, quer ErrLimite", err)
	}
}

// A conferência de prova repetida compara com as importações ativas da mesma
// banca e ano; a que para na capa fica cancelada, com o motivo, e segura o
// hash — o reenvio dos mesmos PDFs cai nela, e não numa importação nova.
func TestProvas_RepetidaParaNaCapaESeguraOHash(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	criar := func(hash string, ano int) prova.Importacao {
		t.Helper()
		i := novaImportacao(criador, hash)
		i.Rascunho = prova.Rascunho{Banca: "FCC", Orgao: "TRT15", Ano: ano, Cargo: "28"}
		criada, err := repo.Criar(ctx, i, 10)
		if err != nil {
			t.Fatal(err)
		}
		return criada
	}

	// A mais antiga é a próxima da fila: é a que chega à capa.
	nova := criar("nova", 2025)
	ativa := criar("ativa", 2025)
	criar("outro-ano", 2024)
	cancelada := criar("cancelada", 2025)
	cancelada.Cancelar()
	if err := repo.Salvar(ctx, cancelada, cancelada.Versao); err != nil {
		t.Fatal(err)
	}

	job, err := repo.Reservar(ctx)
	if err != nil || job.ID != nova.ID {
		t.Fatalf("reservou %v (%v), quer a nova", job.ID, err)
	}
	outras, err := repo.ImportacoesAtivasDoAno(ctx, "fcc", 2025, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(outras) != 1 || outras[0].ID != ativa.ID || outras[0].Rascunho.Cargo != "28" {
		t.Fatalf("ativas do ano = %+v, quer só a ativa de 2025", outras)
	}

	job.JaImportada(outras[0].Rascunho, false)
	if err := repo.ConcluirEtapa(ctx, job, prova.EtapaMetadados, nil, 0); err != nil {
		t.Fatal(err)
	}
	gravada, err := repo.Obter(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gravada.Estado != prova.EstadoCancelada || gravada.Erro != job.Erro || gravada.Erro == "" {
		t.Fatalf("estado = %s, erro = %q; quer cancelada com o motivo", gravada.Estado, gravada.Erro)
	}
	reenvio, err := repo.Criar(ctx, novaImportacao(criador, "nova"), 10)
	if err != nil || reenvio.ID != job.ID {
		t.Fatalf("o reenvio abriu outra importação: %v %v", reenvio.ID, err)
	}
}

// O nome com que o curador enviou cada PDF vai e volta do banco — na
// importação, no resumo da lista e na troca do gabarito.
func TestProvas_NomeDosArquivos(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	i := novaImportacao(criador, "nomes")
	i.GabaritoArquivo = uuid.NewString()
	i.NomeDocumento, i.NomeGabarito = "fcc-2025-trt-15-prova.pdf", "fcc-2025-trt-15-gabarito.pdf"

	criada, err := repo.Criar(ctx, i, 5)
	if err != nil {
		t.Fatal(err)
	}
	criada.NomeGabarito = "definitivo.pdf"
	if err := repo.Salvar(ctx, criada, criada.Versao); err != nil {
		t.Fatal(err)
	}

	lida, err := repo.Obter(ctx, i.ID)
	if err != nil || lida.NomeDocumento != "fcc-2025-trt-15-prova.pdf" || lida.NomeGabarito != "definitivo.pdf" {
		t.Fatalf("importação lida: %q, %q (%v)", lida.NomeDocumento, lida.NomeGabarito, err)
	}
	resumos, err := repo.ListarResumos(ctx)
	if err != nil || len(resumos) != 1 || resumos[0].NomeDocumento != "fcc-2025-trt-15-prova.pdf" {
		t.Fatalf("resumos = %+v (%v)", resumos, err)
	}
}

func TestProvas_FilaAceitaSoATentativaQueDetemAReserva(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	if _, err := repo.Criar(ctx, novaImportacao(criador, "a"), 5); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Criar(ctx, novaImportacao(criador, "b"), 5); err != nil {
		t.Fatal(err)
	}

	job, err := repo.Reservar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Reservar(ctx); !errors.Is(err, prova.ErrNaoEncontrada) {
		t.Fatalf("duas importações em processamento ao mesmo tempo: %v", err)
	}

	vencida := job
	vencida.Tentativa = uuid.NewString()
	vencida.Estado = prova.EstadoEmRevisao
	if err := repo.ConcluirEtapa(ctx, vencida, 0, nil, 0); !errors.Is(err, prova.ErrConflito) {
		t.Fatalf("tentativa sem reserva gravou: %v", err)
	}

	// O curador cancelou no meio da etapa: o resultado que chega depois é
	// descartado, e a revisão humana fica.
	atual, err := repo.Obter(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	atual.Estado = prova.EstadoCancelada
	if err := repo.Salvar(ctx, atual, atual.Versao); err != nil {
		t.Fatal(err)
	}
	job.Estado = prova.EstadoNaFila
	if err := repo.ConcluirEtapa(ctx, job, 0, nil, 0); !errors.Is(err, prova.ErrConflito) {
		t.Fatalf("etapa aceita depois do cancelamento: %v", err)
	}

	// Com a reserva livre, a próxima importação anda.
	if _, err := repo.Reservar(ctx); err != nil {
		t.Fatalf("fila travada depois do cancelamento: %v", err)
	}
}

func TestProvas_FalhaTransitoriaVoltaAFilaComEspera(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	criada, err := repo.Criar(ctx, novaImportacao(criador, "f"), 2)
	if err != nil {
		t.Fatal(err)
	}

	job, err := repo.Reservar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Falhar(ctx, job, "Gemini sobrecarregado", 15*time.Second); err != nil {
		t.Fatal(err)
	}

	i, err := repo.Obter(ctx, criada.ID)
	if err != nil {
		t.Fatal(err)
	}
	if i.Estado != prova.EstadoNaFila || i.Falhas != 1 || i.Erro == "" {
		t.Fatalf("importação = %+v", i)
	}
	// A espera ainda não passou: ninguém pega a importação agora.
	if _, err := repo.Reservar(ctx); !errors.Is(err, prova.ErrNaoEncontrada) {
		t.Fatalf("importação reservada antes da espera: %v", err)
	}
}

func TestProvas_PublicacaoRevisaoERetirada(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	criada, err := repo.Criar(ctx, novaImportacao(criador, "p"), 2)
	if err != nil {
		t.Fatal(err)
	}
	recorte := uuid.NewString()
	i := levarARevisao(t, repo, rascunhoPublicavel(recorte))

	if err := repo.Salvar(ctx, i, i.Versao-1); !errors.Is(err, prova.ErrConflito) {
		t.Fatalf("gravou com versão velha: %v", err)
	}
	if _, err := repo.Arquivo(ctx, recorte, false); !errors.Is(err, prova.ErrNaoEncontrada) {
		t.Fatalf("recorte de rascunho visível a estudante: %v", err)
	}
	if _, err := repo.Arquivo(ctx, recorte, true); err != nil {
		t.Fatalf("curador não vê o recorte do rascunho: %v", err)
	}

	id, err := repo.Publicar(ctx, i, criador)
	if err != nil {
		t.Fatal(err)
	}
	if repetida, err := repo.Publicar(ctx, i, criador); err != nil || repetida != id {
		t.Fatalf("publicar de novo criou outra prova: %v %v", repetida, err)
	}
	if ext, err := repo.Arquivo(ctx, recorte, false); err != nil || ext != "png" {
		t.Fatalf("recorte publicado indisponível: %q %v", ext, err)
	}

	cat, err := repo.Catalogo(ctx, port.FiltroCatalogo{Orgao: "tjce", Disciplina: "rede", Limite: 20})
	if err != nil || len(cat) != 1 || cat[0].Conteudo.Questoes != nil {
		t.Fatalf("catálogo = %+v, %v", cat, err)
	}
	if cat, _ := repo.Catalogo(ctx, port.FiltroCatalogo{Ano: "2025", Limite: 20}); len(cat) != 0 {
		t.Fatalf("filtro de ano ignorado: %+v", cat)
	}
	// O aluno procura o cargo pelo nome; o código também serve.
	for _, cargo := range []string{"infraestrutura", "e05"} {
		if cat, _ := repo.Catalogo(ctx, port.FiltroCatalogo{Cargo: cargo, Limite: 20}); len(cat) != 1 {
			t.Fatalf("filtro de cargo %q: %+v", cargo, cat)
		}
	}
	qs, err := repo.Questoes(ctx, id, 1, "")
	if err != nil || len(qs) != 1 || qs[0].Blocos[1].Arquivo != recorte || qs[0].Blocos[1].Origem == nil {
		t.Fatalf("questões = %+v, %v", qs, err)
	}
	// Publicada, a importação fica com a identificação: as questões moram na revisão.
	if origem, err := repo.Obter(ctx, criada.ID); err != nil || len(origem.Rascunho.Questoes) != 0 || origem.Rascunho.Orgao != "TJCE" {
		t.Fatalf("importação publicada = %+v, %v", origem.Rascunho, err)
	}

	// Uma revisão nova herda os arquivos, e a publicada segue no catálogo.
	base, err := repo.ImportacaoDaPublicacao(ctx, id)
	if err != nil || base.ID != criada.ID {
		t.Fatalf("origem da publicação = %v, %v", base.ID, err)
	}
	publicada, err := repo.Publicacao(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	revisao := novaImportacao(criador, "")
	revisao.Estado, revisao.ProvaID, revisao.Documento = prova.EstadoEmRevisao, id, base.Documento
	revisao.Rascunho = publicada.Conteudo
	revisao, err = repo.Criar(ctx, revisao, 2)
	if err != nil {
		t.Fatal(err)
	}
	herdados, err := repo.ArquivosDaImportacao(ctx, revisao.ID)
	if err != nil || !slices.Contains(herdados, recorte) {
		t.Fatalf("revisão sem os arquivos da publicação: %v %v", herdados, err)
	}
	segunda, err := repo.Publicar(ctx, revisao, criador)
	if err != nil || segunda != id {
		t.Fatalf("revisão virou outra prova: %v %v", segunda, err)
	}
	if p, err := repo.Publicacao(ctx, id); err != nil || p.Revisao != 2 || len(p.Conteudo.Questoes) != 1 {
		t.Fatalf("publicação = %+v, %v; quer revisão 2 com a questão", p, err)
	}
	// A revisão nova repete a questão sem gravar o conteúdo de novo.
	var conteudos, lugares int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM provas_questoes_conteudo), (SELECT count(*) FROM provas_questoes)`).
		Scan(&conteudos, &lugares); err != nil || conteudos != 1 || lugares != 2 {
		t.Fatalf("conteúdos = %d, lugares = %d, %v; quer 1 e 2", conteudos, lugares, err)
	}

	if err := repo.Retirar(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Arquivo(ctx, recorte, false); !errors.Is(err, prova.ErrNaoEncontrada) {
		t.Fatalf("arquivo de prova retirada visível: %v", err)
	}
	if _, err := repo.Publicacao(ctx, id); !errors.Is(err, prova.ErrNaoEncontrada) {
		t.Fatalf("prova retirada no catálogo: %v", err)
	}
}

// A questão de Conhecimentos Gerais é a mesma nos dois cargos: cada prova
// guarda o lugar dela — resposta, posição no seu PDF — e o conteúdo é um só.
func TestProvas_QuestaoComumAOutroCargoGuardadaUmaVez(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	recorte := uuid.NewString()

	if _, err := repo.Criar(ctx, novaImportacao(criador, "e05"), 2); err != nil {
		t.Fatal(err)
	}
	e05, err := repo.Publicar(ctx, levarARevisao(t, repo, rascunhoPublicavel(recorte)), criador)
	if err != nil {
		t.Fatal(err)
	}

	f06 := novaImportacao(criador, "f06")
	if f06, err = repo.Criar(ctx, f06, 2); err != nil {
		t.Fatal(err)
	}
	r := rascunhoPublicavel(recorte)
	r.Cargo, r.CargoNome = "F06", "Analista Judiciário – Sistemas da Informação"
	r.Questoes[0].Blocos[1].Origem = &prova.Origem{Pagina: 2, Retangulo: []float64{900, 800, 1200, 1000}}
	r.Questoes[0].IgualA = "TJCE 2026 · E05, questão 1"
	i := levarARevisao(t, repo, r)
	if err := repo.RegistrarArquivo(ctx, i.ID, recorte, "png"); err != nil {
		t.Fatal(err)
	}
	f06ID, err := repo.Publicar(ctx, i, criador)
	if err != nil {
		t.Fatal(err)
	}

	var conteudos int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM provas_questoes_conteudo`).Scan(&conteudos); err != nil || conteudos != 1 {
		t.Fatalf("conteúdos = %d, %v; quer 1 para os dois cargos", conteudos, err)
	}
	p, err := repo.Publicacao(ctx, f06ID)
	if err != nil {
		t.Fatal(err)
	}
	if f := p.Conteudo.Questoes[0].Blocos[1]; f.Origem == nil || f.Origem.Pagina != 2 || p.Conteudo.Questoes[0].IgualA == "" {
		t.Fatalf("a F06 perdeu o lugar dela: %+v", p.Conteudo.Questoes[0])
	}

	irmas, err := repo.ProvasDoAno(ctx, "fcc", 2026, f06ID)
	if err != nil || len(irmas) != 1 || irmas[0].ID != e05 || irmas[0].Conteudo.Orgao != "TJCE" {
		t.Fatalf("provas do ano = %+v, %v; quer só a E05", irmas, err)
	}
	if outras, _ := repo.ProvasDoAno(ctx, "FCC", 2025, ""); len(outras) != 0 {
		t.Fatalf("prova de outro ano entrou: %+v", outras)
	}
}

// A mesma prova importada duas vezes, com títulos diferentes: excluir uma apaga
// tudo o que veio dela — e só dela. A questão guardada uma vez para os dois
// cargos continua na outra.
func TestProvas_ExcluirProvaApagaSoOQueEDela(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	recorte := uuid.NewString()

	// A E05 tem a questão comum e uma só dela, gabarito e anotação.
	if _, err := repo.Criar(ctx, novaImportacao(criador, "e05"), 2); err != nil {
		t.Fatal(err)
	}
	r := rascunhoPublicavel(recorte)
	so := r.Questoes[0]
	so.Numero, so.Blocos = 2, []prova.Bloco{{Tipo: "texto", Texto: "Só da E05"}}
	r.Total, r.Questoes = 2, append(r.Questoes, so)
	r.Gabarito = prova.Gabarito{Cargo: "E05", Caderno: "4", Tipo: "definitivo", Respostas: map[string]string{"1": "A", "2": ""}}
	e05, err := repo.Publicar(ctx, levarARevisao(t, repo, r), criador)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SalvarAnotacao(ctx, criador, e05, prova.Anotacao{Numero: 1, Texto: "porque sim"}); err != nil {
		t.Fatal(err)
	}
	// E uma revisão aberta dela.
	revisao := novaImportacao(criador, "e05-revisao")
	if _, err := repo.Criar(ctx, revisao, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE provas_importacoes SET prova_id = $1, estado = 'em_revisao' WHERE id = $2`, e05, revisao.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.Criar(ctx, novaImportacao(criador, "f06"), 5); err != nil {
		t.Fatal(err)
	}
	f := rascunhoPublicavel(recorte)
	f.Cargo = "F06"
	f06, err := repo.Publicar(ctx, levarARevisao(t, repo, f), criador)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.ExcluirProva(ctx, uuid.NewString()); !errors.Is(err, prova.ErrNaoEncontrada) {
		t.Fatalf("prova que não existe: err = %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE provas_importacoes SET estado = 'processando' WHERE id = $1`, revisao.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.ExcluirProva(ctx, e05); !errors.Is(err, prova.ErrConflito) {
		t.Fatalf("com importação processando: err = %v, quer ErrConflito", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE provas_importacoes SET estado = 'em_revisao' WHERE id = $1`, revisao.ID); err != nil {
		t.Fatal(err)
	}

	if err := repo.ExcluirProva(ctx, e05); err != nil {
		t.Fatalf("ExcluirProva: %v", err)
	}

	var sobras int
	if err := pool.QueryRow(ctx,
		`SELECT (SELECT count(*) FROM provas WHERE id = $1)
		      + (SELECT count(*) FROM provas_revisoes WHERE prova_id = $1)
		      + (SELECT count(*) FROM provas_questoes WHERE prova_id = $1)
		      + (SELECT count(*) FROM provas_gabaritos WHERE prova_id = $1)
		      + (SELECT count(*) FROM provas_gabarito_respostas WHERE prova_id = $1)
		      + (SELECT count(*) FROM provas_anotacoes WHERE prova_id = $1)
		      + (SELECT count(*) FROM provas_importacoes WHERE prova_id = $1 OR id = $2)`,
		e05, revisao.ID,
	).Scan(&sobras); err != nil || sobras != 0 {
		t.Fatalf("sobrou %d linhas da E05 (%v)", sobras, err)
	}
	if _, err := repo.Publicacao(ctx, e05); !errors.Is(err, prova.ErrNaoEncontrada) {
		t.Fatalf("Publicacao da excluída: err = %v", err)
	}

	p, err := repo.Publicacao(ctx, f06)
	if err != nil || len(p.Conteudo.Questoes) != 1 || p.Conteudo.Questoes[0].Blocos[0].Texto != "Enunciado" {
		t.Fatalf("a F06 perdeu a questão comum: %+v, %v", p.Conteudo.Questoes, err)
	}
	var conteudos int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM provas_questoes_conteudo`).Scan(&conteudos); err != nil || conteudos != 1 {
		t.Fatalf("conteúdos = %d, %v; quer só o comum, que a F06 ainda usa", conteudos, err)
	}
}

// A prova trazida de outro ambiente nasce em revisão, sem passar pela fila, e é
// publicada na hora: a publicação e a base para abrir revisão ficam como as de
// uma importação comum, com o PDF e as figuras registrados.
func TestProvas_ImportacaoCriadaEmRevisaoPublica(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	recorte := uuid.NewString()

	i := novaImportacao(criador, "pacote")
	i.Estado, i.Rascunho = prova.EstadoEmRevisao, rascunhoPublicavel(recorte)
	i.Regioes = []prova.Origem{{Pagina: 1, Regiao: "0", Retangulo: []float64{0, 0, 595, 842}}}
	i.NomeDocumento = "trt18-prova.pdf"
	// O teto de pendentes não segura: essa importação sai para o catálogo já.
	criada, err := repo.Criar(ctx, i, math.MaxInt32)
	if err != nil || criada.ID != i.ID || criada.Estado != prova.EstadoEmRevisao {
		t.Fatalf("Criar: %+v, %v", criada, err)
	}
	id, err := repo.Publicar(ctx, criada, criador)
	if err != nil {
		t.Fatalf("Publicar: %v", err)
	}

	p, err := repo.Publicacao(ctx, id)
	if err != nil || len(p.Conteudo.Questoes) != 1 || p.Conteudo.Questoes[0].Blocos[1].Arquivo != recorte {
		t.Fatalf("publicada = %+v, %v", p.Conteudo.Questoes, err)
	}
	base, err := repo.ImportacaoDaPublicacao(ctx, id)
	if err != nil || base.ID != i.ID || len(base.Regioes) != 1 || base.Documento != i.Documento || base.NomeDocumento != "trt18-prova.pdf" {
		t.Fatalf("base da revisão = %+v, %v", base, err)
	}
	// A figura é servida a quem não é curador, porque está numa prova publicada.
	if ext, err := repo.Arquivo(ctx, recorte, false); err != nil || ext != "png" {
		t.Fatalf("figura: %q, %v", ext, err)
	}

	// O mesmo caderno de novo cai na importação que já existe.
	if outra, err := repo.Criar(ctx, novaImportacao(criador, "pacote"), math.MaxInt32); err != nil || outra.ID != i.ID {
		t.Fatalf("mesmo hash: %+v, %v", outra, err)
	}
}

// O título corrigido aparece no catálogo e na curadoria, sem abrir revisão.
func TestProvas_RenomearProva(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	if _, err := repo.Criar(ctx, novaImportacao(criador, "renomear"), 2); err != nil {
		t.Fatal(err)
	}
	i := levarARevisao(t, repo, rascunhoPublicavel(uuid.NewString()))
	id, err := repo.Publicar(ctx, i, criador)
	if err != nil {
		t.Fatal(err)
	}

	const titulo = "Técnico Judiciário – Tecnologia da Informação"
	if err := repo.RenomearProva(ctx, id, titulo); err != nil {
		t.Fatalf("RenomearProva: %v", err)
	}
	if err := repo.RenomearProva(ctx, uuid.NewString(), titulo); !errors.Is(err, prova.ErrNaoEncontrada) {
		t.Fatalf("prova que não existe: err = %v", err)
	}

	p, err := repo.Publicacao(ctx, id)
	if err != nil || p.Conteudo.CargoNome != titulo || p.Conteudo.Cargo != "E05" || len(p.Conteudo.Questoes) != 1 {
		t.Fatalf("publicada = %q (cargo %q, %d questões), %v", p.Conteudo.CargoNome, p.Conteudo.Cargo, len(p.Conteudo.Questoes), err)
	}
	resumos, err := repo.ListarResumos(ctx)
	if err != nil {
		t.Fatal(err)
	}
	k := slices.IndexFunc(resumos, func(r prova.Importacao) bool { return r.ID == i.ID })
	if k < 0 || resumos[k].Rascunho.CargoNome != titulo {
		t.Fatalf("curadoria não viu o título novo: %+v", resumos)
	}
}

// A questão comum aos dois cargos aparece uma vez no treino por matéria, e
// sempre pela prova que estreou primeiro — até ela sair do catálogo.
// O gabarito é uma entidade à parte: publicado, vai para as tabelas dele, e
// nem a questão publicada nem a revisão guardam a resposta. A questão lida
// volta com a resposta do gabarito, e a revisão aberta da publicada, com o
// gabarito inteiro.
func TestProvas_GabaritoSeparadoDasQuestoes(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	recorte := uuid.NewString()
	if _, err := repo.Criar(ctx, novaImportacao(criador, "gabarito"), 2); err != nil {
		t.Fatal(err)
	}
	r := rascunhoPublicavel(recorte)
	r.Questoes[0].Resposta, r.Questoes[0].Situacao = "D", "Gabarito sem alteração"
	r.Gabarito = prova.Gabarito{
		Cargo: "E05", Caderno: "4", Tipo: "preliminar",
		Respostas: map[string]string{"1": "D"}, Situacoes: map[string]string{"1": "Gabarito sem alteração"},
	}
	i := levarARevisao(t, repo, r)
	id, err := repo.Publicar(ctx, i, criador)
	if err != nil {
		t.Fatal(err)
	}

	var comResposta, revisaoComRespostas, importacaoComGabarito bool
	if err := pool.QueryRow(ctx,
		`SELECT (SELECT bool_or(lugar ? 'Resposta') FROM provas_questoes WHERE prova_id = $1),
		        (SELECT conteudo->'Gabarito'->'Respostas' <> 'null'::jsonb FROM provas_revisoes WHERE prova_id = $1),
		        (SELECT rascunho ? 'Gabarito' FROM provas_importacoes WHERE id = $2)`,
		id, i.ID,
	).Scan(&comResposta, &revisaoComRespostas, &importacaoComGabarito); err != nil {
		t.Fatal(err)
	}
	if comResposta || revisaoComRespostas || importacaoComGabarito {
		t.Fatalf("resposta fora do gabarito: questão %v, revisão %v, importação %v",
			comResposta, revisaoComRespostas, importacaoComGabarito)
	}

	questoes, err := repo.Questoes(ctx, id, 0, "")
	if err != nil || len(questoes) != 1 || questoes[0].Resposta != "D" || questoes[0].Situacao != "Gabarito sem alteração" {
		t.Fatalf("questões = %+v (%v); quer a resposta D do gabarito", questoes, err)
	}
	p, err := repo.Publicacao(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if g := p.Conteudo.Gabarito; g.Tipo != "preliminar" || g.Cargo != "E05" || g.Respostas["1"] != "D" {
		t.Fatalf("gabarito da publicação = %+v", g)
	}

	// Letra fora de A–E o banco recusa, mesmo que alguém passe da pendência.
	if _, err := pool.Exec(ctx,
		`INSERT INTO provas_gabarito_respostas (prova_id, revisao, numero, resposta) VALUES ($1, 1, 2, 'F')`, id,
	); err == nil {
		t.Fatal("o banco aceitou a resposta F")
	}
}

// A excluída não vai para as questões, mas continua no gabarito e na
// identificação: a revisão reaberta da publicada sabe que ela saiu.
func TestProvas_ExcluidaSobreviveAPublicacao(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	if _, err := repo.Criar(ctx, novaImportacao(criador, "anulada"), 2); err != nil {
		t.Fatal(err)
	}
	r := rascunhoPublicavel(uuid.NewString())
	r.Total, r.Excluidas = 2, []int{2}
	r.Questoes[0].Resposta = "D"
	r.Gabarito = prova.Gabarito{
		Cargo: "E05", Caderno: "4", Tipo: "definitivo",
		Respostas: map[string]string{"1": "D", "2": ""}, Situacoes: map[string]string{"2": "Anulada"},
	}
	if p := r.Pendencias(prova.Criterios{}); len(p) > 0 {
		t.Fatalf("rascunho do teste com pendências: %v", p)
	}
	id, err := repo.Publicar(ctx, levarARevisao(t, repo, r), criador)
	if err != nil {
		t.Fatal(err)
	}

	p, err := repo.Publicacao(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Conteudo; !slices.Equal(c.Excluidas, []int{2}) || c.QuestoesNaProva() != 1 || len(c.Questoes) != 1 {
		t.Fatalf("publicada: excluídas %v, %d na prova, %d questões", c.Excluidas, c.QuestoesNaProva(), len(c.Questoes))
	}
	if resposta, ok := p.Conteudo.Gabarito.Respostas["2"]; !ok || resposta != "" {
		t.Fatalf("gabarito da publicada = %+v, quer a 2 anulada", p.Conteudo.Gabarito)
	}
	if pend := p.Conteudo.Pendencias(prova.Criterios{}); len(pend) > 0 {
		t.Fatalf("a revisão reaberta nasceria com pendências: %v", pend)
	}
}

// Rascunho e revisão gravados quando só a anulada saía da prova guardam a
// chave AnuladasExcluidas. Lidos agora, a questão continua fora — senão
// faltaria, e a revisão reaberta não publicaria.
func TestProvas_ExcluidaNaChaveAntiga(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	if _, err := repo.Criar(ctx, novaImportacao(criador, "chave-antiga"), 2); err != nil {
		t.Fatal(err)
	}
	r := rascunhoPublicavel(uuid.NewString())
	r.Total, r.Excluidas = 2, []int{2}
	i := levarARevisao(t, repo, r)
	if _, err := pool.Exec(ctx,
		`UPDATE provas_importacoes
		    SET rascunho = jsonb_set(rascunho - 'Excluidas', '{AnuladasExcluidas}', rascunho->'Excluidas')
		  WHERE id = $1`, i.ID,
	); err != nil {
		t.Fatal(err)
	}

	i, err := repo.Obter(ctx, i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(i.Rascunho.Excluidas, []int{2}) {
		t.Fatalf("rascunho: excluídas %v, quer [2]", i.Rascunho.Excluidas)
	}
	id, err := repo.Publicar(ctx, i, criador)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE provas_revisoes
		    SET conteudo = jsonb_set(conteudo - 'Excluidas', '{AnuladasExcluidas}', conteudo->'Excluidas')
		  WHERE prova_id = $1`, id,
	); err != nil {
		t.Fatal(err)
	}

	p, err := repo.Publicacao(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Conteudo.Excluidas, []int{2}) || p.Conteudo.QuestoesNaProva() != 1 {
		t.Fatalf("publicada: excluídas %v, %d na prova", p.Conteudo.Excluidas, p.Conteudo.QuestoesNaProva())
	}
}

func TestProvas_QuestoesAvulsasUmaVezPorConteudo(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	recorte := uuid.NewString()

	if _, err := repo.Criar(ctx, novaImportacao(criador, "e05"), 2); err != nil {
		t.Fatal(err)
	}
	e05 := rascunhoPublicavel(recorte)
	e05.Questoes[0].Resposta = "C"
	e05.Gabarito = prova.Gabarito{Cargo: "E05", Caderno: "4", Respostas: map[string]string{"1": "C"}}
	e05ID, err := repo.Publicar(ctx, levarARevisao(t, repo, e05), criador)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := repo.Criar(ctx, novaImportacao(criador, "f06"), 2); err != nil {
		t.Fatal(err)
	}
	f06 := rascunhoPublicavel(recorte)
	f06.Cargo, f06.CargoNome, f06.Ano = "F06", "Analista Judiciário – Sistemas", 2025
	propria := f06.Questoes[0]
	propria.Numero, propria.Disciplina, propria.Assunto = 2, "Língua Portuguesa", "Crase"
	propria.Blocos = []prova.Bloco{{Tipo: "texto", Texto: "Só da F06"}}
	f06.Questoes = append(f06.Questoes, propria)
	i := levarARevisao(t, repo, f06)
	if err := repo.RegistrarArquivo(ctx, i.ID, recorte, "png"); err != nil {
		t.Fatal(err)
	}
	f06ID, err := repo.Publicar(ctx, i, criador)
	if err != nil {
		t.Fatal(err)
	}

	avulsas, err := repo.QuestoesAvulsas(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []prova.QuestaoAvulsa{
		{ProvaID: e05ID, Numero: 1, Disciplina: "Redes", Resposta: "C", Orgao: "TJCE", Ano: 2026, Cargo: "E05",
			CargoNome: "Analista Judiciário – Infraestrutura de TI"},
		{ProvaID: f06ID, Numero: 2, Disciplina: "Língua Portuguesa", Assunto: "Crase", Orgao: "TJCE", Ano: 2025, Cargo: "F06",
			CargoNome: "Analista Judiciário – Sistemas"},
	}
	if !slices.Equal(avulsas, want) {
		t.Fatalf("avulsas = %+v\nquer %+v", avulsas, want)
	}

	// Fora do catálogo a E05, a questão comum continua no treino, pela F06.
	if err := repo.Retirar(ctx, e05ID); err != nil {
		t.Fatal(err)
	}
	avulsas, err = repo.QuestoesAvulsas(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(avulsas) != 2 || avulsas[0].ProvaID != f06ID || avulsas[0].Numero != 1 {
		t.Fatalf("avulsas depois de retirar a E05 = %+v", avulsas)
	}
}

func TestProvas_ListaDeResumosNaoTrazConteudo(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	if _, err := repo.Criar(ctx, novaImportacao(criador, "r"), 2); err != nil {
		t.Fatal(err)
	}
	levarARevisao(t, repo, rascunhoPublicavel(uuid.NewString()))

	lista, err := repo.ListarResumos(ctx)
	if err != nil || len(lista) != 1 {
		t.Fatalf("lista = %d, %v", len(lista), err)
	}
	if r := lista[0].Rascunho; r.Orgao != "TJCE" || r.Questoes != nil {
		t.Fatalf("resumo = %+v", r)
	}
}

// Excluir leva etapas e vínculos junto; o PDF fica sem dono e sai na limpeza.
// A reserva do worker muda o estado sem mudar a versão, e excluir com o estado
// velho é conflito — não se apaga o que está processando.
func TestProvas_ExcluirSoltaOsArquivos(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))
	criada, err := repo.Criar(ctx, novaImportacao(criador, "e"), 2)
	if err != nil {
		t.Fatal(err)
	}

	job, err := repo.Reservar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Excluir(ctx, criada); !errors.Is(err, prova.ErrConflito) {
		t.Fatalf("Excluir durante o processamento: err = %v, quer ErrConflito", err)
	}
	job.Etapa, job.Estado = prova.ProximaEtapa(prova.EtapaPreparar, 1)
	if err := repo.ConcluirEtapa(ctx, job, prova.EtapaPreparar, map[string]int{"regioes": 1}, time.Second); err != nil {
		t.Fatal(err)
	}
	parada, err := repo.Obter(ctx, criada.ID)
	if err != nil || parada.Estado != prova.EstadoNaFila {
		t.Fatalf("depois da etapa: estado = %q, err = %v", parada.Estado, err)
	}

	if err := repo.Excluir(ctx, parada); err != nil {
		t.Fatalf("Excluir: %v", err)
	}

	if _, err := repo.Obter(ctx, criada.ID); !errors.Is(err, prova.ErrNaoEncontrada) {
		t.Fatalf("importação continua lá: %v", err)
	}
	var etapas int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM provas_etapas WHERE importacao_id = $1`, criada.ID,
	).Scan(&etapas); err != nil || etapas != 0 {
		t.Fatalf("etapas que sobraram = %d, err = %v", etapas, err)
	}
	orfaos, err := repo.LimparReferencias(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(orfaos, criada.Documento+".pdf") {
		t.Fatalf("PDF da importação excluída não ficou sem dono: %v", orfaos)
	}
}

// Rascunho cancelado e parado solta os arquivos, que voltam como órfãos para
// sair do volume. Os de prova publicada nunca.
func TestProvas_LimpezaSoSoltaCanceladas(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	criador := novoCurador(t, postgres.NewUsuarioRepo(pool))

	cancelada, err := repo.Criar(ctx, novaImportacao(criador, "c"), 5)
	if err != nil {
		t.Fatal(err)
	}
	cancelada.Estado = prova.EstadoCancelada
	if err := repo.Salvar(ctx, cancelada, cancelada.Versao); err != nil {
		t.Fatal(err)
	}
	ativa, err := repo.Criar(ctx, novaImportacao(criador, "a"), 5)
	if err != nil {
		t.Fatal(err)
	}

	orfaos, err := repo.LimparReferencias(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Contains(orfaos, cancelada.Documento+".pdf") {
		t.Fatalf("PDF da cancelada não foi solto: %v", orfaos)
	}
	if slices.Contains(orfaos, ativa.Documento+".pdf") {
		t.Fatalf("PDF de importação ativa foi solto: %v", orfaos)
	}
}

// A anotação é por estudante, prova e número: gravar de novo substitui, outro
// estudante não vê, apagar tira.
func TestProvas_AnotacoesDoEstudante(t *testing.T) {
	t.Parallel()

	pool := pgtest.Novo(t)
	ctx := t.Context()
	repo := postgres.NewProvaRepo(pool)
	usuarios := postgres.NewUsuarioRepo(pool)
	criador, outro := novoCurador(t, usuarios), novoCurador(t, usuarios)
	if _, err := repo.Criar(ctx, novaImportacao(criador, "an"), 2); err != nil {
		t.Fatal(err)
	}
	provaID, err := repo.Publicar(ctx, levarARevisao(t, repo, rascunhoPublicavel(uuid.NewString())), criador)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := repo.SalvarAnotacao(ctx, criador, provaID, prova.Anotacao{Numero: 1, Texto: "primeira"}); err != nil {
		t.Fatal(err)
	}
	trocada, err := repo.SalvarAnotacao(ctx, criador, provaID, prova.Anotacao{Numero: 1, Texto: "## trocada"})
	if err != nil || trocada.AtualizadaEm.IsZero() {
		t.Fatalf("regravar = %+v, %v", trocada, err)
	}

	minhas, err := repo.Anotacoes(ctx, criador, provaID)
	if err != nil || len(minhas) != 1 || minhas[0].Texto != "## trocada" {
		t.Fatalf("anotações do criador = %+v, %v", minhas, err)
	}
	if dele, err := repo.Anotacoes(ctx, outro, provaID); err != nil || len(dele) != 0 {
		t.Fatalf("outro estudante vê a anotação: %+v, %v", dele, err)
	}

	if err := repo.ExcluirAnotacao(ctx, criador, provaID, 1); err != nil {
		t.Fatal(err)
	}
	if minhas, _ := repo.Anotacoes(ctx, criador, provaID); len(minhas) != 0 {
		t.Fatalf("anotação apagada continua: %+v", minhas)
	}
}
