package service

// Levar uma prova publicada de um ambiente a outro — de staging para produção,
// na estreia do catálogo — sem refazer a importação e a revisão: o pacote tem o
// conteúdo publicado e os arquivos de que ele precisa, e do outro lado entra
// direto no catálogo.

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"slices"

	"studygo/internal/domain/prova"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// ProvaParaLevar é a revisão em vigor de uma prova com o que ela precisa em
// outro ambiente. O PDF e as regiões não aparecem para o aluno, mas são a base
// de "Abrir revisão": sem eles, a prova levada não poderia ser corrigida.
type ProvaParaLevar struct {
	Conteudo                    prova.Rascunho
	Regioes                     []prova.Origem
	Documento, GabaritoArquivo  string
	NomeDocumento, NomeGabarito string
	// Arquivos leva cada arquivo, pelo nome "<id>.<extensão>", ao caminho no
	// volume: o PDF, o gabarito e as figuras.
	Arquivos map[string]string
}

// PacoteDeProva é a prova que chega de outro ambiente para ser publicada aqui.
type PacoteDeProva struct {
	Conteudo                    prova.Rascunho
	Regioes                     []prova.Origem
	NomeDocumento, NomeGabarito string
	Documento, Gabarito         []byte
	// Figuras, pelo id que os blocos do conteúdo citam.
	Figuras map[string][]byte
}

// ExportarProva junta a revisão em vigor e os arquivos dela. Falta de arquivo
// no volume é erro aqui, antes de o pacote começar a sair: no meio da resposta
// não dá mais para avisar.
func (s *ProvaService) ExportarProva(ctx context.Context, usuario, provaID string) (ProvaParaLevar, error) {
	if err := s.autorizar(usuario); err != nil {
		return ProvaParaLevar{}, err
	}
	p, err := s.Repo.Publicacao(ctx, provaID)
	if err != nil {
		return ProvaParaLevar{}, err
	}
	base, err := s.Repo.ImportacaoDaPublicacao(ctx, provaID)
	if err != nil {
		return ProvaParaLevar{}, err
	}

	out := ProvaParaLevar{
		Conteudo: p.Conteudo, Regioes: base.Regioes,
		Documento: base.Documento, GabaritoArquivo: base.GabaritoArquivo,
		NomeDocumento: base.NomeDocumento, NomeGabarito: base.NomeGabarito,
		Arquivos: map[string]string{},
	}
	levar := func(id, ext string) error {
		if !s.Arquivos.Existe(id, ext) {
			return fmt.Errorf("arquivo %s.%s da prova ausente do volume", id, ext)
		}
		caminho, err := s.Arquivos.Caminho(id, ext)
		out.Arquivos[id+"."+ext] = caminho

		return err
	}
	if err := levar(base.Documento, "pdf"); err != nil {
		return ProvaParaLevar{}, err
	}
	if base.GabaritoArquivo != "" {
		if err := levar(base.GabaritoArquivo, "pdf"); err != nil {
			return ProvaParaLevar{}, err
		}
	}
	for _, id := range p.Conteudo.Arquivos() {
		if err := levar(id, "png"); err != nil {
			return ProvaParaLevar{}, err
		}
	}

	return out, nil
}

// ExportarCatalogo junta as provas para levar: a pedida, ou todas as do
// catálogo, num pacote só.
func (s *ProvaService) ExportarCatalogo(ctx context.Context, usuario, provaID string) ([]ProvaParaLevar, error) {
	if err := s.autorizar(usuario); err != nil {
		return nil, err
	}
	ids := []string{provaID}
	if provaID == "" {
		ids = nil
		for offset := 0; ; offset += PorPaginaCatalogo {
			pagina, err := s.Repo.Catalogo(ctx, port.FiltroCatalogo{Offset: offset, Limite: PorPaginaCatalogo})
			if err != nil {
				return nil, err
			}
			for _, p := range pagina {
				ids = append(ids, p.ID)
			}
			if len(pagina) < PorPaginaCatalogo {
				break
			}
		}
	}
	if len(ids) == 0 {
		return nil, erroDeValidacao("não há prova publicada para exportar")
	}

	out := make([]ProvaParaLevar, 0, len(ids))
	for _, id := range ids {
		p, err := s.ExportarProva(ctx, usuario, id)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	return out, nil
}

// ImportarPacote publica aqui a prova que veio de outro ambiente. Ela já foi
// revisada e publicada lá, então entra direto no catálogo, sem passar de novo
// pela fila nem pela conferência daqui — mas não com defeito de integridade,
// nem repetida: a prova que já está no catálogo, ou o caderno que já foi
// importado aqui, são recusados, e importar o mesmo pacote duas vezes não
// duplica nada.
//
// As figuras guardam o id de lá, que é o que os blocos citam; o PDF ganha id
// novo, porque só a importação o cita.
func (s *ProvaService) ImportarPacote(ctx context.Context, usuario string, pct PacoteDeProva) (prova.Publicacao, error) {
	if err := s.autorizar(usuario); err != nil {
		return prova.Publicacao{}, err
	}
	if !parecePDF(pct.Documento) || (len(pct.Gabarito) > 0 && !parecePDF(pct.Gabarito)) {
		return prova.Publicacao{}, erroDeValidacao("o pacote precisa trazer o PDF do caderno")
	}
	r := pct.Conteudo
	r.AcertarApoios()
	r.LimparBlocos()
	if p := r.Pendencias(false); len(p) > 0 {
		return prova.Publicacao{}, erroDeValidacao(fmt.Sprintf(
			"a prova do pacote tem %d pendências; a primeira: %s", len(p), p[0],
		))
	}
	figuras := slices.Compact(slices.Sorted(slices.Values(r.Arquivos())))
	for _, id := range figuras {
		if _, err := uuid.Parse(id); err != nil || !parecePNG(pct.Figuras[id]) {
			return prova.Publicacao{}, erroDeValidacao(fmt.Sprintf("falta no pacote a figura %s", id))
		}
	}
	if p, repetida, err := s.noCatalogo(ctx, prova.Importacao{Rascunho: r}); err != nil {
		return prova.Publicacao{}, err
	} else if repetida {
		return prova.Publicacao{}, erroDeValidacao(fmt.Sprintf(
			"esta prova (%s) já está no catálogo daqui; nada foi importado", p.Conteudo.Rotulo(),
		))
	}

	h := sha256.Sum256(pct.Documento)
	i := prova.Importacao{
		ID: uuid.NewString(), Criador: usuario, Hash: hex.EncodeToString(h[:]),
		Documento: uuid.NewString(), NomeDocumento: prova.NomeDoArquivo(pct.NomeDocumento),
		Estado: prova.EstadoEmRevisao, Etapa: prova.TotalEtapas(len(pct.Regioes)),
		Regioes: pct.Regioes, Rascunho: r,
	}
	gravados := []string{i.Documento + ".pdf"}
	if err := s.Arquivos.Guardar(i.Documento, pct.Documento); err != nil {
		return prova.Publicacao{}, err
	}
	if len(pct.Gabarito) > 0 {
		i.GabaritoArquivo, i.NomeGabarito = uuid.NewString(), prova.NomeDoArquivo(pct.NomeGabarito)
		gravados = append(gravados, i.GabaritoArquivo+".pdf")
		if err := s.Arquivos.Guardar(i.GabaritoArquivo, pct.Gabarito); err != nil {
			s.descartar(gravados)
			return prova.Publicacao{}, err
		}
	}
	for _, id := range figuras {
		// A figura que já está aqui é a mesma (o id é o do conteúdo): outra
		// prova pode usá-la, e sobrescrever ou descartar não é deste pacote.
		if s.Arquivos.Existe(id, "png") {
			continue
		}
		gravados = append(gravados, id+".png")
		if err := s.Arquivos.GuardarComo(id, "png", pct.Figuras[id]); err != nil {
			s.descartar(gravados)
			return prova.Publicacao{}, err
		}
	}

	// Sem teto de pendentes: ele segura a fila do Gemini, e esta importação não
	// passa por ela — sai de em_revisao para o catálogo logo abaixo.
	criada, err := s.Repo.Criar(ctx, i, math.MaxInt32)
	if err != nil || criada.ID != i.ID {
		s.descartar(gravados)
	}
	if err != nil {
		return prova.Publicacao{}, err
	}
	if criada.ID != i.ID {
		return prova.Publicacao{}, erroDeValidacao(fmt.Sprintf(
			"este caderno já tem uma importação aqui (%s, %s); nada foi importado",
			cmp.Or(criada.NomeDocumento, criada.Rascunho.Rotulo()), criada.Estado,
		))
	}

	provaID, err := s.Repo.Publicar(ctx, criada, usuario)
	if err != nil {
		// A importação criada não fica pela metade na curadoria; os arquivos
		// dela, sem dono, saem na limpeza do worker.
		_ = s.Repo.Excluir(ctx, criada)
		return prova.Publicacao{}, err
	}

	return s.Repo.Publicacao(ctx, provaID)
}

func parecePNG(b []byte) bool {
	return bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n"))
}
