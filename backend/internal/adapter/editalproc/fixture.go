package editalproc

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"studygo/internal/port"
)

// Um edital de verdade, congelado.
//
// `testdata/edital_fcc_tcego.json` é a extração real de um edital da FCC
// (TCE-GO, 2 cargos, 25 páginas), capturada UMA vez contra o edital-processor
// com a Gemini de verdade. A partir daí ela é replay: os testes não chamam a
// API, não gastam cota, não dependem de rede e dão sempre o mesmo resultado.
//
// Isto substitui reimportar o PDF a cada execução. O que ela NÃO substitui é a
// suíte Python do processor, que continua sendo quem testa PDF, OCR e o
// contrato com a Gemini — este pacote só consome o resultado.
//
// A fixture é dado esperado, não seed de banco: um campo novo no contrato do
// processor deve fazê-la falhar, e a correção é recapturar, não editar à mão.

//go:embed testdata/edital_fcc_tcego.json
var editalFCC []byte

// EditalFixture é a extração completa de um edital, por cargo.
type EditalFixture struct {
	Analise   fixtureAnalise              `json:"analise"`
	Estrutura map[string]fixtureEstrutura `json:"estrutura"`
	Conteudo  map[string]fixtureConteudo  `json:"conteudo"`
}

type fixtureAnalise struct {
	Banca        string          `json:"banca"`
	TotalPaginas int             `json:"totalPaginas"`
	PaginasOCR   int             `json:"paginasOcr"`
	Cargos       []fixtureCargo  `json:"cargos"`
	Alertas      []fixtureAlerta `json:"alertas"`
}

type fixtureCargo struct {
	Codigo        string `json:"codigo"`
	Nome          string `json:"nome"`
	Especialidade string `json:"especialidade"`
	Escolaridade  string `json:"escolaridade"`
	Vagas         *int   `json:"vagas"`
}

type fixtureAlerta struct {
	Codigo    string `json:"codigo"`
	Gravidade string `json:"gravidade"`
	Mensagem  string `json:"mensagem"`
	Campo     string `json:"campo"`
}

type fixtureEstrutura struct {
	Nome        string              `json:"nome"`
	Prova       string              `json:"prova"`
	Gerais      []fixtureGrupo      `json:"gerais"`
	Especificas []fixtureGrupo      `json:"especificas"`
	Discursivas []fixtureDiscursiva `json:"discursivas"`
	Duracao     *fixtureDuracao     `json:"duracao"`
	Marcos      []fixtureMarco      `json:"marcos"`
	Alertas     []fixtureAlerta     `json:"alertas"`
}

type fixtureGrupo struct {
	Kind        string              `json:"kind"`
	Rotulo      string              `json:"rotulo"`
	Total       *int                `json:"total"`
	Peso        *float64            `json:"peso"`
	PesoEscopo  string              `json:"pesoEscopo"`
	Disciplinas []fixtureDisciplina `json:"disciplinas"`
}

type fixtureDisciplina struct {
	Nome     string   `json:"nome"`
	Questoes *int     `json:"questoes"`
	Peso     *float64 `json:"peso"`
}

type fixtureDiscursiva struct {
	Modalidade string `json:"modalidade"`
	Rotulo     string `json:"rotulo"`
	Questoes   *int   `json:"questoes"`
}

type fixtureDuracao struct {
	Minutos int    `json:"minutos"`
	Escopo  string `json:"escopo"`
}

type fixtureMarco struct {
	Data      string `json:"data"`
	DataFim   string `json:"dataFim"`
	Titulo    string `json:"titulo"`
	ExigeAcao bool   `json:"exigeAcao"`
}

type fixtureConteudo struct {
	Itens   []fixtureItemConteudo `json:"itens"`
	Alertas []fixtureAlerta       `json:"alertas"`
}

type fixtureItemConteudo struct {
	Nome  string   `json:"nome"`
	Temas []string `json:"temas"`
}

var (
	umaVez  sync.Once
	fixture EditalFixture
	errFix  error
)

// Fixture devolve o edital congelado, decodificado uma vez.
func Fixture() (EditalFixture, error) {
	umaVez.Do(func() {
		errFix = json.Unmarshal(editalFCC, &fixture)
	})

	return fixture, errFix
}

// DeFixture é um EditalProcessor que responde a partir do edital congelado, sem
// rede e sem Gemini.
//
// O `documentoID` que ele devolve é fixo: os passos seguintes só precisam
// apresentá-lo de volta, e inventar um uuid por chamada tornaria o teste
// não-determinístico sem ganho nenhum.
type DeFixture struct {
	dados EditalFixture
}

var _ port.EditalProcessor = (*DeFixture)(nil)

// documentoDaFixture é o handle que os três passos compartilham.
const documentoDaFixture = "fixture-fcc-tcego"

// NovoDeFixture devolve o processor de fixture. Erra se a fixture embutida não
// puder ser lida — o que só acontece se ela for corrompida no repositório.
func NovoDeFixture() (*DeFixture, error) {
	dados, err := Fixture()
	if err != nil {
		return nil, fmt.Errorf("lendo a fixture do edital: %w", err)
	}

	return &DeFixture{dados: dados}, nil
}

func (d *DeFixture) Disponivel() bool { return true }

func (d *DeFixture) Analisar(
	_ context.Context,
	_ string,
	up port.EditalUpload,
) (port.EditalAnalise, error) {
	if up.Vazia() {
		return port.EditalAnalise{}, fmt.Errorf("nenhum edital enviado")
	}

	a := d.dados.Analise

	cargos := make([]port.EditalCargo, 0, len(a.Cargos))
	for _, c := range a.Cargos {
		cargos = append(cargos, port.EditalCargo{
			Codigo: c.Codigo, Nome: c.Nome, Especialidade: c.Especialidade,
			Escolaridade: c.Escolaridade, Vagas: c.Vagas,
		})
	}

	return port.EditalAnalise{
		DocumentoID:  documentoDaFixture,
		Banca:        a.Banca,
		TotalPaginas: a.TotalPaginas,
		PaginasOCR:   a.PaginasOCR,
		Cargos:       cargos,
		Alertas:      alertas(a.Alertas),
	}, nil
}

func (d *DeFixture) Estrutura(
	_ context.Context,
	_, documentoID, cargo string,
) (port.EditalEstrutura, error) {
	if documentoID != documentoDaFixture {
		return port.EditalEstrutura{}, fmt.Errorf("documento %q não está na fixture", documentoID)
	}

	e, ok := d.dados.Estrutura[cargo]
	if !ok {
		return port.EditalEstrutura{}, fmt.Errorf(
			"cargo %q não está na fixture (tem: %v)", cargo, d.cargos(),
		)
	}

	discursivas := make([]port.EditalDiscursiva, 0, len(e.Discursivas))
	for _, x := range e.Discursivas {
		discursivas = append(discursivas, port.EditalDiscursiva{
			Modalidade: x.Modalidade, Rotulo: x.Rotulo, Questoes: x.Questoes,
		})
	}

	marcos := make([]port.EditalMarco, 0, len(e.Marcos))
	for _, m := range e.Marcos {
		marcos = append(marcos, port.EditalMarco{
			Data: m.Data, DataFim: m.DataFim, Titulo: m.Titulo, ExigeAcao: m.ExigeAcao,
		})
	}

	var duracao *port.EditalDuracao
	if e.Duracao != nil {
		duracao = &port.EditalDuracao{Minutos: e.Duracao.Minutos, Escopo: e.Duracao.Escopo}
	}

	return port.EditalEstrutura{
		NomeSugerido:      e.Nome,
		DataProva:         e.Prova,
		GruposGerais:      grupos(e.Gerais),
		GruposEspecificos: grupos(e.Especificas),
		Discursivas:       discursivas,
		Duracao:           duracao,
		Marcos:            marcos,
		Alertas:           alertas(e.Alertas),
	}, nil
}

func (d *DeFixture) Conteudo(
	_ context.Context,
	_, documentoID, cargo string,
	disciplinas []string,
	_ port.EditalUpload,
) (port.EditalConteudo, error) {
	c, ok := d.dados.Conteudo[cargo]
	if !ok {
		return port.EditalConteudo{}, fmt.Errorf("cargo %q não está na fixture", cargo)
	}

	// Só as disciplinas pedidas voltam — é o que o processor de verdade faz, e
	// um teste que peça três não deve receber catorze.
	pedidas := make(map[string]bool, len(disciplinas))
	for _, nome := range disciplinas {
		pedidas[nome] = true
	}

	itens := make([]port.EditalConteudoDisciplina, 0, len(c.Itens))

	for _, it := range c.Itens {
		if len(pedidas) > 0 && !pedidas[it.Nome] {
			continue
		}

		itens = append(itens, port.EditalConteudoDisciplina{Nome: it.Nome, Temas: it.Temas})
	}

	return port.EditalConteudo{Itens: itens, Alertas: alertas(c.Alertas)}, nil
}

func (d *DeFixture) cargos() []string {
	out := make([]string, 0, len(d.dados.Estrutura))
	for c := range d.dados.Estrutura {
		out = append(out, c)
	}

	return out
}

func grupos(gs []fixtureGrupo) []port.EditalGrupo {
	out := make([]port.EditalGrupo, 0, len(gs))

	for _, g := range gs {
		discs := make([]port.EditalDisciplina, 0, len(g.Disciplinas))
		for _, d := range g.Disciplinas {
			discs = append(discs, port.EditalDisciplina{
				Nome: d.Nome, Questoes: d.Questoes, Peso: d.Peso,
			})
		}

		out = append(out, port.EditalGrupo{
			Kind: g.Kind, Rotulo: g.Rotulo, Total: g.Total,
			Peso: g.Peso, PesoEscopo: g.PesoEscopo, Disciplinas: discs,
		})
	}

	return out
}

func alertas(as []fixtureAlerta) []port.EditalAlerta {
	out := make([]port.EditalAlerta, 0, len(as))

	for _, a := range as {
		out = append(out, port.EditalAlerta{
			Codigo: a.Codigo, Gravidade: a.Gravidade,
			Mensagem: a.Mensagem, Campo: a.Campo,
		})
	}

	return out
}
