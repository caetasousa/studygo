package mapasprontos

import (
	"testing"

	"studygo/internal/domain/mapa"
)

// Guarda-corpo do conteúdo embutido. Como pode quebrar — escrito antes do
// código:
//
//	P1  um texto ou um arquivo de questões que a importação recusa entra no
//	    binário, e "Adicionar" falha na conta de quem clicou (ou o servidor
//	    nem sobe)
//	P2  o arquivo de questões é de outro mapa, ou o nome do arquivo não é o
//	    slug: a lista oferece um mapa e adiciona outro
//	P3  uma questão de múltipla escolha chega sem a explicação de cada
//	    alternativa (o pedido foi a justificativa de todas), ou a explicação
//	    marca como certa uma letra que não é o gabarito
func TestTodos_OConteudoEmbutidoEntraNaConta(t *testing.T) {
	t.Parallel()

	ps, err := Todos()
	if err != nil {
		t.Fatal(err)
	}

	if len(ps) == 0 {
		t.Fatal("nenhum mapa pronto embutido")
	}

	for _, p := range ps {
		m, err := mapa.Ler(p.Texto)
		if err != nil {
			t.Fatalf("o texto não se lê: %v", err)
		}

		if p.Questoes.Mapa != m.Slug {
			t.Errorf("%s: o arquivo de questões é do mapa %q", m.Slug, p.Questoes.Mapa)
		}

		qs, err := mapa.ValidarQuestoes(m, p.Questoes)
		if err != nil {
			t.Fatalf("%s: %v", m.Slug, err)
		}

		for _, q := range qs {
			if q.DeJulgar() {
				continue
			}

			_, por := q.Explicacoes()
			if len(por) != len(q.Alternativas) {
				t.Errorf("%s: sem a explicação de cada alternativa", q.Chave)
				continue
			}

			for i, e := range por {
				certa := string(rune('A'+i)) == q.Gabarito
				if diz := len(e) >= 7 && e[:7] == "Correto"; diz != certa {
					t.Errorf("%s: a explicação da %c contradiz o gabarito %s", q.Chave, 'A'+i, q.Gabarito)
				}
			}
		}
	}
}
