package plano

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"studygo/internal/domain/concurso"

	"github.com/google/uuid"
)

// A planilha do plano, de volta para dentro.
//
// O CSV que o app exporta é a cópia de segurança do estudante — e é também o
// caminho por onde ele traz o histórico de outra instalação. Ler de volta o
// PRÓPRIO formato é o que fecha esse ciclo: o que o export escreve, o import
// entende.
//
// Só os REGISTROS entram: tempo, questões, acertos e conclusão. O cronograma em
// si (que dia ensina o quê) é decisão do motor e do estudante nesta instalação,
// e uma planilha não o reescreve — ela só diz o que foi estudado, e em qual
// matéria de qual dia.

// ErrPlanilhaIlegivel marca um arquivo que não dá para ler como planilha do
// plano: vazio, corrompido, ou sem as colunas mínimas.
var ErrPlanilhaIlegivel = errors.New("planilha ilegível")

// LinhaPlanilha é uma linha do CSV depois de os cabeçalhos serem resolvidos.
//
// Numero é a linha no arquivo, contada a partir de 1 como o Excel a mostra: uma
// linha recusada precisa dizer ONDE está o problema.
type LinhaPlanilha struct {
	Numero     int
	Data       time.Time
	Codigo     string
	Disciplina string
	Tema       string
	Minutos    *int
	Questoes   *int
	Acertos    *int
	Concluido  *bool
}

// LinhaCasada é uma linha que encontrou onde entrar.
//
// Criada diz que a atividade não existia e foi reconstruída a partir da própria
// linha: é o caso de quem traz o histórico de outra instalação, onde o
// cronograma daqueles dias foi outro.
type LinhaCasada struct {
	Linha    LinhaPlanilha
	Registro RegistroAtividade
	Criada   bool
}

// LinhaRecusada é uma linha que não encontrou onde entrar, com o motivo em
// português — a tela mostra isso ao lado do número da linha.
type LinhaRecusada struct {
	Linha  LinhaPlanilha
	Motivo string
}

// ResultadoPlanilha é o que a importação faria, antes de fazer.
//
// Novas são as atividades que precisam existir para as linhas Criadas terem
// onde se apoiar — um registro é sempre de UMA atividade, e sem ela não há onde
// gravar o que a planilha diz que aconteceu.
type ResultadoPlanilha struct {
	Casadas   []LinhaCasada
	Recusadas []LinhaRecusada
	Novas     []Atividade
}

// JanelaDaPlanilha é o trecho do calendário em que a importação pode
// reconstruir história.
//
// Começa no início do plano porque um dia anterior a ele não existe no
// cronograma: a atividade criada lá não apareceria em tela nenhuma. Termina
// hoje porque o futuro é do motor — uma planilha não agenda o que ainda não
// aconteceu.
type JanelaDaPlanilha struct {
	Inicio time.Time
	Hoje   time.Time
}

// colunas mapeia cada campo aos cabeçalhos aceitos. O primeiro é o que o export
// escreve; os outros existem para a planilha montada à mão.
var colunas = map[string][]string{
	"data":       {"data", "dia_data", "date"},
	"codigo":     {"codigo", "código", "sigla", "tag"},
	"disciplina": {"disciplina", "materia", "matéria"},
	"tema":       {"tema", "assunto", "topico", "tópico"},
	"minutos":    {"minutos", "min", "tempo"},
	"horas":      {"horas", "hora", "h"},
	"questoes":   {"questoes", "questões", "resolvidas"},
	"acertos":    {"acertos", "certas", "corretas"},
	"concluido":  {"concluido", "concluído", "feito"},
}

// LerPlanilha lê o CSV do plano e devolve as linhas que trazem registro.
//
// Ignora o que não é registro: a segunda tabela do export (o caderno de erros)
// e as linhas de cronograma sem nada lançado. Uma planilha que não traz nenhum
// registro é um erro — o usuário mandou o arquivo errado, e dizer isso é melhor
// que importar zero linhas em silêncio.
func LerPlanilha(r io.Reader) ([]LinhaPlanilha, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("%w: não consegui ler o arquivo: %s", ErrPlanilhaIlegivel, err)
	}

	texto := strings.TrimPrefix(string(buf), "\ufeff")

	leitor := csv.NewReader(strings.NewReader(texto))
	// O export tem duas tabelas com larguras diferentes, separadas por uma linha
	// em branco; sem isto o leitor recusaria o arquivo inteiro.
	leitor.FieldsPerRecord = -1
	leitor.LazyQuotes = true
	leitor.Comma = separadorDe(texto)

	registros, err := leitor.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: não consegui ler o CSV: %s", ErrPlanilhaIlegivel, err)
	}

	inicio, idx := acharCabecalho(registros)
	if idx == nil {
		return nil, fmt.Errorf(
			"%w: não achei as colunas de data e disciplina — exporte o CSV do plano "+
				"e importe esse arquivo, ou monte uma planilha com esse cabeçalho",
			ErrPlanilhaIlegivel,
		)
	}

	out := []LinhaPlanilha{}

	for i := inicio + 1; i < len(registros); i++ {
		rec := registros[i]

		// A linha em branco encerra a tabela de cronograma: o que vem depois é o
		// caderno de erros, que a importação não toca.
		if vazia(rec) {
			break
		}

		l, ok := linhaDaPlanilha(rec, idx, i+1)
		if !ok {
			continue
		}

		out = append(out, l)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf(
			"%w: não achei nenhuma linha com tempo, questões ou conclusão nesta planilha",
			ErrPlanilhaIlegivel,
		)
	}

	return out, nil
}

// CasarPlanilha diz onde cada linha entraria, sem gravar nada.
//
// O casamento é por DIA e MATÉRIA, que é como o estudante lê a planilha. O tema
// desempata quando o dia agenda a mesma matéria duas vezes; sem tema, a ordem
// do dia resolve. Cada atividade recebe no máximo uma linha: duas linhas para a
// mesma vaga seriam duas verdades sobre o mesmo estudo.
func CasarPlanilha(
	atividades []Atividade,
	dias []Dia,
	linhas []LinhaPlanilha,
	cur concurso.Concurso,
	janela JanelaDaPlanilha,
) ResultadoPlanilha {
	out := ResultadoPlanilha{
		Casadas:   []LinhaCasada{},
		Recusadas: []LinhaRecusada{},
		Novas:     []Atividade{},
	}
	usadas := map[uuid.UUID]bool{}
	// Quantas atividades cada dia já tem, contando as que esta importação criou:
	// é daí que sai a posição da próxima.
	ocupacao := map[time.Time]int{}

	for _, a := range atividades {
		ocupacao[day(a.Data)]++
	}

	for _, l := range linhas {
		alvo, ok := escolher(daMateria(AtividadesDoDia(atividades, l.Data), l, cur), l, usadas)
		if ok {
			usadas[alvo.ID] = true
			out.Casadas = append(out.Casadas, LinhaCasada{
				Linha:    l,
				Registro: registroDaLinha(alvo.ID, l),
			})

			continue
		}

		// Nada agendado ali para esta matéria. Se o dia já passou e está dentro do
		// plano, a planilha é a fonte: a atividade é reconstruída para o registro
		// ter onde se apoiar.
		nova, motivo := reconstruir(l, dias, cur, janela, ocupacao)
		if motivo != "" {
			out.Recusadas = append(out.Recusadas, LinhaRecusada{Linha: l, Motivo: motivo})

			continue
		}

		ocupacao[day(l.Data)]++
		usadas[nova.ID] = true
		out.Novas = append(out.Novas, nova)
		out.Casadas = append(out.Casadas, LinhaCasada{
			Linha:    l,
			Registro: registroDaLinha(nova.ID, l),
			Criada:   true,
		})
	}

	return out
}

// reconstruir monta a atividade que a linha descreve, ou diz por que não dá.
//
// O motivo é a mensagem que a tela mostra ao lado do número da linha, então ele
// tem de dizer o que fazer em seguida — não só que deu errado.
func reconstruir(
	l LinhaPlanilha,
	dias []Dia,
	cur concurso.Concurso,
	janela JanelaDaPlanilha,
	ocupacao map[time.Time]int,
) (Atividade, string) {
	data := day(l.Data)

	if !janela.Inicio.IsZero() && data.Before(janela.Inicio) {
		return Atividade{}, "a linha é de " + data.Format("02/01/2006") +
			" e o plano começa em " + janela.Inicio.Format("02/01/2006") +
			" — mude a data de início do plano em Ajustes para trazer esse período"
	}

	if !janela.Hoje.IsZero() && data.After(janela.Hoje) {
		return Atividade{}, data.Format("02/01/2006") +
			" ainda não chegou — a planilha traz o que já foi estudado"
	}

	// Um dia que o plano não estuda não aparece no cronograma, e a atividade
	// criada ali seria invisível: existiria no banco e em tela nenhuma.
	if !DestinoValido(dias, data) {
		return Atividade{}, data.Format("02/01/2006") +
			" não é um dia de estudo deste plano — ajuste os dias da semana em Ajustes"
	}

	d := disciplinaDaLinha(l, cur)
	if d == nil {
		return Atividade{}, materiaDaLinha(l) + " não é uma matéria deste concurso"
	}

	id := d.ID

	return Atividade{
		ID:           uuid.New(),
		Data:         data,
		Posicao:      ocupacao[data],
		DisciplinaID: &id,
		Disciplina:   d.Codigo,
		Tema:         l.Tema,
		Passada:      1,
		Tipo:         AtividadeConteudo,
		// Foi o estudante quem pôs isto aqui, ao dizer que estudou naquele dia.
		Movida: true,
	}, ""
}

// HorasDeMinutos converte o tempo digitado em minutos para as horas que o
// registro guarda, com as duas casas que a coluna do banco comporta.
func HorasDeMinutos(minutos int) float64 {
	return math.Round(float64(minutos)/60*100) / 100
}

// MinutosDeHoras é o caminho de volta, para a planilha sair na mesma unidade em
// que o estudo é lançado.
func MinutosDeHoras(horas float64) int {
	return int(math.Round(horas * 60))
}

func registroDaLinha(id uuid.UUID, l LinhaPlanilha) RegistroAtividade {
	reg := RegistroAtividade{
		AtividadeID: id,
		Questoes:    l.Questoes,
		Acertos:     AcertosValidos(l.Questoes, l.Acertos),
	}

	if l.Minutos != nil {
		h := HorasDeMinutos(*l.Minutos)
		reg.Horas = &h
	}

	// Sem a coluna, o que tem tempo ou questão foi estudado: é o que a linha
	// está afirmando ao existir.
	if l.Concluido != nil {
		reg.Concluido = *l.Concluido
	} else {
		reg.Concluido = (l.Minutos != nil && *l.Minutos > 0) || (l.Questoes != nil && *l.Questoes > 0)
	}

	return reg
}

// daMateria filtra as atividades do dia que são da matéria da linha. Aceita o
// código e o nome, porque a planilha pode trazer qualquer um dos dois.
//
// Um dia fixo — simulado, discursiva, véspera, revisão — não tem matéria: a
// planilha traz o TIPO no lugar dela, que é o que o export escreve. Também
// pode ter sido estudado, então também casa.
func daMateria(doDia []Atividade, l LinhaPlanilha, cur concurso.Concurso) []Atividade {
	codigo := strings.TrimSpace(l.Codigo)
	if codigo == "" {
		codigo = codigoPorNome(cur, l.Disciplina)
	}

	out := make([]Atividade, 0, len(doDia))

	for _, a := range doDia {
		if a.Disciplina != "" {
			if codigo != "" && chave(a.Disciplina) == chave(codigo) {
				out = append(out, a)
			}

			continue
		}

		if codigo == "" && chave(string(a.Tipo)) == chave(l.Disciplina) {
			out = append(out, a)
		}
	}

	return out
}

// escolher pega a ocorrência ainda livre, preferindo a de mesmo tema: um dia que
// agenda a matéria duas vezes tem duas linhas, e cada uma vai para a sua.
func escolher(candidatas []Atividade, l LinhaPlanilha, usadas map[uuid.UUID]bool) (Atividade, bool) {
	tema := strings.TrimSpace(l.Tema)

	if tema != "" {
		for _, a := range candidatas {
			if !usadas[a.ID] && chave(a.Tema) == chave(tema) {
				return a, true
			}
		}
	}

	for _, a := range candidatas {
		if !usadas[a.ID] {
			return a, true
		}
	}

	return Atividade{}, false
}

// codigoPorNome acha a matéria pelo nome como ele veio na planilha — ou pelo
// próprio código, quando foi ele que a coluna trouxe.
func codigoPorNome(cur concurso.Concurso, nome string) string {
	k := chave(nome)
	if k == "" {
		return ""
	}

	for _, d := range cur.Disciplinas {
		if chave(d.Nome) == k || chave(d.Codigo) == k {
			return d.Codigo
		}
	}

	return ""
}

// disciplinaDaLinha resolve a matéria da linha pelo código ou pelo nome.
func disciplinaDaLinha(l LinhaPlanilha, cur concurso.Concurso) *concurso.Disciplina {
	codigo := strings.TrimSpace(l.Codigo)
	if codigo == "" {
		codigo = codigoPorNome(cur, l.Disciplina)
	}

	if codigo == "" {
		return nil
	}

	for i := range cur.Disciplinas {
		if chave(cur.Disciplinas[i].Codigo) == chave(codigo) {
			return &cur.Disciplinas[i]
		}
	}

	return nil
}

func materiaDaLinha(l LinhaPlanilha) string {
	if s := strings.TrimSpace(l.Disciplina); s != "" {
		return s
	}

	if s := strings.TrimSpace(l.Codigo); s != "" {
		return s
	}

	return "a matéria da linha"
}

// acharCabecalho procura a linha de cabeçalho da tabela de cronograma. Não é
// necessariamente a primeira: uma planilha editada à mão costuma ganhar um
// título em cima.
func acharCabecalho(registros [][]string) (int, map[string]int) {
	for i, rec := range registros {
		idx := mapearColunas(rec)
		if _, temData := idx["data"]; !temData {
			continue
		}

		_, temCodigo := idx["codigo"]
		_, temDisciplina := idx["disciplina"]

		if temCodigo || temDisciplina {
			return i, idx
		}
	}

	return 0, nil
}

func mapearColunas(cabecalho []string) map[string]int {
	idx := map[string]int{}

	for i, col := range cabecalho {
		chave := chave(col)

		for campo, apelidos := range colunas {
			if _, achado := idx[campo]; achado {
				continue
			}

			for _, a := range apelidos {
				if chave == a {
					idx[campo] = i

					break
				}
			}
		}
	}

	return idx
}

// chave reduz um texto ao que dá para comparar entre duas instalações:
// minúsculas, sem acento, sem pontuação e sem espaço.
//
// Serve aos cabeçalhos e, principalmente, ao NOME da matéria. Dois planos do
// mesmo edital escrevem o mesmo nome de formas que não são o mesmo texto: o
// acento pode vir composto (í) ou decomposto (i + ´) conforme o sistema que
// gerou o arquivo, a vírgula da enumeração aparece ou não, o hífen troca de
// lugar. Comparar byte a byte fazia "Língua Portuguesa" não ser "Língua
// Portuguesa", e a planilha inteira era recusada com a mensagem mais confusa
// possível: a matéria que está na tela "não é uma matéria deste concurso".
//
// Descartar o que não é letra nem dígito resolve os três casos de uma vez: o
// acento decomposto vira a letra base (a marca combinante cai), a pontuação
// some e o espaço não conta.
func chave(s string) string {
	var b strings.Builder

	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case strings.ContainsRune("áàâãä", r):
			b.WriteRune('a')
		case strings.ContainsRune("éèêë", r):
			b.WriteRune('e')
		case strings.ContainsRune("íìîï", r):
			b.WriteRune('i')
		case strings.ContainsRune("óòôõö", r):
			b.WriteRune('o')
		case strings.ContainsRune("úùûü", r):
			b.WriteRune('u')
		case r == 'ç':
			b.WriteRune('c')
		}
	}

	return b.String()
}

func linhaDaPlanilha(rec []string, idx map[string]int, numero int) (LinhaPlanilha, bool) {
	data, ok := dataDaPlanilha(campo(rec, idx, "data"))
	if !ok {
		return LinhaPlanilha{}, false
	}

	l := LinhaPlanilha{
		Numero:     numero,
		Data:       data,
		Codigo:     strings.TrimSpace(campo(rec, idx, "codigo")),
		Disciplina: strings.TrimSpace(campo(rec, idx, "disciplina")),
		Tema:       strings.TrimSpace(campo(rec, idx, "tema")),
		Questoes:   inteiroOuNil(campo(rec, idx, "questoes")),
		Acertos:    inteiroOuNil(campo(rec, idx, "acertos")),
		Concluido:  boolOuNil(campo(rec, idx, "concluido")),
	}

	// Minutos manda: é a unidade em que o estudo é lançado. Horas existe porque
	// é o que as planilhas antigas trazem.
	if m := inteiroOuNil(campo(rec, idx, "minutos")); m != nil {
		l.Minutos = m
	} else if h := floatOuNil(campo(rec, idx, "horas")); h != nil {
		m := MinutosDeHoras(*h)
		l.Minutos = &m
	}

	// Linha de cronograma sem nada lançado não é registro: importá-la gravaria
	// um registro vazio por cima do que já existe.
	//
	// "concluído: não" sozinho é ausência de estudo, não um lançamento — o
	// export escreve isso em TODA linha ainda não estudada, e tratá-las como
	// registro faria uma importação apagar o histórico do outro lado.
	if l.Minutos == nil && l.Questoes == nil && l.Acertos == nil &&
		(l.Concluido == nil || !*l.Concluido) {
		return LinhaPlanilha{}, false
	}

	return l, true
}

func campo(rec []string, idx map[string]int, nome string) string {
	i, ok := idx[nome]
	if !ok || i >= len(rec) {
		return ""
	}

	return rec[i]
}

// formatosData são os que a planilha pode trazer: o do export brasileiro e o
// ISO que o Excel às vezes grava.
var formatosData = []string{"02/01/2006", "2006-01-02", "02/01/06", "2/1/2006"}

func dataDaPlanilha(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}

	for _, f := range formatosData {
		if t, err := time.Parse(f, s); err == nil {
			return DayOf(t.UTC()), true
		}
	}

	return time.Time{}, false
}

func inteiroOuNil(s string) *int {
	f := floatOuNil(s)
	if f == nil {
		return nil
	}

	n := int(math.Round(*f))
	if n < 0 {
		return nil
	}

	return &n
}

func floatOuNil(s string) *float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return nil
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return nil
	}

	return &v
}

var afirmativos = map[string]bool{"sim": true, "s": true, "true": true, "1": true, "x": true, "ok": true}

var negativos = map[string]bool{"nao": true, "n": true, "false": true, "0": true, "": false}

func boolOuNil(s string) *bool {
	chave := chave(s)
	if chave == "" {
		return nil
	}

	if afirmativos[chave] {
		v := true

		return &v
	}

	if _, ok := negativos[chave]; ok {
		v := false

		return &v
	}

	return nil
}

func vazia(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}

	return true
}

// separadorDe descobre se a planilha usa vírgula ou ponto e vírgula. O Excel em
// português salva com ponto e vírgula, e é dele que vem a maioria dos arquivos.
func separadorDe(texto string) rune {
	linha := texto
	if i := strings.IndexByte(texto, '\n'); i >= 0 {
		linha = texto[:i]
	}

	if strings.Count(linha, ";") > strings.Count(linha, ",") {
		return ';'
	}

	return ','
}
