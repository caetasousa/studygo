package mapa

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// As questões da aula não moram no mapa: vêm num arquivo à parte, importado na
// página dele, e cada uma fica presa a um ramo. Sem alternativas, a questão é
// de julgar (Certo ou Errado, como a Cebraspe cobra); com elas, de múltipla
// escolha, de 2 a 5.
const (
	Certo  = "CERTO"
	Errado = "ERRADO"

	MaxQuestoes    = 1000
	MaxEnunciado   = 5000 // caracteres
	MaxComentario  = 5000
	MaxAlternativa = 2000
	MaxOrigem      = 300
	MaxChave       = 80
)

var letras = []string{"A", "B", "C", "D", "E"}

var (
	// ErrQuestaoNaoEncontrada vale também para a questão de outra conta ou já
	// retirada do arquivo.
	ErrQuestaoNaoEncontrada = errors.New("questão não encontrada")
	// ErrRespostaInvalida é a resposta que não cabe na questão: letra além das
	// alternativas, ou letra numa questão de Certo/Errado.
	ErrRespostaInvalida = errors.New("resposta inválida para esta questão")
)

// ErrQuestoesInvalidas traz todos os problemas do arquivo de uma vez, cada um
// com a questão a que se refere.
type ErrQuestoesInvalidas struct{ Problemas []string }

func (e ErrQuestoesInvalidas) Error() string {
	const maximo = 8

	ps := e.Problemas
	resto := ""

	if len(ps) > maximo {
		resto = fmt.Sprintf(" (e mais %d)", len(ps)-maximo)
		ps = ps[:maximo]
	}

	return "arquivo de questões inválido: " + strings.Join(ps, "; ") + resto
}

// Questao é uma questão de prova sobre o assunto do mapa.
type Questao struct {
	// Chave é o id que o arquivo dá à questão: importar de novo casa por ela.
	Chave string
	// Ramo é o título do ramo do mapa a que a questão pertence, como o mapa o
	// escreve (sem o negrito).
	Ramo string
	// Origem diz de onde a questão veio: banca, ano, órgão, cargo.
	Origem       string
	Enunciado    string
	Alternativas []string
	// Gabarito é a letra (A–E) ou, na de julgar, CERTO ou ERRADO.
	Gabarito   string
	Comentario string
}

// DeJulgar diz se a questão é de Certo ou Errado.
func (q Questao) DeJulgar() bool { return len(q.Alternativas) == 0 }

// Corrigir diz se a resposta é o gabarito; a resposta que não cabe na questão
// é recusada, e não conta como erro.
func (q Questao) Corrigir(resposta string) (bool, error) {
	r := strings.ToUpper(strings.TrimSpace(resposta))

	if q.DeJulgar() {
		if r != Certo && r != Errado {
			return false, ErrRespostaInvalida
		}
	} else if !slices.Contains(letras[:len(q.Alternativas)], r) {
		return false, ErrRespostaInvalida
	}

	return r == q.Gabarito, nil
}

// Assinatura resume o conteúdo: igual, a reimportação não mexe na questão.
func (q Questao) Assinatura() string {
	h := sha256.New()
	partes := append([]string{q.Ramo, q.Origem, q.Enunciado, q.Gabarito, q.Comentario}, q.Alternativas...)

	for _, p := range partes {
		fmt.Fprintf(h, "%s\x1f", p)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// ArquivoDeQuestoes é o que chega para importar: o mapa a que o arquivo diz
// pertencer e as questões.
type ArquivoDeQuestoes struct {
	Mapa     string
	Questoes []Questao
}

// ValidarQuestoes confere o arquivo contra o mapa em que está sendo importado e
// devolve as questões prontas para gravar: gabarito em maiúsculas e o ramo
// escrito como o mapa o escreve. Todos os problemas vêm de uma vez.
func ValidarQuestoes(m Mapa, a ArquivoDeQuestoes) ([]Questao, error) {
	var ps []string

	add := func(f string, args ...any) { ps = append(ps, fmt.Sprintf(f, args...)) }

	if a.Mapa != m.Slug {
		add("o arquivo é do mapa %q, e esta página é do %q", a.Mapa, m.Slug)
	}

	switch {
	case len(a.Questoes) == 0:
		add("o arquivo não traz nenhuma questão")
	case len(a.Questoes) > MaxQuestoes:
		add("o arquivo passa de %d questões", MaxQuestoes)
	}

	// Os ramos pelo texto dobrado: "Práticas de **gerenciamento**" e "praticas
	// de gerenciamento" são o mesmo ramo.
	ramos := make(map[string]string, len(m.Ramos))
	for _, r := range m.Ramos {
		ramos[dobrar(r.Texto)] = strings.ReplaceAll(r.Texto, "**", "")
	}

	chaves := map[string]bool{}
	prontas := make([]Questao, 0, len(a.Questoes))

	for i, q := range a.Questoes {
		q.Chave = strings.TrimSpace(q.Chave)
		nome := "questão " + q.Chave

		switch {
		case q.Chave == "":
			nome = fmt.Sprintf("questão %d", i+1)
			add("%s: sem id", nome)
		case chaves[q.Chave]:
			add("chave %q repetida", q.Chave)
		case utf8.RuneCountInString(q.Chave) > MaxChave:
			add("%s: id com mais de %d caracteres", nome, MaxChave)
		}

		chaves[q.Chave] = true

		for _, p := range q.problemas(ramos) {
			add("%s: %s", nome, p)
		}

		if r, ok := ramos[dobrar(q.Ramo)]; ok {
			q.Ramo = r
		}

		q.Gabarito = strings.ToUpper(strings.TrimSpace(q.Gabarito))
		q.Origem = strings.TrimSpace(q.Origem)
		prontas = append(prontas, q)
	}

	if len(ps) > 0 {
		return nil, ErrQuestoesInvalidas{Problemas: ps}
	}

	return prontas, nil
}

func (q Questao) problemas(ramos map[string]string) []string {
	var ps []string

	add := func(f string, a ...any) { ps = append(ps, fmt.Sprintf(f, a...)) }

	if _, ok := ramos[dobrar(q.Ramo)]; !ok {
		add("o ramo %q não existe no mapa", q.Ramo)
	}

	if strings.TrimSpace(q.Enunciado) == "" {
		add("enunciado vazio")
	} else if utf8.RuneCountInString(q.Enunciado) > MaxEnunciado {
		add("enunciado com mais de %d caracteres", MaxEnunciado)
	}

	if strings.TrimSpace(q.Comentario) == "" {
		add("comentário vazio")
	} else if utf8.RuneCountInString(q.Comentario) > MaxComentario {
		add("comentário com mais de %d caracteres", MaxComentario)
	}

	if utf8.RuneCountInString(q.Origem) > MaxOrigem {
		add("origem com mais de %d caracteres", MaxOrigem)
	}

	gabarito := strings.ToUpper(strings.TrimSpace(q.Gabarito))

	if q.DeJulgar() {
		if gabarito != Certo && gabarito != Errado {
			add("gabarito %q — use Certo ou Errado", q.Gabarito)
		}

		return ps
	}

	if gabarito == Certo || gabarito == Errado {
		add("Certo/Errado não leva alternativas")

		return ps
	}

	if n := len(q.Alternativas); n < 2 || n > len(letras) {
		add("precisa de 2 a 5 alternativas, tem %d", n)

		return ps
	}

	vistas := map[string]bool{}

	for i, alt := range q.Alternativas {
		chave := dobrar(alt)

		switch {
		case chave == "":
			add("alternativa %s vazia", letras[i])
		case vistas[chave]:
			add("alternativa %s repete outra", letras[i])
		case utf8.RuneCountInString(alt) > MaxAlternativa:
			add("alternativa %s com mais de %d caracteres", letras[i], MaxAlternativa)
		}

		vistas[chave] = true
	}

	if validas := letras[:len(q.Alternativas)]; !slices.Contains(validas, gabarito) {
		add("gabarito %q fora das alternativas (A–%s)", q.Gabarito, validas[len(validas)-1])
	}

	return ps
}

// QuestaoGravada é o que a importação precisa saber das questões que já
// existem: quem é quem, e se mudou.
type QuestaoGravada struct {
	ID         uuid.UUID
	Chave      string
	Assinatura string
	Ativa      bool
}

// QuestaoAtualizada leva o id da gravada: é ele que prende as respostas.
type QuestaoAtualizada struct {
	ID      uuid.UUID
	Questao Questao
}

// PlanoDeQuestoes diz o que fazer com as questões do arquivo. Nenhuma é
// apagada: a que saiu do arquivo é desativada, e as respostas a ela ficam.
// Ordem é a de todas as questões do arquivo, pela chave.
type PlanoDeQuestoes struct {
	Novas       []Questao
	Atualizadas []QuestaoAtualizada
	Desativar   []uuid.UUID
	Mantidas    int
	Ordem       []string
}

// PlanejarQuestoes casa as questões do arquivo com as gravadas pela chave.
func PlanejarQuestoes(gravadas []QuestaoGravada, doArquivo []Questao) PlanoDeQuestoes {
	porChave := make(map[string]QuestaoGravada, len(gravadas))
	for _, g := range gravadas {
		porChave[g.Chave] = g
	}

	var plano PlanoDeQuestoes

	noArquivo := make(map[string]bool, len(doArquivo))

	for _, q := range doArquivo {
		noArquivo[q.Chave] = true
		plano.Ordem = append(plano.Ordem, q.Chave)

		g, existe := porChave[q.Chave]

		switch {
		case !existe:
			plano.Novas = append(plano.Novas, q)
		case g.Ativa && g.Assinatura == q.Assinatura():
			plano.Mantidas++
		default:
			plano.Atualizadas = append(plano.Atualizadas, QuestaoAtualizada{ID: g.ID, Questao: q})
		}
	}

	for _, g := range gravadas {
		if g.Ativa && !noArquivo[g.Chave] {
			plano.Desativar = append(plano.Desativar, g.ID)
		}
	}

	return plano
}

// QuestaoComResposta é a questão ativa com a última resposta, se houver.
type QuestaoComResposta struct {
	ID      uuid.UUID
	Questao Questao
	Ultima  *Resposta
}

// Resposta é uma tentativa: a mais recente é a que a tela mostra.
type Resposta struct {
	Resposta string
	Acertou  bool
	Em       time.Time
}
