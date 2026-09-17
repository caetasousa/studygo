package prova

import (
	"slices"
	"strings"
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

// chaveDaMateria é o que faz duas grafias serem a mesma matéria: a extração
// e o curador escrevem "Noções Sobre Direitos…" numa prova e "Noções sobre
// Direitos…" noutra, e quem filtra procura uma matéria só.
func chaveDaMateria(nome string) string {
	return strings.ToLower(strings.Join(strings.Fields(nome), " "))
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
		usos[k][strings.Join(strings.Fields(q.Disciplina), " ")]++
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
		q.Assunto = strings.Join(strings.Fields(q.Assunto), " ")
		q.Grupo = GrupoDaMateria(q.Disciplina)
		out = append(out, q)
	}

	return out
}

// Os grupos de matéria do treino, na ordem em que aparecem.
const (
	GrupoBasicas     = "basicas"
	GrupoLegislacao  = "legislacao"
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
	// Lei, regimento e norma de conduta — "Noções de Direito Administrativo",
	// "Direitos das Pessoas com Deficiência", "Administração Pública",
	// "Sustentabilidade" (as resoluções do CNJ), "Legislação Aplicada à TI".
	legislacao = []string{
		"legisla", "direito", "regimento", "estatuto", "etica", "constitui",
		"administracao publica", "sustentabilidade",
	}
)

// GrupoDaMateria diz em que grupo a matéria entra no treino. A matéria da
// questão é texto livre — da extração ou do curador —, então o grupo sai do
// nome: o que não é básica nem legislação é específica de TI.
func GrupoDaMateria(nome string) string {
	n := semAcento.Replace(strings.ToLower(nome))
	contem := func(partes []string) bool {
		return slices.ContainsFunc(partes, func(p string) bool { return strings.Contains(n, p) })
	}
	switch {
	case contem(basicas):
		return GrupoBasicas
	case contem(legislacao):
		return GrupoLegislacao
	default:
		return GrupoEspecificas
	}
}
