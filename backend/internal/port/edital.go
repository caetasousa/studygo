package port

import (
	"context"
	"errors"
)

// ErrImportacaoIndisponivel is returned when the edital processor is not
// configured or cannot be reached. Only AI import is affected — manual
// registration and the rest of the API keep working.
var ErrImportacaoIndisponivel = errors.New("importação por IA indisponível")

// ErrProvedorIndisponivel wraps a transient failure from the edital processor —
// an overloaded model, an upstream 5xx, a timeout. The request is not malformed
// and can be retried later, so the API surfaces it as a 503 with a retry hint.
var ErrProvedorIndisponivel = errors.New("processador de editais indisponível no momento")

// EditalUpload is the raw edital as it first arrives: the uploaded PDF, or text
// pasted by the user. After step 1 it is replaced by an opaque DocumentoID.
type EditalUpload struct {
	PDF   []byte
	MIME  string
	Texto string
}

// Vazia reports whether there is nothing to send.
func (u EditalUpload) Vazia() bool {
	return len(u.PDF) == 0 && u.Texto == ""
}

// EditalCargo is one position offered by an edital. Vagas is nil when the edital
// did not state a number (never 0 as a stand-in).
type EditalCargo struct {
	Codigo        string
	Nome          string
	Especialidade string
	Escolaridade  string
	Vagas         *int
}

// EditalAlerta is a review flag the processor raised: a missing field, a group
// that could not be mapped, a question count that does not add up. Gravidade is
// "info", "warning" or "blocker".
type EditalAlerta struct {
	Codigo    string
	Gravidade string
	Mensagem  string
	Campo     string // JSON Pointer, optional
}

// EditalAnalise is wizard step 1: the document handle plus the cheap top-level
// facts.
type EditalAnalise struct {
	DocumentoID  string
	Banca        string
	TotalPaginas int
	PaginasOCR   int
	Cargos       []EditalCargo
	Alertas      []EditalAlerta
}

// EditalDisciplina is a subject within a knowledge group. Questoes and Peso are
// nil unless the edital stated them for that discipline specifically.
type EditalDisciplina struct {
	Nome     string
	Questoes *int
	Peso     *float64
}

// EditalGrupo is a knowledge group. Kind is "ger", "esp" or "outro"; PesoEscopo
// is "group", "discipline" or "".
type EditalGrupo struct {
	Kind        string
	Rotulo      string
	Total       *int
	Peso        *float64
	PesoEscopo  string
	Disciplinas []EditalDisciplina
}

// EditalDiscursiva describes a discursive exam. Modalidade is "redacao",
// "estudo_de_caso" or "outro".
type EditalDiscursiva struct {
	Modalidade string
	Rotulo     string
	Questoes   *int
}

// EditalDuracao is the exam-time limit. Escopo is "exam_set", "single_prova" or
// "unknown".
type EditalDuracao struct {
	Minutos int
	Escopo  string
}

// EditalMarco is a dated milestone from the edital schedule.
type EditalMarco struct {
	Data      string // YYYY-MM-DD
	DataFim   string // YYYY-MM-DD, optional
	Titulo    string
	ExigeAcao bool
}

// EditalEstrutura is wizard step 2: the groups and exam basics for one cargo.
type EditalEstrutura struct {
	NomeSugerido      string
	DataProva         string // YYYY-MM-DD
	GruposGerais      []EditalGrupo
	GruposEspecificos []EditalGrupo
	Discursivas       []EditalDiscursiva
	Duracao           *EditalDuracao
	Marcos            []EditalMarco
	Alertas           []EditalAlerta
}

// EditalConteudoDisciplina is one discipline's extracted syllabus topics.
type EditalConteudoDisciplina struct {
	Nome  string
	Temas []string
}

// EditalConteudo is wizard step 3.
type EditalConteudo struct {
	Itens   []EditalConteudoDisciplina
	Alertas []EditalAlerta
}

// EditalProcessor talks to the internal service that turns an edital PDF into a
// reviewable structured preview. That service does the PDF work, OCR and LLM
// calls; this backend orchestrates the wizard and persists the result only
// after the user confirms.
type EditalProcessor interface {
	// Disponivel reports whether the processor is configured.
	Disponivel() bool
	// Analisar uploads the edital and returns the document handle plus cargos.
	// ownerRef is an opaque per-user handle the processor binds to the
	// document; later steps must present the same one.
	Analisar(ctx context.Context, ownerRef string, up EditalUpload) (EditalAnalise, error)
	// Estrutura extracts the exam structure for one cargo.
	Estrutura(ctx context.Context, ownerRef, documentoID, cargo string) (EditalEstrutura, error)
	// Conteudo extracts the syllabus topics for the given disciplines. It also
	// accepts a fresh upload (documentoID empty, up non-empty) so the
	// edit-screen "extract topics" flow keeps working without the wizard.
	Conteudo(ctx context.Context, ownerRef, documentoID, cargo string, disciplinas []string, up EditalUpload) (EditalConteudo, error)
}
