package prova

import (
	"html"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// QuestaoAvulsa é uma questão do catálogo vista fora da prova dela: o bastante
// para o estudante escolher o que treinar e saber, sem abrir a prova, se já
// acertou. O conteúdo continua na prova, que a tela carrega na vez da questão.
type QuestaoAvulsa struct {
	ProvaID    string
	Numero     int
	Disciplina string
	Assunto    string
	// Grupo junta as matérias no treino: básicas, legislação ou específicas.
	Grupo string
	// TemGabarito: a questão tem linha no gabarito da prova (a anulada tem,
	// sem letra). Sem ela, a questão não vai ao treino.
	TemGabarito bool
	Resposta    string
	// A identificação da prova de onde ela vem.
	Orgao     string
	Ano       int
	Cargo     string
	CargoNome string
}

// FiltroDeAvulsas é o que o estudante quer treinar. Campo vazio não filtra.
type FiltroDeAvulsas struct {
	Materias []string
	Ano      int
}

// As palavras que não distinguem uma matéria de outra: cada banca põe as suas
// ("Noções de Direito Administrativo" é Direito Administrativo).
var palavrasVazias = map[string]bool{
	"a": true, "as": true, "o": true, "os": true, "e": true, "de": true, "da": true,
	"do": true, "das": true, "dos": true, "em": true, "no": true, "na": true,
	"sobre": true, "nocao": true, "nocoes": true, "conhecimento": true,
	"conhecimentos": true, "geral": true, "gerais": true, "basico": true,
	"basicos": true, "aplicado": true, "aplicada": true,
}

// chaveDaMateria é o que faz duas grafias serem a mesma matéria. A extração e
// o curador escrevem "Noções Sobre Direitos…" numa prova, "Direitos…" noutra e
// "Seguran&ccedil;a da Informa&ccedil&atilde;o" numa terceira; "Matemática e
// Raciocínio Lógico" e "Raciocínio Lógico-Matemático" são a mesma matéria com
// as palavras noutra ordem. A chave é o conjunto das palavras que importam,
// sem acento, sem plural e sem o fim que muda com o gênero.
func chaveDaMateria(nome string) string {
	palavras := []string{}
	for _, palavra := range strings.FieldsFunc(
		semAcento.Replace(strings.ToLower(NomeDaMateria(nome))),
		func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) },
	) {
		if palavrasVazias[palavra] {
			continue
		}
		palavras = append(palavras, radical(palavra))
	}
	slices.Sort(palavras)

	chave := strings.Join(slices.Compact(palavras), " ")
	// As bancas usam estes dois nomes para a mesma matéria do treino.
	if chave == "logic raciocini" {
		return "logic matematic raciocini"
	}
	return chave
}

// radical tira o plural e a vogal final, que muda com o gênero: "lógico" e
// "lógica", "sistemas operacionais" e "sistema operacional", "informações" e
// "informação" são a mesma palavra para achar a matéria.
func radical(palavra string) string {
	r := palavra
	switch {
	case strings.HasSuffix(r, "ais"): // operacionais → operacional
		r = strings.TrimSuffix(r, "ais") + "al"
	case strings.HasSuffix(r, "eis"):
		r = strings.TrimSuffix(r, "eis") + "el"
	case strings.HasSuffix(r, "ois"):
		r = strings.TrimSuffix(r, "ois") + "ol"
	case strings.HasSuffix(r, "oes"), strings.HasSuffix(r, "aes"):
		r = r[:len(r)-3] + "ao" // informações → informação
	case strings.HasSuffix(r, "s"):
		r = strings.TrimSuffix(r, "s")
	}
	if len(r) > 3 && strings.ContainsRune("aeo", rune(r[len(r)-1])) {
		r = r[:len(r)-1]
	}

	return r
}

// NomeDaMateria é o nome como se escreve: sem espaço a mais e com as letras
// que o HTML da página da banca deixou escapar ("Governan&ccedil;a de TI").
func NomeDaMateria(nome string) string {
	return strings.Join(strings.Fields(norm.NFC.String(html.UnescapeString(nome))), " ")
}

// Avulsas prepara as questões para o treino por matéria: descarta as que não
// têm matéria nem gabarito, dá um nome só a cada matéria e aplica o filtro.
//
// O nome escolhido é a grafia mais usada no catálogo inteiro — antes do filtro,
// para não mudar conforme o que se pediu. No empate fica a primeira em ordem
// alfabética, pelo mesmo motivo.
func Avulsas(qs []QuestaoAvulsa, f FiltroDeAvulsas) []QuestaoAvulsa {
	usos := map[string]map[string]int{}
	for _, q := range qs {
		k := chaveDaMateria(q.Disciplina)
		if k == "" {
			continue
		}
		if usos[k] == nil {
			usos[k] = map[string]int{}
		}
		usos[k][NomeDaMateria(q.Disciplina)]++
	}

	nome := make(map[string]string, len(usos))
	for k, grafias := range usos {
		melhor := ""
		for g, n := range grafias {
			if melhor == "" || n > grafias[melhor] || (n == grafias[melhor] && g < melhor) {
				melhor = g
			}
		}
		nome[k] = melhor
	}

	pedidas := make([]string, 0, len(f.Materias))
	for _, m := range f.Materias {
		if k := chaveDaMateria(m); k != "" {
			pedidas = append(pedidas, k)
		}
	}

	out := make([]QuestaoAvulsa, 0, len(qs))
	for _, q := range qs {
		k := chaveDaMateria(q.Disciplina)
		// Sem gabarito, a questão não é treinável: o aluno responderia sem ter
		// como conferir.
		if k == "" || !q.TemGabarito ||
			(f.Ano != 0 && q.Ano != f.Ano) || (len(pedidas) > 0 && !slices.Contains(pedidas, k)) {
			continue
		}
		q.Disciplina = nome[k]
		q.Assunto = NomeDaMateria(q.Assunto)
		q.Grupo = GrupoDaMateria(q.Disciplina)
		out = append(out, q)
	}

	return out
}

// Os grupos de matéria do treino, na ordem em que aparecem. A separação é
// prática, não acadêmica: o que serve para qualquer concurso, o que só serve
// para aquele órgão e o que é do cargo de TI.
const (
	GrupoBasicas     = "basicas"
	GrupoLegislacao  = "legislacao"
	GrupoOrgao       = "orgao"
	GrupoEspecificas = "especificas"
)

var (
	semAcento = strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i",
		"ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c",
	)
	// Português, matemática e a informática do dia a dia, com os nomes que as
	// bancas dão: "Língua Portuguesa", "Raciocínio Lógico-Matemático",
	// "Noções de Informática" (Word, Excel). "Lógica" e "informática" sozinhas
	// não: são também a de programação e a de um cargo de TI.
	basicas = []string{
		"portugues", "lingua", "redacao", "matematica", "raciocinio",
		"nocoes de informatica", "informatica basica", "office", "word", "excel", "planilha",
	}
	// Só vale para aquele concurso: o regimento do tribunal, o estatuto dos
	// servidores do estado, o código de ética da casa, as resoluções do CNJ e
	// do CSJT, e o que a banca cobra sobre o estado.
	doOrgao = []string{
		"regimento", "etica", "estatuto", "institucional", "organizacao judiciaria",
		"resolu", "lei organica", "estadual", "do estado", "geografia e historia",
	}
	// Lei e administração pública que caem em concurso de qualquer órgão.
	legislacao = []string{
		"legisla", "direito", "constitui", "administracao publica", "administracao financeira",
		"orcament", "auditoria", "controle interno", "regulacao", "sustentabilidade",
		"direitos humanos", "deficiencia",
	}
	// A lei que é do cargo de TI, e não do concurso: LGPD, Marco Civil,
	// contratações de TIC. Vem antes das outras, por citar "legislação" —
	// "Legislação Aplicada à TI" é matéria de específicas.
	deTI = []string{
		" ti", " tic", "tecnologia da informacao", "lgpd", "marco civil",
		"dados pessoais", "governo digital",
	}
)

// GrupoDaMateria diz em que grupo a matéria entra no treino. A matéria da
// questão é texto livre — da extração ou do curador —, então o grupo sai do
// nome: o que não é básica, nem do órgão, nem legislação geral é específica
// de TI.
func GrupoDaMateria(nome string) string {
	n := semAcento.Replace(strings.ToLower(nome))
	contem := func(partes []string) bool {
		return slices.ContainsFunc(partes, func(p string) bool { return strings.Contains(n, p) })
	}
	switch {
	case contem(basicas):
		return GrupoBasicas
	case contem(deTI):
		return GrupoEspecificas
	case contem(doOrgao):
		return GrupoOrgao
	case contem(legislacao):
		return GrupoLegislacao
	default:
		return GrupoEspecificas
	}
}
