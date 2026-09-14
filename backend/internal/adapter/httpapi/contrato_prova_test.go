package httpapi

import (
	"testing"
	"time"

	"studygo/internal/domain/prova"
	"studygo/internal/service"
)

// Snapshot do contrato das provas. O rascunho vai e volta entre a tela de
// revisão e a API, então a forma dele é contrato nos dois sentidos.

// rascunhoDeContrato é preenchido de propósito: o primeiro bloco é imagem com
// origem, para o snapshot descrever o caso completo e não só o texto.
func rascunhoDeContrato() prova.Rascunho {
	origem := prova.Origem{Pagina: 1, Retangulo: []float64{0, 0, 595, 845}, Regiao: "0"}
	figura := prova.Bloco{Tipo: "imagem", Arquivo: "a", Descricao: "diagrama", Origem: &origem, Revisado: true, Largura: 60}

	return prova.Rascunho{
		Banca: "FCC", Orgao: "TJCE", Ano: 2026, Cargo: "E05", Caderno: "004", Total: 60,
		Questoes: []prova.Questao{{
			Numero: 44, Disciplina: "Redes", Blocos: []prova.Bloco{figura},
			Alternativas: []prova.Alternativa{{Letra: "A", Blocos: []prova.Bloco{figura}}},
			Apoios:       []string{"r0-t1"}, Origens: []prova.Origem{origem},
			Resposta: "A", Situacao: "Gabarito sem alteração", Revisada: true, Completa: true,
		}},
		Apoios: []prova.Apoio{{ID: "r0-t1", Blocos: []prova.Bloco{figura}, Questoes: []int{1}, Revisado: true}},
		Gabarito: prova.Gabarito{
			Cargo: "E05", Caderno: "4", Tipo: "preliminar",
			Respostas: map[string]string{"44": "A"}, Situacoes: map[string]string{"44": "sem alteração"},
		},
		Alertas:   []string{"alerta"},
		Extracoes: []prova.Extracao{{Modelo: "m", TokensEntrada: 1, TokensSaida: 2, Regiao: "0", Prompt: "p", Versao: "1"}},
	}
}

func TestContratoHTTP_ProvaImportacao(t *testing.T) {
	t.Parallel()

	i := service.ImportacaoDeProva{
		Importacao: prova.Importacao{
			ID: "i", Estado: prova.EstadoEmRevisao, Versao: 3, Etapa: 17, ProvaID: "p", Erro: "e",
			Regioes:  []prova.Origem{{Pagina: 1, Retangulo: []float64{0, 0, 1, 1}, Regiao: "0"}},
			Rascunho: rascunhoDeContrato(), CriadoEm: time.Unix(0, 0), AtualizadoEm: time.Unix(0, 0),
		},
		TotalEtapas: 17,
		Pendencias:  []string{"pendência"},
	}

	compararComGolden(t, "prova_importacao.json", forma(t, importacaoProvaParaDTO(i)))
}

func TestContratoHTTP_QuestaoAvulsa(t *testing.T) {
	t.Parallel()

	q := prova.QuestaoAvulsa{
		ProvaID: "p", Numero: 7, Disciplina: "Língua Portuguesa", Resposta: "C",
		Orgao: "TJCE", Ano: 2026, Cargo: "E05", CargoNome: "Analista Judiciário",
	}

	compararComGolden(t, "prova_questao_avulsa.json", forma(t, questaoAvulsaParaDTO(q)))
}

func TestContratoHTTP_ProvaPublicada(t *testing.T) {
	t.Parallel()

	p := prova.Publicacao{ID: "p", Revisao: 2, Conteudo: rascunhoDeContrato(), PublicadoEm: time.Unix(0, 0)}

	compararComGolden(t, "prova.json", forma(t, provaParaDTO(p)))
}
