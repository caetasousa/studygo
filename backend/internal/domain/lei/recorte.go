package lei

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// O recorte do edital: a parte da lei que a matéria cobra. Guardado como as
// refs das raízes ("tit3.cap7", "art37"); tudo abaixo delas está no recorte.
// Vazio quer dizer a lei inteira.
//
// O edital quase nunca cita artigo. Ele nomeia o assunto — "Constituição…:
// Administração Pública; fiscalização contábil, financeira, orçamentária…" —
// e o assunto é o título de uma divisão da lei. O recorte sai de casar cada
// assunto com esses títulos, mais os artigos que o tópico cite. O que não casa
// com nada não delimita: na dúvida, a lei inteira, que a pessoa pode estreitar
// à mão.

var (
	// Palavras que não distinguem um título de outro.
	vazias = map[string]bool{
		"a": true, "o": true, "as": true, "os": true, "e": true, "ou": true, "de": true,
		"da": true, "do": true, "das": true, "dos": true, "em": true, "na": true, "no": true,
		"nas": true, "nos": true, "ao": true, "aos": true, "sobre": true, "que": true, "se": true,
		"com": true, "para": true, "por": true, "um": true, "uma": true,
	}
	// Títulos que se repetem em toda lei e não dizem assunto nenhum (R3).
	genericas = map[string]bool{
		"disposicoes": true, "disposicao": true, "gerais": true, "geral": true,
		"preliminares": true, "finais": true, "transitorias": true, "comuns": true,
	}
	palavra     = regexp.MustCompile(`[a-z0-9]+`)
	citacaoArts = regexp.MustCompile(`\b(?:arts?\.?|artigos?)\s*((?:\d+(?:\s*-\s*[a-z]\b)?\s*(?:,|;|\be\b|\ba\b|\bao\b|\bate\b|-)?\s*)+)`)
	numeroArt   = regexp.MustCompile(`\d+`)
	conectivoA  = regexp.MustCompile(`^\s*(?:a|ao|ate|-)\s*$`)
)

var agrupamentoLei = map[string]bool{
	"parte": true, "livro": true, "titulo": true, "capitulo": true, "secao": true, "subsecao": true,
}

// RecorteDoEdital lê, nos tópicos de uma matéria, o que ela cobra desta lei.
// nil é a lei inteira: nenhum tópico a cita, ou um deles a cita sem delimitar.
func RecorteDoEdital(l Lei, temas []string, ds []Dispositivo) []string {
	var refs []string
	citada := false
	for _, t := range temas {
		if !l.CitadaEm([]string{t}) {
			continue // R9: o tópico é de outra lei.
		}
		citada = true
		doTema := refsDoTema(l, t, ds)
		if len(doTema) == 0 {
			return nil // R1: um tópico pede a lei inteira.
		}
		refs = append(refs, doTema...)
	}
	if !citada {
		return nil
	}

	return RaizesNaOrdem(ds, refs)
}

func refsDoTema(l Lei, tema string, ds []Dispositivo) []string {
	existe := make(map[string]bool, len(ds))
	for _, d := range ds {
		existe[d.Ref] = true
	}

	var refs []string
	for _, ref := range artigosCitados(dobrar(tema)) {
		if existe[ref] { // R6
			refs = append(refs, ref)
		}
	}

	for _, pedaco := range strings.FieldsFunc(tema, func(r rune) bool { return r == ';' || r == ':' }) {
		// O pedaço que nomeia a lei identifica, não delimita (R1): "Lei
		// Orgânica do TCE (Lei nº 16.168)" não é o título de divisão nenhuma.
		if l.CitadaEm([]string{pedaco}) {
			continue
		}
		doPedaco := palavras(pedaco)
		for _, d := range ds {
			if agrupamentoLei[d.Tipo] && tituloContido(d.Nome, doPedaco) {
				refs = append(refs, d.Ref)
			}
		}
	}

	return refs
}

// tituloContido: todas as palavras do título estão no pedaço do tópico (R2,
// R4), e o título diz algum assunto (R3).
func tituloContido(titulo string, pedaco map[string]bool) bool {
	doTitulo := palavras(titulo)
	assunto := false
	for p := range doTitulo {
		if !pedaco[p] {
			return false
		}
		assunto = assunto || !genericas[p]
	}

	return assunto
}

func palavras(s string) map[string]bool {
	out := map[string]bool{}
	for _, p := range palavra.FindAllString(dobrar(s), -1) {
		if !vazias[p] {
			out[p] = true
		}
	}

	return out
}

// artigosCitados lê "arts. 37 a 43", "art. 5º", "artigos 1º ao 17" e "arts.
// 70, 71 e 74" num texto já dobrado.
func artigosCitados(t string) []string {
	t = strings.NewReplacer("º", "", "°", "", "ª", "").Replace(t)

	var refs []string
	for _, m := range citacaoArts.FindAllStringSubmatch(t, -1) {
		lista := m[1]
		nums := numeroArt.FindAllStringIndex(lista, -1)
		for i := 0; i < len(nums); i++ {
			de, _ := strconv.Atoi(lista[nums[i][0]:nums[i][1]])
			// "37 a 43": o conectivo entre dois números é uma faixa.
			if i+1 < len(nums) && conectivoA.MatchString(lista[nums[i][1]:nums[i+1][0]]) {
				ate, _ := strconv.Atoi(lista[nums[i+1][0]:nums[i+1][1]])
				for n := de; n <= ate && n-de < 500; n++ {
					refs = append(refs, "art"+strconv.Itoa(n))
				}
				i++

				continue
			}
			refs = append(refs, "art"+strconv.Itoa(de))
		}
	}

	return refs
}

// RaizesNaOrdem deixa cada ref uma vez, na ordem da lei, sem as que já estão
// dentro de outra do recorte (R7).
func RaizesNaOrdem(ds []Dispositivo, refs []string) []string {
	pais := paisDe(ds)
	var out []string
	for _, d := range ds {
		if !slices.Contains(refs, d.Ref) {
			continue
		}
		if pai := pais[d.Ref]; pai != "" && descende(pais, pai, refs) {
			continue
		}
		out = append(out, d.Ref)
	}

	return out
}

// TrechoDoRecorte é uma raiz do recorte como a tela a mostra: "Seção IX — DA
// FISCALIZAÇÃO… (arts. 70 a 75)".
type TrechoDoRecorte struct {
	Ref     string
	Rotulo  string
	Nome    string
	Artigos string
}

// DescreverRecorte dá, para cada raiz, o rótulo, o nome e a faixa de artigos.
func DescreverRecorte(ds []Dispositivo, refs []string) []TrechoDoRecorte {
	pais := paisDe(ds)
	var out []TrechoDoRecorte
	for _, d := range ds {
		if !slices.Contains(refs, d.Ref) {
			continue
		}
		var artigos []string
		for _, x := range ds {
			if x.Tipo == "artigo" && descende(pais, x.Ref, []string{d.Ref}) {
				artigos = append(artigos, strings.TrimSpace(strings.TrimPrefix(x.Rotulo, "Art.")))
			}
		}
		t := TrechoDoRecorte{Ref: d.Ref, Rotulo: d.Rotulo, Nome: d.Nome}
		switch len(artigos) {
		case 0:
		case 1:
			t.Artigos = "art. " + artigos[0]
		default:
			t.Artigos = "arts. " + artigos[0] + " a " + artigos[len(artigos)-1]
		}
		out = append(out, t)
	}

	return out
}

// NoRecorte diz se o dispositivo está no recorte: descende de uma raiz. Recorte
// vazio é a lei inteira.
func NoRecorte(ds []Dispositivo, recorte []string, ref string) bool {
	return len(recorte) == 0 || descende(paisDe(ds), ref, recorte)
}
