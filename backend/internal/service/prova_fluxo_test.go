package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"studygo/internal/domain/prova"
	"studygo/internal/port"
)

// A orquestração da curadoria de provas: quem pode o quê, o que acontece com
// os arquivos quando a importação não nasce, e a fila andando etapa a etapa.
// A fila de verdade — reserva, tentativa vencida, versão — é testada no
// PostgreSQL, em adapter/postgres/prova_repo_test.go.

const curador = "11111111-1111-1111-1111-111111111111"

var pdfMinimo = []byte("%PDF-1.7 prova")

// fakeProvas guarda importações em memória e registra o que a fila fez.
type fakeProvas struct {
	port.ProvaRepository

	importacoes map[string]prova.Importacao
	existente   *prova.Importacao // Criar devolve esta, como num reenvio
	errCriar    error

	etapas  []int
	falhas  []time.Duration // espera, por falha registrada; zero é desistir
	arquivo map[string]string
	// irmas são as provas publicadas do mesmo concurso.
	irmas []prova.Publicacao
}

func (f *fakeProvas) ProvasDoAno(_ context.Context, _ string, _ int, exceto string) ([]prova.Publicacao, error) {
	var out []prova.Publicacao
	for _, p := range f.irmas {
		if p.ID != exceto {
			out = append(out, p)
		}
	}

	return out, nil
}

func (f *fakeProvas) ImportacoesAtivasDoAno(_ context.Context, banca string, ano int, exceto string) ([]prova.Importacao, error) {
	var out []prova.Importacao
	for id, i := range f.importacoes {
		ativa := i.Estado == prova.EstadoNaFila || i.Estado == prova.EstadoProcessando ||
			i.Estado == prova.EstadoEmRevisao || i.Estado == prova.EstadoFalhou
		if ativa && id != exceto && strings.EqualFold(i.Rascunho.Banca, banca) && i.Rascunho.Ano == ano {
			out = append(out, i)
		}
	}

	return out, nil
}

func (f *fakeProvas) Publicacao(_ context.Context, id string) (prova.Publicacao, error) {
	for _, p := range f.irmas {
		if p.ID == id {
			return p, nil
		}
	}
	return prova.Publicacao{}, prova.ErrNaoEncontrada
}

func novoFakeProvas() *fakeProvas {
	return &fakeProvas{importacoes: map[string]prova.Importacao{}, arquivo: map[string]string{}}
}

func (f *fakeProvas) Criar(_ context.Context, i prova.Importacao, _ int) (prova.Importacao, error) {
	if f.errCriar != nil {
		return prova.Importacao{}, f.errCriar
	}
	if f.existente != nil {
		return *f.existente, nil
	}
	i.Versao = 1
	f.importacoes[i.ID] = i

	return i, nil
}

func (f *fakeProvas) Obter(_ context.Context, id string) (prova.Importacao, error) {
	i, ok := f.importacoes[id]
	if !ok {
		return prova.Importacao{}, prova.ErrNaoEncontrada
	}

	return i, nil
}

func (f *fakeProvas) Salvar(_ context.Context, i prova.Importacao, _ int) error {
	i.Versao++
	f.importacoes[i.ID] = i

	return nil
}

func (f *fakeProvas) Excluir(_ context.Context, i prova.Importacao) error {
	atual, ok := f.importacoes[i.ID]
	if !ok || atual.Versao != i.Versao || atual.Estado != i.Estado {
		return prova.ErrConflito
	}
	delete(f.importacoes, i.ID)

	return nil
}

func (f *fakeProvas) Reservar(context.Context) (prova.Importacao, error) {
	for id, i := range f.importacoes {
		if i.Estado == prova.EstadoNaFila {
			i.Estado = prova.EstadoProcessando
			i.Chamadas++
			i.Tentativa = "t"
			f.importacoes[id] = i

			return i, nil
		}
	}

	return prova.Importacao{}, prova.ErrNaoEncontrada
}

func (f *fakeProvas) Renovar(context.Context, string, string) (bool, error) { return true, nil }

func (f *fakeProvas) ConcluirEtapa(_ context.Context, i prova.Importacao, etapa int, _ any, _ time.Duration) error {
	f.etapas = append(f.etapas, etapa)
	i.Versao++
	f.importacoes[i.ID] = i

	return nil
}

func (f *fakeProvas) Falhar(_ context.Context, i prova.Importacao, msg string, espera time.Duration) error {
	f.falhas = append(f.falhas, espera)
	i.Estado = prova.EstadoFalhou
	i.Erro = msg
	f.importacoes[i.ID] = i

	return nil
}

func (f *fakeProvas) RegistrarArquivo(_ context.Context, _, id, ext string) error {
	f.arquivo[id] = ext
	return nil
}

func (f *fakeProvas) ArquivosDaImportacao(context.Context, string) ([]string, error) {
	ids := []string{}
	for id := range f.arquivo {
		ids = append(ids, id)
	}

	return ids, nil
}

// fakeVolume é o volume de arquivos em memória.
type fakeVolume struct{ nomes map[string]bool }

func (v *fakeVolume) Guardar(id string, _ []byte) error { v.nomes[id+".pdf"] = true; return nil }
func (v *fakeVolume) GuardarComo(id, ext string, _ []byte) error {
	v.nomes[id+"."+ext] = true
	return nil
}
func (v *fakeVolume) Remover(nome string) error  { delete(v.nomes, nome); return nil }
func (v *fakeVolume) Existe(id, ext string) bool { return v.nomes[id+"."+ext] }
func (v *fakeVolume) Caminho(id, ext string) (string, error) {
	return "/provas/" + id + "." + ext, nil
}

// fakeExtrator responde como o processador, com uma prova de duas regiões.
type fakeExtrator struct {
	regioes        []prova.Origem
	porRegiao      map[string]prova.Rascunho
	gabarito       prova.Gabarito
	err            error
	errClassificar error
	// errRegiao é o erro de uma região só, pelo rótulo.
	errRegiao map[string]error
	extraidas []string
	// cadernoDoGabarito é o caderno que a leitura do gabarito recebeu.
	cadernoDoGabarito string
	// capa, quando dada, é o que a leitura da capa devolve.
	capa *prova.Metadados
}

func (e *fakeExtrator) Preparar(context.Context, string) ([]prova.Origem, error) {
	return e.regioes, e.err
}

func (e *fakeExtrator) Metadados(context.Context, string, prova.Origem) (prova.Metadados, error) {
	if e.capa != nil {
		return *e.capa, e.err
	}
	return prova.Metadados{Orgao: "TJCE", Ano: 2026, Cargo: "E05", Caderno: "004", Total: 2}, e.err
}

func (e *fakeExtrator) Extrair(_ context.Context, _ string, o prova.Origem) (prova.Rascunho, error) {
	e.extraidas = append(e.extraidas, o.Regiao)
	if err := e.errRegiao[o.Regiao]; err != nil {
		return prova.Rascunho{}, err
	}
	return e.porRegiao[o.Regiao], e.err
}

func (e *fakeExtrator) Gabarito(_ context.Context, _, caderno string) (prova.Gabarito, error) {
	e.cadernoDoGabarito = caderno
	return e.gabarito, e.err
}

func (e *fakeExtrator) Classificar(_ context.Context, qs []prova.ResumoDeQuestao) (map[int]string, error) {
	if e.errClassificar != nil {
		return nil, e.errClassificar
	}
	out := map[int]string{}
	for _, q := range qs {
		out[q.Numero] = fmt.Sprint("Matéria ", q.Numero)
	}

	return out, nil
}

func (e *fakeExtrator) Recortar(context.Context, string, prova.Origem) (string, error) {
	return "22222222-2222-2222-2222-222222222222", e.err
}

func questaoExtraida(numero int, completa bool) prova.Questao {
	q := prova.Questao{Numero: numero, Completa: completa, Blocos: []prova.Bloco{{Tipo: "texto", Texto: fmt.Sprint("q", numero)}}}
	for _, l := range []string{"A", "B", "C", "D", "E"} {
		q.Alternativas = append(q.Alternativas, prova.Alternativa{Letra: l, Blocos: []prova.Bloco{{Tipo: "texto", Texto: l}}})
	}

	return q
}

func extratorDeDuasRegioes() *fakeExtrator {
	return &fakeExtrator{
		regioes: []prova.Origem{{Pagina: 1, Regiao: "0"}, {Pagina: 1, Regiao: "1"}},
		porRegiao: map[string]prova.Rascunho{
			// A sobreposição corta a questão 2 no fim da região 0 e a repete
			// inteira na 1 — e a região 1 chega antes da 2 na ordem de número.
			"0": {Questoes: []prova.Questao{questaoExtraida(1, true), questaoExtraida(2, false)}},
			"1": {Questoes: []prova.Questao{questaoExtraida(2, true)}},
		},
		gabarito: prova.Gabarito{
			Cargo: "E05", Caderno: "4", Tipo: "preliminar",
			Respostas: map[string]string{"1": "B", "2": "D"},
		},
	}
}

func novoProvaServiceDeTeste(repo *fakeProvas, extrator *fakeExtrator) (*ProvaService, *fakeVolume) {
	volume := &fakeVolume{nomes: map[string]bool{}}

	return &ProvaService{
		Repo: repo, Processor: extrator, Arquivos: volume,
		Curadores:    map[string]bool{curador: true},
		MaxPendentes: 2, MaxChamadas: 50, MaxProcessamento: time.Hour, MaxEtapa: time.Minute,
		ExigirConferencia: true,
	}, volume
}

var semLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func processarTudo(t *testing.T, s *ProvaService, repo *fakeProvas, id string) prova.Importacao {
	t.Helper()

	for range 20 {
		if err := s.ProcessarUma(context.Background(), semLog); err != nil {
			t.Fatalf("ProcessarUma: %v", err)
		}
		if i := repo.importacoes[id]; i.Estado != prova.EstadoNaFila {
			return i
		}
	}
	t.Fatal("a fila não terminou em 20 etapas")

	return prova.Importacao{}
}

func TestProvas_SoCuradorEscreve(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())
	ctx := context.Background()
	const estudante = "33333333-3333-3333-3333-333333333333"

	escritas := map[string]func() error{
		"Importar": func() error { _, err := s.Importar(ctx, estudante, EnvioDeProva{Prova: pdfMinimo}); return err },
		"Obter":    func() error { _, err := s.Obter(ctx, estudante, "x"); return err },
		"Listar":   func() error { _, err := s.Listar(ctx, estudante); return err },
		"Salvar":   func() error { _, err := s.Salvar(ctx, estudante, "x", 1, prova.Rascunho{}); return err },
		"Publicar": func() error { _, err := s.Publicar(ctx, estudante, "x", 1); return err },
		"Cancelar": func() error { _, err := s.Cancelar(ctx, estudante, "x", 1); return err },
		"Reprocessar": func() error {
			_, err := s.Reprocessar(ctx, estudante, "x", 1)
			return err
		},
		"Recortar": func() error { _, err := s.Recortar(ctx, estudante, "x", 1, prova.Origem{}); return err },
		"RelerTrecho": func() error {
			_, err := s.RelerTrecho(ctx, estudante, "x", 1, 1, prova.Origem{})
			return err
		},
		"AtualizarGabarito": func() error {
			_, err := s.AtualizarGabarito(ctx, estudante, "x", 1, pdfMinimo, "definitivo.pdf")
			return err
		},
		"Revisar":       func() error { _, err := s.Revisar(ctx, estudante, "x"); return err },
		"Excluir":       func() error { return s.Excluir(ctx, estudante, "x", 1) },
		"Retirar":       func() error { return s.Retirar(ctx, estudante, "x") },
		"ExcluirProva":  func() error { return s.ExcluirProva(ctx, estudante, "x") },
		"ExportarProva": func() error { _, err := s.ExportarProva(ctx, estudante, "x"); return err },
		"ExportarCatalogo": func() error {
			_, err := s.ExportarCatalogo(ctx, estudante, "")
			return err
		},
		"ImportarPacote": func() error {
			_, err := s.ImportarPacote(ctx, estudante, PacoteDeProva{Documento: pdfMinimo})
			return err
		},
		"RenomearProva": func() error {
			return s.RenomearProva(ctx, estudante, "x", "Técnico Judiciário")
		},
	}
	for nome, escrever := range escritas {
		if err := escrever(); !errors.Is(err, prova.ErrAcesso) {
			t.Errorf("%s de quem não é curador: err = %v, quer ErrAcesso", nome, err)
		}
	}
}

// Excluir apaga o rascunho; a publicada é o histórico da prova e a que está
// processando precisa ser cancelada antes.
func TestProvas_Excluir(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome   string
		estado string
		apaga  bool
	}{
		{"na fila", prova.EstadoNaFila, true},
		{"em revisão", prova.EstadoEmRevisao, true},
		{"falhou", prova.EstadoFalhou, true},
		{"cancelada", prova.EstadoCancelada, true},
		{"processando", prova.EstadoProcessando, false},
		{"publicada", prova.EstadoPublicada, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			repo := novoFakeProvas()
			s, _ := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())
			ctx := context.Background()
			nova, err := s.Importar(ctx, curador, EnvioDeProva{Prova: pdfMinimo})
			if err != nil {
				t.Fatal(err)
			}
			i := repo.importacoes[nova.ID]
			i.Estado = c.estado
			repo.importacoes[nova.ID] = i

			if err := s.Excluir(ctx, curador, i.ID, i.Versao+1); !errors.Is(err, prova.ErrConflito) {
				t.Fatalf("Excluir com versão velha: err = %v, quer ErrConflito", err)
			}

			err = s.Excluir(ctx, curador, i.ID, i.Versao)
			_, sobrou := repo.importacoes[i.ID]
			if c.apaga {
				if err != nil || sobrou {
					t.Fatalf("Excluir: err = %v, sobrou = %v", err, sobrou)
				}
				return
			}
			var v ErrValidacao
			if !errors.As(err, &v) || !sobrou {
				t.Fatalf("Excluir %s: err = %v, sobrou = %v; quer recusa", c.estado, err, sobrou)
			}
		})
	}
}

// Cancelada, a importação solta os PDFs: importar os mesmos arquivos começa
// do zero em vez de devolver a cancelada.
func TestProvas_CancelarSoltaOsPDFs(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())
	ctx := context.Background()
	nova, err := s.Importar(ctx, curador, EnvioDeProva{Prova: pdfMinimo})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Cancelar(ctx, curador, nova.ID, nova.Versao); err != nil {
		t.Fatal(err)
	}

	if h := repo.importacoes[nova.ID].Hash; h != "" {
		t.Fatalf("hash da cancelada = %q, quer vazio", h)
	}
}

// No ambiente local, qualquer conta é curadora: a lista de UUIDs perde a
// validade toda vez que o banco é recriado.
func TestProvas_TodosCuradoresLiberaQualquerConta(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())
	s.Curadores, s.TodosCuradores = map[string]bool{}, true
	ctx := context.Background()
	const estudante = "33333333-3333-3333-3333-333333333333"

	nova, err := s.Importar(ctx, estudante, EnvioDeProva{Prova: pdfMinimo})
	if err != nil {
		t.Fatalf("Importar de uma conta qualquer: %v", err)
	}

	if _, err := s.Obter(ctx, estudante, nova.ID); err != nil {
		t.Fatalf("Obter a própria importação: %v", err)
	}
}

// Reenviar os mesmos PDFs devolve a importação existente; os arquivos que o
// reenvio gravou não têm linha nenhuma que os aponte e precisam sair.
func TestProvas_ReenvioNaoDeixaArquivoOrfao(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	repo.existente = &prova.Importacao{ID: "existente", Estado: prova.EstadoEmRevisao}
	s, volume := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())

	i, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo, Gabarito: pdfMinimo})
	if err != nil {
		t.Fatalf("Importar: %v", err)
	}

	if i.ID != "existente" {
		t.Fatalf("id = %q, quer a importação existente", i.ID)
	}
	if len(volume.nomes) != 0 {
		t.Fatalf("arquivos órfãos no volume: %v", volume.nomes)
	}
}

func TestProvas_LimiteNaoDeixaArquivoOrfao(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	repo.errCriar = prova.ErrLimite
	s, volume := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())

	if _, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo}); !errors.Is(err, prova.ErrLimite) {
		t.Fatalf("err = %v, quer ErrLimite", err)
	}
	if len(volume.nomes) != 0 {
		t.Fatalf("arquivos órfãos no volume: %v", volume.nomes)
	}
}

func TestProvas_ImportarRecusaQuemNaoEPDF(t *testing.T) {
	t.Parallel()

	s, _ := novoProvaServiceDeTeste(novoFakeProvas(), extratorDeDuasRegioes())

	_, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: []byte("<html>")})

	var v ErrValidacao
	if !errors.As(err, &v) {
		t.Fatalf("err = %v, quer ErrValidacao", err)
	}
}

// A fila inteira: capa, gabarito, uma etapa por região e a consolidação. A
// resposta de cada questão vem do gabarito, e a questão cortada pela
// sobreposição termina inteira.
func TestProvas_FilaDaCapaAteARevisao(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	extrator := extratorDeDuasRegioes()
	s, _ := novoProvaServiceDeTeste(repo, extrator)

	criada, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo, Gabarito: pdfMinimo})
	if err != nil {
		t.Fatalf("Importar: %v", err)
	}

	i := processarTudo(t, s, repo, criada.ID)

	if i.Estado != prova.EstadoEmRevisao {
		t.Fatalf("estado = %s (%s), quer em_revisao", i.Estado, i.Erro)
	}
	quer := []int{prova.EtapaPreparar, prova.EtapaMetadados, prova.EtapaGabarito, 3, 4, 5}
	if fmt.Sprint(repo.etapas) != fmt.Sprint(quer) {
		t.Errorf("etapas = %v, quer %v", repo.etapas, quer)
	}
	if i.Etapa != prova.TotalEtapas(2) {
		t.Errorf("etapa final = %d, quer %d", i.Etapa, prova.TotalEtapas(2))
	}

	r := i.Rascunho
	if r.Orgao != "TJCE" || r.Total != 2 || r.Cargo != "E05" {
		t.Errorf("metadados da capa não aplicados: %+v", r)
	}
	if len(r.Questoes) != 2 || !r.Questoes[1].Completa {
		t.Fatalf("questões = %+v", r.Questoes)
	}
	if r.Questoes[0].Resposta != "B" || r.Questoes[1].Resposta != "D" {
		t.Errorf("respostas = %q %q, quer as do gabarito", r.Questoes[0].Resposta, r.Questoes[1].Resposta)
	}
	if r.Questoes[1].Disciplina != "Matéria 2" {
		t.Errorf("disciplina = %q, quer a matéria sugerida na consolidação", r.Questoes[1].Disciplina)
	}
}

// O caderno do MPEAL chegou sem capa legível: sem total, cada questão virava
// "Numeração inválida". O gabarito do cargo diz quantas são.
func TestProvas_CapaSemTotalFicaComOTotalDoGabarito(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	extrator := extratorDeDuasRegioes()
	extrator.capa = &prova.Metadados{Orgao: "MPEAL", CargoNome: "CONHECIMENTOS GERAIS", Caderno: "004"}
	s, _ := novoProvaServiceDeTeste(repo, extrator)

	criada, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo, Gabarito: pdfMinimo})
	if err != nil {
		t.Fatalf("Importar: %v", err)
	}
	r := processarTudo(t, s, repo, criada.ID).Rascunho

	if r.Total != 2 || !slices.ContainsFunc(r.Alertas, func(a string) bool { return strings.Contains(a, "o total ficou 2") }) {
		t.Fatalf("total = %d, alertas %v; quer o total do gabarito, avisado", r.Total, r.Alertas)
	}
	// O que a capa não disse vira aviso, nomeado, com o código que o gabarito
	// sugere; nada impede publicar.
	quer := []string{
		"Confira na etapa Dados: ano, código do cargo.",
		`O gabarito é do cargo E05 e a prova está sem o código do cargo. Se a capa diz "Caderno de Prova 'E05'", ` +
			"use esse código na etapa Dados; se não, o gabarito é de outro cargo.",
	}
	if a := r.Avisos(); !slices.Equal(a, quer) {
		t.Fatalf("avisos = %q", a)
	}
	if p := r.Pendencias(prova.Criterios{Gabarito: true}); len(p) > 0 {
		t.Fatalf("pendências = %q", p)
	}
}

// A matéria é sugestão: a IA recusar a classificação não pode derrubar uma
// importação que já extraiu todas as questões.
func TestProvas_ClassificacaoRecusadaNaoFalhaAImportacao(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	extrator := extratorDeDuasRegioes()
	extrator.errClassificar = fmt.Errorf("%w: recusou", port.ErrDocumentoRecusado)
	s, _ := novoProvaServiceDeTeste(repo, extrator)
	criada, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo})
	if err != nil {
		t.Fatal(err)
	}

	i := processarTudo(t, s, repo, criada.ID)

	if i.Estado != prova.EstadoEmRevisao {
		t.Fatalf("estado = %s (%s)", i.Estado, i.Erro)
	}
	if !strings.Contains(strings.Join(i.Rascunho.Alertas, " "), "matérias") {
		t.Fatalf("alertas = %v, quer o aviso da classificação", i.Rascunho.Alertas)
	}
}

// Só a falha que passa sozinha volta à fila, e com espera que cresce a cada
// falha seguida — até as tentativas acabarem.
func TestProvas_FalhaTransitoriaRepeteComEspera(t *testing.T) {
	t.Parallel()

	sobrecarga := fmt.Errorf("%w: 503", port.ErrProcessamentoTransitorio)
	casos := []struct {
		nome             string
		err              error
		falhasAnteriores int
		espera           time.Duration
	}{
		{"documento recusado", fmt.Errorf("%w: página inexistente", port.ErrDocumentoRecusado), 0, 0},
		{"Gemini sobrecarregado", sobrecarga, 0, 15 * time.Second},
		{"erro desconhecido", errors.New("conexão recusada"), 0, 15 * time.Second},
		{"quarta falha seguida", sobrecarga, 3, 2 * time.Minute},
		{"tentativas esgotadas", sobrecarga, 5, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			repo := novoFakeProvas()
			extrator := extratorDeDuasRegioes()
			extrator.err = c.err
			s, _ := novoProvaServiceDeTeste(repo, extrator)
			nova, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo})
			if err != nil {
				t.Fatal(err)
			}
			i := repo.importacoes[nova.ID]
			i.Falhas = c.falhasAnteriores
			repo.importacoes[nova.ID] = i

			if err := s.ProcessarUma(context.Background(), semLog); err != nil {
				t.Fatal(err)
			}

			if len(repo.falhas) != 1 || repo.falhas[0] != c.espera {
				t.Fatalf("falhas = %v, quer espera de %v", repo.falhas, c.espera)
			}
		})
	}
}

func TestProvas_TetoDeChamadasParaSemChamarOProcessador(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	extrator := extratorDeDuasRegioes()
	s, _ := novoProvaServiceDeTeste(repo, extrator)
	s.MaxChamadas = 0
	if _, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo}); err != nil {
		t.Fatal(err)
	}

	if err := s.ProcessarUma(context.Background(), semLog); err != nil {
		t.Fatal(err)
	}

	if len(repo.falhas) != 1 || repo.falhas[0] != 0 {
		t.Fatalf("falhas = %v, quer uma falha definitiva", repo.falhas)
	}
	if len(extrator.extraidas) != 0 {
		t.Fatal("chamou o processador depois do teto")
	}
}

func TestProvas_SalvarPreservaRegistroDasChamadas(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())
	repo.importacoes["i"] = prova.Importacao{
		ID: "i", Estado: prova.EstadoEmRevisao, Versao: 3,
		Rascunho: prova.Rascunho{Extracoes: []prova.Extracao{{Modelo: "gemini", TokensSaida: 900}}},
	}

	editado := prova.Rascunho{Orgao: "TJCE", Extracoes: nil}
	if _, err := s.Salvar(context.Background(), curador, "i", 3, editado); err != nil {
		t.Fatalf("Salvar: %v", err)
	}

	if got := repo.importacoes["i"].Rascunho.Extracoes; len(got) != 1 || got[0].TokensSaida != 900 {
		t.Fatalf("extrações = %+v, quer as gravadas", got)
	}
}

func TestProvas_SalvarComVersaoVelhaConflita(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())
	repo.importacoes["i"] = prova.Importacao{ID: "i", Estado: prova.EstadoEmRevisao, Versao: 3}

	if _, err := s.Salvar(context.Background(), curador, "i", 2, prova.Rascunho{}); !errors.Is(err, prova.ErrConflito) {
		t.Fatalf("err = %v, quer ErrConflito", err)
	}
}

func TestProvas_PublicarComPendenciaRecusa(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())
	repo.importacoes["i"] = prova.Importacao{ID: "i", Estado: prova.EstadoEmRevisao, Versao: 1}

	_, err := s.Publicar(context.Background(), curador, "i", 1)

	var v ErrValidacao
	if !errors.As(err, &v) {
		t.Fatalf("err = %v, quer ErrValidacao com as pendências", err)
	}
}

// Com a conferência desligada, um rascunho íntegro publica sem nenhuma questão
// marcada; o gabarito continua pedido.
func TestProvas_PublicarSemConferenciaQuandoDesligada(t *testing.T) {
	t.Parallel()

	repo := &fakePublicacao{fakeProvas: novoFakeProvas()}
	s, volume := novoProvaServiceDeTeste(repo.fakeProvas, extratorDeDuasRegioes())
	s.Repo = repo
	s.ExigirConferencia = false
	volume.nomes["doc.pdf"] = true
	r := prova.Rascunho{Banca: "FCC", Orgao: "TJCE", Ano: 2026, Cargo: "E05", Caderno: "004", Total: 1,
		Questoes: []prova.Questao{questaoExtraida(1, true)},
		Gabarito: prova.Gabarito{Cargo: "E05", Caderno: "4", Respostas: map[string]string{"1": "A"}}}
	repo.importacoes["i"] = prova.Importacao{ID: "i", Documento: "doc", Estado: prova.EstadoEmRevisao, Versao: 1, Rascunho: r}

	id, err := s.Publicar(context.Background(), curador, "i", 1)
	if err != nil || id == "" {
		t.Fatalf("Publicar = %q, %v", id, err)
	}

	s.ExigirConferencia = true
	repo.importacoes["i"] = prova.Importacao{ID: "i", Documento: "doc", Estado: prova.EstadoEmRevisao, Versao: 1, Rascunho: r}
	var v ErrValidacao
	if _, err := s.Publicar(context.Background(), curador, "i", 1); !errors.As(err, &v) {
		t.Fatalf("com a conferência exigida, publicou sem ela: %v", err)
	}
}

// fakePublicacao registra a publicação sem reproduzir o banco.
type fakePublicacao struct {
	*fakeProvas
	// publicada é a importação que chegou ao repositório para publicar.
	publicada prova.Importacao
	// publicacao é a prova que o repositório devolve.
	publicacao prova.Publicacao
}

func (f *fakePublicacao) Publicacao(context.Context, string) (prova.Publicacao, error) {
	if f.publicacao.ID != "" {
		return f.publicacao, nil
	}
	return prova.Publicacao{ID: "prova-1"}, nil
}

func (f *fakePublicacao) Catalogo(context.Context, port.FiltroCatalogo) ([]prova.Publicacao, error) {
	if f.publicacao.ID == "" {
		return nil, nil
	}
	return []prova.Publicacao{f.publicacao}, nil
}

func (f *fakePublicacao) ImportacaoDaPublicacao(context.Context, string) (prova.Importacao, error) {
	return f.importacoes["base"], nil
}

// 56 conferidas e 4 por fazer: publica as prontas, e só elas chegam ao
// repositório; as outras ficam nas excluídas da revisão.
func TestProvas_PublicarLevaSoAsProntas(t *testing.T) {
	t.Parallel()

	repo := &fakePublicacao{fakeProvas: novoFakeProvas()}
	s, volume := novoProvaServiceDeTeste(repo.fakeProvas, extratorDeDuasRegioes())
	s.Repo = repo
	s.ExigirConferencia = true
	volume.nomes["doc.pdf"] = true
	conferida, pendente := questaoExtraida(1, true), questaoExtraida(2, true)
	conferida.Revisada = true
	r := prova.Rascunho{Banca: "FCC", Orgao: "TJCE", Ano: 2026, Cargo: "E05", Caderno: "004", Total: 3,
		Questoes: []prova.Questao{conferida, pendente},
		Gabarito: prova.Gabarito{Cargo: "E05", Caderno: "4", Respostas: map[string]string{"1": "A", "2": "B", "3": "C"}}}
	repo.importacoes["i"] = prova.Importacao{ID: "i", Documento: "doc", Estado: prova.EstadoEmRevisao, Versao: 1, Rascunho: r}

	vista, err := s.Obter(context.Background(), curador, "i")
	if err != nil || vista.Publicaveis != 1 || len(vista.DeFora) != 2 || len(vista.Pendencias) != 0 {
		t.Fatalf("revisão: %d publicáveis, de fora %v, pendências %v (%v)", vista.Publicaveis, vista.DeFora, vista.Pendencias, err)
	}
	if _, err := s.Publicar(context.Background(), curador, "i", 1); err != nil {
		t.Fatalf("Publicar: %v", err)
	}
	if p := repo.publicada.Rascunho; len(p.Questoes) != 1 || !slices.Equal(p.Excluidas, []int{2, 3}) {
		t.Fatalf("ao repositório: %d questões, excluídas %v", len(p.Questoes), p.Excluidas)
	}
}

// O aluno nunca vê questão sem resposta no gabarito, nem na prova nem na lista
// do catálogo; a revisão do curador continua com todas.
func TestProvas_QuestaoSemGabaritoNaoChegaAoAluno(t *testing.T) {
	t.Parallel()

	repo := &fakePublicacao{fakeProvas: novoFakeProvas()}
	s, _ := novoProvaServiceDeTeste(repo.fakeProvas, extratorDeDuasRegioes())
	s.Repo = repo
	repo.publicacao = prova.Publicacao{ID: "prova-1", Conteudo: prova.Rascunho{
		Total:    2,
		Questoes: []prova.Questao{questaoExtraida(1, true), questaoExtraida(2, true)},
		Gabarito: prova.Gabarito{Respostas: map[string]string{"1": "B"}},
	}}

	p, err := s.Publicacao(context.Background(), "prova-1", 0, "")
	if err != nil {
		t.Fatalf("Publicacao: %v", err)
	}
	if len(p.Conteudo.Questoes) != 1 || p.Conteudo.Questoes[0].Numero != 1 || p.Conteudo.QuestoesNaProva() != 1 {
		t.Fatalf("prova do aluno = %+v", p.Conteudo)
	}

	lista, err := s.Catalogo(context.Background(), port.FiltroCatalogo{})
	if err != nil || len(lista) != 1 || lista[0].Conteudo.QuestoesNaProva() != 1 {
		t.Fatalf("catálogo = %+v (%v)", lista, err)
	}
	// A revisão do curador abre com as duas.
	if i, err := s.Revisar(context.Background(), curador, "prova-1"); err != nil || len(i.Rascunho.Questoes) != 2 {
		t.Fatalf("revisão = %+v (%v)", i.Rascunho.Questoes, err)
	}
}

// A prova publicada antes da tabela de gabaritos não tem as respostas, mas tem
// o PDF: "Abrir revisão" já põe o gabarito para ler.
func TestProvas_RevisarProvaSemRespostasLeOGabarito(t *testing.T) {
	t.Parallel()

	repo := &fakePublicacao{fakeProvas: novoFakeProvas()}
	s, _ := novoProvaServiceDeTeste(repo.fakeProvas, extratorDeDuasRegioes())
	s.Repo = repo
	repo.importacoes["base"] = prova.Importacao{
		ID: "base", Documento: "doc", GabaritoArquivo: "gab", Estado: prova.EstadoPublicada,
		Regioes: []prova.Origem{{Pagina: 1, Regiao: "0"}},
	}

	i, err := s.Revisar(context.Background(), curador, "prova-1")
	if err != nil {
		t.Fatalf("Revisar: %v", err)
	}
	if i.Estado != prova.EstadoNaFila || i.Etapa != prova.EtapaSoGabarito || i.GabaritoArquivo != "gab" {
		t.Fatalf("revisão = %s, etapa %d, gabarito %q; quer o gabarito na fila", i.Estado, i.Etapa, i.GabaritoArquivo)
	}
}

// Extrair de novo parte do mesmo PDF e do mesmo gabarito, do zero, e vira
// revisão da mesma prova — não uma prova nova no catálogo.
func TestProvas_ReextrairAbreRevisaoDaMesmaProva(t *testing.T) {
	t.Parallel()

	repo := &fakePublicacao{fakeProvas: novoFakeProvas()}
	s, _ := novoProvaServiceDeTeste(repo.fakeProvas, extratorDeDuasRegioes())
	s.Repo = repo
	repo.importacoes["base"] = prova.Importacao{
		ID: "base", Documento: "doc", GabaritoArquivo: "gab", Estado: prova.EstadoPublicada, Etapa: 17,
		Rascunho: prova.Rascunho{Questoes: []prova.Questao{questaoExtraida(1, true)}},
	}

	i, err := s.Reextrair(context.Background(), curador, "prova-1")
	if err != nil {
		t.Fatalf("Reextrair: %v", err)
	}

	if i.Estado != prova.EstadoNaFila || i.Etapa != prova.EtapaPreparar || len(i.Rascunho.Questoes) != 0 {
		t.Fatalf("importação = %+v, quer na fila, do começo e sem o rascunho antigo", i.Importacao)
	}
	if i.Documento != "doc" || i.GabaritoArquivo != "gab" || i.ProvaID != "prova-1" || i.Hash != "" {
		t.Fatalf("importação = %+v, quer os arquivos da base e a mesma prova", i.Importacao)
	}
}

func (f *fakePublicacao) Publicar(_ context.Context, i prova.Importacao, _ string) (string, error) {
	f.publicada = i
	return "prova-1", nil
}

// O gabarito definitivo sai depois do preliminar. Trocá-lo não pode reextrair
// as questões — cada região é uma chamada paga — nem manter a conferência.
func TestProvas_NovoGabaritoNaoReextrai(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	extrator := extratorDeDuasRegioes()
	s, _ := novoProvaServiceDeTeste(repo, extrator)
	q := questaoExtraida(1, true)
	q.Resposta, q.Revisada = "B", true
	repo.importacoes["i"] = prova.Importacao{
		ID: "i", Estado: prova.EstadoEmRevisao, Versao: 1, Regioes: extrator.regioes,
		Rascunho: prova.Rascunho{Caderno: "003", Questoes: []prova.Questao{q}},
	}
	extrator.gabarito = prova.Gabarito{Tipo: "definitivo", Respostas: map[string]string{"1": "C"}}

	if _, err := s.AtualizarGabarito(context.Background(), curador, "i", 1, pdfMinimo, "definitivo.pdf"); err != nil {
		t.Fatalf("AtualizarGabarito: %v", err)
	}
	i := processarTudo(t, s, repo, "i")

	if i.Estado != prova.EstadoEmRevisao || len(extrator.extraidas) != 0 {
		t.Fatalf("estado = %s, regiões reextraídas = %v", i.Estado, extrator.extraidas)
	}
	if got := i.Rascunho.Questoes[0]; got.Resposta != "C" || got.Revisada {
		t.Fatalf("questão = %+v, quer resposta C e conferência desfeita", got)
	}
	// O arquivo novo pode ser a relação com todos os tipos: vai o caderno
	// conferido na revisão.
	if extrator.cadernoDoGabarito != "003" {
		t.Fatalf("caderno enviado à leitura do gabarito = %q, quer 003", extrator.cadernoDoGabarito)
	}
}

// Reenviar o mesmo gabarito, ou o definitivo que mudou uma resposta, não pode
// desfazer a conferência das questões que continuam iguais.
func TestProvas_NovoGabaritoSoDesconfereOQueMudou(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	extrator := extratorDeDuasRegioes()
	s, _ := novoProvaServiceDeTeste(repo, extrator)
	q1, q2, q3 := questaoExtraida(1, true), questaoExtraida(2, true), questaoExtraida(3, true)
	q1.Resposta, q2.Resposta, q3.Resposta = "A", "B", "C"
	q1.Revisada, q2.Revisada, q3.Revisada = true, true, true
	repo.importacoes["i"] = prova.Importacao{
		ID: "i", Estado: prova.EstadoEmRevisao, Versao: 1, Regioes: extrator.regioes,
		Rascunho: prova.Rascunho{Questoes: []prova.Questao{q1, q2, q3}},
	}
	// O definitivo escreve a situação em todas; na 1, só isso mudou.
	extrator.gabarito = prova.Gabarito{
		Tipo:      "definitivo",
		Respostas: map[string]string{"1": "A", "2": "D", "3": ""},
		Situacoes: map[string]string{"1": "Gabarito sem alteração", "2": "Alterada", "3": "Anulada"},
	}

	if _, err := s.AtualizarGabarito(context.Background(), curador, "i", 1, pdfMinimo, "definitivo.pdf"); err != nil {
		t.Fatalf("AtualizarGabarito: %v", err)
	}
	i := processarTudo(t, s, repo, "i")

	conferidas := map[int]bool{}
	for _, q := range i.Rascunho.Questoes {
		conferidas[q.Numero] = q.Revisada
	}
	if !conferidas[1] || conferidas[2] || conferidas[3] {
		t.Fatalf("conferidas = %v; quer só a 1, a única com a mesma letra", conferidas)
	}
	if q := i.Rascunho.Questoes[0]; q.Situacao != "Gabarito sem alteração" {
		t.Fatalf("situação da 1 = %q; a situação nova entra mesmo sem desconferir", q.Situacao)
	}
}

// O caso da prova do TJCE: a região 0 corta a questão 2 no fim, e a região 1,
// que a via inteira, a pula. Depois da última região, a questão ganha uma
// leitura só dela, e o texto de apoio que apareça no recorte não se repete.
func TestProvas_QuestaoCortadaGanhaReleitura(t *testing.T) {
	t.Parallel()

	em := func(q prova.Questao, y0, y1 float64) prova.Questao {
		q.Origens = []prova.Origem{{Pagina: 1, Regiao: "0", Retangulo: []float64{100, y0, 480, y1}}}
		return q
	}
	cortada := em(questaoExtraida(2, false), 700, 845)
	cortada.Alternativas = cortada.Alternativas[:1]
	extrator := extratorDeDuasRegioes()
	extrator.regioes = []prova.Origem{
		{Pagina: 1, Regiao: "0", Retangulo: []float64{0, 0, 595, 845}},
		{Pagina: 1, Regiao: "1", Retangulo: []float64{0, 690, 595, 1535}},
	}
	extrator.porRegiao = map[string]prova.Rascunho{
		"0":  {Questoes: []prova.Questao{em(questaoExtraida(1, true), 50, 600), cortada}},
		"1":  {Questoes: []prova.Questao{em(questaoExtraida(3, true), 900, 1100)}},
		"q2": {Questoes: []prova.Questao{questaoExtraida(2, true)}, Apoios: []prova.Apoio{{ID: "rq2-t1"}}},
	}
	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extrator)
	nova, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo})
	if err != nil {
		t.Fatal(err)
	}

	final := processarTudo(t, s, repo, nova.ID)

	if got := strings.Join(extrator.extraidas, ","); got != "0,1,q2" {
		t.Fatalf("regiões lidas = %s, quer 0,1,q2", got)
	}
	i := slices.IndexFunc(final.Rascunho.Questoes, func(q prova.Questao) bool { return q.Numero == 2 })
	if i < 0 || !final.Rascunho.Questoes[i].Completa || len(final.Rascunho.Questoes[i].Alternativas) != 5 {
		t.Fatalf("questão 2 depois da releitura = %+v", final.Rascunho.Questoes)
	}
	if len(final.Rascunho.Apoios) != 0 {
		t.Fatalf("a releitura trouxe texto de apoio: %+v", final.Rascunho.Apoios)
	}
	if final.Estado != prova.EstadoEmRevisao || final.Etapa != prova.TotalEtapas(3) {
		t.Fatalf("estado = %s, etapa = %d; quer em revisão na etapa %d", final.Estado, final.Etapa, prova.TotalEtapas(3))
	}
}

// A releitura é reforço: recusada pelo processador, a importação chega à
// revisão com a questão como as regiões a leram e um alerta — antes, a prova
// inteira falhava por um retângulo recusado.
func TestProvas_ReleituraRecusadaNaoDerrubaAImportacao(t *testing.T) {
	t.Parallel()

	em := func(q prova.Questao, y0, y1 float64) prova.Questao {
		q.Origens = []prova.Origem{{Pagina: 1, Regiao: "0", Retangulo: []float64{100, y0, 480, y1}}}
		return q
	}
	cortada := em(questaoExtraida(2, false), 700, 845)
	cortada.Alternativas = cortada.Alternativas[:1]
	extrator := extratorDeDuasRegioes()
	extrator.regioes = []prova.Origem{
		{Pagina: 1, Regiao: "0", Retangulo: []float64{0, 0, 595, 845}},
		{Pagina: 1, Regiao: "1", Retangulo: []float64{0, 690, 595, 1535}},
	}
	extrator.porRegiao = map[string]prova.Rascunho{
		"0": {Questoes: []prova.Questao{em(questaoExtraida(1, true), 50, 600), cortada}},
		"1": {Questoes: []prova.Questao{em(questaoExtraida(3, true), 900, 1100)}},
	}
	extrator.errRegiao = map[string]error{
		"q2": fmt.Errorf("%w: retângulo fora da página", port.ErrDocumentoRecusado),
	}
	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extrator)
	nova, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo})
	if err != nil {
		t.Fatal(err)
	}

	final := processarTudo(t, s, repo, nova.ID)

	if final.Estado != prova.EstadoEmRevisao {
		t.Fatalf("estado = %s (%s), quer em revisão", final.Estado, final.Erro)
	}
	if !slices.ContainsFunc(final.Rascunho.Alertas, func(a string) bool {
		return strings.Contains(a, "releitura da questão 2 não foi feita") && strings.Contains(a, "fora da página")
	}) {
		t.Fatalf("alertas = %v, quer o da releitura recusada", final.Rascunho.Alertas)
	}
}

// A releitura automática pode não resolver; o curador pede de novo, e a fila
// relê só a questão que continua incompleta, sem mexer no resto.
func TestProvas_RelerQuestaoQueContinuouIncompleta(t *testing.T) {
	t.Parallel()

	em := func(q prova.Questao, y0, y1 float64) prova.Questao {
		q.Origens = []prova.Origem{{Pagina: 1, Regiao: "0", Retangulo: []float64{100, y0, 480, y1}}}
		return q
	}
	cortada := em(questaoExtraida(2, false), 700, 845)
	cortada.Alternativas = cortada.Alternativas[:1]
	extrator := extratorDeDuasRegioes()
	extrator.regioes = []prova.Origem{
		{Pagina: 1, Regiao: "0", Retangulo: []float64{0, 0, 595, 845}},
		{Pagina: 1, Regiao: "1", Retangulo: []float64{0, 690, 595, 1535}},
	}
	extrator.porRegiao = map[string]prova.Rascunho{
		"0": {Questoes: []prova.Questao{em(questaoExtraida(1, true), 50, 600), cortada}},
		"1": {Questoes: []prova.Questao{em(questaoExtraida(3, true), 900, 1100)}},
	}
	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extrator)
	ctx := context.Background()
	nova, err := s.Importar(ctx, curador, EnvioDeProva{Prova: pdfMinimo})
	if err != nil {
		t.Fatal(err)
	}
	revisao := processarTudo(t, s, repo, nova.ID)

	extrator.porRegiao["q2"] = prova.Rascunho{Questoes: []prova.Questao{questaoExtraida(2, true)}}
	relida, err := s.Reler(ctx, curador, nova.ID, revisao.Versao)
	if err != nil {
		t.Fatalf("Reler: %v", err)
	}
	if relida.Estado != prova.EstadoNaFila || relida.Etapa != prova.EtapaPrimeiraRegiao+2 || len(relida.Regioes) != 3 {
		t.Fatalf("depois de Reler: estado %s, etapa %d, %d regiões", relida.Estado, relida.Etapa, len(relida.Regioes))
	}

	final := processarTudo(t, s, repo, nova.ID)

	i := slices.IndexFunc(final.Rascunho.Questoes, func(q prova.Questao) bool { return q.Numero == 2 })
	if i < 0 || len(final.Rascunho.Questoes[i].Alternativas) != 5 || final.Estado != prova.EstadoEmRevisao {
		t.Fatalf("questão 2 depois de reler = %+v, estado %s", final.Rascunho.Questoes, final.Estado)
	}

	// Sem questão incompleta, não há o que reler.
	var v ErrValidacao
	if _, err := s.Reler(ctx, curador, nova.ID, final.Versao); !errors.As(err, &v) {
		t.Fatalf("Reler sem incompleta: err = %v, quer ErrValidacao", err)
	}
}

// emRevisaoPorPagina é a prova de duas regiões, uma por página, já na
// revisão: é dentro das páginas que o curador marca os trechos.
func emRevisaoPorPagina(t *testing.T) (*ProvaService, *fakeProvas, *fakeExtrator, prova.Importacao) {
	t.Helper()

	extrator := extratorDeDuasRegioes()
	extrator.regioes = []prova.Origem{
		{Pagina: 1, Regiao: "0", Retangulo: []float64{0, 0, 595, 842}},
		{Pagina: 2, Regiao: "1", Retangulo: []float64{0, 0, 595, 842}},
	}
	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extrator)
	nova, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo})
	if err != nil {
		t.Fatal(err)
	}
	revisao := processarTudo(t, s, repo, nova.ID)
	extrator.extraidas = nil

	return s, repo, extrator, revisao
}

var trechoDaQuestao2 = prova.Origem{Pagina: 2, Retangulo: []float64{30, 100, 560, 500}, Regiao: "1"}

// A questão que nem a releitura acertou: o curador marca o trecho dela no
// original, e a fila lê só ele — sem consolidar de novo, sem mexer nas outras.
func TestProvas_RelerTrechoTrocaSoAQuestao(t *testing.T) {
	t.Parallel()

	s, repo, extrator, revisao := emRevisaoPorPagina(t)
	ctx := context.Background()
	primeira := revisao.Rascunho.Questoes[0]
	relida := questaoExtraida(2, true)
	relida.Blocos[0].Texto = "a questão 2 inteira"
	extrator.porRegiao["t2"] = prova.Rascunho{Questoes: []prova.Questao{relida}}

	marcada, err := s.RelerTrecho(ctx, curador, revisao.ID, revisao.Versao, 2, trechoDaQuestao2)
	if err != nil {
		t.Fatalf("RelerTrecho: %v", err)
	}
	if marcada.Estado != prova.EstadoNaFila || marcada.Etapa != prova.EtapaTrecho {
		t.Fatalf("depois de marcar: estado %s, etapa %d", marcada.Estado, marcada.Etapa)
	}

	final := processarTudo(t, s, repo, revisao.ID)

	if got := strings.Join(extrator.extraidas, ","); got != "t2" {
		t.Fatalf("regiões lidas = %s, quer só t2", got)
	}
	if final.Estado != prova.EstadoEmRevisao || repo.etapas[len(repo.etapas)-1] != prova.EtapaTrecho {
		t.Fatalf("estado = %s (%s), etapas = %v", final.Estado, final.Erro, repo.etapas)
	}
	if q := final.Rascunho.Questoes[1]; q.Blocos[0].Texto != "a questão 2 inteira" || q.Revisada {
		t.Fatalf("questão 2 = %+v, quer a leitura do trecho, sem conferência", q)
	}
	if !reflect.DeepEqual(final.Rascunho.Questoes[0], primeira) {
		t.Fatalf("a questão 1 mudou:\nantes  %+v\ndepois %+v", primeira, final.Rascunho.Questoes[0])
	}
}

// O texto de apoio cortado: o curador marca o texto inteiro, e a fila lê só
// ele, pedindo ao processador só o texto — sem mexer nas questões.
func TestProvas_RelerTextoDeApoio(t *testing.T) {
	t.Parallel()

	s, repo, extrator, revisao := emRevisaoPorPagina(t)
	ctx := context.Background()
	i := repo.importacoes[revisao.ID]
	i.Rascunho.Apoios = []prova.Apoio{{
		ID: "r1-t1", Questoes: []int{1, 2}, Revisado: true,
		Blocos: []prova.Bloco{{Tipo: "texto", Texto: "só o primeiro parágrafo"}},
	}}
	i.Rascunho.AcertarApoios()
	repo.importacoes[revisao.ID] = i
	questoes := slices.Clone(i.Rascunho.Questoes)
	extrator.porRegiao["ta:r1-t1"] = prova.Rascunho{Apoios: []prova.Apoio{{
		ID: "x", Blocos: []prova.Bloco{{Tipo: "texto", Texto: "o texto inteiro, do título à fonte"}},
	}}}

	var v ErrValidacao
	if _, err := s.RelerTextoDeApoio(ctx, curador, revisao.ID, i.Versao, "nao-existe", trechoDaQuestao2); !errors.As(err, &v) {
		t.Fatalf("texto que não existe: err = %v, quer ErrValidacao", err)
	}
	fora := prova.Origem{Pagina: 3, Retangulo: []float64{30, 100, 560, 500}}
	if _, err := s.RelerTextoDeApoio(ctx, curador, revisao.ID, i.Versao, "r1-t1", fora); !errors.As(err, &v) {
		t.Fatalf("trecho fora do caderno: err = %v, quer ErrValidacao", err)
	}
	marcada, err := s.RelerTextoDeApoio(ctx, curador, revisao.ID, i.Versao, "r1-t1", trechoDaQuestao2)
	if err != nil || marcada.Estado != prova.EstadoNaFila || marcada.Etapa != prova.EtapaTrecho {
		t.Fatalf("RelerTextoDeApoio: %+v, %v", marcada.Importacao, err)
	}

	final := processarTudo(t, s, repo, revisao.ID)

	if got := strings.Join(extrator.extraidas, ","); got != "ta:r1-t1" || final.Estado != prova.EstadoEmRevisao {
		t.Fatalf("regiões lidas = %s, estado = %s (%s)", got, final.Estado, final.Erro)
	}
	if a := final.Rascunho.Apoios[0]; a.Blocos[0].Texto != "o texto inteiro, do título à fonte" || a.Revisado || !slices.Equal(a.Questoes, []int{1, 2}) {
		t.Fatalf("texto = %+v", a)
	}
	if !reflect.DeepEqual(final.Rascunho.Questoes, questoes) {
		t.Fatalf("as questões mudaram:\nantes  %+v\ndepois %+v", questoes, final.Rascunho.Questoes)
	}
}

// Recusado pelo processador, o trecho não derruba a importação: ela volta à
// revisão com a questão como estava e o motivo nos alertas.
func TestProvas_TrechoRecusadoVoltaARevisao(t *testing.T) {
	t.Parallel()

	s, repo, extrator, revisao := emRevisaoPorPagina(t)
	extrator.errRegiao = map[string]error{
		"t2": fmt.Errorf("%w: a IA se recusou a ler o trecho", port.ErrDocumentoRecusado),
	}
	antes := revisao.Rascunho.Questoes[1]

	if _, err := s.RelerTrecho(context.Background(), curador, revisao.ID, revisao.Versao, 2, trechoDaQuestao2); err != nil {
		t.Fatalf("RelerTrecho: %v", err)
	}
	final := processarTudo(t, s, repo, revisao.ID)

	if final.Estado != prova.EstadoEmRevisao || !reflect.DeepEqual(final.Rascunho.Questoes[1], antes) {
		t.Fatalf("estado = %s, questão 2 = %+v", final.Estado, final.Rascunho.Questoes[1])
	}
	if !slices.ContainsFunc(final.Rascunho.Alertas, func(a string) bool {
		return strings.Contains(a, "questão 2 não foi lido") && strings.Contains(a, "se recusou")
	}) {
		t.Fatalf("alertas = %v, quer o do trecho recusado", final.Rascunho.Alertas)
	}
}

func TestProvas_RelerTrechoRecusaOQueNaoDaParaLer(t *testing.T) {
	t.Parallel()

	s, repo, _, revisao := emRevisaoPorPagina(t)
	ctx := context.Background()

	var v ErrValidacao
	fora := prova.Origem{Pagina: 2, Retangulo: []float64{30, 100, 560, 900}}
	if _, err := s.RelerTrecho(ctx, curador, revisao.ID, revisao.Versao, 2, fora); !errors.As(err, &v) {
		t.Errorf("trecho fora da página: err = %v, quer ErrValidacao", err)
	}
	if _, err := s.RelerTrecho(ctx, curador, revisao.ID, revisao.Versao+1, 2, trechoDaQuestao2); !errors.Is(err, prova.ErrConflito) {
		t.Errorf("versão velha: err = %v, quer ErrConflito", err)
	}

	// Na fila, o trecho anterior ainda não foi lido.
	if _, err := s.RelerTrecho(ctx, curador, revisao.ID, revisao.Versao, 2, trechoDaQuestao2); err != nil {
		t.Fatal(err)
	}
	naFila := repo.importacoes[revisao.ID]
	if _, err := s.RelerTrecho(ctx, curador, revisao.ID, naFila.Versao, 2, trechoDaQuestao2); !errors.Is(err, prova.ErrConflito) {
		t.Errorf("com um trecho na fila: err = %v, quer ErrConflito", err)
	}
}

// O caso do TRT-15 em staging: a mesma prova importada de novo. A capa diz que
// é a publicada, e a importação para ali — sem ler gabarito nem questão —,
// cancelada com o motivo e segurando o hash, para o reenvio cair nela.
func TestProvas_ProvaNoCatalogoParaNaCapa(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	extrator := extratorDeDuasRegioes()
	s, _ := novoProvaServiceDeTeste(repo, extrator)
	// A capa lê "TJCE 2026 · E05"; a publicada foi lida "TJ-CE", tipo 001.
	repo.irmas = []prova.Publicacao{{ID: "publicada", Conteudo: prova.Rascunho{
		Banca: "FCC", Orgao: "TJ-CE", Ano: 2026, Cargo: "E05", Caderno: "001",
	}}}
	nova, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo, Gabarito: pdfMinimo})
	if err != nil {
		t.Fatal(err)
	}

	final := processarTudo(t, s, repo, nova.ID)

	if final.Estado != prova.EstadoCancelada || !strings.Contains(final.Erro, "já está no catálogo") {
		t.Fatalf("estado = %s, erro = %q; quer cancelada com o motivo", final.Estado, final.Erro)
	}
	if len(extrator.extraidas) != 0 || !slices.Equal(repo.etapas, []int{prova.EtapaPreparar, prova.EtapaMetadados}) {
		t.Fatalf("regiões lidas = %v, etapas = %v; quer só preparar e a capa", extrator.extraidas, repo.etapas)
	}
	if final.Hash == "" {
		t.Fatal("a repetida soltou o hash: o reenvio dos mesmos PDFs abriria outra importação")
	}
}

// A mesma prova numa importação que ainda está em revisão também conta; a
// revisão de uma prova publicada, não — ela é a própria prova.
func TestProvas_ProvaEmOutraImportacaoParaNaCapa(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())
	ctx := context.Background()
	primeira, err := s.Importar(ctx, curador, EnvioDeProva{Prova: pdfMinimo})
	if err != nil {
		t.Fatal(err)
	}
	if i := processarTudo(t, s, repo, primeira.ID); i.Estado != prova.EstadoEmRevisao {
		t.Fatalf("primeira: estado %s", i.Estado)
	}

	segunda, err := s.Importar(ctx, curador, EnvioDeProva{Prova: []byte("%PDF-1.7 outro arquivo da mesma prova")})
	if err != nil {
		t.Fatal(err)
	}
	final := processarTudo(t, s, repo, segunda.ID)

	if final.Estado != prova.EstadoCancelada || !strings.Contains(final.Erro, "já está em outra importação") {
		t.Fatalf("segunda: estado = %s, erro = %q", final.Estado, final.Erro)
	}
	if i := repo.importacoes[primeira.ID]; i.Estado != prova.EstadoEmRevisao {
		t.Fatalf("a primeira mudou: %s", i.Estado)
	}
}

// Capa sem código: a importação passa da conferência da capa, o curador
// preenche o cargo, e é a publicação que recusa a prova que já está no
// catálogo.
func TestProvas_PublicarRecusaProvaQueJaEstaNoCatalogo(t *testing.T) {
	t.Parallel()

	repo := &fakePublicacao{fakeProvas: novoFakeProvas()}
	s, volume := novoProvaServiceDeTeste(repo.fakeProvas, extratorDeDuasRegioes())
	s.Repo = repo
	s.ExigirConferencia = false
	volume.nomes["doc.pdf"] = true
	repo.irmas = []prova.Publicacao{{ID: "publicada", Conteudo: prova.Rascunho{
		Banca: "FCC", Orgao: "TJCE", Ano: 2026, Cargo: "E05", CargoNome: "Analista",
	}}}
	r := prova.Rascunho{Banca: "FCC", Orgao: "TJCE", Ano: 2026, Cargo: "E05", Caderno: "004", Total: 1,
		Questoes: []prova.Questao{questaoExtraida(1, true)},
		Gabarito: prova.Gabarito{Cargo: "E05", Caderno: "4", Respostas: map[string]string{"1": "A"}}}
	repo.importacoes["i"] = prova.Importacao{ID: "i", Documento: "doc", Estado: prova.EstadoEmRevisao, Versao: 1, Rascunho: r}

	_, err := s.Publicar(context.Background(), curador, "i", 1)

	var v ErrValidacao
	if !errors.As(err, &v) || !strings.Contains(err.Error(), "já está no catálogo") {
		t.Fatalf("err = %v, quer ErrValidacao dizendo que a prova já está no catálogo", err)
	}

	// A revisão da própria publicada publica normalmente.
	revisao := repo.importacoes["i"]
	revisao.ProvaID = "publicada"
	repo.importacoes["i"] = revisao
	if _, err := s.Publicar(context.Background(), curador, "i", 1); err != nil {
		t.Fatalf("revisão da publicada: %v", err)
	}
}

// A curadoria mostra de que arquivo veio cada importação: o nome do caderno e
// o do gabarito, também o que foi trocado depois e o da revisão aberta da
// publicada.
func TestProvas_GuardaONomeDosArquivos(t *testing.T) {
	t.Parallel()

	repo := &fakePublicacao{fakeProvas: novoFakeProvas()}
	s, _ := novoProvaServiceDeTeste(repo.fakeProvas, extratorDeDuasRegioes())
	s.Repo = repo
	ctx := context.Background()

	nova, err := s.Importar(ctx, curador, EnvioDeProva{
		Prova: pdfMinimo, Gabarito: pdfMinimo,
		NomeProva: `C:\fakepath\fcc-2025-trt-15-tecnico-prova.pdf`, NomeGabarito: " fcc-2025-trt-15-tecnico-gabarito.pdf ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if nova.NomeDocumento != "fcc-2025-trt-15-tecnico-prova.pdf" || nova.NomeGabarito != "fcc-2025-trt-15-tecnico-gabarito.pdf" {
		t.Fatalf("nomes = %q, %q", nova.NomeDocumento, nova.NomeGabarito)
	}

	emRevisao := repo.importacoes[nova.ID]
	emRevisao.Estado = prova.EstadoEmRevisao
	repo.importacoes[nova.ID] = emRevisao
	trocado, err := s.AtualizarGabarito(ctx, curador, nova.ID, emRevisao.Versao, pdfMinimo, "definitivo.pdf")
	if err != nil || trocado.NomeGabarito != "definitivo.pdf" || trocado.NomeDocumento != nova.NomeDocumento {
		t.Fatalf("depois de trocar o gabarito: %q, %q (%v)", trocado.NomeDocumento, trocado.NomeGabarito, err)
	}

	repo.importacoes["base"] = repo.importacoes[nova.ID]
	revisao, err := s.Revisar(ctx, curador, "prova-1")
	if err != nil || revisao.NomeDocumento != nova.NomeDocumento || revisao.NomeGabarito != "definitivo.pdf" {
		t.Fatalf("revisão da publicada: %q, %q (%v)", revisao.NomeDocumento, revisao.NomeGabarito, err)
	}
}

// O hash é o do caderno: a mesma prova com outro gabarito, ou sem ele, é o
// mesmo reenvio.
func TestProvas_HashEOCaderno(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())
	ctx := context.Background()

	var hashes []string
	for _, gabarito := range [][]byte{nil, pdfMinimo, []byte("%PDF-1.7 gabarito definitivo")} {
		i, err := s.Importar(ctx, curador, EnvioDeProva{Prova: pdfMinimo, Gabarito: gabarito})
		if err != nil {
			t.Fatal(err)
		}
		hashes = append(hashes, repo.importacoes[i.ID].Hash)
	}
	outra, err := s.Importar(ctx, curador, EnvioDeProva{Prova: []byte("%PDF-1.7 outra prova")})
	if err != nil {
		t.Fatal(err)
	}

	if hashes[0] == "" || hashes[0] != hashes[1] || hashes[1] != hashes[2] {
		t.Fatalf("hashes do mesmo caderno = %v, quer um só", hashes)
	}
	if repo.importacoes[outra.ID].Hash == hashes[0] {
		t.Fatal("outra prova com o mesmo hash")
	}
}

// Rascunho gravado antes da regra, com questão apontando para um texto que
// não existe: a tela e as pendências já o veem acertado.
func TestProvas_LigacaoParaTextoInexistenteNaoViraPendencia(t *testing.T) {
	t.Parallel()

	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extratorDeDuasRegioes())
	ctx := context.Background()
	nova, err := s.Importar(ctx, curador, EnvioDeProva{Prova: pdfMinimo})
	if err != nil {
		t.Fatal(err)
	}
	i := repo.importacoes[nova.ID]
	i.Estado = prova.EstadoEmRevisao
	q := questaoExtraida(4, true)
	q.Apoios = []string{"r0-t1", "rq4-t1"}
	i.Rascunho.Questoes = []prova.Questao{q}
	i.Rascunho.Apoios = []prova.Apoio{{ID: "r0-t1", Questoes: []int{1, 2, 3, 4}, Blocos: []prova.Bloco{{Tipo: "texto", Texto: "A vida"}}}}
	repo.importacoes[nova.ID] = i

	got, err := s.Obter(ctx, curador, nova.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range got.Pendencias {
		if strings.Contains(p, "não existe mais") {
			t.Fatalf("pendência de texto inexistente: %s", p)
		}
	}
	if a := got.Rascunho.Questoes[0].Apoios; len(a) != 1 || a[0] != "r0-t1" {
		t.Fatalf("textos da questão 4 = %v, quer só r0-t1", a)
	}
}

// fakeAnotacoes guarda anotações por usuário, prova e número, e responde a
// Questoes com as questões de uma prova publicada de mentira.
type fakeAnotacoes struct {
	*fakeProvas

	publicadas map[string][]prova.Questao
	anotacoes  map[string]prova.Anotacao
}

func (f *fakeAnotacoes) chave(usuario, provaID string, numero int) string {
	return fmt.Sprint(usuario, "/", provaID, "/", numero)
}

func (f *fakeAnotacoes) Questoes(_ context.Context, provaID string, numero int, _ string) ([]prova.Questao, error) {
	var out []prova.Questao
	for _, q := range f.publicadas[provaID] {
		if numero == 0 || q.Numero == numero {
			out = append(out, q)
		}
	}

	return out, nil
}

func (f *fakeAnotacoes) Anotacoes(_ context.Context, usuario, provaID string) ([]prova.Anotacao, error) {
	var out []prova.Anotacao
	for n := 1; n <= 200; n++ {
		if a, ok := f.anotacoes[f.chave(usuario, provaID, n)]; ok {
			out = append(out, a)
		}
	}

	return out, nil
}

func (f *fakeAnotacoes) SalvarAnotacao(_ context.Context, usuario, provaID string, a prova.Anotacao) (prova.Anotacao, error) {
	a.AtualizadaEm = time.Now()
	f.anotacoes[f.chave(usuario, provaID, a.Numero)] = a

	return a, nil
}

func (f *fakeAnotacoes) ExcluirAnotacao(_ context.Context, usuario, provaID string, numero int) error {
	delete(f.anotacoes, f.chave(usuario, provaID, numero))
	return nil
}

// A anotação é do estudante que a escreveu, em cada questão que existe na
// prova; apagar o texto apaga a anotação.
func TestProvas_AnotacaoDaQuestao(t *testing.T) {
	t.Parallel()

	const provaID, estudante, outro = "p1", "33333333-3333-3333-3333-333333333333", "44444444-4444-4444-4444-444444444444"
	repo := &fakeAnotacoes{
		fakeProvas: novoFakeProvas(),
		publicadas: map[string][]prova.Questao{provaID: {questaoExtraida(1, true), questaoExtraida(2, true)}},
		anotacoes:  map[string]prova.Anotacao{},
	}
	s := &ProvaService{Repo: repo}
	ctx := context.Background()

	a, err := s.Anotar(ctx, estudante, provaID, 2, "  ## Por que C\n\nO art. 5º diz…  ")
	if err != nil || a.Texto != "## Por que C\n\nO art. 5º diz…" || a.AtualizadaEm.IsZero() {
		t.Fatalf("Anotar = %+v, %v", a, err)
	}
	if lista, _ := s.Anotacoes(ctx, estudante, provaID); len(lista) != 1 || lista[0].Numero != 2 {
		t.Fatalf("anotações do estudante = %+v", lista)
	}
	if lista, _ := s.Anotacoes(ctx, outro, provaID); len(lista) != 0 {
		t.Fatalf("outro estudante vê a anotação: %+v", lista)
	}

	if _, err := s.Anotar(ctx, estudante, provaID, 9, "questão que não existe"); !errors.Is(err, prova.ErrNaoEncontrada) {
		t.Fatalf("anotar questão inexistente: err = %v, quer ErrNaoEncontrada", err)
	}
	var v ErrValidacao
	if _, err := s.Anotar(ctx, estudante, provaID, 1, strings.Repeat("a", prova.TamanhoMaximoDaAnotacao+1)); !errors.As(err, &v) {
		t.Fatalf("anotação longa demais: err = %v, quer ErrValidacao", err)
	}

	if _, err := s.Anotar(ctx, estudante, provaID, 2, "   "); err != nil {
		t.Fatal(err)
	}
	if lista, _ := s.Anotacoes(ctx, estudante, provaID); len(lista) != 0 {
		t.Fatalf("texto vazio não apagou: %+v", lista)
	}
}

// A questão de Conhecimentos Gerais que outro cargo do mesmo concurso já
// publicou chega na revisão como a publicada, com a resposta do gabarito desta
// prova; o recorte dela passa a ser desta importação também.
func TestProvas_ImportacaoReaproveitaQuestaoDeOutroCargo(t *testing.T) {
	t.Parallel()

	comum := func(numero int, texto string) prova.Questao {
		q := questaoExtraida(numero, true)
		q.Blocos[0].Texto = texto
		return q
	}
	lida := comum(1, "No  texto, Sêneca caracteriza o presente como")
	lida.Blocos = append(lida.Blocos, prova.Bloco{Tipo: "imagem", Arquivo: "recorte-desta"})
	extrator := extratorDeDuasRegioes()
	extrator.porRegiao = map[string]prova.Rascunho{
		"0": {Questoes: []prova.Questao{lida}},
		"1": {Questoes: []prova.Questao{comum(2, "Uma questão só deste cargo, sobre redes de computadores")}},
	}
	publicada := comum(1, "No texto, Sêneca caracteriza o presente como")
	publicada.Blocos = append(publicada.Blocos, prova.Bloco{Tipo: "imagem", Arquivo: "recorte-da-e04", Largura: 30})
	publicada.Resposta = "C" // o gabarito da E04 não vale aqui
	repo := novoFakeProvas()
	repo.irmas = []prova.Publicacao{{ID: "e04", Conteudo: prova.Rascunho{
		Banca: "FCC", Orgao: "TJCE", Ano: 2026, Cargo: "E04", Questoes: []prova.Questao{publicada},
	}}}
	s, _ := novoProvaServiceDeTeste(repo, extrator)
	nova, err := s.Importar(context.Background(), curador, EnvioDeProva{Prova: pdfMinimo, Gabarito: pdfMinimo})
	if err != nil {
		t.Fatal(err)
	}

	final := processarTudo(t, s, repo, nova.ID)

	q1, q2 := final.Rascunho.Questoes[0], final.Rascunho.Questoes[1]
	if q1.IgualA != "TJCE 2026 · E04, questão 1" || q1.Blocos[1].Arquivo != "recorte-da-e04" || q1.Resposta != "B" {
		t.Fatalf("questão 1 = %+v; quer a da E04, com a resposta B do gabarito desta prova", q1)
	}
	if repo.arquivo["recorte-da-e04"] != "png" {
		t.Fatal("o recorte reaproveitado não foi registrado nesta importação")
	}
	if q2.IgualA != "" {
		t.Fatalf("questão só deste cargo foi reaproveitada: %+v", q2)
	}
}

// A irmã foi publicada depois que esta foi importada: "procurar as já
// cadastradas" troca por referência o que ela tem, e a questão fica conferida.
func TestProvas_ProcurarCadastradasNaRevisao(t *testing.T) {
	t.Parallel()

	comum := func(numero int, texto string) prova.Questao {
		q := questaoExtraida(numero, true)
		q.Blocos[0].Texto = texto
		return q
	}
	extrator := extratorDeDuasRegioes()
	extrator.porRegiao = map[string]prova.Rascunho{
		"0": {Questoes: []prova.Questao{comum(1, "No texto, Sâneca caracteriza o presente como")}},
		"1": {Questoes: []prova.Questao{comum(2, "Uma questão só deste cargo, sobre redes de computadores")}},
	}
	repo := novoFakeProvas()
	s, _ := novoProvaServiceDeTeste(repo, extrator)
	ctx := context.Background()
	nova, err := s.Importar(ctx, curador, EnvioDeProva{Prova: pdfMinimo, Gabarito: pdfMinimo})
	if err != nil {
		t.Fatal(err)
	}
	revisao := processarTudo(t, s, repo, nova.ID)
	if revisao.Rascunho.Questoes[0].IgualA != "" {
		t.Fatal("reaproveitou sem irmã publicada")
	}

	repo.irmas = []prova.Publicacao{{ID: "f06", Conteudo: prova.Rascunho{
		Banca: "FCC", Orgao: "TJ-CE", Ano: 2026, Cargo: "F06",
		Questoes: []prova.Questao{comum(1, "No texto, Sêneca caracteriza o presente como")},
	}}}
	depois, err := s.ProcurarCadastradas(ctx, curador, nova.ID, revisao.Versao)
	if err != nil {
		t.Fatal(err)
	}

	q1 := depois.Rascunho.Questoes[0]
	if q1.IgualA != "TJCE 2026 · F06, questão 1" && q1.IgualA != "TJ-CE 2026 · F06, questão 1" {
		t.Fatalf("questão 1 = %+v", q1)
	}
	if !q1.Revisada || !strings.Contains(q1.Blocos[0].Texto, "Sêneca") {
		t.Fatalf("a referência não ficou conferida com o texto de lá: %+v", q1)
	}
}
