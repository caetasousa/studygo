package editalproc

import (
	"sync"
	"time"
)

// Disjuntor corta as chamadas ao processador quando ele para de responder.
//
// O problema que ele resolve não é a falha em si — é o preço de descobri-la.
// Uma importação sobe um PDF de até 20 MB e espera o cliente inteiro para ver
// um 503; com o processador fora do ar, todo mundo paga esse minuto para saber
// o que a primeira falha já tinha contado. Pior: `Disponivel()` respondia pela
// CONFIGURAÇÃO (a URL está definida?) e não pela saúde, então a tela continuava
// convidando o estudante para um assistente que não tinha como terminar.
//
// Três estados, no vocabulário de sempre:
//
//	fechado    — passa tudo. Falhas consecutivas contam.
//	aberto     — recusa na hora, sem tocar na rede, por `espera`.
//	meio-aberto— deixa UMA chamada passar. Se ela vai bem, fecha; se não, abre
//	             de novo. É o que evita voltar a todo vapor sobre um serviço que
//	             ainda está se levantando.
//
// Só falha de DISPONIBILIDADE conta. Um edital que o processador analisou e
// recusou é resposta saudável: contá-la abriria o disjuntor por causa de um
// PDF ruim, tirando do ar a importação de todos os outros.
type Disjuntor struct {
	limite int           // falhas consecutivas que abrem
	espera time.Duration // quanto fica aberto antes de sondar
	agora  func() time.Time

	mu        sync.Mutex
	falhas    int
	abertoAte time.Time
	sondando  bool
}

// NovoDisjuntor devolve um disjuntor fechado.
func NovoDisjuntor(limite int, espera time.Duration) *Disjuntor {
	return &Disjuntor{
		limite: limite,
		espera: espera,
		agora:  time.Now,
	}
}

// Permitir diz se a chamada pode seguir. Quando devolve true no estado
// meio-aberto, ela é a sonda: nenhuma outra passa até ela ser reportada.
func (d *Disjuntor) Permitir() bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.abertoAte.IsZero() {
		return true
	}

	if d.agora().Before(d.abertoAte) {
		return false
	}

	// Passou a espera: uma única sonda atravessa.
	if d.sondando {
		return false
	}

	d.sondando = true

	return true
}

// Sucesso fecha o disjuntor e zera a contagem.
func (d *Disjuntor) Sucesso() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.falhas = 0
	d.abertoAte = time.Time{}
	d.sondando = false
}

// Falha conta uma indisponibilidade e abre quando o limite é atingido. Uma
// sonda que falha reabre imediatamente, sem esperar chegar ao limite de novo.
func (d *Disjuntor) Falha() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.sondando {
		d.sondando = false
		d.abertoAte = d.agora().Add(d.espera)

		return
	}

	d.falhas++

	if d.falhas >= d.limite {
		d.abertoAte = d.agora().Add(d.espera)
	}
}

// Fechado diz se o processador é considerado saudável AGORA. É o que
// `Disponivel()` consulta para que a tela deixe de oferecer o assistente
// enquanto ele não tem como funcionar.
//
// Não consome a sonda: perguntar o estado não é tentar usar o serviço.
func (d *Disjuntor) Fechado() bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.abertoAte.IsZero() || !d.agora().Before(d.abertoAte)
}
