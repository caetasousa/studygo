// Package prova é o catálogo de provas anteriores: o rascunho que a extração
// produz, as regras que o curador precisa cumprir para publicá-lo e a forma
// como resultados de regiões sobrepostas viram uma prova só.
package prova

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	ErrAcesso        = errors.New("curadoria de provas não autorizada")
	ErrConflito      = errors.New("a importação mudou desde que foi carregada; recarregue antes de continuar")
	ErrNaoEncontrada = errors.New("prova ou importação não encontrada")
	ErrLimite        = errors.New("limite de importações pendentes atingido")
)

// Estados de uma importação.
const (
	EstadoNaFila      = "na_fila"
	EstadoProcessando = "processando"
	EstadoEmRevisao   = "em_revisao"
	EstadoFalhou      = "falhou"
	EstadoPublicada   = "publicada"
	EstadoCancelada   = "cancelada"
)

// Etapas de uma importação, na ordem em que o worker as executa. Cada região
// ocupa uma etapa a partir de EtapaPrimeiraRegiao; depois da última vem a
// consolidação.
//
// Os metadados têm etapa própria, lida só da capa, porque cada região via um
// pedaço do cabeçalho e devolvia um órgão e um ano diferentes — e o primeiro
// palpite vencia.
const (
	// EtapaTrecho lê só o trecho que o curador marcou em volta de uma questão
	// mal lida (ver TrechoPendente) e volta direto à revisão.
	EtapaTrecho = -2
	// EtapaSoGabarito reprocessa apenas o gabarito de um rascunho em revisão,
	// sem reextrair as questões.
	EtapaSoGabarito     = -1
	EtapaPreparar       = 0
	EtapaMetadados      = 1
	EtapaGabarito       = 2
	EtapaPrimeiraRegiao = 3
)

// TotalEtapas conta as etapas fixas, uma por região e a consolidação final.
func TotalEtapas(regioes int) int { return EtapaPrimeiraRegiao + regioes + 1 }

// ProximaEtapa diz para onde a importação vai depois de concluir `executada`:
// a próxima etapa na fila, ou a revisão quando não resta nada a processar.
func ProximaEtapa(executada, regioes int) (int, string) {
	consolidacao := EtapaPrimeiraRegiao + regioes
	if executada == EtapaSoGabarito || executada == EtapaTrecho || executada >= consolidacao {
		return TotalEtapas(regioes), EstadoEmRevisao
	}

	return executada + 1, EstadoNaFila
}

// tentativasPorEtapa limita as falhas seguidas de uma mesma etapa.
const tentativasPorEtapa = 6

// EsperaParaRepetir diz quanto esperar antes de repetir uma etapa que falhou
// por motivo que passa sozinho, sabendo quantas falhas seguidas ela já teve
// antes desta; zero quando as tentativas acabaram. A espera dobra (15s, 30s,
// 1min, 2min, 4min): a sobrecarga do Gemini costuma durar minutos, e repetir
// logo só gastava as tentativas — três em meio minuto derrubavam a importação.
func EsperaParaRepetir(falhasAnteriores int) time.Duration {
	if falhasAnteriores < 0 || falhasAnteriores >= tentativasPorEtapa-1 {
		return 0
	}

	return 15 * time.Second << falhasAnteriores
}

// Origem aponta um retângulo do PDF original, em pontos da página física.
type Origem struct {
	Pagina    int
	Retangulo []float64
	Regiao    string
}

// Bloco é um pedaço ordenado de conteúdo. Imagem é sempre recorte do original;
// a IA só sugere onde recortar.
type Bloco struct {
	Tipo, Texto, Formato, Arquivo, Descricao string
	Origem                                   *Origem
	Revisado                                 bool
	// Largura é o tamanho da figura na tela, em porcentagem da coluna do
	// conteúdo; zero deixa no tamanho do arquivo (DimensionarFiguras põe a
	// proporção do caderno).
	Largura int
}

type Alternativa struct {
	Letra  string
	Blocos []Bloco
}

type Questao struct {
	Numero       int
	Disciplina   string
	Blocos       []Bloco
	Alternativas []Alternativa
	// Apoios lista os ids dos textos compartilhados que a questão usa.
	Apoios   []string
	Origens  []Origem
	Resposta string
	Situacao string
	Revisada bool
	// Completa é falsa quando a região cortou a questão; o curador confirma
	// que juntou os pedaços antes de publicar.
	Completa bool
	// IgualA diz de onde a questão foi reaproveitada ("TJCE 2026 · E05, questão
	// 3"): o conteúdo é o já publicado lá.
	IgualA string
}

// Apoio é o texto ou figura compartilhado por várias questões.
type Apoio struct {
	ID       string
	Blocos   []Bloco
	Questoes []int
	// Aviso é a frase do caderno que diz quais questões usam o texto
	// ("Considere o texto … questões de 1 a 10"). Ela sai do texto, que é só o
	// texto, e fica aqui para o curador conferir a faixa.
	Aviso string
	// Origens aponta onde o texto está no caderno original.
	Origens  []Origem
	Revisado bool
	// IgualA diz de qual prova o texto foi reaproveitado.
	IgualA string
}

// rotulo é como o curador reconhece o texto: pelas questões, não pelo id.
func (a Apoio) rotulo() string {
	if len(a.Questoes) == 0 {
		return "texto de apoio " + a.ID + ", que não está ligado a nenhuma questão"
	}

	return "texto de apoio das questões " + faixas(a.Questoes)
}

// faixas escreve [1..10, 12] como "1-10, 12".
func faixas(numeros []int) string {
	ns := slices.Sorted(slices.Values(numeros))
	var partes []string
	for i := 0; i < len(ns); i++ {
		j := i
		for j+1 < len(ns) && ns[j+1] == ns[j]+1 {
			j++
		}
		if j > i {
			partes = append(partes, fmt.Sprintf("%d-%d", ns[i], ns[j]))
		} else {
			partes = append(partes, strconv.Itoa(ns[i]))
		}
		i = j
	}

	return strings.Join(partes, ", ")
}

// Extracao registra modelo e consumo de cada chamada, para medir custo real.
type Extracao struct {
	Modelo                     string
	TokensEntrada, TokensSaida int
	Regiao, Prompt, Versao     string
}

// ResumoDeQuestao é o que a classificação por matéria lê de uma questão.
type ResumoDeQuestao struct {
	Numero int
	// Secao é a disciplina que a questão tem agora — o título da seção do
	// caderno, que às vezes já é a matéria e às vezes é genérico.
	Secao string
	Texto string
}

// Metadados identificam o caderno. Vêm da capa.
//
// Cargo é o código que o caderno e o gabarito citam ("F06"): é por ele que o
// gabarito se confere. CargoNome é o nome por extenso, que é o que o aluno
// reconhece.
type Metadados struct {
	Orgao                     string
	Ano                       int
	Cargo, CargoNome, Caderno string
	Total                     int
}

type Rascunho struct {
	Extracoes                 []Extracao
	Banca, Orgao              string
	Ano                       int
	Cargo, CargoNome, Caderno string
	Total                     int
	Questoes                  []Questao
	Apoios                    []Apoio
	Gabarito                  Gabarito
	Alertas                   []string
	// AnuladasExcluidas são os números das questões que a banca anulou e o
	// curador tirou da prova. Total continua sendo o da capa: é a numeração do
	// caderno, e a anulada excluída só deixa de ser esperada.
	AnuladasExcluidas []int
}

// faltando diz quais números a pendência de total não achou — "O rascunho tem
// 59 questões" não dizia qual procurar.
func (r Rascunho) faltando() string {
	presentes := map[int]bool{}
	for _, q := range r.Questoes {
		presentes[q.Numero] = true
	}
	for _, n := range r.AnuladasExcluidas {
		presentes[n] = true
	}
	faltam := []string{}
	for n := 1; n <= r.Total; n++ {
		if !presentes[n] {
			faltam = append(faltam, strconv.Itoa(n))
		}
	}
	switch len(faltam) {
	case 0:
		return ""
	case 1:
		return " Falta a questão " + faltam[0] + "."
	default:
		return " Faltam as questões " + strings.Join(faltam, ", ") + "."
	}
}

// QuestoesNaProva é quantas questões a prova publicada tem: o total da capa
// sem as anuladas excluídas.
func (r Rascunho) QuestoesNaProva() int { return r.Total - len(r.AnuladasExcluidas) }

type Importacao struct {
	ID, Criador, Hash, Documento, GabaritoArquivo string
	// NomeDocumento e NomeGabarito são os nomes com que o curador enviou os
	// PDFs (NomeDoArquivo): a curadoria mostra de que arquivo veio cada uma.
	NomeDocumento, NomeGabarito      string
	Estado, Erro, Tentativa, ProvaID string
	ProcessadoMS                     int64
	Versao, Etapa, Falhas, Chamadas  int
	Regioes                          []Origem
	Rascunho                         Rascunho
	CriadoEm, AtualizadoEm           time.Time
}

// Tamanho máximo do nome guardado; o sistema de arquivos não passa disso.
const maxNomeDoArquivo = 255

// NomeDoArquivo é o nome do PDF como o curador o vê na pasta dele: sem o
// caminho que alguns navegadores mandam, sem espaço nas pontas e sem passar do
// tamanho de um nome de arquivo.
func NomeDoArquivo(enviado string) string {
	nome := enviado[strings.LastIndexAny(enviado, `/\`)+1:]
	nome = strings.TrimSpace(strings.ToValidUTF8(nome, ""))
	if r := []rune(nome); len(r) > maxNomeDoArquivo {
		nome = string(r[:maxNomeDoArquivo])
	}

	return nome
}

// Cancelar tira a importação da fila e da revisão. Ela deixa de segurar os
// PDFs: sem o hash, os mesmos arquivos podem ser importados de novo do zero.
func (i *Importacao) Cancelar() {
	i.Estado = EstadoCancelada
	i.Erro = ""
	i.Hash = ""
}

// MesmaProva diz se os dois rascunhos são a mesma prova: banca, órgão, ano e
// código do cargo. O tipo do caderno não conta — os tipos da FCC são a mesma
// prova em outra ordem, e o catálogo teria o mesmo concurso duas vezes. Sem
// ano ou sem código de um dos lados não dá para dizer, e não é.
func MesmaProva(a, b Rascunho) bool {
	if a.Ano <= 0 || a.Ano != b.Ano || !strings.EqualFold(a.Banca, b.Banca) || !MesmoOrgao(a.Orgao, b.Orgao) {
		return false
	}

	return a.Cargo != "" && b.Cargo != "" && (mesmoCargo(a.Cargo, b.Cargo) || mesmoCargo(b.Cargo, a.Cargo))
}

// JaImportada para a importação que repete uma prova do catálogo ou de outra
// importação: nada além da capa é lido. Ela fica cancelada, com o motivo, e
// continua segurando o hash — reenviar os mesmos PDFs cai nela, e não numa
// importação nova.
func (i *Importacao) JaImportada(outra Rascunho, publicada bool) {
	i.Estado = EstadoCancelada
	if publicada {
		i.Erro = fmt.Sprintf("Esta prova (%s) já está no catálogo, e nada foi importado de novo. "+
			"Para corrigir a publicada, abra-a e use \"Abrir revisão\".", outra.Rotulo())
		return
	}
	i.Erro = fmt.Sprintf("Esta prova (%s) já está em outra importação, e nada foi importado de novo. "+
		"Continue por aquela, na curadoria; se ela não serve, exclua-a e importe de novo.", outra.Rotulo())
}

// Excluivel diz se a importação pode ser apagada. A publicada é o histórico
// da prova; a que está processando teria o passo em andamento gravando
// arquivos para ninguém — cancela-se antes.
func (i Importacao) Excluivel() bool {
	return i.Estado != EstadoPublicada && i.Estado != EstadoProcessando
}

// Anotacao é o que o estudante escreveu numa questão — o porquê da resposta,
// o que ele pesquisou —, em markdown. Só quem escreveu lê.
type Anotacao struct {
	Numero       int
	Texto        string
	AtualizadaEm time.Time
}

// TamanhoMaximoDaAnotacao, em caracteres: nota de estudo, não livro.
const TamanhoMaximoDaAnotacao = 20000

// Publicacao é uma revisão publicada no catálogo.
type Publicacao struct {
	ID          string
	Revisao     int
	Conteudo    Rascunho
	PublicadoEm time.Time
}

// Arquivos lista os recortes que o rascunho referencia.
func (r Rascunho) Arquivos() []string {
	var out []string
	colher := func(bs []Bloco) {
		for _, b := range bs {
			if b.Arquivo != "" {
				out = append(out, b.Arquivo)
			}
		}
	}
	for _, q := range r.Questoes {
		colher(q.Blocos)
		for _, a := range q.Alternativas {
			colher(a.Blocos)
		}
	}
	for _, a := range r.Apoios {
		colher(a.Blocos)
	}

	return out
}

// AplicarMetadados preenche a identificação do caderno. Campo que a capa não
// trouxe fica como estava, para o curador completar.
func (r *Rascunho) AplicarMetadados(m Metadados) {
	if m.Orgao != "" {
		r.Orgao = m.Orgao
	}
	if m.Ano != 0 {
		r.Ano = m.Ano
	}
	if m.Cargo != "" {
		r.Cargo = m.Cargo
	}
	if m.CargoNome != "" {
		r.CargoNome = m.CargoNome
	}
	if m.Caderno != "" {
		r.Caderno = m.Caderno
	}
	if m.Total != 0 {
		r.Total = m.Total
	}
}

// OrdenarQuestoes deixa as questões na ordem do caderno.
func (r *Rascunho) OrdenarQuestoes() {
	slices.SortStableFunc(r.Questoes, func(a, b Questao) int { return a.Numero - b.Numero })
}

// tamanhoDoResumo corta o texto que vai para a classificação: o começo do
// enunciado e das alternativas basta para dizer a matéria, e sessenta questões
// inteiras encareceriam a chamada.
const tamanhoDoResumo = 600

// ParaClassificar resume as questões para a sugestão de matéria.
func (r Rascunho) ParaClassificar() []ResumoDeQuestao {
	out := make([]ResumoDeQuestao, 0, len(r.Questoes))
	for _, q := range r.Questoes {
		var sb strings.Builder
		escrever := func(bs []Bloco) {
			for _, b := range bs {
				if b.Tipo != "imagem" {
					sb.WriteString(b.Texto)
					sb.WriteString(" ")
				}
			}
		}
		escrever(q.Blocos)
		for _, a := range q.Alternativas {
			sb.WriteString("(" + a.Letra + ") ")
			escrever(a.Blocos)
		}
		texto := []rune(strings.Join(strings.Fields(sb.String()), " "))
		out = append(out, ResumoDeQuestao{
			Numero: q.Numero, Secao: q.Disciplina, Texto: string(texto[:min(len(texto), tamanhoDoResumo)]),
		})
	}

	return out
}

// secaoGenerica diz se o título da seção não nomeia matéria: "Conhecimentos
// Gerais", "Conhecimentos Específicos", ou nada. Título que já é matéria
// ("Língua Portuguesa") manda sobre qualquer sugestão.
func secaoGenerica(s string) bool {
	s = strings.TrimSpace(s)

	return s == "" || strings.HasPrefix(strings.ToUpper(s), "CONHECIMENTOS")
}

// MateriasAplicaveis filtra as sugestões para as questões cuja seção é
// genérica. É a mesma regra de AplicarMaterias, para a tela não sugerir o que
// a consolidação não aplicaria.
func (r Rascunho) MateriasAplicaveis(materias map[int]string) map[int]string {
	out := map[int]string{}
	for _, q := range r.Questoes {
		if m := strings.TrimSpace(materias[q.Numero]); m != "" && secaoGenerica(q.Disciplina) {
			out[q.Numero] = m
		}
	}

	return out
}

// AplicarMaterias troca a seção genérica pela matéria sugerida. Questão sem
// sugestão, ou cuja seção já é matéria, fica como está.
func (r *Rascunho) AplicarMaterias(materias map[int]string) {
	aplicaveis := r.MateriasAplicaveis(materias)
	for j := range r.Questoes {
		if m, ok := aplicaveis[r.Questoes[j].Numero]; ok {
			r.Questoes[j].Disciplina = m
		}
	}
}

// HerdarDisciplinas dá à questão sem disciplina a da anterior. No caderno, o
// título da seção vale até o próximo título, mas cada região só vê o título que
// cai dentro dela — sem isto, a maioria das questões chegava sem disciplina e
// sumia do filtro. É sugestão: o curador confere, como todo o resto.
//
// Espera as questões em ordem (ver OrdenarQuestoes).
func (r *Rascunho) HerdarDisciplinas() {
	anterior := ""
	for j := range r.Questoes {
		q := &r.Questoes[j]
		if q.Disciplina == "" {
			q.Disciplina = anterior
		}
		anterior = q.Disciplina
	}
}

// Mesclar incorpora o resultado de uma região.
//
// As regiões se sobrepõem, então a mesma questão costuma chegar duas vezes. A
// versão completa vence o fragmento; dois fragmentos se juntam para o curador
// separar; e duas versões completas que não batem viram alerta, porque uma delas
// foi mal lida e só o original diz qual. Nada é descartado em silêncio.
func (r *Rascunho) Mesclar(n Rascunho) {
	r.Extracoes = append(r.Extracoes, n.Extracoes...)
	r.Alertas = append(r.Alertas, n.Alertas...)
	r.Apoios = append(r.Apoios, n.Apoios...)

	for _, q := range n.Questoes {
		j := slices.IndexFunc(r.Questoes, func(o Questao) bool { return o.Numero == q.Numero })
		if j < 0 {
			r.Questoes = append(r.Questoes, q)
			continue
		}

		// As origens são onde a questão aparece no caderno, e a revisão mostra
		// cada uma: só pedaços se somam. Uma leitura inteira já cobre a questão, e
		// somar a segunda mostraria a mesma questão duas vezes.
		atual := &r.Questoes[j]
		switch {
		// Inteira vence a que não é: o pedaço cortado e também a entrada que o
		// modelo marca como completa sem trazer alternativa nenhuma.
		case inteira(q) && !inteira(*atual), !atual.Completa && q.Completa, vazia(*atual):
			q.Apoios = unir(atual.Apoios, q.Apoios)
			*atual = q
		case !atual.Completa && !q.Completa:
			atual.Blocos = append(atual.Blocos, q.Blocos...)
			atual.Alternativas = append(atual.Alternativas, q.Alternativas...)
			atual.Origens = unirOrigens(atual.Origens, q.Origens)
			atual.Apoios = unir(atual.Apoios, q.Apoios)
		default:
			atual.Apoios = unir(atual.Apoios, q.Apoios)
			if q.Completa && textoDaQuestao(*atual) != textoDaQuestao(q) {
				r.Alertas = append(r.Alertas, fmt.Sprintf(
					"Questão %d foi lida duas vezes com conteúdo diferente (regiões %s); "+
						"a primeira leitura foi mantida — confira com o original.",
					q.Numero, strings.Join(regioesDe(append(slices.Clone(atual.Origens), q.Origens...)), " e "),
				))
			}
		}
	}

	r.AcertarApoios()
}

// AcertarApoios deriva os textos de cada questão da lista de questões de cada
// texto de apoio — a que o curador vê e edita na etapa de textos. A lista da
// questão só repete a do texto; o que existia só nela apontava para nada: o id
// do texto que uma releitura leu de novo e foi descartado, ou de um removido.
func (r *Rascunho) AcertarApoios() {
	for j := range r.Questoes {
		var ids []string
		for _, a := range r.Apoios {
			if slices.Contains(a.Questoes, r.Questoes[j].Numero) {
				ids = append(ids, a.ID)
			}
		}
		r.Questoes[j].Apoios = ids
	}
}

func unir(a, b []string) []string {
	for _, s := range b {
		if !slices.Contains(a, s) {
			a = append(a, s)
		}
	}

	return a
}

func unirOrigens(a, b []Origem) []Origem {
	for _, o := range b {
		if !slices.ContainsFunc(a, func(x Origem) bool { return x.Regiao == o.Regiao && x.Pagina == o.Pagina }) {
			a = append(a, o)
		}
	}

	return a
}

func regioesDe(os []Origem) []string {
	out := make([]string, 0, len(os))
	for _, o := range os {
		out = append(out, o.Regiao)
	}

	return out
}

// textoDaQuestao reduz a questão a letras e dígitos: duas leituras do mesmo
// trecho diferem em espaço e pontuação sem que o conteúdo mude.
func textoDaQuestao(q Questao) string {
	var sb strings.Builder
	escrever := func(bs []Bloco) {
		for _, b := range bs {
			for _, c := range strings.ToLower(b.Texto) {
				if unicode.IsLetter(c) || unicode.IsDigit(c) {
					sb.WriteRune(c)
				}
			}
		}
	}
	escrever(q.Blocos)
	for _, a := range q.Alternativas {
		sb.WriteString(a.Letra)
		escrever(a.Blocos)
	}

	return sb.String()
}

// normalizarCaderno compara só os dígitos, sem zeros à esquerda: a capa diz
// "TIPO-004" ou "004", o gabarito diz "4", e os três são o mesmo caderno.
func normalizarCaderno(s string) string {
	digitos := strings.Map(func(c rune) rune {
		if unicode.IsDigit(c) {
			return c
		}
		return -1
	}, s)
	if digitos == "" {
		return strings.ToUpper(strings.TrimSpace(s))
	}

	return strings.TrimLeft(digitos, "0")
}

func normalizarCargo(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// mesmoCargo diz se o cargo da prova é o do gabarito. O gabarito traz só o
// código; a prova pode trazê-lo junto do nome ("F06 - Analista…"), e basta ele
// aparecer como palavra.
func mesmoCargo(daProva, doGabarito string) bool {
	p, g := normalizarCargo(daProva), normalizarCargo(doGabarito)
	if p == g {
		return true
	}
	palavras := strings.FieldsFunc(p, func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) })

	return slices.Contains(palavras, g)
}

// citaTexto: o enunciado fala de um texto ("De acordo com o texto", "no
// texto"). "Editor de texto" e "arquivo texto" não contam.
var citaTexto = regexp.MustCompile(`(?i)\b(?:o|no|do|ao|pelo)\s+texto\b`)

// ConfirmarSemProblema dá como conferida a questão em que não sobrou nada para
// o curador ver: inteira, com as cinco alternativas preenchidas, a resposta do
// gabarito, os textos que ela usa ligados, e nenhum alerta da extração falando
// dela. O curador revisa só as outras — antes, conferia sessenta para achar as
// três com defeito.
//
// Questão com figura só depois que o curador conferiu cada recorte: o recorte
// sai do retângulo que a IA apontou, e só olhando o original se sabe se pegou a
// figura inteira, sem a vizinha. Conferidos os recortes, a figura era o que
// faltava — a questão está conferida.
func (r *Rascunho) ConfirmarSemProblema() {
	apoios := map[string]bool{}
	for _, a := range r.Apoios {
		apoios[a.ID] = true
	}
	for i := range r.Questoes {
		q := &r.Questoes[i]
		if !q.Revisada && figurasConferidas(*q) && r.semProblema(*q, apoios) {
			q.Revisada = true
		}
	}
}

// ConfirmarPelosRecortes dá como conferida a questão com figura em que o
// curador acabou de conferir o último recorte, se nada mais falta nela: para
// ele, conferir a figura era conferir a questão. Só na gravação em que isso
// acontece — desmarcar a questão de propósito, depois, fica desmarcado.
func ConfirmarPelosRecortes(antigo Rascunho, novo *Rascunho) {
	apoios := map[string]bool{}
	for _, a := range novo.Apoios {
		apoios[a.ID] = true
	}
	for i := range novo.Questoes {
		q := &novo.Questoes[i]
		if q.Revisada || !temFigura(*q) || !figurasConferidas(*q) {
			continue
		}
		k := slices.IndexFunc(antigo.Questoes, func(o Questao) bool { return o.Numero == q.Numero })
		if k >= 0 && figurasConferidas(antigo.Questoes[k]) {
			continue
		}
		if novo.semProblema(*q, apoios) {
			q.Revisada = true
		}
	}
}

// figurasConferidas: a questão não tem figura, ou o curador conferiu todas.
func figurasConferidas(q Questao) bool {
	conferida := func(b Bloco) bool { return b.Tipo != "imagem" || b.Revisado }
	if !todosOsBlocos(q.Blocos, conferida) {
		return false
	}
	for _, a := range q.Alternativas {
		if !todosOsBlocos(a.Blocos, conferida) {
			return false
		}
	}

	return true
}

func todosOsBlocos(bs []Bloco, ok func(Bloco) bool) bool {
	return !slices.ContainsFunc(bs, func(b Bloco) bool { return !ok(b) })
}

// temFigura diz se a questão tem imagem no enunciado ou numa alternativa.
func temFigura(q Questao) bool {
	eFigura := func(b Bloco) bool { return b.Tipo == "imagem" }
	if slices.ContainsFunc(q.Blocos, eFigura) {
		return true
	}

	return slices.ContainsFunc(q.Alternativas, func(a Alternativa) bool { return slices.ContainsFunc(a.Blocos, eFigura) })
}

func (r Rascunho) semProblema(q Questao, apoios map[string]bool) bool {
	if !inteira(q) || semConteudo(q.Blocos) {
		return false
	}
	letras := map[string]bool{}
	for _, a := range q.Alternativas {
		if semConteudo(a.Blocos) {
			return false
		}
		letras[a.Letra] = true
	}
	if len(letras) != 5 {
		return false
	}
	if oficial, ok := r.Gabarito.Respostas[strconv.Itoa(q.Numero)]; ok && oficial != q.Resposta {
		return false
	}
	for _, id := range q.Apoios {
		if !apoios[id] {
			return false
		}
	}
	var enunciado strings.Builder
	for _, b := range q.Blocos {
		enunciado.WriteString(b.Texto + " ")
	}
	if len(q.Apoios) == 0 && citaTexto.MatchString(enunciado.String()) {
		return false
	}
	// Alerta que cita a questão — duas leituras diferentes, alternativas
	// repetidas no enunciado — é o curador que resolve.
	falaDela := regexp.MustCompile(fmt.Sprintf(`(?i)\bquestão %d\b`, q.Numero))

	return !slices.ContainsFunc(r.Alertas, falaDela.MatchString)
}

// semConteudo: nenhum bloco com texto ou figura.
func semConteudo(bs []Bloco) bool {
	return !slices.ContainsFunc(bs, func(b Bloco) bool { return b.Tipo == "imagem" || strings.TrimSpace(b.Texto) != "" })
}

// Pendencias lista o que impede a publicação. Vazia, o rascunho pode ir para
// o catálogo.
//
// exigirConferencia decide se a falta da conferência do curador — na questão,
// no material de apoio e no recorte — bloqueia. Sem ela, só a integridade
// conta: numeração, alternativas, figura recortada, gabarito do mesmo caderno.
//
// O caderno do gabarito é comparado pelos dígitos (ver normalizarCaderno).
func (r Rascunho) Pendencias(exigirConferencia bool) []string {
	out := []string{}
	if r.Banca != "FCC" || r.Orgao == "" || r.Ano < 1900 || r.Cargo == "" || r.Caderno == "" {
		out = append(out, "Confira banca FCC, órgão, ano, cargo e caderno.")
	}
	if r.Total <= 0 || len(r.Questoes) != r.QuestoesNaProva() {
		out = append(out, fmt.Sprintf(
			"O rascunho tem %d questões e o total esperado é %d.%s", len(r.Questoes), r.QuestoesNaProva(), r.faltando(),
		))
	}
	// A exclusão vale enquanto o gabarito anular a questão: trocado por um que
	// dá a resposta, ela volta a ser esperada.
	excluidas := map[int]bool{}
	for _, n := range r.AnuladasExcluidas {
		resposta, noGabarito := r.Gabarito.Respostas[strconv.Itoa(n)]
		if excluidas[n] || n < 1 || n > r.Total {
			out = append(out, fmt.Sprintf("Numeração inválida entre as anuladas excluídas: %d.", n))
		} else if !noGabarito || resposta != "" {
			out = append(out, fmt.Sprintf(
				"A questão %d foi excluída como anulada, mas o gabarito não a anula; devolva-a à prova.", n,
			))
		}
		excluidas[n] = true
	}
	if g := r.Gabarito.Cargo; g != "" && !mesmoCargo(r.Cargo, g) {
		// A mensagem diz o que conferir: o código está na capa, em "Caderno de
		// Prova 'F06'", e a leitura às vezes traz o nome no lugar dele.
		prova := "está sem o código do cargo"
		if r.Cargo != "" {
			prova = fmt.Sprintf("está com o cargo %q", r.Cargo)
		}
		out = append(out, fmt.Sprintf(
			"O gabarito é do cargo %s e a prova %s. Se a capa diz \"Caderno de Prova '%s'\", "+
				"use esse código na etapa Dados; se não, o gabarito é de outro cargo.", g, prova, g,
		))
	}
	if r.Gabarito.Caderno != "" && normalizarCaderno(r.Gabarito.Caderno) != normalizarCaderno(r.Caderno) {
		out = append(out, "O tipo de gabarito não corresponde ao caderno da prova.")
	}
	if len(r.Gabarito.Respostas) > 0 &&
		(r.Gabarito.Cargo == "" || r.Gabarito.Caderno == "" || len(r.Gabarito.Respostas) != r.Total) {
		out = append(out, "Confira a identificação e a quantidade de respostas do gabarito.")
	}
	for _, chave := range slices.Sorted(maps.Keys(r.Gabarito.Respostas)) {
		letra := r.Gabarito.Respostas[chave]
		if n, err := strconv.Atoi(chave); err != nil || n < 1 || !slices.Contains(letrasDoGabarito, letra) {
			out = append(out, fmt.Sprintf("O gabarito tem resposta inválida: %q na questão %q.", letra, chave))
		}
	}

	apoios := map[string]bool{}
	for _, a := range r.Apoios {
		apoios[a.ID] = true
		local := a.rotulo()
		// Texto que nenhuma questão usa não aparece para o aluno: ou falta
		// ligar, ou é lixo da extração.
		if len(a.Questoes) == 0 {
			out = append(out, "Ligue às questões ou remova o "+local+".")
		} else if exigirConferencia && !a.Revisado {
			out = append(out, "Confira o "+local+".")
		}
		out = append(out, pendenciasDosBlocos(a.Blocos, local, exigirConferencia)...)
	}

	numeros := map[int]bool{}
	for _, q := range r.Questoes {
		local := fmt.Sprintf("questão %d", q.Numero)
		if numeros[q.Numero] || q.Numero < 1 || q.Numero > r.Total {
			out = append(out, fmt.Sprintf("Numeração inválida: %d.", q.Numero))
		}
		numeros[q.Numero] = true
		if excluidas[q.Numero] {
			out = append(out, fmt.Sprintf(
				"A questão %d está no rascunho e entre as anuladas excluídas; remova-a ou devolva-a à prova.", q.Numero,
			))
		}

		if !q.Completa || len(q.Blocos) == 0 || len(q.Alternativas) != 5 {
			out = append(out, fmt.Sprintf("A %s está incompleta.", local))
		}
		if exigirConferencia && !q.Revisada {
			out = append(out, fmt.Sprintf("A %s não foi conferida.", local))
		}

		letras := map[string]bool{}
		for _, a := range q.Alternativas {
			if len(a.Letra) != 1 || !strings.Contains("ABCDE", a.Letra) || letras[a.Letra] || len(a.Blocos) == 0 {
				out = append(out, fmt.Sprintf("Alternativas inválidas na %s.", local))
			}
			letras[a.Letra] = true
			out = append(out, pendenciasDosBlocos(a.Blocos, local, exigirConferencia)...)
		}
		for _, id := range q.Apoios {
			if !apoios[id] {
				out = append(out, fmt.Sprintf("A %s aponta um texto de apoio que não existe mais (%s); ligue de novo na etapa Textos de apoio.", local, id))
			}
		}
		if q.Resposta != "" && !letras[q.Resposta] {
			out = append(out, fmt.Sprintf("Resposta inválida na %s.", local))
		}
		if len(r.Gabarito.Respostas) > 0 {
			oficial, ok := r.Gabarito.Respostas[strconv.Itoa(q.Numero)]
			if !ok || q.Resposta != oficial {
				out = append(out, fmt.Sprintf(
					"A resposta da %s difere do gabarito; corrija a transcrição oficial.", local,
				))
			}
		}
		out = append(out, pendenciasDosBlocos(q.Blocos, local, exigirConferencia)...)
	}

	return out
}

// LimparBlocos tira o que não carrega nada: bloco de texto vazio, código em
// branco, e espaço sozinho que não fica entre dois trechos de texto. O espaço
// entre dois trechos — o que separa duas palavras em itálico — fica: tirá-lo
// grudava as palavras. Esses blocos vêm da extração, e o curador, que edita
// texto e não blocos, não tinha como vê-los: viravam pendência sem saída.
func (r *Rascunho) LimparBlocos() {
	for i := range r.Questoes {
		q := &r.Questoes[i]
		q.Blocos = limparBlocos(q.Blocos)
		for j := range q.Alternativas {
			q.Alternativas[j].Blocos = limparBlocos(q.Alternativas[j].Blocos)
		}
	}
	for i := range r.Apoios {
		r.Apoios[i].Blocos = limparBlocos(r.Apoios[i].Blocos)
	}
}

func limparBlocos(bs []Bloco) []Bloco {
	cheios := make([]Bloco, 0, len(bs))
	for _, b := range bs {
		if b.Tipo != "imagem" && (b.Texto == "" || (b.Tipo == "codigo" && strings.TrimSpace(b.Texto) == "")) {
			continue
		}
		cheios = append(cheios, b)
	}
	eTexto := func(k int) bool { return k >= 0 && k < len(cheios) && cheios[k].Tipo == "texto" }
	out := make([]Bloco, 0, len(cheios))
	for k, b := range cheios {
		soEspaco := b.Tipo == "texto" && strings.TrimSpace(b.Texto) == ""
		if soEspaco && !(eTexto(k-1) && eTexto(k+1)) {
			continue
		}
		out = append(out, b)
	}

	return out
}

func pendenciasDosBlocos(bs []Bloco, local string, exigirConferencia bool) []string {
	out := []string{}
	for _, b := range bs {
		switch b.Tipo {
		case "texto", "codigo":
			// Espaço entre dois trechos formatados é texto: separa as palavras.
			if b.Texto == "" || (b.Tipo == "codigo" && strings.TrimSpace(b.Texto) == "") {
				out = append(out, "Bloco de texto vazio na "+local+".")
			}
		case "imagem":
			// Sem posição no PDF desta prova é a figura reaproveitada de outra:
			// o recorte existe, e é o que o aluno vê.
			if b.Arquivo == "" {
				out = append(out, "Imagem sem recorte na "+local+".")
			} else if exigirConferencia && !b.Revisado {
				out = append(out, "Recorte não conferido na "+local+".")
			}
			if b.Largura < 0 || b.Largura > 100 {
				out = append(out, "Tamanho de figura fora de 0 a 100% na "+local+".")
			}
		default:
			out = append(out, "Tipo de bloco inválido na "+local+".")
		}
	}

	return out
}

// InvalidarEdicoes desfaz a conferência de tudo o que mudou nesta gravação.
//
// Conferir e alterar na mesma gravação não vale: o curador confirma o que viu
// salvo, não o que acabou de digitar. Por isso a questão alterada perde a marca,
// e um recorte novo também — mesmo que tenha chegado marcado.
func InvalidarEdicoes(antigo Rascunho, novo *Rascunho) {
	existentes := map[string]bool{}
	for _, id := range antigo.Arquivos() {
		existentes[id] = true
	}
	desmarcarRecortesNovos := func(bs []Bloco) {
		for i := range bs {
			if bs[i].Tipo == "imagem" && !existentes[bs[i].Arquivo] {
				bs[i].Revisado = false
			}
		}
	}

	for j := range novo.Questoes {
		q := &novo.Questoes[j]
		desmarcarRecortesNovos(q.Blocos)
		for _, a := range q.Alternativas {
			desmarcarRecortesNovos(a.Blocos)
		}
		k := slices.IndexFunc(antigo.Questoes, func(o Questao) bool { return o.Numero == q.Numero })
		if k < 0 || !reflect.DeepEqual(semMarcasQuestao(antigo.Questoes[k]), semMarcasQuestao(*q)) {
			q.Revisada = false
		}
		// Editada, a reaproveitada deixou de ser a publicada lá.
		if q.IgualA != "" && k >= 0 {
			ca, _ := antigo.Questoes[k].Separar()
			cn, _ := q.Separar()
			if ca.Impressao() != cn.Impressao() {
				q.IgualA = ""
			}
		}
	}

	for j := range novo.Apoios {
		a := &novo.Apoios[j]
		desmarcarRecortesNovos(a.Blocos)
		k := slices.IndexFunc(antigo.Apoios, func(o Apoio) bool { return o.ID == a.ID })
		if k < 0 || !reflect.DeepEqual(semMarcasApoio(antigo.Apoios[k]), semMarcasApoio(*a)) {
			a.Revisado = false
		}
		if a.IgualA != "" && k >= 0 {
			ca, _ := antigo.Apoios[k].Separar()
			cn, _ := a.Separar()
			if ca.Impressao() != cn.Impressao() {
				a.IgualA = ""
			}
		}
	}
}

// semMarcas* devolvem uma cópia sem as marcas de conferência e com lista vazia
// igual a lista ausente: o que chega do navegador e o que estava gravado passam
// por caminhos diferentes, e essa diferença não é edição.
func semMarcasQuestao(q Questao) Questao {
	q.Revisada = false
	q.Blocos = semMarcasBlocos(q.Blocos)
	alternativas := make([]Alternativa, 0, len(q.Alternativas))
	for _, a := range q.Alternativas {
		alternativas = append(alternativas, Alternativa{Letra: a.Letra, Blocos: semMarcasBlocos(a.Blocos)})
	}
	q.Alternativas = nilSeVazia(alternativas)
	q.Apoios = nilSeVazia(q.Apoios)
	q.Origens = nilSeVazia(q.Origens)

	return q
}

func semMarcasApoio(a Apoio) Apoio {
	a.Revisado = false
	a.Blocos = semMarcasBlocos(a.Blocos)
	a.Questoes = nilSeVazia(a.Questoes)

	return a
}

func semMarcasBlocos(bs []Bloco) []Bloco {
	if len(bs) == 0 {
		return nil
	}
	out := make([]Bloco, len(bs))
	for i, b := range bs {
		// O tamanho na tela é apresentação: mudá-lo não é editar o conteúdo.
		b.Revisado = false
		b.Largura = 0
		out[i] = b
	}

	return out
}

func nilSeVazia[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}

	return s
}
