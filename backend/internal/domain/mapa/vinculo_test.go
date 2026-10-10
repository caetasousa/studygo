package mapa

import (
	"errors"
	"slices"
	"testing"
)

// Os tópicos que o mapa marca no cronograma. Como pode quebrar (M27):
//   - a importação marca o tópico que só tem a palavra dentro de outra ("ITIL"
//     não é "UTILIZAR"), ou deixa de marcar por causa de caixa e acento;
//   - escolher na tela um tópico que não é da matéria é aceito, e o mapa fica
//     apontando para um assunto que o cronograma nunca mostra;
//   - o mesmo tópico escolhido duas vezes vira dois, ou a ordem da escolha
//     embaralha a da ementa.

func TestTemasCitados_SoOsTopicosQueCitamUmTermo(t *testing.T) {
	m := Mapa{Reconhecer: []string{"COBIT", "ISO/IEC 38500"}}
	temas := []string{
		"Governança de TI: estratégia e alinhamento",
		"Boas práticas e modelos de referência: COBIT 2019, ITIL v4 e ISO/IEC 38500:2024",
		"Cobit na prática",     // caixa não conta
		"Utilizar o Cobitário", // a palavra dentro de outra não conta
	}

	got := m.TemasCitados(temas)
	want := []string{temas[1], temas[2]}

	if !slices.Equal(got, want) {
		t.Fatalf("TemasCitados = %q, quer %q", got, want)
	}
}

func TestTemasCitados_SemReconhecerNaoMarcaNada(t *testing.T) {
	if got := (Mapa{}).TemasCitados([]string{"COBIT"}); len(got) != 0 {
		t.Fatalf("sem termos, nenhum tópico; veio %q", got)
	}
}

func TestEscolherTemas_NaOrdemDaMateriaSemRepetir(t *testing.T) {
	daMateria := []string{"Gestão de serviços", "Portfólio", "Continuidade"}

	got, err := EscolherTemas([]string{"Continuidade", " Gestão de serviços ", "Continuidade"}, daMateria)
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"Gestão de serviços", "Continuidade"}; !slices.Equal(got, want) {
		t.Fatalf("EscolherTemas = %q, quer %q", got, want)
	}
}

func TestEscolherTemas_TopicoQueNaoEDaMateria(t *testing.T) {
	_, err := EscolherTemas([]string{"Portfólio", "Redes"}, []string{"Portfólio"})
	if !errors.Is(err, ErrTemaForaDaMateria) {
		t.Fatalf("erro = %v, quer ErrTemaForaDaMateria", err)
	}
}

func TestEscolherTemas_NenhumEAMateriaInteira(t *testing.T) {
	got, err := EscolherTemas(nil, []string{"Portfólio"})
	if err != nil || len(got) != 0 {
		t.Fatalf("EscolherTemas(nil) = %q, %v; quer vazio, sem erro", got, err)
	}
}

// O tópico que volta de uma exportação noutra ementa (M34). Como pode quebrar:
//   - a mesma ementa cortada de outro jeito ("A; B" num tópico só aqui, dois
//     tópicos lá) faz o vínculo se perder;
//   - um ponto final ou a caixa diferente basta para não achar;
//   - o tópico curto casa com dois da ementa, e o mapa vai para o errado.
func TestTemaCorrespondente_NaEmentaCortadaDeOutroJeito(t *testing.T) {
	daMateria := []string{
		"Testes de software: unitários, de integração; testes automatizados",
		"Revisão de código e refatoração",
		"dívida técnica.",
		"princípios SOLID, DRY, KISS e YAGNI",
		"Redes: DNS e DHCP",
		"Serviços de diretório: DNS interno",
	}

	casos := []struct {
		tema, quer string
		acha       bool
	}{
		{"Revisão de código e refatoração", "Revisão de código e refatoração", true},
		{"testes automatizados", "Testes de software: unitários, de integração; testes automatizados", true},
		{"Dívida técnica", "dívida técnica.", true},
		{"principais princípios SOLID, DRY, KISS e YAGNI", "princípios SOLID, DRY, KISS e YAGNI", true},
		// "DNS" está em dois tópicos: nenhum é escolhido no chute.
		{"DNS", "", false},
		{"Crase", "", false},
	}

	for _, c := range casos {
		got, ok := TemaCorrespondente(c.tema, daMateria)
		if got != c.quer || ok != c.acha {
			t.Errorf("TemaCorrespondente(%q) = %q, %v; quer %q, %v", c.tema, got, ok, c.quer, c.acha)
		}
	}
}
