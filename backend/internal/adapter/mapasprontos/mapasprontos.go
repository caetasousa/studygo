// Package mapasprontos embute no binário os mapas que vêm com o app: o texto
// (<slug>.md) e as questões (<slug>.questoes.json), no mesmo formato que a
// tela importa. Diferente dos mapas das aulas, que derivam de material pago e
// ficam fora do repositório, estes são públicos — as provas de português da
// FCC, com o comentário de cada alternativa escrito à parte.
package mapasprontos

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"studygo/internal/domain/mapa"
)

//go:embed conteudo
var conteudo embed.FS

// arquivoDeQuestoes é o <slug>.questoes.json, com as tags do formato.
type arquivoDeQuestoes struct {
	Mapa     string `json:"mapa"`
	Questoes []struct {
		ID           string   `json:"id"`
		Ramo         string   `json:"ramo"`
		Origem       string   `json:"origem"`
		Enunciado    string   `json:"enunciado"`
		Alternativas []string `json:"alternativas"`
		Gabarito     string   `json:"gabarito"`
		Comentario   string   `json:"comentario"`
	} `json:"questoes"`
}

// Todos lê os mapas embutidos, na ordem do nome do arquivo. Um arquivo que
// não se lê é erro de build, não de conta: quem chama deve parar o servidor.
func Todos() ([]mapa.MapaPronto, error) {
	nomes, err := fs.Glob(conteudo, "conteudo/*.md")
	if err != nil {
		return nil, err
	}

	sort.Strings(nomes)

	out := make([]mapa.MapaPronto, 0, len(nomes))

	for _, nome := range nomes {
		texto, err := conteudo.ReadFile(nome)
		if err != nil {
			return nil, err
		}

		slug := strings.TrimSuffix(path.Base(nome), ".md")

		bruto, err := conteudo.ReadFile(path.Join("conteudo", slug+".questoes.json"))
		if err != nil {
			return nil, fmt.Errorf("mapa pronto %s sem questões: %w", slug, err)
		}

		var a arquivoDeQuestoes
		if err := json.Unmarshal(bruto, &a); err != nil {
			return nil, fmt.Errorf("questões do mapa pronto %s: %w", slug, err)
		}

		qs := make([]mapa.Questao, 0, len(a.Questoes))
		for _, q := range a.Questoes {
			qs = append(qs, mapa.Questao{
				Chave: q.ID, Ramo: q.Ramo, Origem: q.Origem, Enunciado: q.Enunciado,
				Alternativas: q.Alternativas, Gabarito: q.Gabarito, Comentario: q.Comentario,
			})
		}

		out = append(out, mapa.MapaPronto{
			Texto:    string(texto),
			Questoes: mapa.ArquivoDeQuestoes{Mapa: a.Mapa, Questoes: qs},
		})
	}

	return out, nil
}
