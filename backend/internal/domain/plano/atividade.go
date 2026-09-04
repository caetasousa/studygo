package plano

import (
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Erros que um movimento pode devolver. São distintos para que a API traduza
// cada um numa mensagem que diga o que fazer em seguida, em vez de uma recusa
// genérica.
var (
	// ErrAtividadeNaoEncontrada: nenhuma atividade com esse id no plano.
	ErrAtividadeNaoEncontrada = errors.New("atividade não encontrada")
	// ErrDestinoInvalido: a data de destino não é um dia que aceita conteúdo
	// (fora do plano, ou um dia fixo como o simulado e a véspera).
	ErrDestinoInvalido = errors.New("dia de destino não aceita atividades")
	// ErrDiaConcluido: o dia de destino já está concluído, então mover
	// reescreveria o que foi estudado.
	ErrDiaConcluido = errors.New("dia já concluído")
	// ErrAtividadeConcluida: a própria atividade já foi concluída.
	ErrAtividadeConcluida = errors.New("atividade já concluída")
)

// TipoAtividade é o que uma atividade agendada pede do estudante. É
// deliberadamente separado de Tipo (que classifica o dia inteiro): um mesmo dia
// pode ter um bloco de teoria e um de questões ao mesmo tempo.
type TipoAtividade string

const (
	AtividadeConteudo   TipoAtividade = "conteudo"
	AtividadeRevisao    TipoAtividade = "revisao"
	AtividadeQuestoes   TipoAtividade = "questoes"
	AtividadeSimulado   TipoAtividade = "simulado"
	AtividadeDiscursiva TipoAtividade = "discursiva"
	AtividadeVespera    TipoAtividade = "vespera"
)

// DeDiaInteiro diz se a atividade ocupa o dia todo em vez de uma matéria. Essas
// não apontam para disciplina nenhuma, e o cronograma não as move: são o
// esqueleto do plano.
func (t TipoAtividade) DeDiaInteiro() bool {
	switch t {
	case AtividadeSimulado, AtividadeDiscursiva, AtividadeVespera, AtividadeRevisao:
		return true
	default:
		return false
	}
}

// Atividade é uma unidade agendada do cronograma: uma matéria e o tema dela, uma
// revisão, um simulado.
//
// O cronograma é MATERIALIZADO: toda atividade de todo dia existe desde a
// criação do plano, com id próprio. Não há atividade "derivada" nem id
// sintético — o que está na tela é o que está gravado, e é por isso que um
// registro de estudo pode ter chave estrangeira de verdade para cá.
type Atividade struct {
	ID           uuid.UUID
	Data         time.Time
	Posicao      int
	DisciplinaID *uuid.UUID // nil nas atividades de dia inteiro
	Disciplina   string     // código da disciplina, vazio nas de dia inteiro
	Tema         string
	Passada      int
	Tipo         TipoAtividade
	DuracaoMin   int // 0 = usar a duração padrão do bloco no plano

	// Movida marca a atividade que o estudante colocou onde ela está. Um
	// replanejamento do futuro não a arrasta de volta: a escolha dele vale mais
	// que a do motor.
	Movida bool
}

// Duracao resolve a duração planejada, caindo na do bloco padrão.
func (a Atividade) Duracao(padraoMin int) int {
	if a.DuracaoMin > 0 {
		return a.DuracaoMin
	}

	return padraoMin
}

// DestinoValido diz se uma data pode receber atividades: precisa ser um dia que
// o plano de fato contém, e que carrega conteúdo em vez de um dia fixo
// (simulado, discursiva, véspera).
func DestinoValido(dias []Dia, dt time.Time) bool {
	d := findDia(dias, dt)
	if d == nil {
		return false
	}

	return d.Tipo == TipoEstudo || d.Tipo == TipoRevisaoDirigida || d.Tipo == TipoRevisaoSemanal
}

// Materializar transforma os dias que o motor gerou nas atividades que serão
// gravadas. É o passo que fecha a lacuna entre "plano calculado" e "plano
// existente": depois dele, todo bloco da tela tem uma linha no banco.
//
// Os dias fixos também viram atividade, para que um simulado possa receber
// registro como qualquer outra coisa que se faz num dia.
func Materializar(dias []Dia, porCodigo map[string]uuid.UUID) []Atividade {
	out := make([]Atividade, 0, len(dias)*2)

	for _, d := range dias {
		data := day(d.Data)

		if len(d.Itens) == 0 {
			// Dia fixo: uma atividade só, do dia inteiro.
			out = append(out, Atividade{
				ID:      uuid.New(),
				Data:    data,
				Posicao: 0,
				Tema:    d.Tema,
				Passada: 1,
				Tipo:    tipoDaAtividade(d.Tipo),
			})

			continue
		}

		for i, it := range d.Itens {
			a := Atividade{
				ID:      uuid.New(),
				Data:    data,
				Posicao: i,
				Tema:    it.Tema,
				Passada: it.Passada,
				Tipo:    tipoDaAtividade(d.Tipo),
			}

			// Só o que é de uma matéria aponta para uma disciplina. Um dia de
			// revisão semanal percorre o ciclo inteiro, não uma matéria — ligá-lo
			// a uma faria a estatística creditar horas de revisão geral a ela.
			if !a.Tipo.DeDiaInteiro() {
				a.Disciplina = it.Disciplina
				if id, ok := porCodigo[it.Disciplina]; ok {
					a.DisciplinaID = &id
				}
			}

			out = append(out, a)
		}
	}

	return out
}

// Replanejar recalcula o cronograma dos dias que ainda estão por vir,
// preservando o que não pode ser mexido.
//
// A regra que o estudante espera: o que já passou, o que está concluído e o que
// ele arrumou à mão fica onde está; os dias à frente seguem a configuração nova.
// Uma atividade é preservada quando QUALQUER uma dessas condições vale, e as
// demais dão lugar às recém-geradas.
//
// É isto que substitui a antiga reconciliação: em vez de casar slots gerados
// com linhas guardadas a cada leitura, o replanejamento acontece uma vez, quando
// a configuração muda, e grava o resultado.
func Replanejar(
	atuais []Atividade,
	novas []Atividade,
	desde time.Time,
	concluida func(uuid.UUID) bool,
) []Atividade {
	desde = day(desde)

	preservadas := make([]Atividade, 0, len(atuais))
	ocupadas := map[time.Time]map[int]bool{}

	marcarOcupada := func(a Atividade) {
		dt := day(a.Data)
		if ocupadas[dt] == nil {
			ocupadas[dt] = map[int]bool{}
		}

		ocupadas[dt][a.Posicao] = true
	}

	// Um dia que teve QUALQUER atividade preservada fica inteiro como está.
	//
	// Sem isso, um dia já estudado recebia também a leva recém-gerada e passava a
	// mostrar a mesma matéria duas vezes — metade concluída, metade não. O
	// replanejamento é sobre os dias que ainda estão em aberto; um dia em que o
	// estudante já mexeu não é um deles.
	intocados := map[time.Time]bool{}

	for _, a := range atuais {
		dt := day(a.Data)

		if dt.Before(desde) || concluida(a.ID) || a.Movida {
			preservadas = append(preservadas, a)
			marcarOcupada(a)
			intocados[dt] = true
		}
	}

	// As atividades novas entram só nos dias que o replanejamento alcança.
	for _, n := range novas {
		dt := day(n.Data)
		if dt.Before(desde) || intocados[dt] {
			continue
		}

		pos := n.Posicao
		for ocupadas[dt][pos] {
			pos++
		}

		n.Posicao = pos
		preservadas = append(preservadas, n)
		marcarOcupada(n)
	}

	ordenar(preservadas)

	return preservadas
}

// Mover leva uma atividade para (data, posicao) e devolve a ordenação nova de
// todos os dias que ela tocou, para que quem chama grave as posições densas e
// de uma vez.
//
// Nunca mexe em registro: o plano muda, a história do que foi estudado não.
func Mover(
	atividades []Atividade,
	dias []Dia,
	id uuid.UUID,
	destino time.Time,
	posicao int,
	concluido func(time.Time) bool,
) ([]Atividade, error) {
	idx := indiceDe(atividades, id)
	if idx < 0 {
		return nil, ErrAtividadeNaoEncontrada
	}

	origem := day(atividades[idx].Data)
	destino = day(destino)

	if !DestinoValido(dias, destino) {
		return nil, ErrDestinoInvalido
	}

	// Só o destino é protegido aqui: se a atividade EM SI está concluída é
	// decisão de quem chama, que enxerga os registros. Recusar todo movimento
	// para fora de um dia que guarda trabalho concluído congelaria também as
	// outras matérias daquele dia.
	if DestinoBloqueado(dias, destino, concluido) {
		return nil, ErrDiaConcluido
	}

	movida := atividades[idx]
	movida.Data = destino
	movida.Movida = true

	restantes := make([]Atividade, 0, len(atividades))
	restantes = append(restantes, atividades[:idx]...)
	restantes = append(restantes, atividades[idx+1:]...)

	destinoLista := doDia(restantes, destino)
	posicao = min(max(posicao, 0), len(destinoLista))

	saida := make([]Atividade, 0, len(atividades))
	for _, a := range restantes {
		if !sameDay(a.Data, destino) {
			saida = append(saida, a)
		}
	}

	// O dia de destino é reconstruído explicitamente, inserindo em `posicao`.
	// Só empurrar a posição da movida não basta: as que ela desloca manteriam os
	// números antigos e uma ordenação estável a deixaria atrás delas.
	novaOrdem := make([]Atividade, 0, len(destinoLista)+1)
	novaOrdem = append(novaOrdem, destinoLista[:posicao]...)
	novaOrdem = append(novaOrdem, movida)
	novaOrdem = append(novaOrdem, destinoLista[posicao:]...)

	for i := range novaOrdem {
		novaOrdem[i].Posicao = i
	}

	saida = append(saida, novaOrdem...)

	// O dia de origem perdeu um item; feche o buraco que ele deixou.
	if !sameDay(origem, destino) {
		renumerar(saida, map[time.Time]bool{origem: true})
	}

	ordenar(saida)

	return saida, nil
}

// Trocar troca uma atividade com quem ocupa (destino, posicao), em vez de
// inserir ao lado. É o que "levar Português do dia 31 para o dia 2" costuma
// significar quando o dia 2 já está cheio: as duas matérias trocam de lugar e
// nenhum dos dias muda de tamanho.
//
// Quando a vaga de destino está vazia não há com quem trocar, e isto vira um
// movimento simples.
func Trocar(
	atividades []Atividade,
	dias []Dia,
	id uuid.UUID,
	destino time.Time,
	posicao int,
	concluido func(time.Time) bool,
) ([]Atividade, error) {
	idx := indiceDe(atividades, id)
	if idx < 0 {
		return nil, ErrAtividadeNaoEncontrada
	}

	origem := day(atividades[idx].Data)
	posOrigem := atividades[idx].Posicao
	destino = day(destino)

	if !DestinoValido(dias, destino) {
		return nil, ErrDestinoInvalido
	}

	if DestinoBloqueado(dias, destino, concluido) {
		return nil, ErrDiaConcluido
	}

	// Mesma vaga: nada a fazer, e trocar consigo mesma reescreveria todas as
	// posições por nada.
	if sameDay(origem, destino) && posOrigem == posicao {
		return append([]Atividade(nil), atividades...), nil
	}

	destinoLista := doDia(atividades, destino)

	if posicao < 0 || posicao >= len(destinoLista) {
		return Mover(atividades, dias, id, destino, posicao, concluido)
	}

	alvoID := destinoLista[posicao].ID
	if alvoID == id {
		return append([]Atividade(nil), atividades...), nil
	}

	saida := append([]Atividade(nil), atividades...)

	for i := range saida {
		switch saida[i].ID {
		case id:
			saida[i].Data = destino
			saida[i].Posicao = posicao
			saida[i].Movida = true
		case alvoID:
			saida[i].Data = origem
			saida[i].Posicao = posOrigem
			saida[i].Movida = true
		}
	}

	ordenar(saida)

	return saida, nil
}

// AtividadesDoDia é a lista ordenada que o cartão de um dia renderiza.
func AtividadesDoDia(atividades []Atividade, dt time.Time) []Atividade {
	return doDia(atividades, dt)
}

// PorID localiza uma atividade pelo id.
func PorID(atividades []Atividade, id uuid.UUID) (Atividade, bool) {
	if i := indiceDe(atividades, id); i >= 0 {
		return atividades[i], true
	}

	return Atividade{}, false
}

// MinutosPlanejados soma os minutos planejados de um dia.
func MinutosPlanejados(atividades []Atividade, dt time.Time, padraoMin int) int {
	total := 0
	for _, a := range doDia(atividades, dt) {
		total += a.Duracao(padraoMin)
	}

	return total
}

// AplicarNosDias preenche os itens de cada dia a partir do cronograma gravado.
// É a única direção que existe agora: o que está guardado descreve o dia, sem
// reconciliação com nada.
func AplicarNosDias(dias []Dia, atividades []Atividade) {
	porDia := map[time.Time][]Atividade{}
	for _, a := range atividades {
		k := day(a.Data)
		porDia[k] = append(porDia[k], a)
	}

	for i := range dias {
		lista := porDia[day(dias[i].Data)]

		sort.SliceStable(lista, func(x, y int) bool { return lista[x].Posicao < lista[y].Posicao })

		itens := make([]ItemDia, 0, len(lista))

		for _, a := range lista {
			if a.Tipo.DeDiaInteiro() {
				continue
			}

			itens = append(itens, ItemDia{
				Disciplina:  a.Disciplina,
				Tema:        a.Tema,
				Passada:     a.Passada,
				AtividadeID: a.ID,
			})
		}

		dias[i].Itens = itens
	}
}

// doDia devolve as atividades de um dia, ordenadas por posição.
func doDia(atividades []Atividade, dt time.Time) []Atividade {
	out := []Atividade{}

	for _, a := range atividades {
		if sameDay(a.Data, dt) {
			out = append(out, a)
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Posicao < out[j].Posicao })

	return out
}

// renumerar reescreve as posições 0..n-1 de cada dia afetado, no lugar.
func renumerar(atividades []Atividade, dias map[time.Time]bool) {
	porID := make(map[uuid.UUID]int, len(atividades))
	for i := range atividades {
		porID[atividades[i].ID] = i
	}

	for dt := range dias {
		for i, a := range doDia(atividades, dt) {
			if j, ok := porID[a.ID]; ok {
				atividades[j].Posicao = i
			}
		}
	}
}

// ordenar deixa o cronograma em ordem de (data, posição), que é como o
// repositório o devolve e como a tela o lê.
func ordenar(atividades []Atividade) {
	sort.SliceStable(atividades, func(i, j int) bool {
		if !atividades[i].Data.Equal(atividades[j].Data) {
			return atividades[i].Data.Before(atividades[j].Data)
		}

		return atividades[i].Posicao < atividades[j].Posicao
	})
}

func indiceDe(atividades []Atividade, id uuid.UUID) int {
	for i := range atividades {
		if atividades[i].ID == id {
			return i
		}
	}

	return -1
}

func tipoDaAtividade(t Tipo) TipoAtividade {
	switch t {
	case TipoRevisaoSemanal, TipoRevisaoDirigida:
		return AtividadeRevisao
	case TipoSimulado:
		return AtividadeSimulado
	case TipoDiscursiva:
		return AtividadeDiscursiva
	case TipoVespera:
		return AtividadeVespera
	default:
		return AtividadeConteudo
	}
}
