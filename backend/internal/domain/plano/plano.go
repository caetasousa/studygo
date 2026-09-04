// Package plano é o motor do cronograma: dada a Config do usuário e o catálogo
// do concurso, monta o plano de estudo dia a dia — pesando cada disciplina pelos
// seus pontos, dividindo o dia em blocos e atribuindo as fases de revisão
// semanal e reta final.
//
// O pacote é puro: não lê relógio, não faz I/O e não conhece HTTP, JSON nem
// banco. O que o motor produz é uma PROPOSTA de cronograma; quem a grava é a
// aplicação, e o que está gravado é o cronograma de verdade (ver Atividade).
package plano

import (
	"time"

	"studygo/internal/domain/concurso"

	"github.com/google/uuid"
)

// Tipo é a natureza de um dia do plano.
type Tipo string

const (
	TipoEstudo          Tipo = "est"     // blocos de conteúdo
	TipoRevisaoDirigida Tipo = "revd"    // revisão dirigida da reta final
	TipoSimulado        Tipo = "sim"     // simulado completo
	TipoDiscursiva      Tipo = "disc"    // treino de discursiva
	TipoVespera         Tipo = "vespera" // véspera da prova
	TipoRevisaoSemanal  Tipo = "rev"     // revisão semanal da fase base
)

// Fase é base (primeira passada pelo edital) ou reta (reta final).
type Fase string

const (
	FaseBase Fase = "base"
	FaseReta Fase = "reta"
)

// ItemDia é um bloco de estudo: uma disciplina e o tema dela naquele dia.
type ItemDia struct {
	Disciplina string // código da disciplina
	Tema       string
	Passada    int // 1 = primeira passada, 2 = segunda
	// AtividadeID é a atividade gravada que este item representa. Fica zerado
	// enquanto o cronograma é só uma proposta do motor, e é preenchido quando os
	// dias são lidos a partir do cronograma materializado.
	AtividadeID uuid.UUID
}

// Dia é um dia do plano.
type Dia struct {
	N      int
	Data   time.Time
	Semana int
	Fase   Fase
	Tipo   Tipo
	Itens  []ItemDia
	Tema   string // manchete dos dias que não têm itens
	Meta   int    // meta de questões do dia
}

// Resultado é tudo que o motor produz: os dias mais as ponderações
// intermediárias que a tela de balanceamento precisa.
type Resultado struct {
	Dias       []Dia
	Slots      map[string]int // blocos de conteúdo por disciplina
	SlotsReta  map[string]int // blocos da reta final por disciplina
	Pontos     map[string]int // pontos por disciplina (questões × peso)
	SomaPontos int
	Simulado   Composicao // divisão de questões de um simulado completo
}

// Gerar monta a proposta de cronograma. Nunca devolve slice ou map nulo.
func Gerar(cfg Config, c *concurso.Concurso) Resultado {
	cfg = cfg.Normalizar()

	codes := make([]string, 0, len(c.Disciplinas))
	temas := map[string][]string{}
	pesos := map[string]int{}

	for _, d := range c.Disciplinas {
		codes = append(codes, d.Codigo)
		pesos[d.Codigo] = d.Peso

		// Uma disciplina sem temas cadastrados ainda ganha dias de estudo — o dia
		// só mostra o nome dela no lugar de um tema específico.
		if len(d.Temas) == 0 {
			temas[d.Codigo] = []string{d.Nome}
		} else {
			temas[d.Codigo] = d.Temas
		}
	}

	pontos := map[string]int{}
	soma := 0

	for _, k := range codes {
		pontos[k] = cfg.Questoes[k] * pesos[k]
		soma += pontos[k]
	}

	res := Resultado{
		Dias:       []Dia{},
		Slots:      map[string]int{},
		SlotsReta:  map[string]int{},
		Pontos:     pontos,
		SomaPontos: soma,
	}

	res.Dias = construir(cfg, c, codes, temas, pontos, soma, &res)

	return res
}
