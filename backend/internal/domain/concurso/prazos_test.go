package concurso_test

import (
	"testing"
	"time"

	"studygo/internal/domain/concurso"
)

// PrazosChave acha, entre os marcos do edital, os três que importam a quem
// estuda: as inscrições, o pagamento da inscrição e a prova. Como pode errar —
// escrito antes do código, com os títulos de um edital real da FCC:
//
//	P1  a isenção ("…Isenção do pagamento do valor de inscrição") é tomada por inscrição ou por pagamento
//	P2  recurso ou divulgação das inscrições é tomado pelo período de inscrições
//	P3  o último dia de pagamento (ou "boleto") não é reconhecido
//	P4  a prova não é achada pelo marco de prova, ou vem o recurso da prova
//	P5  edital sem esses marcos devolve algo em vez de nada
func TestPrazosChave(t *testing.T) {
	t.Parallel()

	d := func(m time.Month, dia int) time.Time { return time.Date(2026, m, dia, 0, 0, 0, 0, time.UTC) }
	marcos := []concurso.Marco{
		{Titulo: "Prazo para interposição de impugnação referente à Publicação do Edital.", DataInicio: d(8, 26), ExigeAcao: true},
		{Titulo: "Período da solicitação de Isenção do pagamento do valor de inscrição (exclusivamente via internet)", DataInicio: d(9, 8), ExigeAcao: true},
		{Titulo: "Prazo para interposição de recursos quanto ao resultado dos pedidos de isenção.", DataInicio: d(9, 22), ExigeAcao: true},
		{Titulo: "Período das inscrições (exclusivamente via internet)", DataInicio: d(10, 5), ExigeAcao: true},
		{Titulo: "Último dia para pagamento do valor da inscrição.", DataInicio: d(11, 9), ExigeAcao: true},
		{Titulo: "Divulgação das inscrições deferidas, das vagas reservadas", DataInicio: d(11, 23)},
		{Titulo: "Prazo para recurso quanto ao indeferimento das inscrições", DataInicio: d(11, 24), ExigeAcao: true},
		{Titulo: "Aplicação das Provas Objetivas e Discursivas", DataInicio: d(12, 17), ExigeAcao: true, EProva: true},
		{Titulo: "Prazo de interposição de recurso quanto à aplicação das Provas", DataInicio: d(12, 18), ExigeAcao: true},
	}

	p := concurso.PrazosChave(marcos)

	if p.Inscricao == nil || p.Inscricao.Titulo != "Período das inscrições (exclusivamente via internet)" {
		t.Errorf("P1/P2: inscrição = %+v", p.Inscricao)
	}

	if p.Pagamento == nil || p.Pagamento.Titulo != "Último dia para pagamento do valor da inscrição." {
		t.Errorf("P1/P3: pagamento = %+v", p.Pagamento)
	}

	if p.Prova == nil || p.Prova.Titulo != "Aplicação das Provas Objetivas e Discursivas" {
		t.Errorf("P4: prova = %+v", p.Prova)
	}

	boleto := concurso.PrazosChave([]concurso.Marco{{Titulo: "Vencimento do boleto bancário", DataInicio: d(11, 9)}})
	if boleto.Pagamento == nil {
		t.Error("P3: o vencimento do boleto não foi reconhecido")
	}

	if vazio := concurso.PrazosChave(nil); vazio.Inscricao != nil || vazio.Pagamento != nil || vazio.Prova != nil {
		t.Errorf("P5: edital sem marcos devolveu %+v", vazio)
	}
}
