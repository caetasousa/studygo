package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/mapa"

	"github.com/google/uuid"
)

// ExportacaoDeMapa é o mapa como sai do banco para fora do app: o texto do
// outline, as questões ativas, as imagens e o estudo da conta em volta dele —
// os vínculos com as matérias (e os tópicos) e as respostas. É o que o pacote
// traz de volta (ImportarPacote), inteiro.
type ExportacaoDeMapa struct {
	Slug      string
	Texto     string
	Questoes  []mapa.Questao
	Imagens   []mapa.Imagem
	Vinculos  []mapa.VinculoExportado
	Respostas []mapa.RespostaExportada
}

// Exportar devolve um mapa da conta pronto para sair do app.
func (s *MapaService) Exportar(ctx context.Context, usuarioID uuid.UUID, slug string) (ExportacaoDeMapa, error) {
	m, err := s.mapas.PorSlug(ctx, usuarioID, slug)
	if err != nil {
		return ExportacaoDeMapa{}, err
	}

	qs, err := s.mapas.Questoes(ctx, m.ID)
	if err != nil {
		return ExportacaoDeMapa{}, err
	}

	out := ExportacaoDeMapa{Slug: m.Slug, Texto: m.Texto()}
	for _, q := range qs {
		out.Questoes = append(out.Questoes, q.Questao)
	}

	nomes, err := s.mapas.NomesDasImagens(ctx, m.ID)
	if err != nil {
		return ExportacaoDeMapa{}, err
	}

	for _, nome := range nomes {
		img, err := s.mapas.Imagem(ctx, m.ID, nome)
		if err != nil {
			return ExportacaoDeMapa{}, err
		}

		out.Imagens = append(out.Imagens, img)
	}

	if out.Vinculos, err = s.mapas.VinculosDoMapa(ctx, m.ID); err != nil {
		return ExportacaoDeMapa{}, err
	}

	if out.Respostas, err = s.mapas.RespostasDoMapa(ctx, m.ID); err != nil {
		return ExportacaoDeMapa{}, err
	}

	return out, nil
}

// PacoteDeMapa é um mapa como volta de uma exportação: tudo o que ela levou.
type PacoteDeMapa struct {
	Texto     string
	Questoes  *mapa.ArquivoDeQuestoes
	Imagens   []ArquivoDeImagem
	Vinculos  []mapa.VinculoExportado
	Respostas []mapa.RespostaExportada
	// ComEstudo diz se o pacote trouxe o estudo da conta (vínculos e
	// respostas). Sem ele — um .zip feito à mão —, vale a sugestão de vínculo
	// da importação comum.
	ComEstudo bool
}

// PacoteImportado é o que voltou, e o que não pôde voltar (Avisos).
type PacoteImportado struct {
	Mapa       mapa.Resumo
	Novo       bool
	Questoes   int
	Imagens    int
	Respostas  int
	Vinculadas []MateriaDoMapa
	Avisos     []string
}

// ImportarPacote traz de volta um mapa exportado: o texto, as questões, as
// imagens, os vínculos e as respostas, num envio só. O texto e as questões são
// conferidos antes de gravar qualquer coisa.
//
// O vínculo volta ao concurso de origem, se a conta o tem; se não (outra conta,
// concurso recriado), ao concurso aberto na tela, na matéria de mesmo código
// ou nome. O que não acha lugar — a matéria, um tópico — vira aviso, nunca
// silêncio. As respostas voltam com a hora em que foram dadas, e a que já
// está gravada não se repete.
func (s *MapaService) ImportarPacote(
	ctx context.Context, usuarioID uuid.UUID, concursoAberto string, p PacoteDeMapa,
) (PacoteImportado, error) {
	m, err := mapa.Ler(p.Texto)
	if err != nil {
		return PacoteImportado{}, err
	}

	if p.Questoes != nil {
		if _, err := mapa.ValidarQuestoes(m, *p.Questoes); err != nil {
			return PacoteImportado{}, err
		}
	}

	sugestao := concursoAberto
	if p.ComEstudo {
		// Os vínculos vêm no pacote: a sugestão pelo texto não entra.
		sugestao = ""
	}

	imp, err := s.Importar(ctx, usuarioID, p.Texto, sugestao)
	if err != nil {
		return PacoteImportado{}, err
	}

	out := PacoteImportado{Mapa: imp.Mapa, Novo: imp.Novo, Vinculadas: imp.Vinculadas}

	if p.Questoes != nil {
		if _, err := s.ImportarQuestoes(ctx, usuarioID, m.Slug, *p.Questoes); err != nil {
			return PacoteImportado{}, err
		}

		out.Questoes = len(p.Questoes.Questoes)
	}

	if len(p.Imagens) > 0 {
		if out.Imagens, err = s.EnviarImagens(ctx, usuarioID, m.Slug, p.Imagens); err != nil {
			return PacoteImportado{}, err
		}
	}

	for _, v := range p.Vinculos {
		materia, aviso, err := s.restaurarVinculo(ctx, usuarioID, concursoAberto, imp.Mapa.ID, v)
		if err != nil {
			return PacoteImportado{}, err
		}

		if materia != nil {
			out.Vinculadas = append(out.Vinculadas, *materia)
		}

		out.Avisos = append(out.Avisos, aviso...)
	}

	n, aviso, err := s.restaurarRespostas(ctx, imp.Mapa.ID, p.Respostas)
	if err != nil {
		return PacoteImportado{}, err
	}

	out.Respostas = n
	out.Avisos = append(out.Avisos, aviso...)

	return out, nil
}

func (s *MapaService) restaurarVinculo(
	ctx context.Context, usuarioID uuid.UUID, concursoAberto string, mapaID uuid.UUID, v mapa.VinculoExportado,
) (*MateriaDoMapa, []string, error) {
	nome := v.Codigo + " — " + v.Disciplina

	c, err := s.concursoDoDono(ctx, usuarioID, v.ConcursoSlug)
	if err != nil && concursoAberto != "" {
		c, err = s.concursoDoDono(ctx, usuarioID, concursoAberto)
	}

	if errors.Is(err, concurso.ErrNaoEncontrado) {
		return nil, []string{fmt.Sprintf(
			"o vínculo com %s (%s) não voltou: abra um concurso com essa matéria e importe de novo, ou vincule na página do mapa",
			nome, v.ConcursoNome)}, nil
	}

	if err != nil {
		return nil, nil, err
	}

	i := slices.IndexFunc(c.Disciplinas, func(d concurso.Disciplina) bool { return strings.EqualFold(d.Codigo, v.Codigo) })
	if i < 0 {
		i = slices.IndexFunc(c.Disciplinas, func(d concurso.Disciplina) bool { return strings.EqualFold(d.Nome, v.Disciplina) })
	}

	if i < 0 {
		return nil, []string{fmt.Sprintf("o concurso %s não tem a matéria %s: o vínculo com ela não voltou", c.Nome, nome)}, nil
	}

	d := c.Disciplinas[i]

	var avisos []string

	var achados []string

	for _, t := range v.Temas {
		if achado, ok := mapa.TemaCorrespondente(t, d.Temas); ok {
			achados = append(achados, achado)
		} else {
			avisos = append(avisos, fmt.Sprintf("o tópico %q não existe em %s — %s: ficou de fora do vínculo", t, d.Codigo, d.Nome))
		}
	}

	// Sem nenhum dos tópicos, vincular seria dar o mapa à matéria inteira.
	if len(v.Temas) > 0 && len(achados) == 0 {
		return nil, append(avisos, fmt.Sprintf("o vínculo com %s — %s não voltou: nenhum dos tópicos existe lá", d.Codigo, d.Nome)), nil
	}

	// Na ordem da ementa e sem repetir: dois tópicos de fora podem cair no mesmo daqui.
	temas, err := mapa.EscolherTemas(achados, d.Temas)
	if err != nil {
		return nil, nil, err
	}

	if err := s.mapas.Vincular(ctx, d.ID, mapaID, temas); err != nil {
		return nil, nil, err
	}

	return &MateriaDoMapa{Codigo: d.Codigo, Nome: d.Nome}, avisos, nil
}

func (s *MapaService) restaurarRespostas(ctx context.Context, mapaID uuid.UUID, rs []mapa.RespostaExportada) (int, []string, error) {
	if len(rs) == 0 {
		return 0, nil, nil
	}

	qs, err := s.mapas.Questoes(ctx, mapaID)
	if err != nil {
		return 0, nil, err
	}

	porChave := make(map[string]mapa.QuestaoComResposta, len(qs))
	for _, q := range qs {
		porChave[q.Questao.Chave] = q
	}

	var (
		voltam []mapa.RespostaRestaurada
		fora   int
	)

	for _, r := range rs {
		q, ok := porChave[r.Chave]
		if !ok {
			fora++
			continue
		}

		// Corrigida de novo pelo gabarito de agora: se a questão mudou, o
		// acerto acompanha.
		acertou, err := q.Questao.Corrigir(r.Resposta)
		if err != nil {
			fora++
			continue
		}

		voltam = append(voltam, mapa.RespostaRestaurada{
			QuestaoID: q.ID,
			Resposta:  mapa.Resposta{Resposta: strings.ToUpper(strings.TrimSpace(r.Resposta)), Acertou: acertou, Em: r.Em},
		})
	}

	n, err := s.mapas.RestaurarRespostas(ctx, voltam)
	if err != nil {
		return 0, nil, err
	}

	var avisos []string
	if fora > 0 {
		avisos = append(avisos, fmt.Sprintf("%d respostas de questões que o pacote não tem ficaram de fora", fora))
	}

	return n, avisos, nil
}

// SlugsDaConta lista os mapas da conta, para a exportação de todos ir um a um.
func (s *MapaService) SlugsDaConta(ctx context.Context, usuarioID uuid.UUID) ([]string, error) {
	rs, err := s.mapas.Catalogo(ctx, usuarioID)
	if err != nil {
		return nil, err
	}

	slugs := make([]string, 0, len(rs))
	for _, r := range rs {
		slugs = append(slugs, r.Slug)
	}

	return slugs, nil
}
