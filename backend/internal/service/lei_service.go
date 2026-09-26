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

// Capturar pede ao processador a lei do link. Qualquer conta logada captura
// e publica (decisão de 25/09/2026, enquanto o app é de teste); a rota já
// exige sessão.
func (s *LeiService) Capturar(ctx context.Context, usuarioID uuid.UUID, link string) (string, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return "", erroDeValidacao("cole o link da lei na fonte oficial")
	}

	return s.capturador.IniciarCaptura(ctx, usuarioID.String(), link)
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

	out := LeituraDaLei{Lei: l, Texto: texto, Questoes: make([]QuestaoParaLeitor, 0, len(qs))}
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
				m.Vinculadas = append(m.Vinculadas, LeiNaMateria{r, lei.DescreverRecorte(ds, vinculos[d.ID][i].Recorte)})
			case r.Lei.CitadaEm(d.Temas):
				ds, err := estrutura(r.Lei.ID)
				if err != nil {
					return nil, err
				}
				recorte := lei.RecorteDoEdital(r.Lei, d.Temas, ds)
				m.Sugeridas = append(m.Sugeridas, LeiNaMateria{r, lei.DescreverRecorte(ds, recorte)})
			}
		}
		out = append(out, m)
	}

	return out, nil
}

// Vincular liga (ou desliga) a lei à matéria do concurso do estudante. Sem
// recorte informado, ele sai dos tópicos da matéria — o que o edital pede;
// informado, é o ajuste de quem estuda, e só vale com refs de divisões e
// artigos da lei.
func (s *LeiService) Vincular(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	disciplinaID uuid.UUID,
	leiSlug string,
	ligar bool,
	recorte *[]string,
) error {
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
