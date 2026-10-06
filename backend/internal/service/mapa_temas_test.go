//go:build integration

package service

import (
	"errors"
	"slices"
	"testing"

	"studygo/internal/adapter/postgres"
	"studygo/internal/domain/concurso"
	"studygo/internal/domain/mapa"
)

// Os tópicos que o mapa cobre, no banco de verdade (M27). Como pode falhar —
// escrito antes do código:
//
//	T1  a importação vincula a matéria inteira quando o mapa só cita um tópico,
//	    e o cronograma volta a pôr o mapa em toda atividade
//	T2  o mapa que casa pelo nome da matéria (sem citar tópico) fica sem tópico
//	    nenhum e some do cronograma, em vez de valer para a matéria inteira
//	T3  a escolha feita na tela não fica gravada, sai fora da ordem da ementa,
//	    ou importar o mapa de novo a troca pela sugestão
//	T4  um tópico de outra matéria é aceito
//	T5  desmarcar todos os tópicos apaga o vínculo
//	T6  o tópico renomeado na edição do concurso continua listado, apontando
//	    para um assunto que o cronograma não tem mais

const mapaDeSQL = `# Consultas
slug: consultas
reconhecer: SQL

- SELECT
  - Escolhe as colunas
`

const mapaDeBanco = `# Bancos
slug: bancos
materia: Banco de Dados

- Relacional
  - Tabelas
`

type cenarioDosTemas struct {
	*cenario
	svc *MapaService
}

func novoCenarioDosTemas(t *testing.T) cenarioDosTemas {
	t.Helper()

	ce := novoCenario(t)

	return cenarioDosTemas{cenario: ce, svc: NewMapaService(postgres.NewMapaRepo(ce.pool), ce.deps.Concursos)}
}

// doMapa é o vínculo do mapa na matéria, como o cronograma o recebe.
func (ce cenarioDosTemas) doMapa(t *testing.T, codigo, slug string) (MapaDaMateria, bool) {
	t.Helper()

	ms, err := ce.svc.DoConcurso(t.Context(), ce.usuario, ce.slug)
	if err != nil {
		t.Fatalf("DoConcurso: %v", err)
	}

	for _, d := range ms {
		if d.Codigo != codigo {
			continue
		}

		for _, m := range d.Mapas {
			if m.Resumo.Slug == slug {
				return m, true
			}
		}
	}

	return MapaDaMateria{}, false
}

func (ce cenarioDosTemas) importar(t *testing.T, texto string) {
	t.Helper()

	if _, err := ce.svc.Importar(t.Context(), ce.usuario, texto, ce.slug); err != nil {
		t.Fatalf("importando: %v", err)
	}
}

func (ce cenarioDosTemas) banda(t *testing.T) concurso.Disciplina {
	t.Helper()

	for _, d := range ce.concurso(t).Disciplinas {
		if d.Codigo == "BANDA" {
			return d
		}
	}

	t.Fatal("o concurso do teste não tem BANDA")

	return concurso.Disciplina{}
}

// T1: o mapa que cita SQL fica no tópico SQL, não na matéria inteira.
func TestMapa_ImportarMarcaOsTopicosQueOMapaCita(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDosTemas(t)
	ce.importar(t, mapaDeSQL)

	v, ok := ce.doMapa(t, "BANDA", "consultas")
	if !ok {
		t.Fatal("o mapa que cita SQL não foi vinculado ao Banco de Dados")
	}

	if v.MateriaInteira || !slices.Equal(v.Temas, []string{"SQL"}) {
		t.Fatalf("vínculo = inteira %v, tópicos %q; quer só SQL", v.MateriaInteira, v.Temas)
	}

	ms, err := ce.svc.DoConcurso(t.Context(), ce.usuario, ce.slug)
	if err != nil {
		t.Fatal(err)
	}

	for _, d := range ms {
		if d.Codigo == "BANDA" && !slices.Equal(d.Temas, []string{"Modelagem", "SQL", "Índices", "Transações"}) {
			t.Fatalf("a ementa da matéria veio %q", d.Temas)
		}
	}
}

// T2: casar pelo nome da matéria, sem citar tópico, é a matéria inteira.
func TestMapa_ImportarPeloNomeDaMateriaValeParaAMateriaInteira(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDosTemas(t)
	ce.importar(t, mapaDeBanco)

	v, ok := ce.doMapa(t, "BANDA", "bancos")
	if !ok || !v.MateriaInteira || len(v.Temas) != 0 {
		t.Fatalf("vínculo = %+v, %v; quer a matéria inteira", v, ok)
	}
}

// T3: a escolha fica gravada na ordem da ementa e sobrevive a reimportar.
func TestMapa_EscolherTopicosFicaGravadoESobreviveAReimportacao(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDosTemas(t)
	ce.importar(t, mapaDeSQL)

	if err := ce.svc.Vincular(t.Context(), ce.usuario, ce.slug, ce.banda(t).ID, "consultas", true,
		[]string{"Índices", "SQL"}); err != nil {
		t.Fatalf("Vincular: %v", err)
	}

	ce.importar(t, mapaDeSQL)

	v, _ := ce.doMapa(t, "BANDA", "consultas")
	if v.MateriaInteira || !slices.Equal(v.Temas, []string{"SQL", "Índices"}) {
		t.Fatalf("depois de reimportar: inteira %v, tópicos %q; quer SQL e Índices", v.MateriaInteira, v.Temas)
	}
}

// T4: tópico de outra matéria é recusado, e nada muda.
func TestMapa_TopicoDeOutraMateriaERecusado(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDosTemas(t)
	ce.importar(t, mapaDeSQL)

	err := ce.svc.Vincular(t.Context(), ce.usuario, ce.slug, ce.banda(t).ID, "consultas", true, []string{"Crase"})
	if !errors.Is(err, mapa.ErrTemaForaDaMateria) {
		t.Fatalf("erro = %v, quer ErrTemaForaDaMateria", err)
	}

	if v, _ := ce.doMapa(t, "BANDA", "consultas"); !slices.Equal(v.Temas, []string{"SQL"}) {
		t.Fatalf("a recusa mexeu no vínculo: %q", v.Temas)
	}
}

// T5: nenhum tópico é a matéria inteira — o vínculo continua.
func TestMapa_SemTopicoNenhumVoltaAMateriaInteira(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDosTemas(t)
	ce.importar(t, mapaDeSQL)

	if err := ce.svc.Vincular(t.Context(), ce.usuario, ce.slug, ce.banda(t).ID, "consultas", true, nil); err != nil {
		t.Fatalf("Vincular: %v", err)
	}

	v, ok := ce.doMapa(t, "BANDA", "consultas")
	if !ok || !v.MateriaInteira {
		t.Fatalf("vínculo = %+v, %v; quer a matéria inteira", v, ok)
	}
}

// T6: o tópico que saiu da ementa sai também da lista do mapa.
func TestMapa_TopicoRenomeadoSaiDoVinculo(t *testing.T) {
	t.Parallel()

	ce := novoCenarioDosTemas(t)
	ce.importar(t, mapaDeSQL)

	ce.gravarConcurso(t, func(c *concurso.Concurso) {
		for i := range c.Disciplinas {
			if c.Disciplinas[i].Codigo == "BANDA" {
				c.Disciplinas[i].Temas = []string{"Modelagem", "Linguagem SQL", "Índices", "Transações"}
			}
		}
	})

	v, ok := ce.doMapa(t, "BANDA", "consultas")
	if !ok || v.MateriaInteira || len(v.Temas) != 0 {
		t.Fatalf("vínculo = %+v, %v; quer vínculo sem tópico vivo (e não a matéria inteira)", v, ok)
	}
}
