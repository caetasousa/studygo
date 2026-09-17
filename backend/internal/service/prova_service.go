package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"studygo/internal/domain/prova"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// ProvaService é a curadoria e a consulta do catálogo de provas.
//
// Curador é quem está na lista da configuração. O sistema não tem papéis, e a
// lista é de propósito o menor mecanismo que resolve. Toda escrita passa por
// autorizar: esconder o botão no frontend não é controle de acesso.
type ProvaService struct {
	Repo      port.ProvaRepository
	Processor port.ProvaProcessor
	Arquivos  port.ProvaArquivos
	Curadores map[string]bool
	// TodosCuradores faz de qualquer conta curadora; só o ambiente local liga.
	TodosCuradores bool

	// MaxPendentes limita as importações abertas por curador; cada uma ocupa
	// disco e, enquanto processa, a única vaga do worker.
	MaxPendentes int
	// MaxChamadas e MaxProcessamento são os tetos de custo de UMA importação.
	// Atingido um deles, ela para com o progresso preservado.
	MaxChamadas      int
	MaxProcessamento time.Duration
	// MaxEtapa limita cada chamada ao processador.
	MaxEtapa time.Duration
	// ExigirConferencia deixa de fora da publicação a questão que o curador
	// não conferiu. Desligada, é para testar o fluxo antes de haver quem
	// revise; produção fica com ela ligada.
	ExigirConferencia bool
}

// ImportacaoDeProva é a importação como a curadoria a vê, com o que vai e o
// que fica de fora da publicação já calculados.
type ImportacaoDeProva struct {
	prova.Importacao
	TotalEtapas int
	// Pendencias impedem publicar; Avisos, não. Publicaveis são as questões
	// que vão ao catálogo, e DeFora, as que ficam, com o motivo.
	Pendencias, Avisos     []string
	Publicaveis            int
	DeFora                 []prova.DeFora
	ConferenciaObrigatoria bool
}

// criterios da publicação pela revisão: conferida, se o ambiente pede, e com
// resposta no gabarito.
func (s *ProvaService) criterios() prova.Criterios {
	return prova.Criterios{Conferencia: s.ExigirConferencia, Gabarito: true}
}

func (s *ProvaService) montar(i prova.Importacao) ImportacaoDeProva {
	// Rascunho gravado antes da regra chega com ligação para texto que não
	// existe, ou com bloco vazio da extração; acertado aqui, a tela e as
	// pendências já o veem certo.
	i.Rascunho.AcertarApoios()
	i.Rascunho.LimparBlocos()

	pub, fora := i.Rascunho.ParaPublicar(s.criterios())

	return ImportacaoDeProva{
		Importacao:             i,
		TotalEtapas:            prova.TotalEtapas(len(i.Regioes)),
		Pendencias:             i.Rascunho.Pendencias(s.criterios()),
		Avisos:                 i.Rascunho.Avisos(),
		Publicaveis:            len(pub.Questoes),
		DeFora:                 fora,
		ConferenciaObrigatoria: s.ExigirConferencia,
	}
}

func (s *ProvaService) Curador(usuario string) bool {
	return s.TodosCuradores || s.Curadores[usuario]
}

func (s *ProvaService) autorizar(usuario string) error {
	if !s.Curador(usuario) {
		return prova.ErrAcesso
	}

	return nil
}

func (s *ProvaService) carregar(ctx context.Context, usuario, id string) (prova.Importacao, error) {
	if err := s.autorizar(usuario); err != nil {
		return prova.Importacao{}, err
	}

	return s.Repo.Obter(ctx, id)
}

// EnvioDeProva são os PDFs de uma importação, com os nomes com que o curador
// os enviou.
type EnvioDeProva struct {
	Prova, Gabarito         []byte
	NomeProva, NomeGabarito string
}

// Importar guarda os PDFs e enfileira a extração.
//
// Os arquivos vão para o disco antes da linha, porque uma linha apontando para
// arquivo inexistente quebraria o worker. Quando a linha não nasce — reenvio
// dos mesmos PDFs, limite atingido, erro —, eles saem do disco: nenhuma linha
// os referencia, e a limpeza só enxerga o que está no banco.
func (s *ProvaService) Importar(ctx context.Context, usuario string, e EnvioDeProva) (ImportacaoDeProva, error) {
	if err := s.autorizar(usuario); err != nil {
		return ImportacaoDeProva{}, err
	}
	pdf, gabarito := e.Prova, e.Gabarito
	if !parecePDF(pdf) || (len(gabarito) > 0 && !parecePDF(gabarito)) {
		return ImportacaoDeProva{}, erroDeValidacao("envie a prova e o gabarito em PDF")
	}

	// O hash é só o do caderno: a mesma prova com outro gabarito, ou sem ele,
	// é a mesma prova — o gabarito novo entra por "Trocar o gabarito". Arquivo
	// diferente da mesma prova é pego depois, pela capa (marcarSeRepetida).
	h := sha256.Sum256(pdf)

	i := prova.Importacao{
		ID:            uuid.NewString(),
		Criador:       usuario,
		Hash:          hex.EncodeToString(h[:]),
		Documento:     uuid.NewString(),
		NomeDocumento: prova.NomeDoArquivo(e.NomeProva),
		Estado:        prova.EstadoNaFila,
		Rascunho:      prova.Rascunho{Banca: "FCC"},
	}
	gravados := []string{i.Documento + ".pdf"}
	if err := s.Arquivos.Guardar(i.Documento, pdf); err != nil {
		return ImportacaoDeProva{}, err
	}
	if len(gabarito) > 0 {
		i.GabaritoArquivo = uuid.NewString()
		i.NomeGabarito = prova.NomeDoArquivo(e.NomeGabarito)
		gravados = append(gravados, i.GabaritoArquivo+".pdf")
		if err := s.Arquivos.Guardar(i.GabaritoArquivo, gabarito); err != nil {
			s.descartar(gravados)
			return ImportacaoDeProva{}, err
		}
	}

	criada, err := s.Repo.Criar(ctx, i, s.MaxPendentes)
	if err != nil || criada.ID != i.ID {
		s.descartar(gravados)
	}
	if err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.montar(criada), nil
}

// descartar é melhor esforço: o arquivo que ficar para trás não tem linha que
// o referencie, então ninguém chega a ele.
func (s *ProvaService) descartar(nomes []string) {
	for _, n := range nomes {
		_ = s.Arquivos.Remover(n)
	}
}

func parecePDF(b []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(b[:min(len(b), 1024)]), []byte("%PDF-"))
}

func (s *ProvaService) Obter(ctx context.Context, usuario, id string) (ImportacaoDeProva, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.montar(i), nil
}

// Listar traz resumos: a lista não mostra conteúdo nem pendências.
func (s *ProvaService) Listar(ctx context.Context, usuario string) ([]prova.Importacao, error) {
	if err := s.autorizar(usuario); err != nil {
		return nil, err
	}

	return s.Repo.ListarResumos(ctx)
}

// Salvar grava a revisão do curador. O registro das chamadas ao Gemini não é
// editável: vem do que está gravado, não do que o navegador mandou.
func (s *ProvaService) Salvar(ctx context.Context, usuario, id string, versao int, novo prova.Rascunho) (ImportacaoDeProva, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return ImportacaoDeProva{}, err
	}
	if i.Estado != prova.EstadoEmRevisao || i.Versao != versao {
		return ImportacaoDeProva{}, prova.ErrConflito
	}
	if err := s.verificarArquivos(ctx, i.ID, novo); err != nil {
		return ImportacaoDeProva{}, err
	}

	novo.Extracoes = i.Rascunho.Extracoes
	// Os dois lados acertados: ligação reordenada e bloco vazio que saiu não
	// são edição da questão.
	novo.AcertarApoios()
	i.Rascunho.AcertarApoios()
	novo.LimparBlocos()
	i.Rascunho.LimparBlocos()
	prova.InvalidarEdicoes(i.Rascunho, &novo)
	prova.ConfirmarPelosRecortes(i.Rascunho, &novo)
	// Figura recortada agora na revisão ganha o tamanho do caderno.
	novo.DimensionarFiguras()
	i.Rascunho = novo
	i.Erro = ""
	if err := s.Repo.Salvar(ctx, i, versao); err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.Obter(ctx, usuario, id)
}

// verificarArquivos impede que o rascunho aponte para recorte de outra
// importação — que o curador talvez nem possa ver — ou para arquivo sumido.
func (s *ProvaService) verificarArquivos(ctx context.Context, importacao string, r prova.Rascunho) error {
	ids, err := s.Repo.ArquivosDaImportacao(ctx, importacao)
	if err != nil {
		return err
	}
	permitidos := make(map[string]bool, len(ids))
	for _, id := range ids {
		permitidos[id] = true
	}
	for _, id := range r.Arquivos() {
		if !permitidos[id] || !s.Arquivos.Existe(id, "png") {
			return erroDeValidacao("uma imagem do rascunho não pertence a esta importação; refaça o recorte")
		}
	}

	return nil
}

// Publicar leva o rascunho ao catálogo. Repetir a publicação devolve a mesma
// prova em vez de criar outra.
func (s *ProvaService) Publicar(ctx context.Context, usuario, id string, versao int) (string, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return "", err
	}
	if i.Estado == prova.EstadoPublicada {
		return i.ProvaID, nil
	}
	if i.Estado != prova.EstadoEmRevisao || i.Versao != versao {
		return "", prova.ErrConflito
	}
	i.Rascunho.AcertarApoios()
	i.Rascunho.LimparBlocos()
	i.Rascunho.DimensionarFiguras()
	if p := i.Rascunho.Pendencias(s.criterios()); len(p) > 0 {
		return "", erroDeValidacao(p[0])
	}
	// Vai ao catálogo só o que está pronto; o resto fica nas excluídas da
	// revisão publicada.
	i.Rascunho, _ = i.Rascunho.ParaPublicar(s.criterios())
	// A capa sem código deixa a importação passar da conferência da capa; o
	// curador preenche o cargo, e é aqui que a repetida para.
	p, repetida, err := s.noCatalogo(ctx, i)
	if err != nil {
		return "", err
	}
	if repetida {
		return "", erroDeValidacao(fmt.Sprintf(
			"esta prova (%s) já está no catálogo; para corrigi-la, abra a publicada e use \"Abrir revisão\"",
			p.Conteudo.Rotulo(),
		))
	}
	if err := s.verificarArquivos(ctx, i.ID, i.Rascunho); err != nil {
		return "", err
	}
	if !s.Arquivos.Existe(i.Documento, "pdf") {
		return "", fmt.Errorf("PDF original %s ausente do volume de provas", i.Documento)
	}

	return s.Repo.Publicar(ctx, i, usuario)
}

// Cancelar tira a importação da fila. Uma chamada já em curso pode terminar,
// mas o resultado dela não é aceito: a tentativa perde a reserva.
func (s *ProvaService) Cancelar(ctx context.Context, usuario, id string, versao int) (ImportacaoDeProva, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return ImportacaoDeProva{}, err
	}
	if i.Versao != versao || i.Estado == prova.EstadoPublicada || i.Estado == prova.EstadoCancelada {
		return ImportacaoDeProva{}, prova.ErrConflito
	}

	i.Cancelar()
	if err := s.Repo.Salvar(ctx, i, versao); err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.Obter(ctx, usuario, id)
}

// Reler devolve à fila uma importação em revisão só para reler as questões
// que continuam incompletas, cada uma numa região centrada onde foi cortada.
// O resto do rascunho fica como está: a releitura só troca questão incompleta
// por uma leitura melhor.
func (s *ProvaService) Reler(ctx context.Context, usuario, id string, versao int) (ImportacaoDeProva, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return ImportacaoDeProva{}, err
	}
	if i.Versao != versao || i.Estado != prova.EstadoEmRevisao {
		return ImportacaoDeProva{}, prova.ErrConflito
	}
	if s.estourouTeto(i) {
		return ImportacaoDeProva{}, erroDeValidacao("esta importação atingiu o teto de chamadas ou de tempo")
	}

	regioes := slices.DeleteFunc(slices.Clone(i.Regioes), prova.EReleitura)
	releituras := i.Rascunho.Releituras(regioes)
	if len(releituras) == 0 {
		return ImportacaoDeProva{}, erroDeValidacao("não há questão incompleta que dê para reler")
	}
	i.Regioes = append(regioes, releituras...)
	i.Etapa = prova.EtapaPrimeiraRegiao + len(regioes)
	i.Estado = prova.EstadoNaFila
	i.Erro = ""
	if err := s.Repo.Salvar(ctx, i, versao); err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.Obter(ctx, usuario, id)
}

// RelerTrecho devolve à fila uma importação em revisão só para ler o trecho
// que o curador marcou na questão `numero` — a parte que a extração leu mal
// mesmo depois das releituras. O que o trecho trouxer entra na questão
// (prova.Rascunho.AplicarTrecho); o resto do rascunho fica como está.
func (s *ProvaService) RelerTrecho(ctx context.Context, usuario, id string, versao, numero int, o prova.Origem) (ImportacaoDeProva, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return ImportacaoDeProva{}, err
	}
	if i.Versao != versao || i.Estado != prova.EstadoEmRevisao {
		return ImportacaoDeProva{}, prova.ErrConflito
	}
	if s.estourouTeto(i) {
		return ImportacaoDeProva{}, erroDeValidacao("esta importação atingiu o teto de chamadas ou de tempo")
	}
	trecho, ok := prova.NovoTrecho(i.Regioes, numero, o)
	if !ok {
		return ImportacaoDeProva{}, erroDeValidacao("marque o trecho da questão dentro de uma página do caderno")
	}

	i.RelerTrecho(trecho)
	if err := s.Repo.Salvar(ctx, i, versao); err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.Obter(ctx, usuario, id)
}

// RelerTextoDeApoio põe na fila a leitura do trecho que o curador marcou em
// volta de um texto de apoio que a extração leu mal — cortado, com parágrafo
// faltando. Como o da questão, só o trecho é lido.
func (s *ProvaService) RelerTextoDeApoio(ctx context.Context, usuario, id string, versao int, apoio string, o prova.Origem) (ImportacaoDeProva, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return ImportacaoDeProva{}, err
	}
	if i.Versao != versao || i.Estado != prova.EstadoEmRevisao {
		return ImportacaoDeProva{}, prova.ErrConflito
	}
	if s.estourouTeto(i) {
		return ImportacaoDeProva{}, erroDeValidacao("esta importação atingiu o teto de chamadas ou de tempo")
	}
	if !slices.ContainsFunc(i.Rascunho.Apoios, func(a prova.Apoio) bool { return a.ID == apoio }) {
		return ImportacaoDeProva{}, erroDeValidacao("o texto de apoio não existe no rascunho salvo; salve a revisão antes")
	}
	trecho, ok := prova.NovoTrechoDeApoio(i.Regioes, apoio, o)
	if !ok {
		return ImportacaoDeProva{}, erroDeValidacao("marque o trecho do texto dentro de uma página do caderno")
	}

	i.RelerTrecho(trecho)
	if err := s.Repo.Salvar(ctx, i, versao); err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.Obter(ctx, usuario, id)
}

// ProcurarCadastradas compara de novo a importação em revisão com as provas
// publicadas do mesmo concurso — a irmã pode ter sido publicada depois que esta
// foi importada — e troca por referência as questões que elas já têm.
func (s *ProvaService) ProcurarCadastradas(ctx context.Context, usuario, id string, versao int) (ImportacaoDeProva, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return ImportacaoDeProva{}, err
	}
	if i.Versao != versao || i.Estado != prova.EstadoEmRevisao {
		return ImportacaoDeProva{}, prova.ErrConflito
	}
	if _, err := s.reaproveitar(ctx, &i); err != nil {
		return ImportacaoDeProva{}, err
	}
	i.Rascunho.AcertarApoios()
	i.Rascunho.DimensionarFiguras()
	i.Rascunho.ConfirmarSemProblema()
	if err := s.Repo.Salvar(ctx, i, versao); err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.Obter(ctx, usuario, id)
}

// Excluir apaga a importação e o rascunho dela. Os arquivos que ficarem sem
// nenhuma importação saem na limpeza do worker.
func (s *ProvaService) Excluir(ctx context.Context, usuario, id string, versao int) error {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return err
	}
	if i.Versao != versao {
		return prova.ErrConflito
	}
	if !i.Excluivel() {
		if i.Estado == prova.EstadoPublicada {
			return erroDeValidacao("a importação publicada é o histórico da prova; para tirá-la do catálogo, use \"Tirar do catálogo\" na prova")
		}

		return erroDeValidacao("a importação está processando uma etapa; cancele antes de excluir")
	}

	return s.Repo.Excluir(ctx, i)
}

// Reprocessar devolve à fila uma importação que falhou. Ela retoma da etapa
// em que parou; o que já foi extraído não é refeito.
func (s *ProvaService) Reprocessar(ctx context.Context, usuario, id string, versao int) (ImportacaoDeProva, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return ImportacaoDeProva{}, err
	}
	if i.Versao != versao || i.Estado != prova.EstadoFalhou {
		return ImportacaoDeProva{}, prova.ErrConflito
	}
	if s.estourouTeto(i) {
		return ImportacaoDeProva{}, erroDeValidacao(
			"esta importação atingiu o teto de chamadas ou de tempo; cancele e importe de novo",
		)
	}

	i.Estado = prova.EstadoNaFila
	i.Erro = ""
	if err := s.Repo.Salvar(ctx, i, versao); err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.Obter(ctx, usuario, id)
}

func (s *ProvaService) estourouTeto(i prova.Importacao) bool {
	return i.Chamadas >= s.MaxChamadas ||
		time.Duration(i.ProcessadoMS)*time.Millisecond >= s.MaxProcessamento
}

// Recortar gera o PNG de um retângulo do original, sem chamar o Gemini. Serve
// tanto para ajustar uma figura quanto para mostrar a região ao lado da questão.
func (s *ProvaService) Recortar(ctx context.Context, usuario, id string, versao int, o prova.Origem) (string, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return "", err
	}
	if i.Estado != prova.EstadoEmRevisao || i.Versao != versao {
		return "", prova.ErrConflito
	}
	if o.Pagina < 1 || len(o.Retangulo) != 4 {
		return "", erroDeValidacao("recorte inválido")
	}

	arquivo, err := s.Processor.Recortar(ctx, i.Documento, o)
	if err != nil {
		return "", err
	}
	if err := s.Repo.RegistrarArquivo(ctx, i.ID, arquivo, "png"); err != nil {
		return "", err
	}

	return arquivo, nil
}

// AtualizarGabarito troca o gabarito de um rascunho em revisão sem reextrair
// as questões — é o caso do gabarito definitivo que sai depois do preliminar.
func (s *ProvaService) AtualizarGabarito(ctx context.Context, usuario, id string, versao int, pdf []byte, nome string) (ImportacaoDeProva, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return ImportacaoDeProva{}, err
	}
	if i.Estado != prova.EstadoEmRevisao || i.Versao != versao {
		return ImportacaoDeProva{}, prova.ErrConflito
	}
	if !parecePDF(pdf) {
		return ImportacaoDeProva{}, erroDeValidacao("o gabarito precisa ser PDF")
	}

	arquivo := uuid.NewString()
	if err := s.Arquivos.Guardar(arquivo, pdf); err != nil {
		return ImportacaoDeProva{}, err
	}
	if err := s.Repo.RegistrarArquivo(ctx, id, arquivo, "pdf"); err != nil {
		s.descartar([]string{arquivo + ".pdf"})
		return ImportacaoDeProva{}, err
	}

	i.GabaritoArquivo = arquivo
	i.NomeGabarito = prova.NomeDoArquivo(nome)
	i.Etapa = prova.EtapaSoGabarito
	i.Estado = prova.EstadoNaFila
	if err := s.Repo.Salvar(ctx, i, versao); err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.Obter(ctx, usuario, id)
}

// Revisar abre um rascunho novo a partir da revisão publicada. A publicada
// continua no catálogo até a próxima publicação.
func (s *ProvaService) Revisar(ctx context.Context, usuario, provaID string) (ImportacaoDeProva, error) {
	if err := s.autorizar(usuario); err != nil {
		return ImportacaoDeProva{}, err
	}
	p, err := s.Repo.Publicacao(ctx, provaID)
	if err != nil {
		return ImportacaoDeProva{}, err
	}
	base, err := s.Repo.ImportacaoDaPublicacao(ctx, provaID)
	if err != nil {
		return ImportacaoDeProva{}, err
	}

	nova := prova.Importacao{
		ID:              uuid.NewString(),
		Criador:         usuario,
		Documento:       base.Documento,
		GabaritoArquivo: base.GabaritoArquivo,
		NomeDocumento:   base.NomeDocumento,
		NomeGabarito:    base.NomeGabarito,
		Estado:          prova.EstadoEmRevisao,
		Etapa:           prova.TotalEtapas(len(base.Regioes)),
		Regioes:         base.Regioes,
		Rascunho:        p.Conteudo,
		ProvaID:         provaID,
	}
	criada, err := s.Repo.Criar(ctx, nova, s.MaxPendentes)
	if err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.montar(criada), nil
}

// Reextrair abre uma revisão da prova publicada extraindo o caderno de novo,
// do zero, com o extrator de agora — é o caminho quando ele melhora, já que
// reenviar os mesmos PDFs devolve a importação existente. A publicada segue no
// catálogo até a nova ser publicada, como a próxima revisão da mesma prova.
func (s *ProvaService) Reextrair(ctx context.Context, usuario, provaID string) (ImportacaoDeProva, error) {
	if err := s.autorizar(usuario); err != nil {
		return ImportacaoDeProva{}, err
	}
	if _, err := s.Repo.Publicacao(ctx, provaID); err != nil {
		return ImportacaoDeProva{}, err
	}
	base, err := s.Repo.ImportacaoDaPublicacao(ctx, provaID)
	if err != nil {
		return ImportacaoDeProva{}, err
	}

	criada, err := s.Repo.Criar(ctx, prova.Importacao{
		ID:              uuid.NewString(),
		Criador:         usuario,
		Documento:       base.Documento,
		GabaritoArquivo: base.GabaritoArquivo,
		NomeDocumento:   base.NomeDocumento,
		NomeGabarito:    base.NomeGabarito,
		Estado:          prova.EstadoNaFila,
		Rascunho:        prova.Rascunho{Banca: "FCC"},
		ProvaID:         provaID,
	}, s.MaxPendentes)
	if err != nil {
		return ImportacaoDeProva{}, err
	}

	return s.montar(criada), nil
}

// ExcluirProva apaga de vez a prova e tudo o que veio dela — é o caso da mesma
// prova importada duas vezes, com títulos diferentes. Diferente de Retirar,
// não tem volta.
func (s *ProvaService) ExcluirProva(ctx context.Context, usuario, provaID string) error {
	if err := s.autorizar(usuario); err != nil {
		return err
	}
	err := s.Repo.ExcluirProva(ctx, provaID)
	if errors.Is(err, prova.ErrConflito) {
		return erroDeValidacao("uma importação desta prova está processando; espere terminar ou cancele-a antes de excluir")
	}

	return err
}

// RenomearProva corrige o título da prova publicada sem abrir revisão. Só o
// nome: o código do cargo confere o gabarito e acha a prova repetida, e muda
// pela revisão, que passa pelas pendências.
func (s *ProvaService) RenomearProva(ctx context.Context, usuario, provaID, cargoNome string) error {
	if err := s.autorizar(usuario); err != nil {
		return err
	}
	nome, ok := prova.NomeDoCargo(cargoNome)
	if !ok {
		return erroDeValidacao("o título precisa ter de 1 a 200 caracteres")
	}

	return s.Repo.RenomearProva(ctx, provaID, nome)
}

// Retirar tira a prova do catálogo. As revisões continuam gravadas.
func (s *ProvaService) Retirar(ctx context.Context, usuario, provaID string) error {
	if err := s.autorizar(usuario); err != nil {
		return err
	}

	return s.Repo.Retirar(ctx, provaID)
}

// PorPaginaCatalogo é o tamanho da página do catálogo.
const PorPaginaCatalogo = 20

func (s *ProvaService) Catalogo(ctx context.Context, f port.FiltroCatalogo) ([]prova.Publicacao, error) {
	f.Offset = max(f.Offset, 0)
	f.Limite = PorPaginaCatalogo

	return s.Repo.Catalogo(ctx, f)
}

// Publicacao devolve a revisão em vigor. Com número ou disciplina, só as
// questões que casam — a busca é do banco, não de um filtro em memória.
func (s *ProvaService) Publicacao(ctx context.Context, id string, numero int, disciplina string) (prova.Publicacao, error) {
	p, err := s.Repo.Publicacao(ctx, id)
	if err != nil || (numero == 0 && disciplina == "") {
		return p, err
	}

	p.Conteudo.Questoes, err = s.Repo.Questoes(ctx, id, numero, disciplina)

	return p, err
}

// QuestoesAvulsas são as questões do catálogo para treinar fora da prova,
// filtradas por matéria e ano.
//
// O filtro roda aqui, e não no SQL, porque decidir que duas grafias são a
// mesma matéria é regra do domínio; a lista inteira são linhas curtas, sem o
// conteúdo das questões.
func (s *ProvaService) QuestoesAvulsas(ctx context.Context, f prova.FiltroDeAvulsas) ([]prova.QuestaoAvulsa, error) {
	qs, err := s.Repo.QuestoesAvulsas(ctx)
	if err != nil {
		return nil, err
	}

	return prova.Avulsas(qs, f), nil
}

// Anotacoes devolve as anotações do usuário nas questões de uma prova.
func (s *ProvaService) Anotacoes(ctx context.Context, usuario, provaID string) ([]prova.Anotacao, error) {
	return s.Repo.Anotacoes(ctx, usuario, provaID)
}

// Anotar grava a anotação do usuário numa questão da prova; texto vazio apaga.
// A questão tem de existir na prova em vigor — anotação para número que não
// existe ficaria sem onde aparecer.
func (s *ProvaService) Anotar(ctx context.Context, usuario, provaID string, numero int, texto string) (prova.Anotacao, error) {
	texto = strings.TrimSpace(texto)
	if utf8.RuneCountInString(texto) > prova.TamanhoMaximoDaAnotacao {
		return prova.Anotacao{}, erroDeValidacao(fmt.Sprintf(
			"a anotação passou de %d caracteres; divida em partes ou resuma", prova.TamanhoMaximoDaAnotacao))
	}
	questoes, err := s.Repo.Questoes(ctx, provaID, numero, "")
	if err != nil {
		return prova.Anotacao{}, err
	}
	if numero < 1 || len(questoes) == 0 {
		return prova.Anotacao{}, prova.ErrNaoEncontrada
	}

	a := prova.Anotacao{Numero: numero, Texto: texto}
	if texto == "" {
		return a, s.Repo.ExcluirAnotacao(ctx, usuario, provaID, numero)
	}

	return s.Repo.SalvarAnotacao(ctx, usuario, provaID, a)
}

// Arquivo devolve o caminho de um arquivo que o usuário pode ver. Curador vê
// rascunhos; os demais, só o que pertence a uma prova publicada e visível.
func (s *ProvaService) Arquivo(ctx context.Context, usuario, id string) (string, error) {
	ext, err := s.Repo.Arquivo(ctx, id, s.Curador(usuario))
	if err != nil {
		return "", err
	}

	return s.Arquivos.Caminho(id, ext)
}

// ProcessarUma executa a próxima etapa pendente da fila, se houver.
//
// Uma etapa por vez, uma importação por vez: a VPS é pequena e o Gemini cobra
// por chamada. A reserva é renovada enquanto a etapa roda; se ela se perder —
// o curador cancelou, ou o worker travou e outro assumiu —, o contexto cai e o
// resultado não é gravado.
func (s *ProvaService) ProcessarUma(ctx context.Context, logger *slog.Logger) error {
	i, err := s.Repo.Reservar(ctx)
	if errors.Is(err, prova.ErrNaoEncontrada) {
		return nil
	}
	if err != nil {
		return err
	}

	// Chamadas conta reservas, e Reservar já contou esta.
	if i.Chamadas > s.MaxChamadas || time.Duration(i.ProcessadoMS)*time.Millisecond >= s.MaxProcessamento {
		return s.Repo.Falhar(ctx, i,
			"A importação atingiu o teto de chamadas ou de tempo de processamento; o progresso foi preservado.",
			0,
		)
	}

	etapaCtx, cancelar := context.WithTimeout(ctx, s.MaxEtapa)
	defer cancelar()
	pararRenovacao := s.manterReserva(etapaCtx, cancelar, i)
	defer pararRenovacao()

	executada := i.Etapa
	inicio := time.Now()
	// O erro da tentativa anterior sai com o sucesso desta; o que a etapa
	// deixar em Erro é o motivo de ela ter parado a importação.
	i.Erro = ""
	resultado, err := s.executarEtapa(etapaCtx, &i)
	duracao := time.Since(inicio)
	i.ProcessadoMS += duracao.Milliseconds()

	logger.InfoContext(ctx, "etapa de prova",
		slog.String("importacao", i.ID),
		slog.Int("etapa", executada),
		slog.Duration("duracao", duracao),
		slog.Int("chamadas", i.Chamadas),
		slog.Bool("ok", err == nil),
	)

	if err != nil {
		// Só a recusa explícita do documento é definitiva; rede, timeout e
		// sobrecarga do Gemini passam, e a fila tenta de novo com espera.
		var espera time.Duration
		if !errors.Is(err, port.ErrDocumentoRecusado) {
			espera = prova.EsperaParaRepetir(i.Falhas)
		}

		return s.Repo.Falhar(ctx, i, err.Error(), espera)
	}

	// A etapa pode ter parado a importação — a prova já estava no catálogo.
	if i.Estado != prova.EstadoCancelada {
		i.Etapa, i.Estado = prova.ProximaEtapa(executada, len(i.Regioes))
	}

	return s.Repo.ConcluirEtapa(ctx, i, executada, resultado, duracao)
}

// manterReserva renova a reserva a cada 20s — bem dentro dos 2 minutos que o
// repositório concede — e devolve a função que para a renovação.
func (s *ProvaService) manterReserva(ctx context.Context, cancelar context.CancelFunc, i prova.Importacao) func() {
	feito := make(chan struct{})
	go func() {
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-feito:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				if ok, err := s.Repo.Renovar(ctx, i.ID, i.Tentativa); err != nil || !ok {
					cancelar()
					return
				}
			}
		}
	}()

	return func() { close(feito) }
}

// executarEtapa roda a etapa atual e devolve o resultado isolado dela, que
// fica registrado para auditoria.
func (s *ProvaService) executarEtapa(ctx context.Context, i *prova.Importacao) (any, error) {
	primeira := prova.EtapaPrimeiraRegiao

	switch {
	case i.Etapa == prova.EtapaPreparar:
		regioes, err := s.Processor.Preparar(ctx, i.Documento)
		if err != nil {
			return nil, err
		}
		if len(regioes) == 0 {
			return nil, fmt.Errorf("%w: o PDF não tem páginas", port.ErrDocumentoRecusado)
		}
		i.Regioes = regioes

		return regioes, nil

	case i.Etapa == prova.EtapaMetadados:
		if len(i.Regioes) == 0 {
			return nil, fmt.Errorf("%w: o PDF não tem regiões para ler a capa", port.ErrDocumentoRecusado)
		}
		m, err := s.Processor.Metadados(ctx, i.Documento, i.Regioes[0])
		if err != nil {
			return nil, err
		}
		i.Rascunho.AplicarMetadados(m)
		// A capa já diz que prova é: repetida, para aqui, antes de pagar a
		// leitura das questões.
		if err := s.marcarSeRepetida(ctx, i); err != nil {
			return nil, err
		}

		return m, nil

	case i.Etapa == prova.EtapaGabarito || i.Etapa == prova.EtapaSoGabarito:
		if i.GabaritoArquivo != "" {
			// A capa já foi lida (e, na troca, conferida): o caderno dela
			// escolhe o tipo quando o arquivo traz vários.
			g, err := s.Processor.Gabarito(ctx, i.GabaritoArquivo, i.Rascunho.Caderno)
			if err != nil {
				return nil, err
			}
			i.Rascunho.Gabarito = g
			i.Rascunho.TotalPeloGabarito()
		}
		if i.Etapa == prova.EtapaSoGabarito {
			i.Rascunho.AplicarGabarito()
		}

		return i.Rascunho.Gabarito, nil

	case i.Etapa == prova.EtapaTrecho:
		if trecho, apoio, ok := i.TrechoDeApoioPendente(); ok {
			lido, err := s.Processor.Extrair(ctx, i.Documento, trecho)
			if errors.Is(err, port.ErrDocumentoRecusado) {
				i.Rascunho.TrechoDeApoioRecusado(apoio, err.Error())
				return map[string]string{"recusado": err.Error()}, nil
			}
			if err != nil {
				return nil, err
			}
			i.Rascunho.AplicarTrechoDeApoio(apoio, lido)

			return lido, nil
		}
		trecho, numero, ok := i.TrechoPendente()
		if !ok {
			return nil, fmt.Errorf("%w: não há trecho marcado para ler", port.ErrDocumentoRecusado)
		}
		lido, err := s.Processor.Extrair(ctx, i.Documento, trecho)
		// Como a releitura, o trecho recusado não derruba a importação: o
		// curador volta à revisão com a questão como estava e o motivo.
		if errors.Is(err, port.ErrDocumentoRecusado) {
			i.Rascunho.TrechoRecusado(numero, err.Error())
			return map[string]string{"recusado": err.Error()}, nil
		}
		if err != nil {
			return nil, err
		}
		i.Rascunho.AplicarTrecho(numero, lido)

		return lido, nil

	case i.Etapa >= primeira && i.Etapa < primeira+len(i.Regioes):
		regiao := i.Regioes[i.Etapa-primeira]
		parcial, err := s.Processor.Extrair(ctx, i.Documento, regiao)
		// Releitura recusada não derruba a importação: é reforço, e perder a
		// prova inteira por ela é pior do que uma questão para conferir.
		if err != nil && prova.EReleitura(regiao) && errors.Is(err, port.ErrDocumentoRecusado) {
			i.Rascunho.ReleituraRecusada(regiao, err.Error())
			return map[string]string{"recusada": err.Error()}, nil
		}
		if err != nil {
			return nil, err
		}
		if prova.EReleitura(regiao) {
			i.Rascunho.AplicarReleitura(parcial)
		} else {
			i.Rascunho.Mesclar(parcial)
		}
		// Depois da última região, cada questão que ficou cortada vira uma
		// região a mais na fila, com o recorte só dela.
		if i.Etapa == primeira+len(i.Regioes)-1 {
			i.Regioes = append(i.Regioes, i.Rascunho.Releituras(i.Regioes)...)
		}

		return parcial, nil

	default:
		i.Rascunho.LimparBlocos()
		i.Rascunho.AplicarGabarito()
		i.Rascunho.OrdenarQuestoes()
		i.Rascunho.HerdarDisciplinas()
		reaproveitadas, err := s.reaproveitar(ctx, i)
		if err != nil {
			return nil, err
		}
		materias, err := s.classificar(ctx, &i.Rascunho)
		if err != nil {
			return nil, err
		}
		i.Rascunho.DimensionarFiguras()
		i.Rascunho.ConfirmarSemProblema()

		return map[string]any{
			"questoes": len(i.Rascunho.Questoes), "materias": materias, "reaproveitadas": reaproveitadas,
		}, nil
	}
}

// reaproveitar troca pelas já publicadas as questões e os textos que as provas
// do mesmo concurso (banca, órgão e ano) já têm — as de Conhecimentos Gerais
// repetem em todos os cargos. Os recortes delas são registrados nesta
// importação: é o registro que deixa a figura ser vista e a guarda da limpeza.
// marcarSeRepetida para a importação da prova que já está no catálogo ou em
// outra importação ativa (prova.Importacao.JaImportada). A revisão de uma
// prova publicada — Revisar, Reextrair — é ela mesma, e não conta.
func (s *ProvaService) marcarSeRepetida(ctx context.Context, i *prova.Importacao) error {
	if i.ProvaID != "" {
		return nil
	}
	p, publicada, err := s.noCatalogo(ctx, *i)
	if err != nil {
		return err
	}
	if publicada {
		i.JaImportada(p.Conteudo, true)
		return nil
	}
	outras, err := s.Repo.ImportacoesAtivasDoAno(ctx, i.Rascunho.Banca, i.Rascunho.Ano, i.ID)
	if err != nil {
		return err
	}
	for _, o := range outras {
		if o.ProvaID == "" && prova.MesmaProva(i.Rascunho, o.Rascunho) {
			i.JaImportada(o.Rascunho, false)
			return nil
		}
	}

	return nil
}

// noCatalogo acha, entre as provas publicadas, a mesma prova da importação —
// menos ela própria, quando a importação é a revisão de uma.
func (s *ProvaService) noCatalogo(ctx context.Context, i prova.Importacao) (prova.Publicacao, bool, error) {
	if i.Rascunho.Ano <= 0 {
		return prova.Publicacao{}, false, nil
	}
	provas, err := s.Repo.ProvasDoAno(ctx, i.Rascunho.Banca, i.Rascunho.Ano, i.ProvaID)
	if err != nil {
		return prova.Publicacao{}, false, err
	}
	for _, p := range provas {
		if prova.MesmaProva(i.Rascunho, p.Conteudo) {
			return p, true, nil
		}
	}

	return prova.Publicacao{}, false, nil
}

func (s *ProvaService) reaproveitar(ctx context.Context, i *prova.Importacao) (int, error) {
	r := &i.Rascunho
	if r.Orgao == "" || r.Ano == 0 {
		return 0, nil
	}
	candidatas, err := s.Repo.ProvasDoAno(ctx, r.Banca, r.Ano, i.ProvaID)
	if err != nil {
		return 0, err
	}
	var irmas []prova.Publicacao
	for _, c := range candidatas {
		if !prova.MesmoOrgao(r.Orgao, c.Conteudo.Orgao) {
			continue
		}
		p, err := s.Repo.Publicacao(ctx, c.ID)
		if err != nil {
			return 0, err
		}
		irmas = append(irmas, p)
	}
	if len(irmas) == 0 {
		return 0, nil
	}

	tinha := map[string]bool{}
	for _, id := range r.Arquivos() {
		tinha[id] = true
	}
	r.Reaproveitar(irmas)
	for _, id := range r.Arquivos() {
		if !tinha[id] {
			if err := s.Repo.RegistrarArquivo(ctx, i.ID, id, "png"); err != nil {
				return 0, err
			}
			tinha[id] = true
		}
	}

	n := 0
	for _, q := range r.Questoes {
		if q.IgualA != "" {
			n++
		}
	}

	return n, nil
}

// classificar troca a seção do caderno pela matéria de cada questão — em
// "Conhecimentos Específicos" cabem redes, sistemas e segurança, e o filtro
// por matéria não serve para nada com as quarenta juntas.
//
// A matéria é sugestão: se o processador recusar, o rascunho segue com a seção
// e um aviso. Falha transitória volta à fila como qualquer etapa.
func (s *ProvaService) classificar(ctx context.Context, r *prova.Rascunho) (map[int]string, error) {
	if len(r.Questoes) == 0 {
		return nil, nil
	}
	materias, err := s.Processor.Classificar(ctx, r.ParaClassificar())
	if errors.Is(err, port.ErrDocumentoRecusado) {
		r.Alertas = append(r.Alertas,
			"Não foi possível sugerir as matérias; as questões ficaram com a seção do caderno.")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.AplicarMaterias(materias)

	return materias, nil
}

// SugerirMaterias pede a matéria de cada questão do rascunho salvo, e devolve
// só as que a consolidação aplicaria (seção genérica). Não grava: a sugestão
// vai para a tela, e o curador confere antes de salvar.
func (s *ProvaService) SugerirMaterias(ctx context.Context, usuario, id string) (map[int]string, error) {
	i, err := s.carregar(ctx, usuario, id)
	if err != nil {
		return nil, err
	}
	if i.Estado != prova.EstadoEmRevisao {
		return nil, prova.ErrConflito
	}

	materias, err := s.Processor.Classificar(ctx, i.Rascunho.ParaClassificar())
	if err != nil {
		return nil, err
	}

	return i.Rascunho.MateriasAplicaveis(materias), nil
}

// Rodar é o laço do worker. Sem curadores configurados não há quem importe, e
// ele nem começa.
func (s *ProvaService) Rodar(ctx context.Context, logger *slog.Logger) {
	if !s.TodosCuradores && len(s.Curadores) == 0 {
		return
	}

	const rascunhoParado = 30 * 24 * time.Hour
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	ultimaLimpeza := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.ProcessarUma(ctx, logger); err != nil {
				logger.ErrorContext(ctx, "processando prova", slog.Any("error", err))
			}
			if time.Since(ultimaLimpeza) < time.Hour {
				continue
			}
			ultimaLimpeza = time.Now()
			s.limpar(ctx, logger, time.Now().Add(-rascunhoParado))
		}
	}
}

// limpar expira rascunhos parados e apaga do volume o que ficou sem dono. O
// arquivo só sai do disco depois de a linha sair do banco: o contrário deixaria
// uma janela em que o banco aponta para um arquivo que já não existe.
func (s *ProvaService) limpar(ctx context.Context, logger *slog.Logger, antes time.Time) {
	if err := s.Repo.Expirar(ctx, antes); err != nil {
		logger.ErrorContext(ctx, "expirando rascunhos de provas", slog.Any("error", err))
	}
	orfaos, err := s.Repo.LimparReferencias(ctx, antes)
	if err != nil {
		logger.ErrorContext(ctx, "liberando arquivos de provas", slog.Any("error", err))
		return
	}
	for _, nome := range orfaos {
		if err := s.Arquivos.Remover(nome); err != nil {
			logger.ErrorContext(ctx, "removendo arquivo de prova", slog.String("arquivo", nome), slog.Any("error", err))
		}
	}
}
