package plano

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"studygo/internal/domain/concurso"
)

const (
	vesperaTema = "Véspera: leitura leve dos resumos, conferência de documento e local de prova, e descanso"
	discTema    = "Estudo de caso: prova discursiva completa e autocorreção pelos critérios do edital"
	metaEstudo  = 20
	metaRevD    = 24
)

// RevCicloPadrao is the weekly-review cycle used when a concurso has none of its
// own registered. It mirrors the four-week rotation from the original artifact.
var RevCicloPadrao = []concurso.RevItem{
	{Ordem: 0, Titulo: "Revisão ativa da semana + caderno de erros", Questoes: 30},
	{Ordem: 1, Titulo: "Bateria mista de questões no peso da prova", Questoes: 60},
	{Ordem: 2, Titulo: "Treino da prova discursiva, com autocorreção", Questoes: 0},
	{Ordem: 3, Titulo: "Simulado parcial cronometrado + correção comentada", Questoes: 45},
}

// diaTmp is a mutable scratch day used while assigning phases and roles.
type diaTmp struct {
	data    time.Time
	wd      int
	semana  int
	vespera bool
	papel   string // "", "est", "rev", "revd", "sim", "disc"
}

// construir is the port of the artifact's construir(). It fills res.Slots and
// res.SlotsReta as a side effect and returns the ordered days.
func construir(
	cfg Config,
	c *concurso.Concurso,
	codes []string,
	temas map[string][]string,
	pontos map[string]int,
	soma int,
	res *Resultado,
) []Dia {
	totalCal := diffDays(cfg.Inicio, cfg.Prova)
	if totalCal < 1 {
		totalCal = 1
	}

	revCiclo := c.RevCiclo
	if len(revCiclo) == 0 {
		revCiclo = RevCicloPadrao
	}

	if len(cfg.CicloRevisao) > 0 {
		revCiclo = cfg.CicloRevisao
	}

	vesp := addDays(cfg.Prova, -1)
	ancora := mondayOf(cfg.Inicio)

	estudo := []*diaTmp{}

	for i := 0; i < totalCal; i++ {
		dt := addDays(cfg.Inicio, i)
		wd := weekday(dt)

		if !contains(cfg.DiasEstudo, wd) {
			continue
		}

		if sameDay(dt, vesp) {
			continue
		}

		estudo = append(estudo, &diaTmp{
			data:   dt,
			wd:     wd,
			semana: diffDays(ancora, dt)/7 + 1,
		})
	}

	estudo = append(estudo, &diaTmp{
		data:    vesp,
		wd:      weekday(vesp),
		semana:  diffDays(ancora, vesp)/7 + 1,
		vespera: true,
	})

	if len(estudo) == 0 {
		return []Dia{}
	}

	fase := atribuiFases(cfg, estudo)

	diasEst := filterPapel(estudo, "est")
	diasRevD := filterPapel(estudo, "revd")

	// A distribuição usa o peso com reforço; res.Pontos segue sendo o peso puro
	// da prova, que é o que a tela de balanceamento mostra.
	dist, somaDist := pesosDistribuicao(codes, pontos, cfg)
	n := cfg.BlocosPorDia

	res.Slots = distribui(len(diasEst)*n, codes, dist, somaDist)
	res.SlotsReta = distribui(len(diasRevD)*n, codes, dist, somaDist)

	filas := map[string][]reparteItem{}
	ptr := map[string]int{}
	filasR := map[string][]reparteItem{}
	ptrR := map[string]int{}

	for _, k := range codes {
		filas[k] = reparte(temas[k], res.Slots[k])
		filasR[k] = reparte(temas[k], res.SlotsReta[k])
	}

	ordem := despareia(ordena(res.Slots, dist, codes, somaDist), n)
	ordemR := despareia(ordena(res.SlotsReta, dist, codes, somaDist), n)

	res.Simulado = simulado(cfg, c)
	simTema := res.Simulado.Tema()
	simMeta := res.Simulado.Gerais + res.Simulado.Especificas

	dias := make([]Dia, 0, len(estudo))
	oi, ori := 0, 0

	for _, d := range estudo {
		base := Dia{
			Data:   d.data,
			Semana: d.semana,
			Fase:   fase[d.semana],
			Itens:  []ItemDia{},
		}

		switch {
		case d.vespera:
			base.Fase = FaseReta
			base.Tipo = TipoVespera
			base.Tema = vesperaTema
		case d.papel == "est":
			base.Tipo = TipoEstudo
			base.Meta = metaEstudo
			base.Itens = puxaBloco(ordem, &oi, n, filas, ptr, "")
		case d.papel == "revd":
			base.Tipo = TipoRevisaoDirigida
			base.Meta = metaRevD
			base.Itens = puxaBloco(ordemR, &ori, n, filasR, ptrR, "Revisão dirigida — ")
		case d.papel == "sim":
			base.Tipo = TipoSimulado
			base.Meta = simMeta
			base.Tema = simTema
		case d.papel == "disc":
			base.Tipo = TipoDiscursiva
			base.Tema = discTema
		default: // "rev"
			r := revCiclo[(d.semana-1)%len(revCiclo)]
			base.Tipo = TipoRevisaoSemanal
			base.Tema = r.Titulo
			base.Meta = r.Questoes
		}

		dias = append(dias, base)
	}

	renumera(dias)

	return dias
}

// atribuiFases groups days by week, tags each week base/reta, and sets the
// papel of every non-vespera day. Returns week -> phase.
func atribuiFases(cfg Config, estudo []*diaTmp) map[int]Fase {
	inicioReta := addDays(cfg.Prova, -maxInt(7, cfg.RetaFinalDias))

	semanas := map[int][]*diaTmp{}
	ordemSem := []int{}

	for _, d := range estudo {
		if _, ok := semanas[d.semana]; !ok {
			ordemSem = append(ordemSem, d.semana)
		}

		semanas[d.semana] = append(semanas[d.semana], d)
	}

	sort.Ints(ordemSem)

	fase := map[int]Fase{}
	semanaReta := 0

	for _, sm := range ordemSem {
		grupo := semanas[sm]
		fase[sm] = FaseBase

		if !grupo[0].data.Before(inicioReta) {
			fase[sm] = FaseReta
		}

		conteudo := []*diaTmp{}

		for _, d := range grupo {
			if !d.vespera {
				conteudo = append(conteudo, d)
			}
		}

		if len(conteudo) == 0 {
			continue
		}

		if fase[sm] == FaseBase {
			atribuiBase(cfg, conteudo)
			continue
		}

		semanaReta++
		atribuiReta(conteudo, cfg, semanaReta)
	}

	return fase
}

// atribuiBase lays out one content week.
//
// Review is a DAILY tail of every study day (see Blocos and PctRevisao), fed by
// the error notebook, so by default no day is surrendered to it and the whole
// week is content. `RevisaoSemanal` brings the old fixed day back for the study
// methods that want it.
func atribuiBase(cfg Config, conteudo []*diaTmp) {
	for _, d := range conteudo {
		d.papel = "est"
	}

	if !cfg.RevisaoSemanal {
		return
	}

	rev := conteudo[len(conteudo)-1]

	for _, d := range conteudo {
		if d.wd == cfg.DiaRevisao {
			rev = d
			break
		}
	}

	rev.papel = "rev"
}

// atribuiReta lays out one reta-final week. By default the last day is a full
// mock exam and the day before it the essay — but both are personal calls, and a
// week that has neither is simply all guided review.
func atribuiReta(conteudo []*diaTmp, cfg Config, semanaReta int) {
	temSim := querSimulado(cfg, semanaReta)
	temDisc := cfg.Discursiva

	for _, d := range conteudo {
		d.papel = "revd"
	}

	// Preenche de trás para frente: o simulado fecha a semana, a discursiva vem
	// logo antes. Sem um deles, o outro sobe uma casa.
	ix := len(conteudo) - 1

	if temSim {
		conteudo[ix].papel = "sim"
		ix--
	}

	if temDisc && ix >= 0 {
		conteudo[ix].papel = "disc"
	}
}

// querSimulado answers whether this reta-final week gets a mock exam.
func querSimulado(cfg Config, semanaReta int) bool {
	switch cfg.Simulados {
	case SimuladoNunca:
		return false
	case SimuladoQuinzenal:
		return semanaReta%2 == 1
	default:
		return true
	}
}

// renumera assigns 1-based day numbers and compacts week numbers so they run
// 1..N with no gaps.
func renumera(dias []Dia) {
	mapa := map[int]int{}
	c := 0

	for _, d := range dias {
		if _, ok := mapa[d.Semana]; !ok {
			c++
			mapa[d.Semana] = c
		}
	}

	for i := range dias {
		dias[i].N = i + 1
		dias[i].Semana = mapa[dias[i].Semana]
	}
}

func simulado(cfg Config, c *concurso.Concurso) Composicao {
	var comp Composicao

	for _, d := range c.Disciplinas {
		q := cfg.Questoes[d.Codigo]

		switch d.Bloco {
		case concurso.BlocoGeral:
			comp.Gerais += q
		default:
			comp.Especificas += q
		}
	}

	return comp
}

// Tema is the headline of a full mock-exam day.
func (c Composicao) Tema() string {
	return fmt.Sprintf(
		"Simulado completo no formato oficial: %d gerais + %d específicos, cronometrado, com correção",
		c.Gerais,
		c.Especificas,
	)
}

// puxaBloco consumes up to n discipline slots from ordem and pulls the next
// queued topic for each. prefix, when set, is prepended to the topic text.
func puxaBloco(
	ordem []string,
	cursor *int,
	n int,
	filas map[string][]reparteItem,
	ptr map[string]int,
	prefix string,
) []ItemDia {
	itens := []ItemDia{}

	for b := 0; b < n; b++ {
		if *cursor >= len(ordem) {
			break
		}

		k := ordem[*cursor]
		*cursor++

		if k == "" {
			continue
		}

		tema := "Revisão livre"
		pass := 1

		if ptr[k] < len(filas[k]) {
			tema = filas[k][ptr[k]].tema
			pass = filas[k][ptr[k]].passada
			ptr[k]++
		}

		itens = append(itens, ItemDia{
			Disciplina: k,
			Tema:       prefix + tema,
			Passada:    pass,
		})
	}

	return itens
}

type reparteItem struct {
	tema    string
	passada int
}

// reparte turns a discipline's topic list into exactly `vagas` queued items.
// When there is room for every topic, leftovers become second-pass repeats;
// when there is not, topics are merged into grouped entries.
func reparte(temas []string, vagas int) []reparteItem {
	out := []reparteItem{}

	if vagas <= 0 {
		return out
	}

	if vagas >= len(temas) {
		for _, t := range temas {
			out = append(out, reparteItem{tema: t, passada: 1})
		}

		for i := 0; len(out) < vagas; i++ {
			out = append(out, reparteItem{tema: temas[i%len(temas)], passada: 2})
		}

		sort.SliceStable(out, func(i, j int) bool {
			return out[i].passada < out[j].passada
		})

		return out
	}

	for j := 0; j < vagas; j++ {
		a := j * len(temas) / vagas
		b := (j + 1) * len(temas) / vagas
		out = append(out, reparteItem{
			tema:    strings.Join(temas[a:b], "  ·  "),
			passada: 1,
		})
	}

	return out
}

// distribui splits `total` blocks across disciplines proportional to their
// points, using the largest-remainder method.
func distribui(total int, codes []string, pontos map[string]int, soma int) map[string]int {
	s := map[string]int{}
	rem := map[string]float64{}
	usados := 0

	for _, k := range codes {
		var x float64
		if soma != 0 {
			x = float64(total) * float64(pontos[k]) / float64(soma)
		}

		floor := int(x)
		s[k] = floor
		rem[k] = x - float64(floor)
		usados += floor
	}

	ordenados := append([]string{}, codes...)
	sort.SliceStable(ordenados, func(i, j int) bool {
		return rem[ordenados[i]] > rem[ordenados[j]]
	})

	for i := 0; i < total-usados && i < len(ordenados); i++ {
		s[ordenados[i]]++
	}

	return s
}

// ordena interleaves the per-discipline slots into one sequence, spacing each
// discipline by its point weight (a rate-based / Bresenham scheduler).
func ordena(slots, pontos map[string]int, codes []string, soma int) []string {
	cred := map[string]int{}
	restam := map[string]int{}
	n := 0

	for _, k := range codes {
		restam[k] = slots[k]
		n += slots[k]
	}

	ordem := make([]string, 0, n)

	for i := 0; i < n; i++ {
		esc := ""

		for _, k := range codes {
			if restam[k] == 0 {
				continue
			}

			cred[k] += pontos[k]

			if esc == "" || cred[k] > cred[esc] {
				esc = k
			}
		}

		if esc == "" {
			break
		}

		cred[esc] -= soma
		restam[esc]--
		ordem = append(ordem, esc)
	}

	return ordem
}

// despareia swaps entries so the n blocks of a day never repeat a discipline,
// pulling a later distinct entry forward when a clash is found. The candidate
// has to be absent from this day and its own day has to be free of the
// discipline being pushed out, so a swap never trades one clash for another.
func despareia(ordem []string, n int) []string {
	if n < 2 {
		return ordem
	}

	for inicio := 0; inicio < len(ordem); inicio += n {
		fim := minInt(inicio+n, len(ordem))

		for i := inicio + 1; i < fim; i++ {
			if !contemEntre(ordem, inicio, i, ordem[i]) {
				continue
			}

			for j := fim; j < len(ordem); j++ {
				if contemEntre(ordem, inicio, fim, ordem[j]) {
					continue
				}

				gi := (j / n) * n
				if contemEntre(ordem, gi, minInt(gi+n, len(ordem)), ordem[i]) {
					continue
				}

				ordem[i], ordem[j] = ordem[j], ordem[i]

				break
			}
		}
	}

	return ordem
}

func contemEntre(xs []string, inicio, fim int, alvo string) bool {
	for i := inicio; i < fim && i < len(xs); i++ {
		if xs[i] == alvo {
			return true
		}
	}

	return false
}

// pesosDistribuicao scales the exam weights by each discipline's reforço. The
// values are multiplied by 100 first so a fractional reforço still lands on
// integers; distribui and ordena are both ratio-based, so the scale itself
// changes nothing.
func pesosDistribuicao(codes []string, pontos map[string]int, cfg Config) (map[string]int, int) {
	out := make(map[string]int, len(codes))
	soma := 0

	for _, k := range codes {
		v := int(math.Round(float64(pontos[k]) * cfg.ReforcoDe(k) * 100))
		out[k] = v
		soma += v
	}

	return out, soma
}

func filterPapel(dias []*diaTmp, papel string) []*diaTmp {
	out := []*diaTmp{}

	for _, d := range dias {
		if d.papel == papel {
			out = append(out, d)
		}
	}

	return out
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}

	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
