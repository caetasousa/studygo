package service

import (
	"time"

	"github.com/google/uuid"
)

// Os RESULTADOS da aplicação: o que os casos de uso devolvem.
//
// Nenhum destes tipos tem tag JSON. O contrato de transporte vive em
// adapter/httpapi, que traduz isto em DTO — assim mudar o formato do payload
// não obriga a mexer na aplicação, e a aplicação pode ganhar um campo sem que
// ele vaze para a API sem alguém decidir.

// PlanoMontado é o plano inteiro pronto para a tela.
type PlanoMontado struct {
	Concurso      ConcursoDoPlano
	Config        ConfigDoPlano
	Dias          []DiaDoPlano
	Marcos        []MarcoDoPlano
	Balanceamento []LinhaBalanceamento
	Props         ResumoDoPlano
	Alertas       []Alerta
	HojeIndex     *int
	// TemMovimentacaoManual diz se o estudante rearranjou alguma matéria; é o
	// que habilita "restaurar a ordem automática" na tela.
	TemMovimentacaoManual bool
	GeradoEm              time.Time
}

// ConcursoDoPlano é o catálogo que a SPA precisa para desenhar as telas.
type ConcursoDoPlano struct {
	Slug        string
	Nome        string
	Banca       string
	Cargo       string
	Emoji       string
	Resumo      string
	Disciplinas []DisciplinaDoPlano
	Conteudo    []ItemDeConteudo
}

// DisciplinaDoPlano é uma matéria como a tela a mostra. Codigo é o mnemônico
// exibido nos chips do cronograma — não existe um segundo campo "sigla": o
// código É a sigla, e ter os dois era o que deixava a tela discordar do banco.
type DisciplinaDoPlano struct {
	Codigo     string
	Nome       string
	Bloco      string
	Peso       int
	Cor        int
	CadernoURL string
	Temas      []string
	Fontes     []FonteDoPlano
}

// FonteDoPlano é uma origem de estudo. Tipo "questoes" é o banco de questões da
// matéria, que o botão "abrir no TEC" do dia usa.
type FonteDoPlano struct {
	Titulo string
	URL    string
	Tipo   string
}

// ItemDeConteudo é um bloco da página de conteúdo programático.
type ItemDeConteudo struct {
	Tipo  string
	Texto string
}

// ConfigDoPlano é a configuração como a tela de ajustes a edita.
type ConfigDoPlano struct {
	Inicio        string
	Prova         string
	HorasDia      float64
	DiasEstudo    []int
	DiaRevisao    int
	RetaFinalDias int
	TemaUI        string
	Questoes      map[string]int

	BlocosPorDia   int
	MinutosBloco   int
	MinutosRevisao int
	Reforcos       map[string]float64
	CicloRevisao   []ItemDoCiclo
	RevisaoSemanal bool
	Simulados      string
	Discursiva     bool
	Modos          map[string]string
	PctQuestoes    float64
	LimiarFraco    int
}

// ItemDoCiclo é uma semana da rotação de revisão.
type ItemDoCiclo struct {
	Titulo   string
	Questoes int
}

// DiaDoPlano é um dia do cronograma com o que foi registrado nele.
type DiaDoPlano struct {
	N      int
	Data   string
	Semana int
	Fase   string
	Tipo   string
	Itens  []AtividadeDoDia
	Tema   string
	Meta   int
	Blocos []BlocoDoDia
	// Concluido é DERIVADO das atividades do dia, nunca informado pelo cliente.
	Concluido bool
	Horas     *float64
	Questoes  *int
	Acertos   *int
	Nota      string
	Revisao   *RevisaoDoDia
}

// AtividadeDoDia é um bloco agendado. Todo bloco tem id desde o primeiro
// carregamento: o cronograma é gravado quando o plano nasce.
type AtividadeDoDia struct {
	ID         uuid.UUID
	Disciplina string
	Tema       string
	Passada    int
	// Movida marca a atividade que o estudante colocou ali, e não o motor.
	Movida    bool
	Horas     *float64
	Questoes  *int
	Acertos   *int
	Erros     *int
	Nota      string
	Concluido bool
}

// BlocoDoDia é uma fatia de tempo da rotina do dia.
type BlocoDoDia struct {
	Minutos int
	Titulo  string
	Detalhe string
}

// RevisaoDoDia é a cauda de revisão: o que registrar e a observação já salva.
type RevisaoDoDia struct {
	Disciplina string
	Questoes   *int
	Acertos    *int
	Observacao string
}

// MarcoDoPlano é uma data do edital com seu check.
type MarcoDoPlano struct {
	ID         uuid.UUID
	Rotulo     int
	DataInicio string
	DataFim    *string
	Titulo     string
	ExigeAcao  bool
	EProva     bool
	Cumprido   bool
}

// LinhaBalanceamento é uma linha da tela de balanceamento.
type LinhaBalanceamento struct {
	Codigo         string
	Nome           string
	Bloco          string
	Cor            int
	Questoes       int
	QuestoesEdital int
	Delta          int
	Modo           string
	Peso           int
	Pontos         int
	PctIdeal       float64
	BlocosConteudo int
	BlocosReta     int
	// Temas é quantos tópicos a matéria tem; Passadas, quantas vezes a fase de
	// conteúdo percorre a matéria INTEIRA, e RevisoesGerais o mesmo na reta
	// final. Ambas contam passadas completas pela MATÉRIA, não por tópico:
	// "eu passo por Português 3,5 vezes antes da prova" é a pergunta que o
	// estudante realmente faz.
	Temas          int
	Passadas       float64
	Visitas        int
	RevisoesGerais float64
	// IntervaloDias é de quantos em quantos dias, em média, a matéria volta.
	IntervaloDias float64
	HorasPrevisto float64
	HorasLancado  float64
	Desvio        float64
	AcertoPct     *int
}

// ResumoDoPlano são os números do topo da tela.
type ResumoDoPlano struct {
	FaltamDias     int
	Progresso      int
	HorasTotal     float64
	HorasAlvo      float64
	AcertoPct      *int
	TotalDias      int
	DiasConcluidos int
	// VoltasRevisao é quantas voltas completas por tudo que foi estudado a
	// revisão diária consegue dar antes da reta final. Abaixo de 1 o plano não
	// termina de revisar o que ensinou.
	VoltasRevisao float64
}

// Alerta é um aviso do plano sobre si mesmo, já redigido.
type Alerta struct {
	Nivel  string
	Titulo string
	Texto  string
}

// Estatisticas é a tela de estatísticas.
type Estatisticas struct {
	Serie         []PontoDaSerie
	PorSemana     []ResumoDaSemana
	PorDisciplina []LinhaBalanceamento
	Streak        int
	HorasTotal    float64
	QuestoesTotal int
	AcertoPct     *int
}

// PontoDaSerie é um dia da série histórica.
type PontoDaSerie struct {
	Data     string
	Horas    float64
	Questoes int
	Acertos  int
}

// ResumoDaSemana agrega uma semana do plano. HorasPrevisto é o que o plano
// pedia; Horas é o que foi lançado — a comparação entre os dois é o gráfico.
type ResumoDaSemana struct {
	Semana        int
	HorasPrevisto float64
	Horas         float64
	Questoes      int
	Acertos       int
}

// Caderno é o caderno de erros.
type Caderno struct {
	PorDisciplina []CadernoDaDisciplina
	Anotacoes     []AnotacaoDoCaderno
	// DiasComNota são as anotações que o estudante escreveu no dia — o "porquê"
	// que acompanha os números.
	DiasComNota []DiaComNota
	DiasFracos  []DiaFraco
}

// DiaComNota é um dia em que o estudante deixou uma anotação.
type DiaComNota struct {
	Data        string
	N           int
	Disciplinas []string
	Nota        string
}

// CadernoDaDisciplina são os temas fracos de uma matéria.
type CadernoDaDisciplina struct {
	Codigo string
	Nome   string
	Cor    int
	Itens  []ItemDoCaderno
}

// ItemDoCaderno é um tema com aproveitamento abaixo do limiar.
type ItemDoCaderno struct {
	Tema       string
	Questoes   int
	Acertos    int
	Erros      int
	Aprov      int
	UltimaData string
}

// AnotacaoDoCaderno é uma anotação do estudante.
type AnotacaoDoCaderno struct {
	ID         uuid.UUID
	Data       *string
	Disciplina string
	Tema       string
	Texto      string
	Origem     string
	URL        string
	Resolvido  bool
}

// DiaFraco é um dia com aproveitamento abaixo do limiar.
type DiaFraco struct {
	Data     string
	N        int
	Questoes int
	Acertos  int
	Aprov    int
}
