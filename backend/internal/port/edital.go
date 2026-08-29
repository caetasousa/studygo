package port

import (
	"context"
	"errors"
)

// ErrImportacaoIndisponivel is returned by the null analyser when no AI provider
// is configured.
var ErrImportacaoIndisponivel = errors.New("importação por IA indisponível")

// EditalEntrada is the edital as the analyser sees it. Asking a model to
// transcribe a whole edital does not work, so the source travels with every
// call, in whichever of these forms is cheapest:
//   - ArquivoURI — a PDF already uploaded to the provider; later steps just
//     reference it instead of re-sending megabytes.
//   - Texto — extracted locally when the PDF had a text layer.
//   - PDF — raw bytes, for the first call with a scanned file.
type EditalEntrada struct {
	Texto      string
	PDF        []byte
	MIME       string
	ArquivoURI string
}

// Vazia reports whether there is nothing to analyse.
func (e EditalEntrada) Vazia() bool {
	return e.Texto == "" && len(e.PDF) == 0 && e.ArquivoURI == ""
}

// EditalCargo is one position offered by an edital (a single edital usually has
// several).
type EditalCargo struct {
	Codigo       string
	Nome         string
	Escolaridade string
	Vagas        int
}

// EditalCargos is the first wizard step: the positions plus whatever handle the
// client should reuse on the later steps — the extracted text, or the URI of the
// PDF the analyser uploaded on its behalf.
type EditalCargos struct {
	Texto      string
	ArquivoURI string
	MIME       string
	Banca      string
	Cargos     []EditalCargo
}

// EditalDisciplina is a subject with an optional per-discipline question count.
type EditalDisciplina struct {
	Nome     string
	Questoes int
}

// EditalMarco is a dated milestone from the edital schedule.
type EditalMarco struct {
	Data      string // YYYY-MM-DD
	DataFim   string // YYYY-MM-DD, optional
	Titulo    string
	ExigeAcao bool
}

// EditalEstrutura is the second wizard step: the disciplines and exam basics for
// the chosen cargo. Marcos is filled by the service from Cronograma().
type EditalEstrutura struct {
	Nome             string // suggested concurso name (órgão + cargo)
	Prova            string // YYYY-MM-DD
	ProvaDiscursiva  bool
	Gerais           []EditalDisciplina
	TotalGerais      int
	Especificas      []EditalDisciplina
	TotalEspecificas int
	Marcos           []EditalMarco
}

// EditalConteudoDisciplina is one discipline's extracted syllabus topics.
type EditalConteudoDisciplina struct {
	Nome  string
	Temas []string
}

// EditalAnalisador reads an edital in focused steps so each AI call stays small
// and fast and the user chooses their cargo along the way.
type EditalAnalisador interface {
	// Disponivel reports whether a real provider is wired.
	Disponivel() bool
	// Cargos lists the positions offered by the edital.
	Cargos(ctx context.Context, in EditalEntrada) (EditalCargos, error)
	// Estrutura extracts the disciplines and exam basics for one cargo.
	Estrutura(ctx context.Context, in EditalEntrada, cargo string) (EditalEstrutura, error)
	// Cronograma extracts the edital-wide schedule. The service runs it in
	// parallel with Estrutura.
	Cronograma(ctx context.Context, in EditalEntrada) ([]EditalMarco, error)
	// Conteudo extracts the syllabus topics for the given disciplines.
	Conteudo(ctx context.Context, in EditalEntrada, disciplinas []string) ([]EditalConteudoDisciplina, error)
}
