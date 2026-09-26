package lei

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var (
	ErrCapturaNaoEncontrada = errors.New("captura não encontrada ou expirada — capture de novo")
	ErrCapturaEmAndamento   = errors.New("a captura ainda está rodando")
	ErrCapturaFalhou        = errors.New("a captura falhou — capture de novo")
	ErrCapturaBloqueada     = errors.New("a captura tem problemas que impedem a publicação")
	ErrCapturaIndisponivel  = errors.New("o processador de leis está ocupado ou fora do ar — tente de novo em um minuto")
	ErrFonteNaoEncontrada   = errors.New("não achei a fonte oficial desta norma pelo tópico: cole o link dela")
	ErrLeiJaExiste          = errors.New("já existe uma lei com esse nome curto — para trocar o texto dela, use \"Atualizar texto\" na página da lei")
)

// ErrLinkInvalido é o link que o processador recusou: a mensagem dele diz qual
// fonte usar.
type ErrLinkInvalido struct{ Motivo string }

func (e ErrLinkInvalido) Error() string { return e.Motivo }

// ErrAvisosPendentes lista os avisos da captura que ninguém marcou como
// revisados.
type ErrAvisosPendentes struct{ IDs []string }

func (e ErrAvisosPendentes) Error() string {
	return fmt.Sprintf("marque como revisado cada aviso da captura antes de publicar (%d pendente(s))", len(e.IDs))
}

// Estados da captura, como o processador os informa.
const (
	CapturaRodando = "rodando"
	CapturaPronta  = "pronta"
	CapturaFalhou  = "falhou"
)

// Aviso é o que a captura resolveu sozinha e alguém precisa conferir: o
// Gemini discordou da regra, a numeração salta, não houve Gemini. A regra
// prevaleceu; publicar exige marcar que foi revisado.
type Aviso struct {
	ID     string
	Texto  string
	Trecho string
}

// ResumoDaCaptura é o que a prévia mostra além da árvore.
type ResumoDaCaptura struct {
	Vigentes    int
	Anteriores  int
	Notas       int
	Revogados   int
	Tipos       map[string]int
	Descartados []string
	Riscados    []string
	Juncoes     []string
}

// ResultadoDaCaptura é a lei como o processador a leu. Com bloqueio, não há
// dispositivos para publicar.
type ResultadoDaCaptura struct {
	Fonte        string
	Gemini       bool
	Versao       string
	Dispositivos []Dispositivo
	Bloqueios    []string
	Avisos       []Aviso
	Resumo       ResumoDaCaptura
	// Recorte são as raízes do que foi guardado; vazio é a lei inteira.
	Recorte []string
}

// Pesquisa é a lei que o tópico do edital cita, como a pesquisa a leu: a fonte
// e a estrutura (divisões, artigos e epígrafe), sem o texto inteiro.
type Pesquisa struct {
	Fonte     string
	Link      string
	Epigrafe  string
	Estrutura []Dispositivo
}

// Captura é o andamento de uma captura e, pronta, o resultado.
type Captura struct {
	ID        string
	Estado    string
	Etapa     string
	Feitos    int
	Total     int
	Erro      string
	Resultado *ResultadoDaCaptura
}

// ConferirPublicacao diz se a captura pode virar lei: pronta, sem bloqueio e
// com todos os avisos marcados como revisados. Um aceite que não corresponde a
// aviso nenhum não libera outro (K17b).
func (c Captura) ConferirPublicacao(aceitos []string) error {
	switch c.Estado {
	case CapturaRodando:
		return ErrCapturaEmAndamento
	case CapturaFalhou:
		return ErrCapturaFalhou
	case CapturaPronta:
	default:
		return fmt.Errorf("captura em estado desconhecido %q", c.Estado)
	}

	r := c.Resultado
	if r == nil || len(r.Bloqueios) > 0 || len(r.Dispositivos) == 0 || r.Versao == "" {
		return ErrCapturaBloqueada
	}

	var pendentes []string
	for _, a := range r.Avisos {
		if !slices.Contains(aceitos, a.ID) {
			pendentes = append(pendentes, a.ID)
		}
	}
	if len(pendentes) > 0 {
		return ErrAvisosPendentes{IDs: pendentes}
	}

	return nil
}

var (
	naoSlug   = regexp.MustCompile(`[^a-z0-9]+`)
	numeroLei = regexp.MustCompile(`\b\d{1,3}(?:\.\d{3})+\b`)
)

// SlugDe dá o endereço de uma lei nova a partir do nome curto: "LGPD" vira
// "lgpd", "Lei Orgânica do TCE-GO" vira "lei-organica-do-tce-go". Depois de
// publicada, o slug não muda — é por ele que links e questões a encontram.
func SlugDe(curto string) string {
	return strings.Trim(naoSlug.ReplaceAllString(dobrar(curto), "-"), "-")
}

// Epigrafe é a primeira linha da lei ("LEI Nº 13.709, DE 14 DE AGOSTO DE
// 2018", "CONSTITUIÇÃO DA REPÚBLICA FEDERATIVA DO BRASIL DE 1988"): o nome dela
// antes de alguém dar um.
func Epigrafe(ds []Dispositivo) string {
	for _, d := range ds {
		if d.Tipo == "preambulo" && strings.TrimSpace(d.Texto) != "" {
			return strings.TrimRight(strings.TrimSpace(d.Texto), ".")
		}
	}

	return ""
}

// ReconhecerPadrao é o que sugere a lei para uma matéria quando ninguém disse
// nada: o número dela ("Lei nº 13.709/2018" → "13.709"), que é o que os
// editais citam, tirado do nome ou da epígrafe. Sem número — a Constituição —,
// a própria epígrafe.
func ReconhecerPadrao(nome string, ds []Dispositivo) []string {
	epigrafe := Epigrafe(ds)
	var out []string
	for _, n := range numeroLei.FindAllString(nome+" "+epigrafe, -1) {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	if len(out) == 0 && epigrafe != "" {
		out = append(out, epigrafe)
	}

	return out
}

var leiNumerada = regexp.MustCompile(`(?i)^lei\s+(complementar\s+)?n[º°o.]*\s*([\d.]+).*?(\d{4})\s*$`)

// CurtoDe sugere o nome curto pela epígrafe: "Constituição Federal", "Lei nº
// 13.709/2018", "LC nº 205/2025". Sem padrão conhecido, a própria epígrafe.
func CurtoDe(epigrafe string) string {
	e := strings.TrimSpace(epigrafe)
	if strings.HasPrefix(dobrar(e), "constituicao da republica federativa do brasil") {
		return "Constituição Federal"
	}
	if m := leiNumerada.FindStringSubmatch(e); m != nil {
		tipo := "Lei"
		if m[1] != "" {
			tipo = "LC"
		}

		return tipo + " nº " + m[2] + "/" + m[3]
	}

	return e
}

// Provisoria é a lei que a captura trouxe, antes de ter nome: basta para achar
// os tópicos do edital que a citam.
func Provisoria(ds []Dispositivo) Lei {
	return Lei{Nome: Epigrafe(ds), Reconhecer: ReconhecerPadrao("", ds)}
}
