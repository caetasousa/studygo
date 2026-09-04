package concurso

import (
	"strconv"
	"strings"
	"unicode"
)

// O código de uma disciplina é o que o cronograma mostra em cada chip, então
// precisa ser legível de relance: "POR" e "DIRAD" dizem o que são, "D01" e
// "D02" dizem só a ordem em que foram importados.
//
// Esta é a ÚNICA implementação da regra no repositório. Ela já existiu três
// vezes — aqui, numa função plpgsql de migration e outra vez no frontend, esta
// última com resultado diferente ("DA" em vez de "DIRAD"), o que fazia a tela
// discordar do que estava gravado. O frontend agora exibe o `codigo` que a API
// manda, e a migration que reimplementava isso em SQL não existe mais.

// palavrasIgnoradas são os conectivos e qualificadores genéricos que não
// carregam identidade: incluí-los transformaria todo "Noções de X" em "NOC".
var palavrasIgnoradas = map[string]bool{
	"de": true, "da": true, "do": true, "das": true, "dos": true,
	"e": true, "em": true, "a": true, "o": true, "as": true, "os": true,
	"para": true, "com": true, "no": true, "na": true, "nos": true, "nas": true,
	"noções": true, "nocoes": true, "noção": true, "nocao": true,
	"aplicada": true, "aplicado": true, "aplicadas": true, "aplicados": true,
	"geral": true, "gerais": true, "básica": true, "basica": true,
	"básicas": true, "basicas": true, "introdução": true, "introducao": true,
}

// Sigla deriva um mnemônico do nome de uma disciplina.
//
// Uma palavra significativa dá suas quatro primeiras letras (Português -> PORT);
// duas ou mais dão três da primeira e duas da segunda (Direito Administrativo ->
// DIRAD), o que separa disciplinas irmãs — o caso comum de vários "Direito ..."
// ou "Noções de ..." no mesmo edital.
//
// O resultado é ASCII maiúsculo. Um nome vazio ou inaproveitável devolve "", e
// quem chama decide o fallback.
func Sigla(nome string) string {
	palavras := significativas(nome)
	if len(palavras) == 0 {
		return ""
	}

	if len(palavras) == 1 {
		return prefixo(palavras[0], 4)
	}

	return prefixo(palavras[0], 3) + prefixo(palavras[1], 2)
}

// CodigoUnico devolve um mnemônico para `nome` que ainda não esteja em
// `usados`. Colisões ganham um sufixo numérico ("DIRAD", "DIRAD2") em vez de
// cair no esquema posicional: um nome que colide continua mais reconhecível com
// sufixo do que como "D07". Um nome sem letra nenhuma usa a posição, porque não
// há o que aproveitar.
func CodigoUnico(nome string, posicao int, usados map[string]bool) string {
	base := Sigla(nome)
	if base == "" {
		base = "D" + doisDigitos(posicao+1)
	}

	codigo := base
	for n := 2; usados[codigo]; n++ {
		codigo = base + strconv.Itoa(n)
	}

	return codigo
}

// Siglas atribui um código único a cada nome, na ordem recebida.
func Siglas(nomes []string) []string {
	out := make([]string, len(nomes))
	usados := make(map[string]bool, len(nomes))

	for i, nome := range nomes {
		out[i] = CodigoUnico(nome, i, usados)
		usados[out[i]] = true
	}

	return out
}

// significativas quebra um nome nas palavras que carregam identidade, com os
// acentos dobrados para ASCII, de modo que "Órgão" e "Orgao" se comportem igual.
func significativas(nome string) []string {
	campos := strings.FieldsFunc(nome, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	out := make([]string, 0, len(campos))

	for _, c := range campos {
		limpo := semAcento(strings.ToLower(c))
		if limpo == "" || palavrasIgnoradas[limpo] || palavrasIgnoradas[strings.ToLower(c)] {
			continue
		}

		out = append(out, limpo)
	}

	// Um nome feito só de palavras ignoradas ("Noções Gerais") ainda precisa
	// render alguma coisa, então elas voltam a valer.
	if len(out) == 0 {
		for _, c := range campos {
			if limpo := semAcento(strings.ToLower(c)); limpo != "" {
				out = append(out, limpo)
			}
		}
	}

	return out
}

func prefixo(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}

	return strings.ToUpper(string(r))
}

// semAcento dobra para ASCII as letras acentuadas que o português usa, para que
// um código nunca carregue diacrítico.
func semAcento(s string) string {
	var b strings.Builder

	for _, r := range s {
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
		case r == 'ñ':
			b.WriteRune('n')
		}
	}

	return b.String()
}

func doisDigitos(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}

	return strconv.Itoa(n)
}
