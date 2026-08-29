package service

import (
	"time"

	"annygo/internal/domain/plano"

	"github.com/google/uuid"
)

// The view models below are the wire contract for the plano endpoints. They are
// service-layer (not domain) types, so JSON tags live here and the httpapi
// handler only marshals them.

const isoDate = "2006-01-02"

// ConfigInput is the editable plan configuration coming from the client.
type ConfigInput struct {
	Inicio        string         `json:"inicio"`
	Prova         string         `json:"prova"`
	HorasDia      float64        `json:"horasDia"`
	DiasEstudo    []int          `json:"diasEstudo"`
	DiaRevisao    int            `json:"diaRevisao"`
	RetaFinalDias int            `json:"retaFinalDias"`
	TemaUI        string         `json:"temaUi"`
	Questoes      map[string]int `json:"questoes"`
}

// RegistroInput is one day's log coming from the client.
type RegistroInput struct {
	Horas     *float64 `json:"horas"`
	Concluido bool     `json:"concluido"`
	Questoes  *int     `json:"questoes"`
	Acertos   *int     `json:"acertos"`
	Nota      string   `json:"nota"`
}

// PlanoResposta is the fat payload for GET /api/plano.
type PlanoResposta struct {
	Concurso      ConcursoResposta     `json:"concurso"`
	Config        ConfigResposta       `json:"config"`
	Dias          []DiaResposta        `json:"dias"`
	Marcos        []MarcoResposta      `json:"marcos"`
	Balanceamento []LinhaBalanceamento `json:"balanceamento"`
	Props         PropsResposta        `json:"props"`
	Alertas       []AlertaResposta     `json:"alertas"`
	HojeIndex     *int                 `json:"hojeIndex"`
	Reordenados   []string             `json:"reordenados"`
	GeradoEm      time.Time            `json:"geradoEm"`
}

// ConcursoResposta is the catalogue data the SPA needs to render every page.
type ConcursoResposta struct {
	Slug        string               `json:"slug"`
	Nome        string               `json:"nome"`
	Banca       string               `json:"banca"`
	Cargo       string               `json:"cargo"`
	Emoji       string               `json:"emoji"`
	Resumo      string               `json:"resumo"`
	Disciplinas []DisciplinaResposta `json:"disciplinas"`
	Conteudo    []ConteudoResposta   `json:"conteudo"`
}

type DisciplinaResposta struct {
	Codigo string   `json:"codigo"`
	Nome   string   `json:"nome"`
	Bloco  string   `json:"bloco"`
	Peso   int      `json:"peso"`
	Cor    int      `json:"cor"` // palette index 0..12
	Temas  []string `json:"temas"`
}

type ConteudoResposta struct {
	Tipo  string `json:"tipo"`
	Texto string `json:"texto"`
}

type ConfigResposta struct {
	Inicio        string         `json:"inicio"`
	Prova         string         `json:"prova"`
	HorasDia      float64        `json:"horasDia"`
	DiasEstudo    []int          `json:"diasEstudo"`
	DiaRevisao    int            `json:"diaRevisao"`
	RetaFinalDias int            `json:"retaFinalDias"`
	TemaUI        string         `json:"temaUi"`
	Questoes      map[string]int `json:"questoes"`
}

// DiaResposta is one plan day plus the user's record and timed breakdown.
type DiaResposta struct {
	N          int               `json:"n"`
	Data       string            `json:"data"`
	Semana     int               `json:"semana"`
	Fase       string            `json:"fase"`
	Tipo       string            `json:"tipo"`
	Itens      []ItemResposta    `json:"itens"`
	Tema       string            `json:"tema"`
	Meta       int               `json:"meta"`
	Blocos     []BlocoResposta   `json:"blocos"`
	Registro   *RegistroResposta `json:"registro"`
	Reordenado bool              `json:"reordenado"`
}

type ItemResposta struct {
	Disciplina string `json:"disciplina"`
	Tema       string `json:"tema"`
	Passada    int    `json:"passada"`
}

type BlocoResposta struct {
	Minutos int    `json:"minutos"`
	Titulo  string `json:"titulo"`
	Detalhe string `json:"detalhe"`
}

type RegistroResposta struct {
	Horas     *float64 `json:"horas"`
	Concluido bool     `json:"concluido"`
	Questoes  *int     `json:"questoes"`
	Acertos   *int     `json:"acertos"`
	Nota      string   `json:"nota"`
}

type MarcoResposta struct {
	ID         uuid.UUID `json:"id"`
	Rotulo     int       `json:"rotulo"`
	DataInicio string    `json:"dataInicio"`
	DataFim    *string   `json:"dataFim"`
	Titulo     string    `json:"titulo"`
	ExigeAcao  bool      `json:"exigeAcao"`
	EProva     bool      `json:"eProva"`
	Cumprido   bool      `json:"cumprido"`
}

// LinhaBalanceamento is one row of the balanceamento tables.
type LinhaBalanceamento struct {
	Codigo         string  `json:"codigo"`
	Nome           string  `json:"nome"`
	Bloco          string  `json:"bloco"`
	Cor            int     `json:"cor"`
	Questoes       int     `json:"questoes"`
	Peso           int     `json:"peso"`
	Pontos         int     `json:"pontos"`
	PctIdeal       float64 `json:"pctIdeal"`
	BlocosConteudo int     `json:"blocosConteudo"`
	BlocosReta     int     `json:"blocosReta"`
	HorasPrevisto  float64 `json:"horasPrevisto"`
	HorasLancado   float64 `json:"horasLancado"`
	Desvio         float64 `json:"desvio"`
	AcertoPct      *int    `json:"acertoPct"`
}

type PropsResposta struct {
	FaltamDias     int     `json:"faltamDias"`
	Progresso      int     `json:"progresso"`
	HorasTotal     float64 `json:"horasTotal"`
	HorasAlvo      float64 `json:"horasAlvo"`
	AcertoPct      *int    `json:"acertoPct"`
	TotalDias      int     `json:"totalDias"`
	DiasConcluidos int     `json:"diasConcluidos"`
}

type AlertaResposta struct {
	Nivel  string `json:"nivel"` // "warn" | "danger"
	Titulo string `json:"titulo"`
	Texto  string `json:"texto"`
}

// EstatisticasResposta is GET /api/plano/estatisticas.
type EstatisticasResposta struct {
	Serie         []PontoSerie         `json:"serie"`
	PorSemana     []ResumoSemana       `json:"porSemana"`
	PorDisciplina []LinhaBalanceamento `json:"porDisciplina"`
	Streak        int                  `json:"streak"`
	HorasTotal    float64              `json:"horasTotal"`
	QuestoesTotal int                  `json:"questoesTotal"`
	AcertosTotal  int                  `json:"acertosTotal"`
}

type PontoSerie struct {
	Data      string  `json:"data"`
	N         int     `json:"n"`
	Horas     float64 `json:"horas"`
	Questoes  int     `json:"questoes"`
	Acertos   int     `json:"acertos"`
	Concluido bool    `json:"concluido"`
}

type ResumoSemana struct {
	Semana        int     `json:"semana"`
	HorasPrevisto float64 `json:"horasPrevisto"`
	HorasLancado  float64 `json:"horasLancado"`
	Questoes      int     `json:"questoes"`
}

// CadernoResposta is GET /api/plano/caderno.
type CadernoResposta struct {
	Anotacoes   []AnotacaoResposta `json:"anotacoes"`
	DiasComNota []DiaNota          `json:"diasComNota"`
	DiasFracos  []DiaFraco         `json:"diasFracos"`
}

type AnotacaoResposta struct {
	ID         uuid.UUID `json:"id"`
	Data       *string   `json:"data"`
	Disciplina *string   `json:"disciplina"`
	Texto      string    `json:"texto"`
	Resolvido  bool      `json:"resolvido"`
	CriadoEm   time.Time `json:"criadoEm"`
}

type DiaNota struct {
	Data        string   `json:"data"`
	N           int      `json:"n"`
	Disciplinas []string `json:"disciplinas"`
	Nota        string   `json:"nota"`
}

type DiaFraco struct {
	Data     string `json:"data"`
	N        int    `json:"n"`
	Questoes int    `json:"questoes"`
	Acertos  int    `json:"acertos"`
	Pct      int    `json:"pct"`
}

// AnotacaoInput is the create/update body for a notebook entry.
type AnotacaoInput struct {
	Data       *string `json:"data"`
	Disciplina *string `json:"disciplina"`
	Texto      string  `json:"texto"`
	Resolvido  bool    `json:"resolvido"`
}

func registroToResposta(r plano.Registro) *RegistroResposta {
	return &RegistroResposta{
		Horas:     r.Horas,
		Concluido: r.Concluido,
		Questoes:  r.Questoes,
		Acertos:   r.Acertos,
		Nota:      r.Nota,
	}
}

func parseISODate(s string) (time.Time, bool) {
	t, err := time.Parse(isoDate, s)
	if err != nil {
		return time.Time{}, false
	}

	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
}
