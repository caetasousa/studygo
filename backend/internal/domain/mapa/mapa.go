// Package mapa é o mapa mental de um assunto: uma árvore de itens curtos,
// organizada a partir de uma aula e lida de um outline de texto.
//
// O outline é escrito fora do app e chega pela tela. Aqui ele é lido e
// validado — recuo, marcas, limites — e o resultado é a árvore que o app grava
// e desenha. Nada neste pacote conhece JSON, SQL ou HTTP.
package mapa

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

// Limites do que o app aceita importar. Um mapa é para ser lido de relance: o
// teto existe para que um texto gigante não vire uma requisição que derruba o
// servidor, nem um mapa ilegível.
const (
	MaxItens  = 5000
	MaxNiveis = 10
	MaxTexto  = 500 // caracteres por item
	MaxTitulo = 120
	MaxFonte  = 300
	MaxSlug   = 60
	// MaxReconhecer é quantos termos de reconhecimento um mapa pode indicar.
	MaxReconhecer = 20
)

// ErrNaoEncontrado vale para o mapa que não existe e também para o de outra
// conta: quem não é dono não sabe que ele existe.
var ErrNaoEncontrado = errors.New("mapa mental não encontrado")

// ErrTextoInvalido traz os problemas do outline, com a linha de cada um, para
// quem importa corrigir tudo de uma vez em vez de um por tentativa.
type ErrTextoInvalido struct{ Problemas []string }

func (e ErrTextoInvalido) Error() string {
	const maximo = 8

	ps := e.Problemas
	resto := ""

	if len(ps) > maximo {
		resto = fmt.Sprintf(" (e mais %d)", len(ps)-maximo)
		ps = ps[:maximo]
	}

	return "mapa inválido: " + strings.Join(ps, "; ") + resto
}

// Marca diz o que um item é, além de texto: o que a tela destaca para quem
// revisa antes da prova.
type Marca string

const (
	SemMarca Marca = ""
	// Definicao é o conceito que a banca cobra pelo enunciado.
	Definicao Marca = "def"
	// Pegadinha é o que a banca costuma inverter.
	Pegadinha Marca = "pegadinha"
	// CaiEmProva é o que a aula avisa que cai.
	CaiEmProva Marca = "cai"
	Exemplo    Marca = "ex"
	// MarcaQuestao marca um item que cita uma questão de prova. As questões
	// de verdade moram no arquivo de questões do mapa (ver Questao); a marca
	// segue aceita para não recusar outline antigo.
	MarcaQuestao Marca = "questao"
)

// Marcas são as que o outline entende, na ordem em que a mensagem de erro as lista.
var Marcas = []Marca{Definicao, Pegadinha, CaiEmProva, Exemplo, MarcaQuestao}

// Item é um ponto do mapa e tudo o que está abaixo dele.
type Item struct {
	Texto  string
	Marca  Marca
	Filhos []Item
}

// Mapa é o mapa mental. Os Ramos são os itens de cima; o Titulo é a raiz.
type Mapa struct {
	ID      uuid.UUID
	Slug    string
	Titulo  string
	Fonte   string
	Materia string
	// Reconhecer são termos que, achados num tópico do concurso, indicam a
	// matéria do mapa. Servem só à importação: não são guardados.
	Reconhecer []string
	Ramos      []Item
}

// Contar diz quantos ramos (itens de cima) e quantos itens o mapa tem.
func (m Mapa) Contar() (ramos, itens int) {
	var somar func([]Item)

	somar = func(is []Item) {
		for _, it := range is {
			itens++

			somar(it.Filhos)
		}
	}

	somar(m.Ramos)

	return len(m.Ramos), itens
}

// Resumo é o mapa sem a árvore: o que a lista e o vínculo precisam.
type Resumo struct {
	ID          uuid.UUID
	Slug        string
	Titulo      string
	Fonte       string
	Materia     string
	Ramos       int
	Itens       int
	ImportadoEm time.Time
}

// SugereMateria diz se o mapa parece ser da matéria: o nome dela traz as
// palavras de `materia` do mapa, ou algum tópico dela cita um termo de
// `reconhecer`. Caixa e acento não contam. É uma sugestão, e desfazê-la é um
// clique: por isso a importação a aplica sem perguntar.
func (m Mapa) SugereMateria(nome string, temas []string) bool {
	if m.Materia != "" && contemAsPalavras(dobrar(nome), dobrar(m.Materia)) {
		return true
	}

	for _, termo := range m.Reconhecer {
		agulha := dobrar(termo)
		if agulha == "" {
			continue
		}

		for _, t := range temas {
			if strings.Contains(" "+dobrar(t)+" ", " "+agulha+" ") {
				return true
			}
		}
	}

	return false
}

// palavrasMiudas não distinguem uma matéria de outra: "Governança de TI" e
// "Governança e Gestão de TI" compartilham "governança" e "TI".
var palavrasMiudas = map[string]bool{
	"a": true, "o": true, "as": true, "os": true, "e": true, "de": true, "da": true,
	"do": true, "das": true, "dos": true, "em": true, "na": true, "no": true,
	"para": true, "por": true, "com": true,
}

// contemAsPalavras: todas as palavras que importam de dica estão em nome.
func contemAsPalavras(nome, dica string) bool {
	tem := map[string]bool{}
	for p := range strings.FieldsSeq(nome) {
		tem[p] = true
	}

	importam := 0

	for p := range strings.FieldsSeq(dica) {
		if palavrasMiudas[p] {
			continue
		}

		importam++

		if !tem[p] {
			return false
		}
	}

	return importam > 0
}

// dobrar deixa só letras e números minúsculos, sem acento, separados por um
// espaço: "Governança, de TI!" vira "governanca de ti".
func dobrar(s string) string {
	var b strings.Builder

	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			// o acento solto que o NFD separou da letra
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}

	return strings.Join(strings.Fields(b.String()), " ")
}
