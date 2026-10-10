package service

import (
	"context"
	"fmt"
	"slices"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/mapa"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// PedidosDeMapaService é a fila de PDFs que viram mapa. A tela guarda o PDF;
// o serviço o entrega na hora ao processador (o edital-processor, interno),
// que faz o mapa em segundo plano com o Claude e devolve o resultado pela
// rota interna. É aqui que o resultado vira mapa da conta, pela mesma
// importação da tela — o processador nunca toca no banco.
type PedidosDeMapaService struct {
	mapas       port.MapaRepository
	concursos   port.ConcursoRepository
	importacao  *MapaService
	processador port.ProcessadorDeMapas
	tokens      *TokenDoClaudeService
}

func NewPedidosDeMapaService(
	mapas port.MapaRepository,
	concursos port.ConcursoRepository,
	importacao *MapaService,
	processador port.ProcessadorDeMapas,
	tokens *TokenDoClaudeService,
) *PedidosDeMapaService {
	return &PedidosDeMapaService{
		mapas: mapas, concursos: concursos, importacao: importacao, processador: processador, tokens: tokens,
	}
}

// ResultadoDoProcessador é o que o processador devolve de um pedido.
type ResultadoDoProcessador struct {
	Texto     string
	Questoes  *mapa.ArquivoDeQuestoes
	Imagens   []ArquivoDeImagem
	Temas     []string
	Relatorio string
}

// Pedir guarda o PDF na fila da conta e o entrega ao processador. A matéria é
// opcional; se vier, tem de ser de um concurso da conta. Se o processador não
// aceitar (fora do ar), o pedido fica na fila, e "Pôr na fila de novo" tenta
// outra vez.
func (s *PedidosDeMapaService) Pedir(
	ctx context.Context, usuarioID uuid.UUID, concursoSlug string, disciplinaID uuid.NullUUID, arquivo string, pdf []byte,
) (mapa.Pedido, error) {
	nome, err := mapa.ConferirPDF(arquivo, pdf)
	if err != nil {
		return mapa.Pedido{}, err
	}

	if disciplinaID.Valid {
		c, err := s.concursoDoDono(ctx, usuarioID, concursoSlug)
		if err != nil {
			return mapa.Pedido{}, err
		}

		if !slices.ContainsFunc(c.Disciplinas, func(d concurso.Disciplina) bool { return d.ID == disciplinaID.UUID }) {
			return mapa.Pedido{}, concurso.ErrNaoEncontrado
		}
	}

	p, err := s.mapas.CriarPedido(ctx, usuarioID, disciplinaID, nome, pdf)
	if err != nil {
		return mapa.Pedido{}, err
	}

	return s.despachar(ctx, usuarioID, p, pdf)
}

// Pedidos lista a fila da conta.
func (s *PedidosDeMapaService) Pedidos(ctx context.Context, usuarioID uuid.UUID) ([]mapa.Pedido, error) {
	return s.mapas.Pedidos(ctx, usuarioID)
}

// Reenfileirar entrega de novo ao processador o pedido que falhou, que travou
// processando ou que ficou na fila porque o processador estava fora do ar. O
// pronto não volta, e o que perdeu o PDF na faxina pede reenvio.
func (s *PedidosDeMapaService) Reenfileirar(ctx context.Context, usuarioID, id uuid.UUID) error {
	p, err := s.mapas.Pedido(ctx, usuarioID, id)
	if err != nil {
		return err
	}

	if p.Situacao == mapa.Pronto {
		return mapa.ErrPedidoForaDeHora
	}

	if !p.TemPDF {
		return mapa.ErrPedidoSemPDF
	}

	if err := s.mapas.MudarPedido(ctx, usuarioID, id,
		[]mapa.Situacao{mapa.Falhou, mapa.Processando, mapa.NaFila}, mapa.NaFila, "", "", false); err != nil {
		return err
	}

	pdf, err := s.mapas.PDFDoPedido(ctx, usuarioID, id)
	if err != nil {
		return err
	}

	p.Situacao = mapa.NaFila
	_, err = s.despachar(ctx, usuarioID, p, pdf)

	return err
}

// Excluir tira o pedido da fila. O mapa que ele já gerou fica: é da conta e
// se exclui pela página dele. Um processamento em andamento termina sozinho e
// é recusado ao voltar.
func (s *PedidosDeMapaService) Excluir(ctx context.Context, usuarioID, id uuid.UUID) error {
	return s.mapas.ExcluirPedido(ctx, usuarioID, id)
}

// Redespachar entrega de novo ao processador os pedidos que ele tinha em mãos:
// é o que ele pede ao subir, já que um reinício perde o trabalho em memória.
func (s *PedidosDeMapaService) Redespachar(ctx context.Context) (int, error) {
	pendentes, err := s.mapas.PedidosProcessando(ctx)
	if err != nil {
		return 0, err
	}

	n := 0

	for _, pc := range pendentes {
		if !pc.Pedido.TemPDF {
			continue
		}

		pdf, err := s.mapas.PDFDoPedido(ctx, pc.UsuarioID, pc.Pedido.ID)
		if err != nil {
			return n, err
		}

		if _, err := s.despachar(ctx, pc.UsuarioID, pc.Pedido, pdf); err != nil {
			return n, err
		}

		n++
	}

	return n, nil
}

// despachar entrega o pedido ao processador e o deixa "processando"; se o
// processador não aceita, o pedido fica na fila com o motivo.
func (s *PedidosDeMapaService) despachar(ctx context.Context, usuarioID uuid.UUID, p mapa.Pedido, pdf []byte) (mapa.Pedido, error) {
	t := port.TrabalhoDeMapa{Pedido: p.ID, Dono: usuarioID.String(), Arquivo: p.Arquivo, PDF: pdf}

	if p.DisciplinaID.Valid {
		if c, err := s.concursoDoDono(ctx, usuarioID, p.ConcursoSlug); err == nil {
			for _, d := range c.Disciplinas {
				if d.ID == p.DisciplinaID.UUID {
					t.Materia, t.Temas = d.Codigo+" — "+d.Nome, d.Temas
				}
			}
		}
	}

	catalogo, err := s.mapas.Catalogo(ctx, usuarioID)
	if err != nil {
		return mapa.Pedido{}, err
	}

	for _, m := range catalogo {
		if m.Slug != p.Mapa {
			t.SlugsExistentes = append(t.SlugsExistentes, m.Slug)
		}
	}

	if token, ok, err := s.tokens.Token(ctx, usuarioID); err == nil && ok {
		t.TokenClaude = token
	}

	de := []mapa.Situacao{p.Situacao}

	if err := s.processador.Processar(ctx, t); err != nil {
		// Fica na fila, com o motivo à vista; a tela oferece tentar de novo.
		motivo := fmt.Sprintf("O processador não recebeu o PDF (%v). Tente \"Pôr na fila de novo\" daqui a pouco.", err)
		if err := s.mapas.MudarPedido(ctx, usuarioID, p.ID, de, mapa.NaFila, "", motivo, false); err != nil {
			return mapa.Pedido{}, err
		}

		return s.mapas.Pedido(ctx, usuarioID, p.ID)
	}

	if err := s.mapas.MudarPedido(ctx, usuarioID, p.ID, de, mapa.Processando, "", "", false); err != nil {
		return mapa.Pedido{}, err
	}

	return s.mapas.Pedido(ctx, usuarioID, p.ID)
}

// Concluir importa o resultado do processador na conta do pedido — o mapa, as
// questões, as imagens e o vínculo com os tópicos — e marca o pedido pronto,
// descartando o PDF. Um resultado recusado pela importação volta ao processador
// com o motivo, para o Claude corrigir; o pedido continua processando.
func (s *PedidosDeMapaService) Concluir(ctx context.Context, id uuid.UUID, r ResultadoDoProcessador) (string, error) {
	p, usuarioID, err := s.mapas.PedidoPorID(ctx, id)
	if err != nil {
		return "", err
	}

	if p.Situacao != mapa.Processando {
		return "", mapa.ErrPedidoForaDeHora
	}

	relatorio, err := mapa.ConferirRelatorio(r.Relatorio)
	if err != nil {
		return "", err
	}

	m, err := mapa.Ler(r.Texto)
	if err != nil {
		return "", err
	}

	// Importar o slug de outro mapa da conta o substituiria em silêncio.
	if m.Slug != p.Mapa {
		if _, err := s.mapas.ResumoPorSlug(ctx, usuarioID, m.Slug); err == nil {
			return "", mapa.ErrSlugDeOutroMapa{Slug: m.Slug}
		}
	}

	if _, err := s.importacao.Importar(ctx, usuarioID, r.Texto, ""); err != nil {
		return "", err
	}

	if err := s.mapas.GravarMapaDoPedido(ctx, id, m.Slug); err != nil {
		return "", err
	}

	if r.Questoes != nil {
		if _, err := s.importacao.ImportarQuestoes(ctx, usuarioID, m.Slug, *r.Questoes); err != nil {
			return "", err
		}
	}

	if len(r.Imagens) > 0 {
		if _, err := s.importacao.EnviarImagens(ctx, usuarioID, m.Slug, r.Imagens); err != nil {
			return "", err
		}
	}

	if p.DisciplinaID.Valid {
		if err := s.importacao.Vincular(ctx, usuarioID, p.ConcursoSlug, p.DisciplinaID.UUID, m.Slug, true, r.Temas); err != nil {
			return "", err
		}
	} else {
		relatorio += "\n\nO pedido não tinha matéria: vincule o mapa na página dele."
	}

	return m.Slug, s.mapas.MudarPedido(ctx, usuarioID, id, []mapa.Situacao{mapa.Processando}, mapa.Pronto, m.Slug, relatorio, true)
}

// Falhar marca que o processador desistiu, com o motivo. O PDF fica, para o
// pedido voltar à fila sem reenviar.
func (s *PedidosDeMapaService) Falhar(ctx context.Context, id uuid.UUID, relatorio string) error {
	p, usuarioID, err := s.mapas.PedidoPorID(ctx, id)
	if err != nil {
		return err
	}

	relatorio, err = mapa.ConferirRelatorio(relatorio)
	if err != nil {
		return err
	}

	return s.mapas.MudarPedido(ctx, usuarioID, p.ID, []mapa.Situacao{mapa.Processando}, mapa.Falhou, "", relatorio, false)
}

func (s *PedidosDeMapaService) concursoDoDono(ctx context.Context, usuarioID uuid.UUID, slug string) (concurso.Concurso, error) {
	c, err := s.concursos.PorSlug(ctx, slug)
	if err != nil {
		return concurso.Concurso{}, err
	}

	if c.DonoID != usuarioID {
		return concurso.Concurso{}, concurso.ErrNaoEncontrado
	}

	return c, nil
}
