package plano

import (
	"strings"
	"time"

	"studygo/internal/domain/concurso"
)

// Config são as escolhas do usuário para o plano: as datas e o ritmo, mais o
// método de estudo em si. Cada botão — quantos blocos por dia, quanto dura cada
// um, se simulados entram, o peso extra por disciplina — mora nesta única struct
// que o motor consome.
//
// O valor zero não serve: Normalizar preenche cada campo de método com um padrão
// razoável, e Gerar a chama. Um Simulados vazio é o que marca a metade "método"
// da struct como nunca preenchida.
type Config struct {
	// Datas e ritmo.
	Inicio        time.Time
	Prova         time.Time
	HorasDia      float64 // orçamento diário que o motor divide; derivado de MinutosBloco quando ele existe
	DiasEstudo    []int   // dias da semana, 0=domingo..6=sábado
	DiaRevisao    int
	RetaFinalDias int
	Questoes      map[string]int // código da disciplina -> questões estimadas

	// Método de estudo.
	BlocosPorDia int // quantas disciplinas um dia de estudo cobre
	MinutosBloco int // duração de um bloco normal; 0 = derivar de HorasDia
	// MinutosRevisao é quanto dura o bloco de revisão do dia. Ele fica ao lado
	// dos blocos de conteúdo com duração própria, em vez de comer uma
	// porcentagem deles; 0 significa que o dia não tem bloco de revisão.
	MinutosRevisao int
	Reforcos       map[string]float64     // peso extra por disciplina (1 = normal)
	CicloRevisao   []concurso.ItemRevisao // rotação da revisão semanal; vazia = RevCicloPadrao
	// RevisaoSemanal reserva um dia inteiro da semana para revisão. Desligada
	// por padrão: revisão é um bloco diário (ver MinutosRevisao), alimentado
	// pelo caderno de erros, então entregar um dia inteiro a ela custa conteúdo
	// sem ganho. Continua existindo como chave porque alguns métodos de estudo
	// realmente querem o dia.
	RevisaoSemanal bool
	Simulados      Frequencia      // com que frequência um simulado completo aparece na reta final
	Discursiva     bool            // reservar um dia de discursiva na reta final
	Modos          map[string]Modo // como cada disciplina é estudada
	PctQuestoes    float64         // fatia do bloco de estudo gasta em questões
	LimiarFraco    int             // % abaixo do qual uma bateria conta como fraca
}

// Frequencia é com que frequência um simulado completo aparece na reta final.
type Frequencia string

const (
	SimuladoNunca     Frequencia = "nunca"
	SimuladoQuinzenal Frequencia = "quinzenal"
	SimuladoSemanal   Frequencia = "semanal"
)

// Modo é como uma disciplina é estudada.
type Modo string

const (
	ModoCompleto Modo = "completo" // teoria com resumo, depois questões
	ModoQuestoes Modo = "questoes" // só questões
	ModoTeoria   Modo = "teoria"   // só teoria
)

// Limites dos campos ajustáveis, compartilhados com a validação da aplicação.
const (
	BlocosMin       = 1
	BlocosMax       = 6
	ReforcoMin      = 0.25
	ReforcoMax      = 3
	MinutosBlocoMin = 15
	MinutosBlocoMax = 240
)

// ConfigPadrao devolve os padrões do método de estudo: um simulado completo no
// último dia de cada semana da reta final, uma discursiva no dia anterior e dois
// blocos por dia. As datas e a contagem de questões ficam zeradas para quem
// chama preencher.
func ConfigPadrao() Config {
	return Config{
		BlocosPorDia:   2,
		MinutosRevisao: 20,
		Reforcos:       map[string]float64{},
		RevisaoSemanal: false,
		Simulados:      SimuladoSemanal,
		Discursiva:     true,
		Modos:          map[string]Modo{},
		PctQuestoes:    0.5,
		LimiarFraco:    70,
	}
}

// Normalizar limita cada campo de método a um valor utilizável, para que uma
// configuração vinda do banco ou de um payload da API nunca produza um plano
// quebrado. É idempotente. As datas, os dias de estudo e o mapa de questões são
// validados pela aplicação e não são tocados aqui.
func (c Config) Normalizar() Config {
	d := ConfigPadrao()

	// Simulados == "" significa que o método de estudo nunca foi escolhido:
	// adote os padrões. A única exceção é BlocosPorDia — um valor salvo e dentro
	// da faixa é a resposta do usuário a "quantas disciplinas por dia", e
	// sobrescrevê-lo aqui era o que revertia 3 blocos para 2 na carga seguinte,
	// deixando o cronograma mostrando duas disciplinas por dia.
	//
	// Booleanos e porcentagens continuam incondicionais: o zero deles é
	// indistinguível de "nunca definido", então preservá-los congelaria uma
	// pergunta não respondida como se fosse resposta (Discursiva=false, por
	// exemplo).
	if c.Simulados == "" {
		modos, reforcos, ciclo := c.Modos, c.Reforcos, c.CicloRevisao
		blocos := c.BlocosPorDia

		c.BlocosPorDia = d.BlocosPorDia
		c.MinutosRevisao = d.MinutosRevisao
		c.Simulados = d.Simulados
		c.Discursiva = d.Discursiva
		c.PctQuestoes = d.PctQuestoes
		c.LimiarFraco = d.LimiarFraco

		if blocos >= BlocosMin && blocos <= BlocosMax {
			c.BlocosPorDia = blocos
		}

		c.Modos = modosNaoNulos(modos)
		c.Reforcos = reforcosNaoNulos(reforcos)
		c.CicloRevisao = cicloValido(ciclo)
		c.MinutosBloco = minutosBlocoValido(c.MinutosBloco)
		c.HorasDia = horasDiaEfetiva(c)

		return c
	}

	switch c.Simulados {
	case SimuladoNunca, SimuladoQuinzenal, SimuladoSemanal:
	default:
		c.Simulados = d.Simulados
	}

	if c.PctQuestoes < 0.1 || c.PctQuestoes > 0.9 {
		c.PctQuestoes = d.PctQuestoes
	}

	if c.LimiarFraco < 1 || c.LimiarFraco > 100 {
		c.LimiarFraco = d.LimiarFraco
	}

	if c.BlocosPorDia < BlocosMin || c.BlocosPorDia > BlocosMax {
		c.BlocosPorDia = d.BlocosPorDia
	}

	c.MinutosRevisao = minutosRevisaoValido(c.MinutosRevisao)

	c.Modos = modosNaoNulos(c.Modos)
	c.Reforcos = reforcosNaoNulos(c.Reforcos)
	c.CicloRevisao = cicloValido(c.CicloRevisao)
	c.MinutosBloco = minutosBlocoValido(c.MinutosBloco)
	c.HorasDia = horasDiaEfetiva(c)

	return c
}

// ModoDe diz como uma disciplina é estudada, caindo em ModoCompleto.
func (c Config) ModoDe(codigo string) Modo {
	switch c.Modos[codigo] {
	case ModoQuestoes:
		return ModoQuestoes
	case ModoTeoria:
		return ModoTeoria
	default:
		return ModoCompleto
	}
}

// ReforcoDe é o peso extra de uma disciplina, com padrão 1 e limitado a uma
// faixa em que o plano ainda faz sentido. 2 faz a matéria aparecer com o dobro
// da frequência.
func (c Config) ReforcoDe(codigo string) float64 {
	r, ok := c.Reforcos[codigo]
	if !ok || r == 0 {
		return 1
	}

	return min(max(r, ReforcoMin), ReforcoMax)
}

// minutosRevisaoValido limita o bloco de revisão. Zero é legítimo — um dia sem
// bloco de revisão nenhum — então só valor negativo ou absurdo é corrigido.
func minutosRevisaoValido(m int) int {
	switch {
	case m <= 0:
		return 0
	case m > MinutosBlocoMax:
		return MinutosBlocoMax
	default:
		return m
	}
}

// horasDiaEfetiva mantém HorasDia em sintonia com MinutosBloco: quando o usuário
// define a duração do bloco, é isso, mais BlocosPorDia e a cauda de revisão, que
// diz quanto dura o dia. MinutosBloco == 0 significa "sem duração explícita", e
// HorasDia é usada como está.
func horasDiaEfetiva(c Config) float64 {
	if c.MinutosBloco <= 0 || c.BlocosPorDia <= 0 {
		return c.HorasDia
	}

	// O dia é simplesmente o que ele comporta: os blocos de conteúdo mais o de
	// revisão. Antes era conteúdo dividido por (1 - pctRevisao), o que fazia a
	// fatia de revisão mexer na duração de todo bloco como efeito colateral.
	return float64(c.BlocosPorDia*c.MinutosBloco+c.MinutosRevisao) / 60
}

func minutosBlocoValido(m int) int {
	switch {
	case m <= 0:
		return 0
	case m < MinutosBlocoMin:
		return MinutosBlocoMin
	case m > MinutosBlocoMax:
		return MinutosBlocoMax
	default:
		return m
	}
}

func modosNaoNulos(m map[string]Modo) map[string]Modo {
	if m == nil {
		return map[string]Modo{}
	}

	return m
}

func reforcosNaoNulos(m map[string]float64) map[string]float64 {
	if m == nil {
		return map[string]float64{}
	}

	return m
}

// cicloValido descarta entradas sem título, para que um formulário preenchido
// pela metade não deixe a revisão semanal com uma manchete em branco.
func cicloValido(itens []concurso.ItemRevisao) []concurso.ItemRevisao {
	out := make([]concurso.ItemRevisao, 0, len(itens))

	for _, it := range itens {
		if strings.TrimSpace(it.Titulo) == "" {
			continue
		}

		out = append(out, concurso.ItemRevisao{
			Ordem:    len(out),
			Titulo:   strings.TrimSpace(it.Titulo),
			Questoes: max(0, it.Questoes),
		})
	}

	if len(out) == 0 {
		return nil
	}

	return out
}
