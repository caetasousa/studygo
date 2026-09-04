package plano

// Os avisos que o plano dá sobre si mesmo.
//
// Aqui mora só a DECISÃO — "este plano não cobre todo o conteúdo", "faltam 12
// questões para o edital fechar". A redação em português é do adapter HTTP: a
// regra de quando avisar é do domínio e não pode depender de como a frase é
// escrita.

// Severidade é o quanto um aviso importa.
type Severidade string

const (
	// SeveridadeAviso: dá para conviver, mas é bom saber.
	SeveridadeAviso Severidade = "warn"
	// SeveridadePerigo: o plano não entrega o que promete.
	SeveridadePerigo Severidade = "danger"
)

// DisciplinaIncompleta é uma matéria que o plano não percorre inteira nem uma
// vez. Passadas é a fração coberta: 0 significa que ela não entra no plano.
type DisciplinaIncompleta struct {
	Codigo   string
	Nome     string
	Passadas float64
}

// AlertaCobertura é a falha que um plano de estudo nunca pode esconder: com a
// prova perto, ou com conteúdo demais para os dias disponíveis, o motor
// simplesmente fica sem vagas e matérias inteiras não aparecem. O cronograma
// parecia completo dos dois jeitos — a matéria que falta só estava ausente, e
// ausência é difícil de notar.
type AlertaCobertura struct {
	Severidade  Severidade
	Incompletas []DisciplinaIncompleta
	// SemNenhuma é quantas matérias não entram no plano de jeito nenhum.
	SemNenhuma int
}

// AlertaOrcamento é o descasamento entre as questões distribuídas e as que o
// edital cobra. O motor divide o tempo em proporção estrita, então aumentar uma
// matéria tira tempo de todas as outras — é isto que diz isso em voz alta.
type AlertaOrcamento struct {
	Severidade   Severidade
	Distribuidas int
	NoEdital     int
	// Sobra é distribuídas - edital: positiva sobra, negativa falta.
	Sobra int
	// MaisForaDoEixo são as duas matérias mais distantes do edital na direção
	// que causou o desequilíbrio.
	MaisForaDoEixo []DisciplinaDesviada
}

// DisciplinaDesviada é uma matéria e o quanto ela se afasta do edital.
type DisciplinaDesviada struct {
	Codigo string
	Nome   string
	Delta  int
}

// CoberturaDoPlano avalia se o plano percorre cada matéria ao menos uma vez.
// Devolve nil quando cobre tudo.
func CoberturaDoPlano(linhas []LinhaCobertura) *AlertaCobertura {
	out := AlertaCobertura{Severidade: SeveridadeAviso}

	// Uma matéria sem temas cadastrados NÃO é ignorada: o motor usa o nome dela
	// como manchete do dia, então ela é ensinada como qualquer outra. Calar por
	// falta de tema esconderia justamente o buraco maior — um concurso importado
	// do edital chega só com os nomes, e é aí que o aviso mais importa.
	for _, l := range linhas {
		if l.Passadas >= 1 {
			continue
		}

		out.Incompletas = append(out.Incompletas, DisciplinaIncompleta{
			Codigo:   l.Codigo,
			Nome:     l.Nome,
			Passadas: l.Passadas,
		})

		if l.Passadas == 0 {
			out.SemNenhuma++
		}
	}

	if len(out.Incompletas) == 0 {
		return nil
	}

	// Uma matéria que não aparece de jeito nenhum é um problema de outra ordem
	// que uma apenas encurtada.
	if out.SemNenhuma > 0 {
		out.Severidade = SeveridadePerigo
	}

	return &out
}

// OrcamentoDoPlano avalia se as questões distribuídas fecham com o edital.
// Devolve nil quando fecham.
func OrcamentoDoPlano(linhas []LinhaCobertura) *AlertaOrcamento {
	var distribuidas, edital int

	for _, l := range linhas {
		distribuidas += l.Questoes
		edital += l.QuestoesEdital
	}

	if edital == 0 || distribuidas == edital {
		return nil
	}

	sobra := distribuidas - edital

	out := AlertaOrcamento{
		Severidade:   SeveridadeAviso,
		Distribuidas: distribuidas,
		NoEdital:     edital,
		Sobra:        sobra,
	}

	// Um quarto do edital fora do lugar deixa de ser ajuste fino.
	if abs(sobra)*4 > edital {
		out.Severidade = SeveridadePerigo
	}

	direcao := 1
	if sobra < 0 {
		direcao = -1
	}

	fora := make([]DisciplinaDesviada, 0, len(linhas))

	for _, l := range linhas {
		if l.Delta*direcao > 0 {
			fora = append(fora, DisciplinaDesviada{
				Codigo: l.Codigo, Nome: l.Nome, Delta: l.Delta,
			})
		}
	}

	// As duas mais distantes bastam: uma lista completa vira ruído.
	maioresPrimeiro(fora, direcao)

	if len(fora) > 2 {
		fora = fora[:2]
	}

	out.MaisForaDoEixo = fora

	return &out
}

// LinhaCobertura é o que os dois alertas precisam saber de cada disciplina.
// Fica no domínio porque é a entrada de uma regra de domínio; a tela de
// balanceamento mostra bem mais que isto.
type LinhaCobertura struct {
	Codigo         string
	Nome           string
	Temas          int
	Passadas       float64
	Questoes       int
	QuestoesEdital int
	Delta          int
}

func maioresPrimeiro(xs []DisciplinaDesviada, direcao int) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j].Delta*direcao > xs[j-1].Delta*direcao; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}

	return x
}
