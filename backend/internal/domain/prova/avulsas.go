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
	Resposta   string
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
// têm matéria, dá um nome só a cada matéria e aplica o filtro.
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
		if k == "" || (f.Ano != 0 && q.Ano != f.Ano) || (len(pedidas) > 0 && !slices.Contains(pedidas, k)) {
			continue
		}
		q.Disciplina = nome[k]
		out = append(out, q)
	}

	return out
}
