package concurso

import "strings"

// Prazos são os três marcos do edital que importam a quem estuda: o período
// de inscrições, o último dia de pagamento da inscrição e a prova. Nil quando
// o edital não traz o marco. Os demais (impugnação, isenção, recursos,
// divulgações) ficam na página de datas, sem virar aviso.
type Prazos struct {
	Inscricao *Marco
	Pagamento *Marco
	Prova     *Marco
}

// Palavras de marcos que CITAM a inscrição ou o pagamento sem serem eles: a
// isenção fala em "pagamento do valor de inscrição", o recurso fala das
// "inscrições indeferidas".
var foraDosPrazos = []string{
	"isen", "recurso", "divulga", "deferid", "homologa", "resultado", "convoca", "publica", "impugna",
}

// PrazosChave acha os três marcos na ordem em que o edital os traz: o
// primeiro de cada tipo vale.
func PrazosChave(marcos []Marco) Prazos {
	var p Prazos

	for i := range marcos {
		m := &marcos[i]
		t := strings.ToLower(m.Titulo)

		if m.EProva {
			if p.Prova == nil && !contemAlgum(t, "recurso") {
				p.Prova = m
			}

			continue
		}

		if contemAlgum(t, foraDosPrazos...) {
			continue
		}

		switch {
		case p.Pagamento == nil && (contemAlgum(t, "pagamento", "boleto") || (strings.Contains(t, "taxa") && strings.Contains(t, "inscri"))):
			p.Pagamento = m
		case p.Inscricao == nil && strings.Contains(t, "inscri") && !contemAlgum(t, "pagamento", "boleto"):
			p.Inscricao = m
		}
	}

	return p
}

func contemAlgum(texto string, partes ...string) bool {
	for _, p := range partes {
		if strings.Contains(texto, p) {
			return true
		}
	}

	return false
}
