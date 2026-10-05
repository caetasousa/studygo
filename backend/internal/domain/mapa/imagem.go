package mapa

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Imagens do mapa: um fluxo de BPMN, um diagrama da aula. O outline cita a
// imagem num item próprio — `![legenda](arquivo.png)` — e o arquivo chega à
// parte, pela página do mapa. A imagem é da conta, como o mapa: deriva de
// material pago e nunca vai para o repositório.

const (
	// MaxBytesImagem é o teto de um arquivo: um diagrama recortado da aula cabe
	// com folga, uma foto de celular sem compressão não.
	MaxBytesImagem = 2 << 20 // 2 MiB
	// MaxImagens é quantas imagens um mapa guarda.
	MaxImagens = 100
	// MaxNomeImagem é o tamanho do nome do arquivo.
	MaxNomeImagem = 80
)

var (
	// reNomeImagem é o nome que o outline cita e o arquivo enviado tem: só
	// minúsculas, números e hífen, com a extensão do tipo. O nome vira parte
	// do endereço da imagem, então nada de barra, ponto a mais ou espaço.
	reNomeImagem = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.(png|jpg|jpeg|webp)$`)
	// reItemImagem é o item inteiro que mostra uma imagem.
	reItemImagem = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)]*)\)$`)
)

// ErrImagemNaoEncontrada vale para a imagem que não foi enviada e para a de um
// mapa de outra conta.
var ErrImagemNaoEncontrada = errors.New("imagem do mapa não encontrada")

// ErrImagensInvalidas traz o problema de cada arquivo recusado: o envio entra
// inteiro ou não entra, e quem envia corrige tudo de uma vez.
type ErrImagensInvalidas struct{ Problemas []string }

func (e ErrImagensInvalidas) Error() string {
	return "imagens recusadas: " + strings.Join(e.Problemas, "; ")
}

// Imagem é um arquivo de imagem do mapa. O tipo sai dos bytes, nunca do nome
// que o navegador mandou.
type Imagem struct {
	Nome  string
	Tipo  string
	Dados []byte
}

// ImagemDoItem diz se o texto de um item é uma imagem e qual: a legenda e o
// nome do arquivo.
func ImagemDoItem(texto string) (legenda, nome string, ok bool) {
	achado := reItemImagem.FindStringSubmatch(texto)
	if achado == nil {
		return "", "", false
	}

	return strings.TrimSpace(achado[1]), achado[2], true
}

// problemaDaImagemDoItem confere o item que começa como imagem. Um "![" mal
// fechado vira aviso, e não um texto estranho na tela.
func problemaDaImagemDoItem(texto string) string {
	if !strings.HasPrefix(texto, "![") {
		return ""
	}

	legenda, nome, ok := ImagemDoItem(texto)

	switch {
	case !ok:
		return "imagem mal escrita — use ![legenda](arquivo.png), sozinha no item"
	case legenda == "":
		return "imagem sem legenda — a legenda é o que a busca acha"
	case !nomeDeImagemValido(nome):
		return fmt.Sprintf("nome de imagem %q inválido — use minúsculas, números e hífen, terminando em .png, .jpg ou .webp", nome)
	}

	return ""
}

func nomeDeImagemValido(nome string) bool {
	return len(nome) <= MaxNomeImagem && reNomeImagem.MatchString(nome)
}

// tipoDosBytes reconhece o formato pela assinatura do arquivo.
func tipoDosBytes(d []byte) string {
	switch {
	case bytes.HasPrefix(d, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case bytes.HasPrefix(d, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case len(d) >= 12 && bytes.Equal(d[:4], []byte("RIFF")) && bytes.Equal(d[8:12], []byte("WEBP")):
		return "image/webp"
	}

	return ""
}

func tipoDaExtensao(nome string) string {
	switch nome[strings.LastIndexByte(nome, '.')+1:] {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	}

	return ""
}

// NovaImagem confere um arquivo enviado: o nome, o tamanho e que os bytes são
// mesmo do tipo que a extensão diz. O segundo retorno é o problema, ou vazio.
func NovaImagem(nome string, dados []byte) (Imagem, string) {
	nome = strings.ToLower(strings.TrimSpace(nome))

	switch {
	case !nomeDeImagemValido(nome):
		return Imagem{}, fmt.Sprintf("%q: nome inválido — use minúsculas, números e hífen, terminando em .png, .jpg ou .webp", nome)
	case len(dados) == 0:
		return Imagem{}, fmt.Sprintf("%s: arquivo vazio", nome)
	case len(dados) > MaxBytesImagem:
		return Imagem{}, fmt.Sprintf("%s: maior que %d MiB", nome, MaxBytesImagem>>20)
	}

	tipo := tipoDosBytes(dados)

	switch {
	case tipo == "":
		return Imagem{}, fmt.Sprintf("%s: não é uma imagem PNG, JPEG ou WebP", nome)
	case tipo != tipoDaExtensao(nome):
		return Imagem{}, fmt.Sprintf("%s: o conteúdo é %s, mas a extensão diz outra coisa", nome, strings.TrimPrefix(tipo, "image/"))
	}

	return Imagem{Nome: nome, Tipo: tipo, Dados: dados}, ""
}

// ImagensCitadas são os nomes das imagens que o mapa cita, na ordem do texto.
func (m Mapa) ImagensCitadas() []string {
	var (
		out    []string
		vistas = map[string]bool{}
		descer func([]Item)
	)

	descer = func(itens []Item) {
		for _, it := range itens {
			if _, nome, ok := ImagemDoItem(it.Texto); ok && !vistas[nome] {
				vistas[nome] = true
				out = append(out, nome)
			}

			descer(it.Filhos)
		}
	}

	descer(m.Ramos)

	return out
}
