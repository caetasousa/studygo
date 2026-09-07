package editalproc

import (
	"testing"
	"time"
)

// O disjuntor existe para que a segunda vítima da mesma falha não pague o
// mesmo minuto de espera que a primeira. Estes testes fixam as três transições
// e, principalmente, o que NÃO conta como falha.

// comRelogio devolve um disjuntor cujo tempo o teste controla.
func comRelogio(limite int, espera time.Duration) (*Disjuntor, func(time.Duration)) {
	agora := time.Unix(0, 0)
	d := NovoDisjuntor(limite, espera)
	d.agora = func() time.Time { return agora }

	return d, func(delta time.Duration) { agora = agora.Add(delta) }
}

func TestDisjuntor_abreDepoisDoLimite(t *testing.T) {
	t.Parallel()

	d, _ := comRelogio(3, time.Minute)

	for i := range 2 {
		d.Falha()

		if !d.Permitir() {
			t.Fatalf("depois de %d falhas o disjuntor já abriu; o limite é 3", i+1)
		}
	}

	d.Falha()

	if d.Permitir() {
		t.Error("na terceira falha o disjuntor devia abrir")
	}

	if d.Fechado() {
		t.Error("Fechado() devia acompanhar: a importação some da tela")
	}
}

// Sucesso zera a contagem: falhas espalhadas no tempo não somam até abrir.
func TestDisjuntor_sucessoZeraAContagem(t *testing.T) {
	t.Parallel()

	d, _ := comRelogio(3, time.Minute)

	d.Falha()
	d.Falha()
	d.Sucesso()
	d.Falha()
	d.Falha()

	if !d.Permitir() {
		t.Error("duas falhas depois de um sucesso não deviam abrir")
	}
}

// Passada a espera, UMA sonda atravessa. As outras continuam recusadas até ela
// dizer o que encontrou — voltar a todo vapor derrubaria de novo um serviço que
// ainda está subindo.
func TestDisjuntor_meioAbertoDeixaPassarUmaSo(t *testing.T) {
	t.Parallel()

	d, avancar := comRelogio(1, time.Minute)

	d.Falha()

	if d.Permitir() {
		t.Fatal("devia estar aberto")
	}

	avancar(time.Minute)

	if !d.Permitir() {
		t.Fatal("passada a espera, a sonda devia passar")
	}

	if d.Permitir() {
		t.Error("a segunda chamada não devia passar enquanto a sonda não responde")
	}
}

func TestDisjuntor_sondaBemSucedidaFecha(t *testing.T) {
	t.Parallel()

	d, avancar := comRelogio(1, time.Minute)

	d.Falha()
	avancar(time.Minute)
	d.Permitir() // a sonda
	d.Sucesso()

	if !d.Permitir() || !d.Fechado() {
		t.Error("sonda bem-sucedida devia fechar o disjuntor")
	}
}

// A sonda que falha reabre na hora, sem precisar juntar o limite de novo.
func TestDisjuntor_sondaQueFalhaReabre(t *testing.T) {
	t.Parallel()

	d, avancar := comRelogio(3, time.Minute)

	for range 3 {
		d.Falha()
	}

	avancar(time.Minute)
	d.Permitir() // a sonda
	d.Falha()

	if d.Permitir() {
		t.Error("a sonda falhou: devia reabrir imediatamente")
	}

	avancar(time.Minute)

	if !d.Permitir() {
		t.Error("depois de outra espera, uma nova sonda devia passar")
	}
}

// Fechado() é uma pergunta, não uma tentativa: consultá-lo não pode gastar a
// sonda, ou a tela de listagem de concursos roubaria a vaga da importação.
func TestDisjuntor_fechadoNaoConsomeASonda(t *testing.T) {
	t.Parallel()

	d, avancar := comRelogio(1, time.Minute)

	d.Falha()
	avancar(time.Minute)

	d.Fechado()
	d.Fechado()

	if !d.Permitir() {
		t.Error("Fechado() consumiu a sonda")
	}
}

func TestDisjuntor_comecaFechado(t *testing.T) {
	t.Parallel()

	d, _ := comRelogio(3, time.Minute)

	if !d.Fechado() || !d.Permitir() {
		t.Error("um disjuntor novo devia começar fechado")
	}
}
