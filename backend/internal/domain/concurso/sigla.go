package concurso

import (
	"strings"
	"unicode"
)

// Códigos are what the schedule shows on every activity chip, so they have to
// be readable at a glance: "POR" and "DIRADM" say what they are, "D01" and
// "D02" say only what order they were imported in.
//
// Sigla builds a short mnemonic from a discipline's name, and Siglas assigns a
// unique one to each discipline of a concurso.

// palavrasIgnoradas are the connectives and generic qualifiers that carry no
// identity: including them would turn every "Noções de X" into "NOC".
var palavrasIgnoradas = map[string]bool{
	"de": true, "da": true, "do": true, "das": true, "dos": true,
	"e": true, "em": true, "a": true, "o": true, "as": true, "os": true,
	"para": true, "com": true, "no": true, "na": true, "nos": true, "nas": true,
	"noções": true, "nocoes": true, "noção": true, "nocao": true,
	"aplicada": true, "aplicado": true, "aplicadas": true, "aplicados": true,
	"geral": true, "gerais": true, "básica": true, "basica": true,
	"básicas": true, "basicas": true, "introdução": true, "introducao": true,
}

// Sigla derives a mnemonic code from a discipline name.
//
// One significant word gives its first three letters (Português -> POR); two or
// more give three from the first plus two from the second (Direito
// Administrativo -> DIRAD), which keeps sibling disciplines apart — the common
// case of several "Direito ..." or "Noções de ..." in one edital.
//
// The result is uppercase ASCII, at most 6 characters. An empty or
// unusable name yields "", and the caller decides on a fallback.
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

// Siglas assigns a unique código to every discipline, in slice order, and
// returns them positionally.
//
// Uniqueness is resolved by appending a digit ("DIRAD", "DIRAD2"), never by
// falling back to the positional scheme: a name that collides is still more
// recognisable with a suffix than as "D07". A discipline whose name yields no
// letters at all keeps the positional code, since there is nothing to build on.
func Siglas(nomes []string) []string {
	out := make([]string, len(nomes))
	usados := make(map[string]bool, len(nomes))

	for i, nome := range nomes {
		base := Sigla(nome)
		if base == "" {
			base = "D" + doisDigitos(i+1)
		}

		codigo := base
		for n := 2; usados[codigo]; n++ {
			codigo = base + itoa(n)
		}

		usados[codigo] = true
		out[i] = codigo
	}

	return out
}

// significativas splits a name into the words that carry identity, with accents
// folded away so "Órgão" and "Orgao" behave alike.
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

	// A name made entirely of ignored words ("Noções Gerais") still has to yield
	// something, so fall back to using them.
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

// semAcento folds the Latin-1 accented letters Portuguese actually uses down to
// ASCII, so a código never carries a diacritic.
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
		return "0" + itoa(n)
	}

	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}

	return string(b)
}
