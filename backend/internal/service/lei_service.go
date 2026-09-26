package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/lei"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// LeiService captura e publica leis no catálogo, importa as questões, serve a
// leitura e guarda as respostas e os vínculos entre lei e matéria.
type LeiService struct {
	leis       port.LeiRepository
	concursos  port.ConcursoRepository
	capturador port.CapturadorDeLeis
}

func NewLeiService(leis port.LeiRepository, concursos port.ConcursoRepository, capturador port.CapturadorDeLeis) *LeiService {
	return &LeiService{leis: leis, concursos: concursos, capturador: capturador}
}

// PedidoDeCaptura é o que capturar: o link da fonte e, se houver, o recorte
// (raízes que a pesquisa achou, mais os artigos que a pessoa digitou). Com
// Slug, é a lei que já existe: o recorte novo se soma ao que ela já guarda.
type PedidoDeCaptura struct {
	Link    string
	Recorte []string
	Artigos string
	Slug    string
	// Inteira pede a lei toda, mesmo que a guardada seja só parte.
	Inteira bool
}

// Capturar pede ao processador a lei do link. Qualquer conta logada captura
// e publica (decisão de 25/09/2026, enquanto o app é de teste); a rota já
// exige sessão.
func (s *LeiService) Capturar(ctx context.Context, usuarioID uuid.UUID, pedido PedidoDeCaptura) (string, error) {
	link := strings.TrimSpace(pedido.Link)
	recorte := append(slices.Clone(pedido.Recorte), lei.ArtigosCitados(pedido.Artigos)...)

	if pedido.Slug != "" {
		atual, err := s.leis.PorSlug(ctx, pedido.Slug)
		if err != nil {
			return "", err
		}
		if link == "" {
			link = atual.Fonte
		}
		texto, err := s.leis.TextoAtivo(ctx, atual.ID)
		switch {
		case errors.Is(err, lei.ErrNaoEncontrada):
		case err != nil:
			return "", err
		case len(texto.Recorte) == 0:
			// A versão guardada é a lei inteira: ampliar é recapturá-la inteira.
			recorte = nil
		default:
			recorte = append(texto.Recorte, recorte...)
		}
	}
	if link == "" {
		return "", erroDeValidacao("cole o link da lei na fonte oficial")
	}

	if pedido.Inteira {
		recorte = nil
	}

	return s.capturador.IniciarCaptura(ctx, usuarioID.String(), link, limparLista(recorte))
}

// AssuntoDoTema é um pedaço do tópico e o que ele pede da lei, já descrito.
type AssuntoDoTema struct {
	Texto   string
	Refs    []string
	Trechos []lei.TrechoDoRecorte
}

// PesquisaDoTema é o que a tela mostra antes de importar: a fonte, o que cada
// assunto do tópico pede, o recorte somado e, se a lei já está no catálogo, o
// que ainda falta nela.
type PesquisaDoTema struct {
	lei.Pesquisa
	Nome     string
	Curto    string
	Assuntos []AssuntoDoTema
	Recorte  []string
	Trechos  []lei.TrechoDoRecorte
	// Existente é a lei do catálogo com a mesma fonte.
	Existente *lei.Lei
	// Acao diz o que importar significa aqui: AcaoImportar (lei nova),
	// AcaoVincular (a versão guardada já cobre o tópico) ou AcaoAmpliar (falta
	// parte do que o tópico pede).
	Acao string
}

const (
	AcaoImportar = "importar"
	AcaoVincular = "vincular"
	AcaoAmpliar  = "ampliar"
)

// PesquisarTema acha a lei que o tópico do edital cita e lê o que ele pede
// dela, antes de importar qualquer coisa.
func (s *LeiService) PesquisarTema(ctx context.Context, usuarioID uuid.UUID, tema, link string) (PesquisaDoTema, error) {
	tema = strings.TrimSpace(tema)
	if tema == "" {
		return PesquisaDoTema{}, erroDeValidacao("diga o tópico do edital a pesquisar")
	}
	pq, err := s.capturador.Pesquisar(ctx, usuarioID.String(), tema, strings.TrimSpace(link))
	if err != nil {
		return PesquisaDoTema{}, err
	}

	ds := pq.Estrutura
	assuntos, recorte := lei.LerTema(tema, ds)
	out := PesquisaDoTema{
		Pesquisa: pq, Nome: pq.Epigrafe, Curto: lei.CurtoDe(pq.Epigrafe),
		Recorte: recorte, Trechos: lei.DescreverRecorte(ds, recorte),
	}
	for _, a := range assuntos {
		out.Assuntos = append(out.Assuntos, AssuntoDoTema{
			Texto: a.Texto, Refs: a.Refs, Trechos: lei.DescreverRecorte(ds, lei.RaizesNaOrdem(ds, a.Refs)),
		})
	}

	out.Acao = AcaoImportar
	existente, err := s.leis.PorFonte(ctx, pq.Fonte)
	switch {
	case errors.Is(err, lei.ErrNaoEncontrada):
		return out, nil
	case err != nil:
		return PesquisaDoTema{}, err
	}
	out.Existente, out.Nome, out.Curto = &existente, existente.Nome, existente.Curto
	texto, err := s.leis.TextoAtivo(ctx, existente.ID)
	if err != nil && !errors.Is(err, lei.ErrNaoEncontrada) {
		return PesquisaDoTema{}, err
	}

	out.Acao = AcaoVincular
	if err != nil || len(texto.Recorte) == 0 {
		return out, nil // sem versão, ou a guardada é a lei inteira
	}
	if len(recorte) == 0 {
		out.Acao = AcaoAmpliar // o tópico pede a lei inteira; a guardada é parte

		return out, nil
	}
	for _, ref := range recorte {
		if !lei.NoRecorte(ds, texto.Recorte, ref) {
			out.Acao = AcaoAmpliar
		}
	}

	return out, nil
}

// ResumoDaExclusao é o que a exclusão da lei leva junto, para a tela avisar.
type ResumoDaExclusao struct {
	Curto     string
	Questoes  int
	Respostas int
}

func (s *LeiService) ResumirExclusao(ctx context.Context, slug string) (ResumoDaExclusao, error) {
	l, err := s.leis.PorSlug(ctx, slug)
	if err != nil {
		return ResumoDaExclusao{}, err
	}
	q, r, err := s.leis.ContarParaExcluir(ctx, l.ID)
	if err != nil {
		return ResumoDaExclusao{}, err
	}

	return ResumoDaExclusao{Curto: l.Curto, Questoes: q, Respostas: r}, nil
}

// Excluir apaga a lei do catálogo, com as versões, as questões e as respostas
// a elas (decisão de 26/09/2026: uma importação errada não pode ficar para
// sempre). A tela confirma antes, com o que vai junto.
func (s *LeiService) Excluir(ctx context.Context, slug string) error {
	l, err := s.leis.PorSlug(ctx, slug)
	if err != nil {
		return err
	}

	return s.leis.Excluir(ctx, l.ID)
}

// CapturaComEdital é a prévia com o que o concurso ativo pede desta lei.
type CapturaComEdital struct {
	lei.Captura
	// Epigrafe é o nome que a lei traz na primeira linha: ponto de partida
	// para o nome que a pessoa vai dar.
	Epigrafe string
	Edital   []SugestaoDoEdital
}

// SugestaoDoEdital é uma matéria cujo tópico cita a lei, e o recorte que ele
// pede. Recorte vazio é a lei inteira.
type SugestaoDoEdital struct {
	DisciplinaID uuid.UUID
	Materia      string
	Temas        []string
	Recorte      []string
	Trechos      []lei.TrechoDoRecorte
}

// Captura consulta a captura e, pronta, lê no concurso (se houver) o que o
// edital pede dela: é o momento de publicar só olhando o que cai.
func (s *LeiService) Captura(ctx context.Context, usuarioID uuid.UUID, id, concursoSlug string) (CapturaComEdital, error) {
	c, err := s.capturador.Captura(ctx, usuarioID.String(), id)
	if err != nil {
		return CapturaComEdital{}, err
	}
	out := CapturaComEdital{Captura: c}
	if c.Resultado == nil || len(c.Resultado.Dispositivos) == 0 {
		return out, nil
	}
	ds := c.Resultado.Dispositivos
	out.Epigrafe = lei.Epigrafe(ds)
	if concursoSlug == "" {
		return out, nil
	}

	concurso, err := s.concursoDoDono(ctx, usuarioID, concursoSlug)
	if err != nil {
		return CapturaComEdital{}, err
	}
	provisoria := lei.Provisoria(ds)
	for _, d := range concurso.Disciplinas {
		var temas []string
		for _, t := range d.Temas {
			if provisoria.CitadaEm([]string{t}) {
				temas = append(temas, t)
			}
		}
		if len(temas) == 0 {
			continue
		}
		recorte := lei.RecorteDoEdital(provisoria, temas, ds)
		out.Edital = append(out.Edital, SugestaoDoEdital{
			DisciplinaID: d.ID, Materia: d.Nome, Temas: temas,
			Recorte: recorte, Trechos: lei.DescreverRecorte(ds, recorte),
		})
	}

	return out, nil
}

// PedidoDePublicacao é o que a pessoa conferiu na prévia. Sem Slug, é uma lei
// nova; com Slug, a atualização do texto daquela lei.
type PedidoDePublicacao struct {
	Slug       string
	Nome       string
	Curto      string
	Reconhecer []string
	Aceitos    []string
}

// ResultadoDaPublicacao conta o que a publicação fez. Unidades desatualizadas
// são as que têm questões escritas para outra redação.
type ResultadoDaPublicacao struct {
	Slug                   string
	Curto                  string
	Versao                 string
	NovaVersao             bool
	UnidadesDesatualizadas int
}

// Publicar grava o texto da captura como a versão ativa da lei. O texto vem do
// processador, nunca de quem publica: o navegador só diz o nome e o que
// revisou. As questões não mudam; as unidades passam para a versão nova.
func (s *LeiService) Publicar(
	ctx context.Context,
	usuarioID uuid.UUID,
	capturaID string,
	pedido PedidoDePublicacao,
) (ResultadoDaPublicacao, error) {
	nome, curto := strings.TrimSpace(pedido.Nome), strings.TrimSpace(pedido.Curto)
	if nome == "" || curto == "" {
		return ResultadoDaPublicacao{}, erroDeValidacao("informe o nome da lei e o nome curto")
	}

	c, err := s.capturador.Captura(ctx, usuarioID.String(), capturaID)
	if err != nil {
		return ResultadoDaPublicacao{}, err
	}
	if err := c.ConferirPublicacao(pedido.Aceitos); err != nil {
		return ResultadoDaPublicacao{}, err
	}
	r := c.Resultado

	slug := pedido.Slug
	var unidades []lei.Unidade
	if slug == "" {
		slug = lei.SlugDe(curto)
		if slug == "" {
			return ResultadoDaPublicacao{}, erroDeValidacao("o nome curto precisa de letras ou números")
		}
		// Lei nova com o slug de outra: publicar sobrescreveria a outra (L14).
		switch _, err := s.leis.PorSlug(ctx, slug); {
		case err == nil:
			return ResultadoDaPublicacao{}, lei.ErrLeiJaExiste
		case !errors.Is(err, lei.ErrNaoEncontrada):
			return ResultadoDaPublicacao{}, err
		}
	} else {
		atual, err := s.leis.PorSlug(ctx, slug)
		if err != nil {
			return ResultadoDaPublicacao{}, err
		}
		texto, err := s.leis.TextoAtivo(ctx, atual.ID)
		if err != nil && !errors.Is(err, lei.ErrNaoEncontrada) {
			return ResultadoDaPublicacao{}, err
		}
		unidades = texto.Unidades
	}

	reconhecer := limparLista(pedido.Reconhecer)
	if len(reconhecer) == 0 {
		reconhecer = lei.ReconhecerPadrao(nome, r.Dispositivos)
	}

	p := lei.Pacote{
		Formato: lei.Formato,
		Lei: lei.Lei{
			Slug: slug, Nome: nome, Curto: curto, Fonte: r.Fonte, Reconhecer: reconhecer,
		},
		Versao:       r.Versao,
		Dispositivos: r.Dispositivos,
		Unidades:     unidades,
		Recorte:      r.Recorte,
	}
	if err := p.ValidarTexto(); err != nil {
		return ResultadoDaPublicacao{}, err
	}

	nova, err := s.leis.GravarTexto(ctx, p)
	if err != nil {
		return ResultadoDaPublicacao{}, err
	}

	res := ResultadoDaPublicacao{Slug: slug, Curto: curto, Versao: r.Versao, NovaVersao: nova}
	for _, u := range unidades {
		if u.Hash != lei.HashUnidade(r.Dispositivos, u.Dispositivos) {
			res.UnidadesDesatualizadas++
		}
	}

	return res, nil
}

func derefOuVazio(xs *[]string) []string {
	if xs == nil {
		return nil
	}

	return *xs
}

func limparLista(xs []string) []string {
	var out []string
	for _, x := range xs {
		if x = strings.TrimSpace(x); x != "" && !slices.Contains(out, x) {
			out = append(out, x)
		}
	}

	return out
}

// ResultadoDaImportacaoDeQuestoes conta o que a importação fez.
type ResultadoDaImportacaoDeQuestoes struct {
	Curto       string
	Novas       int
	Atualizadas int
	Desativadas int
	Mantidas    int
}

// ImportarQuestoes grava as unidades e as questões escritas para a versão
// ativa da lei. A unidade sem hash foi escrita agora, para este texto, e
// recebe o dele; a que traz hash de outra redação é recusada até alguém
// revisar as questões dela.
func (s *LeiService) ImportarQuestoes(
	ctx context.Context,
	slug string,
	unidades []lei.Unidade,
	questoes []lei.Questao,
) (ResultadoDaImportacaoDeQuestoes, error) {
	l, err := s.leis.PorSlug(ctx, slug)
	if err != nil {
		return ResultadoDaImportacaoDeQuestoes{}, err
	}
	texto, err := s.leis.TextoAtivo(ctx, l.ID)
	if err != nil {
		return ResultadoDaImportacaoDeQuestoes{}, err
	}

	for i := range unidades {
		if unidades[i].Hash == "" {
			unidades[i].Hash = lei.HashUnidade(texto.Dispositivos, unidades[i].Dispositivos)
		}
	}
	p := lei.Pacote{
		Formato: lei.Formato, Lei: l, Versao: texto.Versao,
		Dispositivos: texto.Dispositivos, Unidades: unidades, Questoes: questoes,
	}
	if err := p.Validar(); err != nil {
		return ResultadoDaImportacaoDeQuestoes{}, err
	}

	gravadas, err := s.leis.QuestoesGravadas(ctx, slug)
	if err != nil {
		return ResultadoDaImportacaoDeQuestoes{}, err
	}
	plano := lei.PlanejarImportacao(gravadas, questoes)
	if err := s.leis.GravarQuestoes(ctx, l.ID, texto.Versao, unidades, plano); err != nil {
		return ResultadoDaImportacaoDeQuestoes{}, err
	}

	return ResultadoDaImportacaoDeQuestoes{
		Curto:       l.Curto,
		Novas:       len(plano.Novas),
		Atualizadas: len(plano.Atualizadas),
		Desativadas: len(plano.Desativar),
		Mantidas:    plano.Mantidas,
	}, nil
}

func (s *LeiService) Catalogo(ctx context.Context) ([]lei.Resumo, error) {
	return s.leis.Catalogo(ctx)
}

// Correcao é o que o estudante vê depois de responder. Antes disso o
// gabarito não sai do servidor.
type Correcao struct {
	Escolhida  string
	Acertou    bool
	Gabarito   string
	Comentario string
	Trecho     string
	Em         time.Time
}

// QuestaoParaLeitor é a questão sem gabarito — a não ser que já respondida.
type QuestaoParaLeitor struct {
	ID           uuid.UUID
	Unidade      string
	Dispositivos []string
	Enunciado    string
	Alternativas []string
	Resposta     *Correcao
}

// LeituraDaLei é a lei aberta no leitor.
type LeituraDaLei struct {
	Lei      lei.Lei
	Texto    lei.Texto
	Questoes []QuestaoParaLeitor
	// Recorte é o que o concurso pede desta lei; nil quando nenhuma matéria
	// dele a cobra (ou a lei foi aberta fora de um concurso).
	Recorte *RecorteNoConcurso
	// Guardado descreve o que a versão guarda quando é só parte da lei.
	Guardado []lei.TrechoDoRecorte
}

// RecorteNoConcurso junta os recortes das matérias do concurso que cobram a
// lei. Refs vazias: alguma delas cobra a lei inteira.
type RecorteNoConcurso struct {
	Refs     []string
	Trechos  []lei.TrechoDoRecorte
	Materias []MateriaDoRecorte
}

type MateriaDoRecorte struct {
	DisciplinaID uuid.UUID
	Nome         string
}

func (s *LeiService) Ler(ctx context.Context, usuarioID uuid.UUID, slug, concursoSlug string) (LeituraDaLei, error) {
	l, err := s.leis.PorSlug(ctx, slug)
	if err != nil {
		return LeituraDaLei{}, err
	}
	texto, err := s.leis.TextoAtivo(ctx, l.ID)
	if err != nil {
		return LeituraDaLei{}, err
	}
	qs, err := s.leis.QuestoesAtivas(ctx, l.ID, usuarioID)
	if err != nil {
		return LeituraDaLei{}, err
	}

	out := LeituraDaLei{
		Lei: l, Texto: texto, Questoes: make([]QuestaoParaLeitor, 0, len(qs)),
		Guardado: lei.DescreverRecorte(texto.Dispositivos, texto.Recorte),
	}
	for _, q := range qs {
		p := QuestaoParaLeitor{
			ID:           q.ID,
			Unidade:      q.Questao.Unidade,
			Dispositivos: q.Questao.Dispositivos,
			Enunciado:    q.Questao.Enunciado,
			Alternativas: q.Questao.Alternativas,
		}
		if q.Ultima != nil {
			c := correcaoDe(q.Questao, *q.Ultima)
			p.Resposta = &c
		}
		out.Questoes = append(out.Questoes, p)
	}

	if concursoSlug != "" {
		if out.Recorte, err = s.recorteNoConcurso(ctx, usuarioID, concursoSlug, l.ID, texto.Dispositivos); err != nil {
			return LeituraDaLei{}, err
		}
	}

	return out, nil
}

func (s *LeiService) recorteNoConcurso(
	ctx context.Context,
	usuarioID uuid.UUID,
	concursoSlug string,
	leiID uuid.UUID,
	ds []lei.Dispositivo,
) (*RecorteNoConcurso, error) {
	c, err := s.concursoDoDono(ctx, usuarioID, concursoSlug)
	if err != nil {
		return nil, err
	}
	vinculos, err := s.leis.Vinculos(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	var (
		r       RecorteNoConcurso
		inteira bool
	)
	for _, d := range c.Disciplinas {
		for _, v := range vinculos[d.ID] {
			if v.LeiID != leiID {
				continue
			}
			r.Materias = append(r.Materias, MateriaDoRecorte{DisciplinaID: d.ID, Nome: d.Nome})
			inteira = inteira || len(v.Recorte) == 0
			r.Refs = append(r.Refs, v.Recorte...)
		}
	}
	if len(r.Materias) == 0 {
		return nil, nil //nolint:nilnil // sem matéria que cobre a lei, não há recorte: é a lei inteira
	}
	if inteira {
		r.Refs = nil
	}
	r.Refs = lei.RaizesNaOrdem(ds, r.Refs)
	r.Trechos = lei.DescreverRecorte(ds, r.Refs)

	return &r, nil
}

func correcaoDe(q lei.Questao, r lei.Resposta) Correcao {
	return Correcao{
		Escolhida:  r.Alternativa,
		Acertou:    r.Acertou,
		Gabarito:   q.Gabarito,
		Comentario: q.Comentario,
		Trecho:     q.Trecho,
		Em:         r.Em,
	}
}

func (s *LeiService) Responder(ctx context.Context, usuarioID, questaoID uuid.UUID, alternativa string) (Correcao, error) {
	q, err := s.leis.Questao(ctx, questaoID)
	if err != nil {
		return Correcao{}, err
	}
	if !q.Ativa {
		return Correcao{}, lei.ErrQuestaoNaoEncontrada
	}

	acertou, err := q.Questao.Corrigir(alternativa)
	if err != nil {
		return Correcao{}, err
	}

	r, err := s.leis.Responder(ctx, lei.Resposta{
		UsuarioID: usuarioID, QuestaoID: q.ID, Alternativa: alternativa, Acertou: acertou,
	})
	if err != nil {
		return Correcao{}, err
	}

	return correcaoDe(q.Questao, r), nil
}

// LeiNaMateria é uma lei do catálogo vista de uma matéria: o recorte que ela
// cobra (vinculada) ou que o tópico pede (sugerida).
type LeiNaMateria struct {
	lei.Resumo
	Recorte []lei.TrechoDoRecorte
}

// LeisDaMateria são as leis de uma disciplina: as vinculadas e as que um
// tópico dela cita e ainda não foram vinculadas.
type LeisDaMateria struct {
	DisciplinaID uuid.UUID
	Codigo       string
	Nome         string
	Vinculadas   []LeiNaMateria
	Sugeridas    []LeiNaMateria
	// Temas são os tópicos da matéria e as leis vinculadas que cobrem cada um:
	// é por eles que a importação começa.
	Temas []TemaDaMateria
}

// TemaDaMateria é um tópico do edital e os slugs das leis vinculadas que
// cobrem o que ele pede — não basta citar a lei: "Lei nº X, art. 3º" não está
// coberto por um vínculo que só tem o capítulo I.
type TemaDaMateria struct {
	Texto string
	// Leis são as vinculadas que cobrem o que o tópico pede.
	Leis []string
	// Sugeridas são as do catálogo que o tópico cita e a matéria ainda não
	// vinculou: a tela as oferece no próprio tópico, em vez de importar de novo.
	Sugeridas []string
}

func (s *LeiService) DoConcurso(ctx context.Context, usuarioID uuid.UUID, slug string) ([]LeisDaMateria, error) {
	c, err := s.concursoDoDono(ctx, usuarioID, slug)
	if err != nil {
		return nil, err
	}
	catalogo, err := s.leis.Catalogo(ctx)
	if err != nil {
		return nil, err
	}
	vinculos, err := s.leis.Vinculos(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	estruturas := map[uuid.UUID][]lei.Dispositivo{}
	estrutura := func(id uuid.UUID) ([]lei.Dispositivo, error) {
		if ds, ok := estruturas[id]; ok {
			return ds, nil
		}
		ds, err := s.leis.Estrutura(ctx, id)
		estruturas[id] = ds

		return ds, err
	}

	out := make([]LeisDaMateria, 0, len(c.Disciplinas))
	for _, d := range c.Disciplinas {
		m := LeisDaMateria{DisciplinaID: d.ID, Codigo: d.Codigo, Nome: d.Nome}
		for _, r := range catalogo {
			i := slices.IndexFunc(vinculos[d.ID], func(v lei.Vinculo) bool { return v.LeiID == r.Lei.ID })
			switch {
			case i >= 0:
				ds, err := estrutura(r.Lei.ID)
				if err != nil {
					return nil, err
				}
				m.Vinculadas = append(m.Vinculadas, LeiNaMateria{r, descreverVinculo(r, ds, vinculos[d.ID][i].Recorte)})
			case r.Lei.CitadaEm(d.Temas):
				ds, err := estrutura(r.Lei.ID)
				if err != nil {
					return nil, err
				}
				recorte := lei.RecorteDoEdital(r.Lei, d.Temas, ds)
				m.Sugeridas = append(m.Sugeridas, LeiNaMateria{r, descreverVinculo(r, ds, recorte)})
			}
		}
		for _, t := range d.Temas {
			tema := TemaDaMateria{Texto: t}
			for _, v := range vinculos[d.ID] {
				i := slices.IndexFunc(m.Vinculadas, func(x LeiNaMateria) bool { return x.Lei.ID == v.LeiID })
				if i < 0 || !m.Vinculadas[i].Lei.CitadaEm([]string{t}) {
					continue
				}
				ds, err := estrutura(v.LeiID)
				if err != nil {
					return nil, err
				}
				if cobre(ds, v.Recorte, lei.RecorteDoEdital(m.Vinculadas[i].Lei, []string{t}, ds)) {
					tema.Leis = append(tema.Leis, m.Vinculadas[i].Lei.Slug)
				}
			}
			for _, sug := range m.Sugeridas {
				if sug.Lei.CitadaEm([]string{t}) {
					tema.Sugeridas = append(tema.Sugeridas, sug.Lei.Slug)
				}
			}
			m.Temas = append(m.Temas, tema)
		}
		out = append(out, m)
	}

	return out, nil
}

// descreverVinculo descreve o que a matéria estuda da lei. Recorte vazio é
// tudo o que a versão ativa guarda: a lei inteira, ou só a parte importada
// (L29) — que não pode aparecer como a lei inteira.
func descreverVinculo(r lei.Resumo, ds []lei.Dispositivo, recorte []string) []lei.TrechoDoRecorte {
	if len(recorte) == 0 {
		recorte = r.Guardado
	}

	return lei.DescreverRecorte(ds, recorte)
}

// cobre diz se o recorte do vínculo tem o que o tópico pede. Pedido vazio é a
// lei inteira (ou artigos que a versão guardada nem tem): só o vínculo com a
// lei inteira cobre.
func cobre(ds []lei.Dispositivo, vinculo, pedido []string) bool {
	if len(vinculo) == 0 {
		return true
	}
	if len(pedido) == 0 {
		return false
	}
	for _, ref := range pedido {
		if !lei.NoRecorte(ds, vinculo, ref) {
			return false
		}
	}

	return true
}

// Vincular liga (ou desliga) a lei à matéria do concurso do estudante. Sem
// recorte informado, ele sai dos tópicos da matéria — o que o edital pede;
// informado, é o ajuste de quem estuda, e só vale com refs de divisões e
// artigos da lei. Os artigos digitados ("74, 75") se somam a ele; com somar,
// tudo se soma ao recorte que a matéria já tinha desta lei.
func (s *LeiService) Vincular(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	disciplinaID uuid.UUID,
	leiSlug string,
	ligar bool,
	recorte *[]string,
	artigos string,
	somar bool,
) error {
	if extra := lei.ArtigosCitados(artigos); len(extra) > 0 {
		todos := append(slices.Clone(derefOuVazio(recorte)), extra...)
		recorte = &todos
	}

	c, err := s.concursoDoDono(ctx, usuarioID, slug)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(c.Disciplinas, func(d concurso.Disciplina) bool { return d.ID == disciplinaID })
	if i < 0 {
		return concurso.ErrNaoEncontrado
	}
	l, err := s.leis.PorSlug(ctx, leiSlug)
	if err != nil {
		return err
	}

	if !ligar {
		return s.leis.Desvincular(ctx, disciplinaID, l.ID)
	}

	ds, err := s.leis.Estrutura(ctx, l.ID)
	if err != nil {
		return err
	}
	var refs []string
	if recorte == nil {
		refs = lei.RecorteDoEdital(l, c.Disciplinas[i].Temas, ds)
	} else {
		existe := map[string]bool{}
		for _, d := range ds {
			existe[d.Ref] = true
		}
		for _, ref := range *recorte {
			if !existe[ref] {
				return erroDeValidacao("o recorte cita " + ref + ", que não é divisão nem artigo desta lei")
			}
		}
		refs = lei.RaizesNaOrdem(ds, *recorte)
	}
	// Pedir a lei inteira (refs vazias) já cobre o que a matéria tinha.
	if somar && len(refs) > 0 {
		vinculos, err := s.leis.Vinculos(ctx, c.ID)
		if err != nil {
			return err
		}
		for _, v := range vinculos[disciplinaID] {
			switch {
			case v.LeiID != l.ID:
			case len(v.Recorte) == 0:
				refs = nil // a matéria já cobra a lei inteira
			default:
				refs = lei.RaizesNaOrdem(ds, append(v.Recorte, refs...))
			}
		}
	}

	return s.leis.Vincular(ctx, disciplinaID, l.ID, refs)
}

func (s *LeiService) concursoDoDono(ctx context.Context, usuarioID uuid.UUID, slug string) (concurso.Concurso, error) {
	c, err := s.concursos.PorSlug(ctx, slug)
	if err != nil {
		return concurso.Concurso{}, err
	}
	if c.DonoID != usuarioID {
		return concurso.Concurso{}, concurso.ErrNaoEncontrado
	}

	return c, nil
}
