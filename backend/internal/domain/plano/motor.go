package plano

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"annygo/internal/domain/concurso"
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

	res.Slots = distribui(len(diasEst)*2, codes, pontos, soma)
	res.SlotsReta = distribui(len(diasRevD)*2, codes, pontos, soma)

	filas := map[string][]reparteItem{}
	ptr := map[string]int{}
	filasR := map[string][]reparteItem{}
	ptrR := map[string]int{}

	for _, k := range codes {
		filas[k] = reparte(temas[k], res.Slots[k])
		filasR[k] = reparte(temas[k], res.SlotsReta[k])
	}

	ordem := despareia(ordena(res.Slots, pontos, codes, soma))
	ordemR := despareia(ordena(res.SlotsReta, pontos, codes, soma))

	simTema, simMeta := simulado(cfg, c)

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
			base.Itens = puxaBloco(codes, ordem, &oi, filas, ptr, "")
		case d.papel == "revd":
			base.Tipo = TipoRevisaoDirigida
			base.Meta = metaRevD
			base.Itens = puxaBloco(codes, ordemR, &ori, filasR, ptrR, "Revisão dirigida — ")
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

		atribuiReta(conteudo)
	}

	return fase
}

func atribuiBase(cfg Config, conteudo []*diaTmp) {
	rev := conteudo[len(conteudo)-1]

	for _, d := range conteudo {
		if d.wd == cfg.DiaRevisao {
			rev = d
			break
		}
	}

	for _, d := range conteudo {
		d.papel = "est"
		if d == rev {
			d.papel = "rev"
		}
	}
}

func atribuiReta(conteudo []*diaTmp) {
	last := len(conteudo) - 1

	for ix, d := range conteudo {
		switch {
		case ix == last:
			d.papel = "sim"
		case ix == last-1:
			d.papel = "disc"
		default:
			d.papel = "revd"
		}
	}

	switch len(conteudo) {
	case 1:
		conteudo[0].papel = "sim"
	case 2:
		conteudo[0].papel = "disc"
		conteudo[1].papel = "sim"
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

func simulado(cfg Config, c *concurso.Concurso) (string, int) {
	ger, esp := 0, 0

	for _, d := range c.Disciplinas {
		q := cfg.Questoes[d.Codigo]

		switch d.Bloco {
		case concurso.BlocoGeral:
			ger += q
		default:
			esp += q
		}
	}

	tema := fmt.Sprintf(
		"Simulado completo no formato oficial: %d gerais + %d específicos, cronometrado, com correção",
		ger,
		esp,
	)

	return tema, ger + esp
}

// puxaBloco consumes up to two discipline slots from ordem and pulls the next
// queued topic for each. prefix, when set, is prepended to the topic text.
func puxaBloco(
	codes []string,
	ordem []string,
	cursor *int,
	filas map[string][]reparteItem,
	ptr map[string]int,
	prefix string,
) []ItemDia {
	itens := []ItemDia{}

	for b := 0; b < 2; b++ {
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

// despareia swaps entries so the two blocks of a day never share a discipline,
// pulling a later distinct entry forward when a clash is found.
func despareia(ordem []string) []string {
	for i := 0; i+1 < len(ordem); i += 2 {
		if ordem[i] != ordem[i+1] {
			continue
		}

		j := i + 2

		for j < len(ordem) {
			same := ordem[j] == ordem[i]

			var adjacente bool
			if j%2 == 0 {
				adjacente = j+1 < len(ordem) && ordem[j+1] == ordem[i]
			} else {
				adjacente = ordem[j-1] == ordem[i]
			}

			if !same && !adjacente {
				break
			}

			j++
		}

		if j < len(ordem) {
			ordem[i+1], ordem[j] = ordem[j], ordem[i+1]
		}
	}

	return ordem
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
