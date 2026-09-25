// Package lei é a lei seca interativa: o texto organizado em dispositivos, as
// questões feitas sobre ele e as regras de quem pode publicá-lo.
//
// A lei chega pronta, num pacote montado fora do app (a captura do
// edital-processor mais as questões escritas localmente); aqui ela é validada,
// versionada e lida. Nada neste pacote baixa, reescreve ou gera texto de lei.
package lei

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

// Formato é a versão do pacote que este código entende.
const Formato = "studygo.lei/1"

var (
	ErrNaoEncontrada        = errors.New("lei não encontrada")
	ErrQuestaoNaoEncontrada = errors.New("questão não encontrada")
	ErrAlternativaInvalida  = errors.New("alternativa inválida: escolha de A a E")
)

// ErrPacoteInvalido traz os problemas do pacote, para quem importa corrigir
// tudo de uma vez em vez de um por tentativa.
type ErrPacoteInvalido struct{ Problemas []string }

func (e ErrPacoteInvalido) Error() string {
	const maximo = 8
	ps := e.Problemas
	resto := ""
	if len(ps) > maximo {
		resto = fmt.Sprintf(" (e mais %d)", len(ps)-maximo)
		ps = ps[:maximo]
	}

	return "pacote inválido: " + strings.Join(ps, "; ") + resto
}

// Lei é a norma no catálogo. O Slug é a identidade estável entre versões.
type Lei struct {
	ID    uuid.UUID
	Slug  string
	Nome  string
	Curto string
	Fonte string
	// Reconhecer são os trechos que, achados num tópico do concurso, sugerem
	// esta lei para aquela matéria ("16.168").
	Reconhecer []string
}

// Dispositivo é um nó do texto: título, capítulo, artigo, inciso… A Ref é o
// endereço jurídico ("art71.inc2") e identifica o dispositivo entre versões —
// a exceção consciente à identidade por id, porque é pela ref que a lei é
// citada.
type Dispositivo struct {
	Ref        string
	Pai        string
	Tipo       string
	Rotulo     string
	Nome       string
	Texto      string
	Notas      []string
	Anteriores []string
	Revogado   bool
}

// Unidade é o trecho do recorte do edital sobre o qual as questões são feitas
// ("Arts. 70 a 75"). O Hash é do texto da unidade quando as questões foram
// escritas: se a lei mudar, elas ficam marcadas como desatualizadas.
type Unidade struct {
	Ref          string
	Titulo       string
	Dispositivos []string
	Hash         string
}

// Questao é uma questão no estilo da banca, amarrada aos dispositivos que a
// justificam e a um trecho literal deles.
type Questao struct {
	Chave        string
	Unidade      string
	Enunciado    string
	Alternativas []string
	Gabarito     string
	Comentario   string
	Dispositivos []string
	Trecho       string
}

// Pacote é o que se importa: uma versão da lei e as questões dela.
type Pacote struct {
	Formato      string
	Lei          Lei
	Versao       string
	Dispositivos []Dispositivo
	Unidades     []Unidade
	Questoes     []Questao
}

var (
	slugValido = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	tipos      = []string{
		"parte", "livro", "titulo", "capitulo", "secao", "subsecao",
		"artigo", "paragrafo", "inciso", "alinea", "item",
		"nome", "preambulo", "fecho", "solto",
	}
	letras = []string{"A", "B", "C", "D", "E"}
)

// HashUnidade resume o texto dos dispositivos da unidade e de todos os seus
// descendentes, na ordem da lei. É o mesmo cálculo do lado de quem escreve as
// questões (cmd/leis), para que as duas pontas concordem sobre "mudou".
func HashUnidade(ds []Dispositivo, raizes []string) string {
	h := sha256.New()
	pais := paisDe(ds)
	for _, d := range ds {
		if descende(pais, d.Ref, raizes) {
			fmt.Fprintf(h, "%s\t%s\n", d.Ref, d.Texto)
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

func paisDe(ds []Dispositivo) map[string]string {
	pais := make(map[string]string, len(ds))
	for _, d := range ds {
		pais[d.Ref] = d.Pai
	}

	return pais
}

func descende(pais map[string]string, ref string, raizes []string) bool {
	for passos := 0; ref != "" && passos < 64; passos++ {
		if slices.Contains(raizes, ref) {
			return true
		}
		ref = pais[ref]
	}

	return false
}

// Validar confere o pacote inteiro e devolve todos os problemas juntos.
func (p Pacote) Validar() error {
	var ps []string
	add := func(f string, a ...any) { ps = append(ps, fmt.Sprintf(f, a...)) }

	if p.Formato != Formato {
		add("formato %q não é %s", p.Formato, Formato)
	}
	if !slugValido.MatchString(p.Lei.Slug) {
		add("slug %q fora do padrão (minúsculas, números e hífen)", p.Lei.Slug)
	}
	if strings.TrimSpace(p.Lei.Nome) == "" || strings.TrimSpace(p.Lei.Curto) == "" {
		add("a lei precisa de nome e de nome curto")
	}
	if strings.TrimSpace(p.Versao) == "" {
		add("sem versão")
	}
	if len(p.Dispositivos) == 0 {
		add("nenhum dispositivo")
	}

	vistos := map[string]bool{}
	for i, d := range p.Dispositivos {
		switch {
		case d.Ref == "":
			add("dispositivo %d com ref vazia", i+1)
		case vistos[d.Ref]:
			add("ref repetida: %s", d.Ref)
		}
		if d.Pai != "" && !vistos[d.Pai] {
			add("%s: o pai %s não existe antes dele", d.Ref, d.Pai)
		}
		if !slices.Contains(tipos, d.Tipo) {
			add("%s: tipo desconhecido %q", d.Ref, d.Tipo)
		}
		vistos[d.Ref] = true
	}

	pais := paisDe(p.Dispositivos)
	unidades := map[string]Unidade{}
	for _, u := range p.Unidades {
		if _, ok := unidades[u.Ref]; ok {
			add("unidade repetida: %s", u.Ref)
		}
		unidades[u.Ref] = u
		for _, ref := range u.Dispositivos {
			if !vistos[ref] {
				add("unidade %s cita %s, que não existe", u.Ref, ref)
			}
		}
		if u.Hash != HashUnidade(p.Dispositivos, u.Dispositivos) {
			add("unidade %s desatualizada: o texto mudou desde que as questões foram escritas", u.Ref)
		}
	}

	textos := make(map[string]string, len(p.Dispositivos))
	for _, d := range p.Dispositivos {
		textos[d.Ref] = d.Texto
	}

	chaves := map[string]bool{}
	for _, q := range p.Questoes {
		nome := q.Chave
		if nome == "" {
			add("questão com chave vazia")
			nome = "(sem chave)"
		} else if chaves[q.Chave] {
			add("questão repetida: %s", q.Chave)
		}
		chaves[q.Chave] = true

		for _, problema := range q.problemas(unidades, vistos, pais, p.Dispositivos, textos) {
			add("%s: %s", nome, problema)
		}
	}

	if len(ps) > 0 {
		return ErrPacoteInvalido{Problemas: ps}
	}

	return nil
}

func (q Questao) problemas(
	unidades map[string]Unidade,
	existe map[string]bool,
	pais map[string]string,
	ds []Dispositivo,
	textos map[string]string,
) []string {
	var ps []string
	add := func(f string, a ...any) { ps = append(ps, fmt.Sprintf(f, a...)) }

	if strings.TrimSpace(q.Enunciado) == "" {
		add("enunciado vazio")
	}
	if strings.TrimSpace(q.Comentario) == "" {
		add("comentário vazio")
	}
	if len(q.Alternativas) != len(letras) {
		add("precisa de 5 alternativas, tem %d", len(q.Alternativas))
	}
	vistas := map[string]bool{}
	for i, a := range q.Alternativas {
		chave := normalizar(a)
		if chave == "" {
			add("alternativa %d vazia", i+1)
		} else if vistas[chave] {
			add("alternativa %d repetida", i+1)
		}
		vistas[chave] = true
	}
	if !slices.Contains(letras, q.Gabarito) {
		add("gabarito %q fora de A–E", q.Gabarito)
	}

	u, temUnidade := unidades[q.Unidade]
	if !temUnidade {
		add("unidade %q não existe", q.Unidade)
	}
	if len(q.Dispositivos) == 0 {
		add("nenhum dispositivo citado")
	}
	for _, ref := range q.Dispositivos {
		switch {
		case !existe[ref]:
			add("cita %s, que não existe", ref)
		case temUnidade && !descende(pais, ref, u.Dispositivos):
			add("cita %s, fora da unidade %s", ref, q.Unidade)
		}
	}

	trecho := normalizar(q.Trecho)
	if trecho == "" {
		add("trecho vazio")
	} else if !trechoNosCitados(trecho, q.Dispositivos, ds, pais, textos) {
		add("o trecho não está, literalmente, nos dispositivos citados")
	}

	return ps
}

func trechoNosCitados(
	trecho string,
	citados []string,
	ds []Dispositivo,
	pais map[string]string,
	textos map[string]string,
) bool {
	for _, d := range ds {
		if descende(pais, d.Ref, citados) && strings.Contains(normalizar(textos[d.Ref]), trecho) {
			return true
		}
	}

	return false
}

// normalizar compara textos sem se importar com espaços repetidos.
func normalizar(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Assinatura resume o conteúdo da questão: igual, a reimportação não mexe nela.
func (q Questao) Assinatura() string {
	h := sha256.New()
	partes := append([]string{q.Unidade, q.Enunciado, q.Gabarito, q.Comentario, q.Trecho}, q.Alternativas...)
	partes = append(partes, q.Dispositivos...)
	for _, p := range partes {
		fmt.Fprintf(h, "%s\x1f", p)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// Corrigir diz se a alternativa escolhida é o gabarito.
func (q Questao) Corrigir(alternativa string) (bool, error) {
	if !slices.Contains(letras, alternativa) {
		return false, ErrAlternativaInvalida
	}

	return alternativa == q.Gabarito, nil
}

// CitadaEm diz se algum tópico cita a lei: "16.168" num tópico sugere a Lei
// 16.168, mas "16.1680" não. Caixa e acento não contam.
func (l Lei) CitadaEm(temas []string) bool {
	for _, r := range l.Reconhecer {
		agulha := dobrar(r)
		if agulha == "" {
			continue
		}
		for _, t := range temas {
			if contemComoPalavra(dobrar(t), agulha) {
				return true
			}
		}
	}

	return false
}

func contemComoPalavra(palheiro, agulha string) bool {
	for inicio := 0; ; {
		i := strings.Index(palheiro[inicio:], agulha)
		if i < 0 {
			return false
		}
		i += inicio
		fim := i + len(agulha)
		antes := i == 0 || !alfanumerico(palheiro[i-1])
		depois := fim == len(palheiro) || !alfanumerico(palheiro[fim])
		if antes && depois {
			return true
		}
		inicio = i + 1
	}
}

func alfanumerico(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z'
}

// dobrar tira acento e caixa: "Constituição" e "CONSTITUICAO" se encontram.
func dobrar(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}

	return normalizar(b.String())
}
