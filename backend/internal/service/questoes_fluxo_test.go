//go:build integration

package service

import (
	"context"
	"testing"
)

// O balanceamento edita as questões de UMA matéria por vez: o pedido traz só
// ela. Como pode errar — escrito antes da correção:
//
//	Q1  editar uma matéria devolve as outras ao número do edital, e a edição anterior se perde
//	Q2  a matéria editada não fica com o número pedido
func TestPlanoService_Salvar_questoesDeUmaMateriaNaoReverteAsOutras(t *testing.T) {
	t.Parallel()

	ce := novoCenario(t)
	ctx := context.Background()
	ce.obter(t)

	svc := NewPlanoService(ce.deps)

	if _, err := svc.Salvar(ctx, ce.usuario, ce.slug, ConfigCommand{Questoes: map[string]int{"LINPO": 30}}); err != nil {
		t.Fatalf("salvando LINPO: %v", err)
	}

	if _, err := svc.Salvar(ctx, ce.usuario, ce.slug, ConfigCommand{Questoes: map[string]int{"BANDA": 5}}); err != nil {
		t.Fatalf("salvando BANDA: %v", err)
	}

	q := ce.plano(t).Config.Questoes
	if q["LINPO"] != 30 {
		t.Errorf("Q1: LINPO voltou para %d, quer 30 (a edição anterior)", q["LINPO"])
	}

	if q["BANDA"] != 5 {
		t.Errorf("Q2: BANDA ficou com %d, quer 5", q["BANDA"])
	}
}
