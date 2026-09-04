package concurso

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// EmojiPadrao é o ícone de um concurso que não escolheu nenhum.
const EmojiPadrao = "📚"

// tamanhoMaximoBaseSlug corta o nome antes do sufixo aleatório, para que a URL
// caiba na barra de endereço sem virar uma linha inteira.
const tamanhoMaximoBaseSlug = 40

// PrimeiroEmoji devolve o primeiro caractere do texto, ou o emoji padrão. O
// formulário aceita qualquer coisa no campo; só o primeiro símbolo vira o
// ícone do concurso.
func PrimeiroEmoji(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) == 0 {
		return EmojiPadrao
	}

	return string(r[0])
}

// semDiacritico mapeia as letras acentuadas comuns em português para a base
// ASCII, para que os slugs continuem legíveis ("técnico" -> "tecnico").
var semDiacritico = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// Slug monta um identificador de URL a partir do nome, com um sufixo aleatório
// curto para que dois concursos de mesmo nome não colidam.
func Slug(nome string) string {
	return BaseSlug(nome) + "-" + hexAleatorio(2)
}

// BaseSlug é a parte determinística do slug — o nome normalizado, sem o sufixo.
// Exposta para que o teste possa afirmar a normalização sem lidar com o acaso.
func BaseSlug(nome string) string {
	var b strings.Builder

	traçoAnterior := false

	for _, r := range semDiacritico.Replace(strings.ToLower(nome)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			traçoAnterior = false
		default:
			if !traçoAnterior && b.Len() > 0 {
				b.WriteByte('-')
				traçoAnterior = true
			}
		}
	}

	base := strings.Trim(b.String(), "-")
	if base == "" {
		return "concurso"
	}

	if len(base) > tamanhoMaximoBaseSlug {
		base = strings.Trim(base[:tamanhoMaximoBaseSlug], "-")
	}

	return base
}

func hexAleatorio(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand só falha se a fonte de entropia do sistema quebrar. Nesse
		// caso o slug perde a garantia de unicidade, e o UNIQUE do banco é quem
		// recusa a colisão — melhor que inventar um sufixo previsível.
		return "0000"
	}

	return hex.EncodeToString(buf)
}
