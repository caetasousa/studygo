package mapa

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// maxProblemas corta a leitura de um texto que não presta: depois de tantos
// avisos o resto só repete o mesmo engano.
const maxProblemas = 30

var (
	reSlug = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	// reMarca é o que abre um item marcado: "[def] texto". Só letras, para que
	// "[1] texto" continue sendo texto; a marca sem texto é item vazio.
	reMarca = regexp.MustCompile(`^\[([A-Za-z]+)\](?: (.*))?$`)
)

// chavesDoMapa são os metadados que o outline aceita logo abaixo do título.
var chavesDoMapa = []string{"slug", "fonte", "materia", "reconhecer"}

// itemLido é um item ainda com o nível do recuo: a árvore só se monta depois
// que o texto inteiro foi validado.
type itemLido struct {
	nivel int
	item  Item
}

// Ler transforma o outline num mapa. O formato está em conteudo/mapas/README.md:
//
//	# Título
//	slug: itil-4            (metadados opcionais)
//	materia: Governança de TI
//
//	- Ramo
//	  - [def] Item com **negrito**
//
// Devolve ErrTextoInvalido, com todos os problemas achados, se o texto não
// presta; nunca devolve um mapa pela metade.
func Ler(texto string) (Mapa, error) {
	texto = strings.TrimPrefix(texto, "\xef\xbb\xbf")

	// O Postgres não guarda NUL em texto, e um texto que não é UTF-8 já chegou
	// corrompido: recusar aqui evita um erro de banco lá na frente.
	if !utf8.ValidString(texto) || strings.ContainsRune(texto, 0) {
		return Mapa{}, ErrTextoInvalido{Problemas: []string{"o texto tem caracteres inválidos"}}
	}

	var (
		m         Mapa
		problemas []string
		lidos     []itemLido
		meta      = map[string]string{}
		comItens  bool
		anterior  = -1
		temTitulo bool
	)

	problema := func(n int, formato string, args ...any) {
		problemas = append(problemas, fmt.Sprintf("linha %d: "+formato, append([]any{n}, args...)...))
	}

	linhas := strings.Split(strings.ReplaceAll(texto, "\r\n", "\n"), "\n")

	for i, bruta := range linhas {
		n := i + 1

		if len(problemas) >= maxProblemas {
			break
		}

		linha := strings.TrimRight(bruta, " \t\r")
		if strings.TrimSpace(linha) == "" {
			continue
		}

		// Só a tabulação do recuo é engano: no meio do texto ela vira espaço.
		margem := len(linha) - len(strings.TrimLeft(linha, " \t"))
		if strings.Contains(linha[:margem], "\t") {
			problema(n, "tabulação no recuo — use 2 espaços por nível")

			continue
		}

		linha = strings.ReplaceAll(linha, "\t", " ")

		if !temTitulo {
			temTitulo = true

			titulo, ok := strings.CutPrefix(linha, "# ")
			if !ok {
				problema(n, "o texto precisa começar com o título (# Título)")

				break
			}

			m.Titulo = strings.TrimSpace(titulo)

			switch {
			case m.Titulo == "":
				problema(n, "o título está vazio")
			case utf8.RuneCountInString(m.Titulo) > MaxTitulo:
				problema(n, "título com mais de %d caracteres", MaxTitulo)
			}

			continue
		}

		recuo := len(linha) - len(strings.TrimLeft(linha, " "))
		corpo := linha[recuo:]

		conteudo, ehItem := itemDe(corpo)
		if !ehItem {
			if comItens {
				problema(n, "esperado um item (- texto), e o que veio foi %q", resumir(corpo))

				continue
			}

			lerMetadado(n, corpo, meta, problema)

			continue
		}

		comItens = true

		if recuo%2 != 0 {
			problema(n, "recuo de %d espaços — use múltiplos de 2", recuo)

			continue
		}

		nivel := recuo / 2

		if nivel >= MaxNiveis {
			problema(n, "fundo demais: o máximo são %d níveis", MaxNiveis)

			continue
		}

		if nivel > anterior+1 {
			problema(n, "o item pula do nível %d para o %d — desça um nível de cada vez", anterior+1, nivel)

			continue
		}

		anterior = nivel

		item, falha := lerItem(conteudo)
		if falha != "" {
			problema(n, "%s", falha)

			continue
		}

		if len(lidos) >= MaxItens {
			problemas = append(problemas, fmt.Sprintf("o mapa passa de %d itens", MaxItens))

			break
		}

		lidos = append(lidos, itemLido{nivel: nivel, item: item})
	}

	if !temTitulo {
		problemas = append(problemas, "o texto está vazio — comece com o título (# Título)")
	}

	if temTitulo && len(lidos) == 0 && len(problemas) == 0 {
		problemas = append(problemas, "o mapa não tem nenhum item (- texto)")
	}

	problemas = append(problemas, aplicarMetadados(&m, meta)...)

	if len(problemas) > 0 {
		return Mapa{}, ErrTextoInvalido{Problemas: problemas}
	}

	m.Ramos = montar(lidos)

	return m, nil
}

// itemDe separa o texto de um item: "- texto" (e só "-" é um item vazio).
func itemDe(corpo string) (string, bool) {
	resto, ok := strings.CutPrefix(corpo, "-")
	if !ok {
		return "", false
	}

	if resto == "" {
		return "", true
	}

	texto, ok := strings.CutPrefix(resto, " ")

	return strings.TrimSpace(texto), ok
}

// lerItem lê a marca e o texto de um item. O segundo retorno é o problema, em
// texto, ou vazio.
func lerItem(texto string) (Item, string) {
	item := Item{Texto: texto}

	if achado := reMarca.FindStringSubmatch(texto); achado != nil {
		marca := Marca(strings.ToLower(achado[1]))
		if !marcaConhecida(marca) {
			return Item{}, fmt.Sprintf("marca desconhecida [%s] — use %s", achado[1], listarMarcas())
		}

		item.Marca = marca
		item.Texto = strings.TrimSpace(achado[2])
	}

	if problema := problemaDaImagemDoItem(item.Texto); problema != "" {
		return Item{}, problema
	}

	switch {
	case item.Texto == "":
		return Item{}, "item vazio"
	case utf8.RuneCountInString(item.Texto) > MaxTexto:
		return Item{}, fmt.Sprintf("item com %d caracteres — o máximo são %d; divida em itens menores", utf8.RuneCountInString(item.Texto), MaxTexto)
	}

	return item, ""
}

func marcaConhecida(m Marca) bool {
	for _, c := range Marcas {
		if c == m {
			return true
		}
	}

	return false
}

func listarMarcas() string {
	nomes := make([]string, 0, len(Marcas))
	for _, m := range Marcas {
		nomes = append(nomes, string(m))
	}

	return strings.Join(nomes[:len(nomes)-1], ", ") + " ou " + nomes[len(nomes)-1]
}

// lerMetadado guarda "chave: valor"; qualquer outra coisa acima dos itens é engano.
func lerMetadado(n int, corpo string, meta map[string]string, problema func(int, string, ...any)) {
	chave, valor, ok := strings.Cut(corpo, ":")
	chave = strings.TrimSpace(chave)

	if !ok || strings.TrimSpace(valor) == "" {
		problema(n, "esperado um metadado (chave: valor) ou um item (- texto), e o que veio foi %q", resumir(corpo))

		return
	}

	conhecida := false

	for _, c := range chavesDoMapa {
		conhecida = conhecida || c == chave
	}

	if !conhecida {
		problema(n, "metadado desconhecido %q — use %s", chave, strings.Join(chavesDoMapa, ", "))

		return
	}

	if _, repetida := meta[chave]; repetida {
		problema(n, "metadado %q repetido", chave)

		return
	}

	meta[chave] = strings.TrimSpace(valor)
}

// aplicarMetadados preenche o mapa com os metadados e devolve o que não vale.
func aplicarMetadados(m *Mapa, meta map[string]string) []string {
	var problemas []string

	m.Fonte = meta["fonte"]
	m.Materia = meta["materia"]

	if utf8.RuneCountInString(m.Fonte) > MaxFonte {
		problemas = append(problemas, fmt.Sprintf("fonte com mais de %d caracteres", MaxFonte))
	}

	if utf8.RuneCountInString(m.Materia) > MaxTitulo {
		problemas = append(problemas, fmt.Sprintf("materia com mais de %d caracteres", MaxTitulo))
	}

	for termo := range strings.SplitSeq(meta["reconhecer"], ",") {
		if termo = strings.TrimSpace(termo); termo != "" {
			m.Reconhecer = append(m.Reconhecer, termo)
		}
	}

	if len(m.Reconhecer) > MaxReconhecer {
		problemas = append(problemas, fmt.Sprintf("reconhecer com mais de %d termos", MaxReconhecer))
	}

	if slug, dado := meta["slug"]; dado {
		if len(slug) > MaxSlug || !reSlug.MatchString(slug) {
			problemas = append(problemas, fmt.Sprintf("slug %q inválido — use letras minúsculas, números e hífens (até %d caracteres)", slug, MaxSlug))
		}

		m.Slug = slug

		return problemas
	}

	m.Slug = slugDe(m.Titulo)
	if m.Slug == "" && m.Titulo != "" {
		problemas = append(problemas, "o título não tem letra nem número para formar o endereço — informe slug: no texto")
	}

	return problemas
}

// slugDe forma o endereço do mapa a partir do título: "Governança de TI" vira
// "governanca-de-ti".
func slugDe(titulo string) string {
	slug := strings.ReplaceAll(dobrar(titulo), " ", "-")
	if len(slug) > MaxSlug {
		slug = strings.Trim(slug[:MaxSlug], "-")
	}

	return slug
}

// montar faz a árvore dos itens em pré-ordem. Os níveis já foram validados
// (nenhum desce mais de um degrau), então cada item cai sob o anterior.
func montar(lidos []itemLido) []Item {
	var (
		proximo int
		subir   func(nivel int) []Item
	)

	subir = func(nivel int) []Item {
		var irmaos []Item

		for proximo < len(lidos) && lidos[proximo].nivel == nivel {
			item := lidos[proximo].item
			proximo++

			item.Filhos = subir(nivel + 1)
			irmaos = append(irmaos, item)
		}

		return irmaos
	}

	return subir(0)
}

// resumir corta o trecho citado numa mensagem de erro.
func resumir(s string) string {
	const maximo = 40

	if r := []rune(s); len(r) > maximo {
		return string(r[:maximo]) + "…"
	}

	return s
}
