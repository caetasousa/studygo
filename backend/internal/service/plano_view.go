package service

import (
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// The view models below are the wire contract for the plano endpoints. They are
// service-layer (not domain) types, so JSON tags live here and the httpapi
// handler only marshals them.

const isoDate = "2006-01-02"

// ConfigInput is the editable plan configuration coming from the client. Every
// study-method field is a pointer/optional: a nil (absent) field leaves that
// setting untouched, so a patch that only touches one control doesn't reset the
// rest — the bug the old flat `DiaRevisao int` had (a zero value = domingo, so
// every partial save silently moved the review day).
type ConfigInput struct {
	Inicio        string         `json:"inicio"`
	Prova         string         `json:"prova"`
	HorasDia      float64        `json:"horasDia"`
	DiasEstudo    []int          `json:"diasEstudo"`
	DiaRevisao    *int           `json:"diaRevisao"`
	RetaFinalDias int            `json:"retaFinalDias"`
	TemaUI        string         `json:"temaUi"`
	Questoes      map[string]int `json:"questoes"`

	// Study method — was the nested `perfil` object, now flat.
	BlocosPorDia   *int               `json:"blocosPorDia"`
	MinutosBloco   *int               `json:"minutosBloco"`
	MinutosRevisao *int               `json:"minutosRevisao"`
	Reforcos       map[string]float64 `json:"reforcos"`
	CicloRevisao   *[]CicloItemInput  `json:"cicloRevisao"`
	RevisaoSemanal *bool              `json:"revisaoSemanal"`
	Simulados      *string            `json:"simulados"`
	Discursiva     *bool              `json:"discursiva"`
	Modos          map[string]string  `json:"modos"`
	PctQuestoes    *float64           `json:"pctQuestoes"`
	LimiarFraco    *int               `json:"limiarFraco"`
}

// CicloItemInput is one week of the base-phase review rotation.
type CicloItemInput struct {
	Titulo   string `json:"titulo"`
	Questoes int    `json:"questoes"`
}

// RegistroInput is one day's log coming from the client.
type RegistroInput struct {
	Horas     *float64             `json:"horas"`
	Concluido bool                 `json:"concluido"`
	Questoes  *int                 `json:"questoes"`
	Acertos   *int                 `json:"acertos"`
	Nota      string               `json:"nota"`
	Blocos    []RegistroBlocoInput `json:"blocos"`
}

// RevisaoInput is the result of one queued review.
type RevisaoInput struct {
	ID       uuid.UUID `json:"-"`
	Questoes int       `json:"questoes"`
	Acertos  int       `json:"acertos"`
}

// RegistroBlocoInput is one discipline's numbers inside a day. When any block is
// sent the day-level totals are derived from them and the client's own totals
// are ignored.
type RegistroBlocoInput struct {
	Disciplina string   `json:"disciplina"`
	Horas      *float64 `json:"horas"`
	Questoes   *int     `json:"questoes"`
	Acertos    *int     `json:"acertos"`
	Nota       string   `json:"nota"`
	Concluido  bool     `json:"concluido"`
	// AtividadeID addresses one scheduled activity. Preferred over Disciplina,
	// which cannot tell two occurrences of the same subject in a day apart.
	AtividadeID string `json:"atividadeId"`
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
	Codigo string          `json:"codigo"`
	Nome   string          `json:"nome"`
	Bloco  string          `json:"bloco"`
	Peso   int             `json:"peso"`
	Cor    int             `json:"cor"` // palette index 0..12
	Temas  []string        `json:"temas"`
	Fontes []FonteResposta `json:"fontes"`
}

// FonteResposta is a study source. Tipo "questoes" is the discipline's question
// bank — the link the day's "abrir no TEC" button uses, with {tema} replaced by
// the topic of the day.
type FonteResposta struct {
	Titulo string `json:"titulo"`
	URL    string `json:"url"`
	Tipo   string `json:"tipo"`
}

type ConteudoResposta struct {
	Tipo  string `json:"tipo"`
	Texto string `json:"texto"`
}

type ConfigResposta struct {
	Inicio        string         `json:"inicio"`
	Prova         string         `json:"prova"`
	HorasDia      float64        `json:"horasDia"` // derivado de minutosBloco × blocosPorDia + cauda de revisão
	DiasEstudo    []int          `json:"diasEstudo"`
	DiaRevisao    int            `json:"diaRevisao"`
	RetaFinalDias int            `json:"retaFinalDias"`
	TemaUI        string         `json:"temaUi"`
	Questoes      map[string]int `json:"questoes"`

	// Study method — flat.
	BlocosPorDia   int                `json:"blocosPorDia"`
	MinutosBloco   int                `json:"minutosBloco"` // duração de um bloco normal; define o dia
	MinutosRevisao int                `json:"minutosRevisao"`
	Reforcos       map[string]float64 `json:"reforcos"`
	CicloRevisao   []CicloItemInput   `json:"cicloRevisao"`
	RevisaoSemanal bool               `json:"revisaoSemanal"`
	Simulados      string             `json:"simulados"`
	Discursiva     bool               `json:"discursiva"`
	Modos          map[string]string  `json:"modos"`
	PctQuestoes    float64            `json:"pctQuestoes"`
	LimiarFraco    int                `json:"limiarFraco"`
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
	// ID addresses this activity for a move. Empty until the plan has been
	// arranged by hand — the UI falls back to whole-day actions until then.
	ID         string `json:"id"`
	Disciplina string `json:"disciplina"`
	Tema       string `json:"tema"`
	Passada    int    `json:"passada"`
	// Movida marks an activity the user placed here, rather than the engine.
	Movida bool `json:"movida"`
}

type BlocoResposta struct {
	Minutos int    `json:"minutos"`
	Titulo  string `json:"titulo"`
	Detalhe string `json:"detalhe"`
}

type RegistroResposta struct {
	Horas     *float64                `json:"horas"`
	Concluido bool                    `json:"concluido"`
	Questoes  *int                    `json:"questoes"`
	Acertos   *int                    `json:"acertos"`
	Erros     *int                    `json:"erros"`
	Nota      string                  `json:"nota"`
	Blocos    []RegistroBlocoResposta `json:"blocos"`
}

type RegistroBlocoResposta struct {
	Disciplina  string   `json:"disciplina"`
	Horas       *float64 `json:"horas"`
	Questoes    *int     `json:"questoes"`
	Acertos     *int     `json:"acertos"`
	Erros       *int     `json:"erros"`
	Nota        string   `json:"nota"`
	Concluido   bool     `json:"concluido"`
	AtividadeID string   `json:"atividadeId"`
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
	QuestoesEdital int     `json:"questoesEdital"`
	Delta          int     `json:"delta"` // questoes - questoesEdital
	Modo           string  `json:"modo"`
	Peso           int     `json:"peso"`
	Pontos         int     `json:"pontos"`
	PctIdeal       float64 `json:"pctIdeal"`
	BlocosConteudo int     `json:"blocosConteudo"`
	BlocosReta     int     `json:"blocosReta"`
	// Temas is how many topics the discipline has.
	//
	// Passadas is how many times the CONTENT phase goes through the whole
	// discipline, and RevisoesGerais how many times the reta final does. Both
	// count complete passes over the SUBJECT, not per topic: "I go through
	// Português 3.5 times before the exam", which is the question a student
	// actually asks.
	Temas    int     `json:"temas"`
	Passadas float64 `json:"passadas"`
	// Revisoes is how many times the daily review queue goes over this
	// discipline's whole topic list before the reta final — the answer to "how
	// many times do I review Português".
	Revisoes       float64 `json:"revisoes"`
	RevisoesGerais float64 `json:"revisoesGerais"`
	// IntervaloDias is how many days pass, on average, between two days that
	// study this discipline. It is what makes the cycle legible: "Português
	// comes back every 7 days" says more about not forgetting it than any
	// count of blocks does. 0 when the discipline appears at most once.
	IntervaloDias float64 `json:"intervaloDias"`
	HorasPrevisto float64 `json:"horasPrevisto"`
	HorasLancado  float64 `json:"horasLancado"`
	Desvio        float64 `json:"desvio"`
	AcertoPct     *int    `json:"acertoPct"`
}

type PropsResposta struct {
	FaltamDias     int     `json:"faltamDias"`
	Progresso      int     `json:"progresso"`
	HorasTotal     float64 `json:"horasTotal"`
	HorasAlvo      float64 `json:"horasAlvo"`
	AcertoPct      *int    `json:"acertoPct"`
	TotalDias      int     `json:"totalDias"`
	DiasConcluidos int     `json:"diasConcluidos"`
	// VoltasRevisao is how many complete laps over everything studied the daily
	// review block gets through before the reta final — "I go over all of it 3.4
	// times". Below 1 the plan does not finish reviewing what it taught.
	VoltasRevisao float64 `json:"voltasRevisao"`
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
	// PorDisciplina is the error notebook itself: what went wrong, per subject,
	// accumulating over the cycle. It is what the daily review tail drills.
	PorDisciplina []CadernoDisciplina `json:"porDisciplina"`
}

// CadernoDisciplina is one discipline's notebook.
type CadernoDisciplina struct {
	Disciplina string                `json:"disciplina"`
	Nome       string                `json:"nome"`
	Cor        int                   `json:"cor"`
	Temas      []ItemCadernoResposta `json:"temas"`
}

type ItemCadernoResposta struct {
	Tema           string `json:"tema"`
	Erros          int    `json:"erros"`
	Questoes       int    `json:"questoes"`
	Acertos        int    `json:"acertos"`
	Aproveitamento int    `json:"aproveitamento"`
	UltimaData     string `json:"ultimaData"`
}

type AnotacaoResposta struct {
	ID         uuid.UUID `json:"id"`
	Data       *string   `json:"data"`
	Disciplina *string   `json:"disciplina"`
	Tema       string    `json:"tema"`
	Texto      string    `json:"texto"`
	Origem     string    `json:"origem"`
	URL        string    `json:"url"`
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
	Tema       string  `json:"tema"`
	Texto      string  `json:"texto"`
	URL        string  `json:"url"`
	Resolvido  bool    `json:"resolvido"`
}

func registroToResposta(r plano.Registro) *RegistroResposta {
	out := &RegistroResposta{
		Horas:     r.Horas,
		Concluido: r.Concluido,
		Questoes:  r.Questoes,
		Acertos:   r.Acertos,
		Erros:     errosDe(r.Questoes, r.Acertos),
		Nota:      r.Nota,
		Blocos:    make([]RegistroBlocoResposta, 0, len(r.Blocos)),
	}

	for _, b := range r.Blocos {
		out.Blocos = append(out.Blocos, RegistroBlocoResposta{
			Disciplina:  b.Disciplina,
			Horas:       b.Horas,
			Questoes:    b.Questoes,
			Acertos:     b.Acertos,
			Erros:       errosDe(b.Questoes, b.Acertos),
			Nota:        b.Nota,
			Concluido:   b.Concluido,
			AtividadeID: b.AtividadeID,
		})
	}

	return out
}

// errosDe derives the wrong-answer count. It is nil unless both numbers are
// recorded, and never negative.
func errosDe(questoes, acertos *int) *int {
	if questoes == nil || acertos == nil {
		return nil
	}

	e := *questoes - *acertos
	if e < 0 {
		e = 0
	}

	return &e
}

func parseISODate(s string) (time.Time, bool) {
	t, err := time.Parse(isoDate, s)
	if err != nil {
		return time.Time{}, false
	}

	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
}

// ImportacaoTECInput carries the spreadsheet the user confirmed, plus the day
// the results belong to (today when empty).
type ImportacaoTECInput struct {
	CSV  string `json:"csv"`
	Data string `json:"data"`
}

// MoverAtividadeInput is a single activity move: which activity, and where it
// lands. Deliberately minimal — the API sends only what changed, not the whole
// schedule.
type MoverAtividadeInput struct {
	ID      string `json:"id"`
	Data    string `json:"data"` // YYYY-MM-DD
	Posicao int    `json:"posicao"`
	// Trocar swaps with whatever occupies the target slot instead of inserting
	// beside it, so neither day changes size. Falls back to a plain move when
	// the slot is empty.
	Trocar bool `json:"trocar"`
}
