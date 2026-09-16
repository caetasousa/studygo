package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"studygo/internal/domain/prova"
	"studygo/internal/port"
)

const figuraLevada = "11111111-2222-3333-4444-555555555555"

var pngMinimo = []byte("\x89PNG\r\n\x1a\nfigura")

// fakeLevar é o repositório visto por quem exporta e importa pacotes.
type fakeLevar struct {
	*fakeProvas
	publicada   prova.Publicacao
	base        prova.Importacao
	publicadas  []prova.Importacao
	errPublicar error
}

func (f *fakeLevar) Publicacao(context.Context, string) (prova.Publicacao, error) {
	return f.publicada, nil
}

func (f *fakeLevar) Catalogo(context.Context, port.FiltroCatalogo) ([]prova.Publicacao, error) {
	if f.publicada.ID == "" {
		return nil, nil
	}
	return []prova.Publicacao{f.publicada}, nil
}

func (f *fakeLevar) ImportacaoDaPublicacao(context.Context, string) (prova.Importacao, error) {
	return f.base, nil
}

func (f *fakeLevar) Publicar(_ context.Context, i prova.Importacao, _ string) (string, error) {
	f.publicadas = append(f.publicadas, i)
	return "prova-nova", f.errPublicar
}

// rascunhoLevado é uma prova publicada lá: íntegra, com uma figura.
func rascunhoLevado() prova.Rascunho {
	q1, q2 := questaoExtraida(1, true), questaoExtraida(2, true)
	q1.Blocos = append(q1.Blocos, prova.Bloco{Tipo: "imagem", Arquivo: figuraLevada})
	return prova.Rascunho{
		Banca: "FCC", Orgao: "TRT 18", Ano: 2023, Cargo: "L12", Caderno: "001", Total: 2,
		Questoes: []prova.Questao{q1, q2},
	}
}

func novoLevar(t *testing.T) (*ProvaService, *fakeLevar, *fakeVolume) {
	t.Helper()
	repo := &fakeLevar{fakeProvas: novoFakeProvas()}
	s, volume := novoProvaServiceDeTeste(repo.fakeProvas, extratorDeDuasRegioes())
	s.Repo = repo
	return s, repo, volume
}

func pacoteLevado() PacoteDeProva {
	return PacoteDeProva{
		Conteudo: rascunhoLevado(), Regioes: []prova.Origem{{Pagina: 1, Regiao: "0", Retangulo: []float64{0, 0, 595, 842}}},
		NomeDocumento: "trt18-prova.pdf", Documento: pdfMinimo,
		Figuras: map[string][]byte{figuraLevada: pngMinimo},
	}
}

func TestProvas_ExportarLevaOConteudoEOsArquivos(t *testing.T) {
	t.Parallel()

	s, repo, volume := novoLevar(t)
	repo.publicada = prova.Publicacao{ID: "p", Conteudo: rascunhoLevado()}
	repo.base = prova.Importacao{Documento: "doc", GabaritoArquivo: "gab", Regioes: []prova.Origem{{Pagina: 1, Regiao: "0"}}, NomeDocumento: "trt18.pdf"}
	volume.nomes["doc.pdf"] = true

	// Figura ausente do volume: recusa antes de começar a mandar o .zip.
	if _, err := s.ExportarProva(context.Background(), curador, "p"); err == nil || !strings.Contains(err.Error(), "ausente") {
		t.Fatalf("sem os arquivos: err = %v", err)
	}
	volume.nomes["gab.pdf"], volume.nomes[figuraLevada+".png"] = true, true

	p, err := s.ExportarProva(context.Background(), curador, "p")
	if err != nil {
		t.Fatal(err)
	}
	nomes := slices.Sorted(func(yield func(string) bool) {
		for k := range p.Arquivos {
			if !yield(k) {
				return
			}
		}
	})
	if !slices.Equal(nomes, []string{figuraLevada + ".png", "doc.pdf", "gab.pdf"}) || p.NomeDocumento != "trt18.pdf" || len(p.Regioes) != 1 {
		t.Fatalf("exportado = %v, %+v", nomes, p)
	}
}

// "Exportar todas" leva o catálogo inteiro num pacote só; sem prova, recusa.
func TestProvas_ExportarCatalogo(t *testing.T) {
	t.Parallel()

	s, repo, volume := novoLevar(t)
	var v ErrValidacao
	if _, err := s.ExportarCatalogo(context.Background(), curador, ""); !errors.As(err, &v) {
		t.Fatalf("catálogo vazio: err = %v", err)
	}
	repo.publicada = prova.Publicacao{ID: "p", Conteudo: rascunhoLevado()}
	repo.base = prova.Importacao{Documento: "doc"}
	volume.nomes["doc.pdf"], volume.nomes[figuraLevada+".png"] = true, true

	provas, err := s.ExportarCatalogo(context.Background(), curador, "")
	if err != nil || len(provas) != 1 || provas[0].Documento != "doc" {
		t.Fatalf("ExportarCatalogo = %+v, %v", provas, err)
	}
}

// Veio de staging, já revisada: entra direto no catálogo daqui.
func TestProvas_ImportarPacotePublica(t *testing.T) {
	t.Parallel()

	s, repo, volume := novoLevar(t)
	repo.publicada = prova.Publicacao{ID: "prova-nova"}

	p, err := s.ImportarPacote(context.Background(), curador, pacoteLevado())
	if err != nil || p.ID != "prova-nova" {
		t.Fatalf("ImportarPacote: %+v, %v", p, err)
	}
	if len(repo.publicadas) != 1 {
		t.Fatalf("publicadas = %d", len(repo.publicadas))
	}
	i := repo.publicadas[0]
	if i.Estado != prova.EstadoEmRevisao || i.Hash == "" || i.Criador != curador || len(i.Regioes) != 1 || i.NomeDocumento != "trt18-prova.pdf" {
		t.Fatalf("importação criada = %+v", i)
	}
	if !volume.nomes[i.Documento+".pdf"] || !volume.nomes[figuraLevada+".png"] {
		t.Fatalf("volume = %v; quer o PDF com id novo e a figura com o id do conteúdo", volume.nomes)
	}
}

func TestProvas_ImportarPacoteRecusa(t *testing.T) {
	t.Parallel()

	casos := map[string]struct {
		mudar func(*PacoteDeProva, *fakeLevar)
		quer  string
	}{
		"sem PDF":              {func(p *PacoteDeProva, _ *fakeLevar) { p.Documento = []byte("não é pdf") }, "PDF do caderno"},
		"com pendência":        {func(p *PacoteDeProva, _ *fakeLevar) { p.Conteudo.Total = 3 }, "pendências"},
		"sem a figura":         {func(p *PacoteDeProva, _ *fakeLevar) { delete(p.Figuras, figuraLevada) }, "falta no pacote a figura"},
		"figura que não é png": {func(p *PacoteDeProva, _ *fakeLevar) { p.Figuras[figuraLevada] = []byte("gif") }, "falta no pacote a figura"},
		"já no catálogo": {func(_ *PacoteDeProva, r *fakeLevar) {
			r.irmas = []prova.Publicacao{{ID: "ja", Conteudo: prova.Rascunho{Banca: "FCC", Orgao: "TRT-18", Ano: 2023, Cargo: "L12"}}}
		}, "já está no catálogo"},
		"caderno já importado": {func(_ *PacoteDeProva, r *fakeLevar) {
			r.existente = &prova.Importacao{ID: "outra", Estado: prova.EstadoCancelada, NomeDocumento: "trt18-prova.pdf"}
		}, "já tem uma importação aqui"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			t.Parallel()

			s, repo, volume := novoLevar(t)
			pct := pacoteLevado()
			c.mudar(&pct, repo)

			_, err := s.ImportarPacote(context.Background(), curador, pct)
			var v ErrValidacao
			if !errors.As(err, &v) || !strings.Contains(v.Msg, c.quer) {
				t.Fatalf("err = %v; quer recusa com %q", err, c.quer)
			}
			if len(repo.publicadas) != 0 || len(volume.nomes) != 0 {
				t.Fatalf("recusado, mas publicou %d e deixou no volume %v", len(repo.publicadas), volume.nomes)
			}
		})
	}
}

// Publicar falhou depois de criar: a importação não fica pela metade na curadoria.
func TestProvas_ImportarPacoteQueFalhaAoPublicarNaoFicaNaCuradoria(t *testing.T) {
	t.Parallel()

	s, repo, _ := novoLevar(t)
	repo.errPublicar = errors.New("banco fora")

	if _, err := s.ImportarPacote(context.Background(), curador, pacoteLevado()); err == nil {
		t.Fatal("quer o erro da publicação")
	}
	if len(repo.importacoes) != 0 {
		t.Fatalf("sobrou importação: %+v", repo.importacoes)
	}
}
