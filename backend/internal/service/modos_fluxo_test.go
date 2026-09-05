package service

import (
	"context"
	"testing"
)

// Regressão: a tela manda UMA matéria por clique ("modos": {"LINPO": "questoes"}).
// Se o serviço trocar o mapa inteiro em vez de mesclar, escolher o método de uma
// matéria apaga o das outras — que é o que o estudante vê como "mexi numa e a de
// baixo mudou".
func TestSalvar_ModoDeUmaMateriaNaoApagaAsOutras(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t)

	svc := NewPlanoService(ce.deps)
	ctx := context.Background()

	primeira := ce.concursos.c.Disciplinas[0].Codigo
	segunda := ce.concursos.c.Disciplinas[1].Codigo

	if _, err := svc.Salvar(ctx, ce.usuario, ce.slug, ConfigCommand{
		Modos: map[string]string{primeira: "questoes"},
	}); err != nil {
		t.Fatalf("salvando a primeira: %v", err)
	}

	if _, err := svc.Salvar(ctx, ce.usuario, ce.slug, ConfigCommand{
		Modos: map[string]string{segunda: "teoria"},
	}); err != nil {
		t.Fatalf("salvando a segunda: %v", err)
	}

	p := ce.obter(t)

	if got := p.Config.Modos[primeira]; got != "questoes" {
		t.Errorf("modo de %s = %q, quer \"questoes\" — a segunda gravação apagou a primeira", primeira, got)
	}

	if got := p.Config.Modos[segunda]; got != "teoria" {
		t.Errorf("modo de %s = %q, quer \"teoria\"", segunda, got)
	}
}

// Mesmo contrato para reforço, que a tela também manda uma matéria por vez.
func TestSalvar_ReforcoDeUmaMateriaNaoApagaOsOutros(t *testing.T) {
	ce := novoCenario(t)
	ce.obter(t)

	svc := NewPlanoService(ce.deps)
	ctx := context.Background()

	primeira := ce.concursos.c.Disciplinas[0].Codigo
	segunda := ce.concursos.c.Disciplinas[1].Codigo

	if _, err := svc.Salvar(ctx, ce.usuario, ce.slug, ConfigCommand{
		Reforcos: map[string]float64{primeira: 2},
	}); err != nil {
		t.Fatalf("salvando o primeiro: %v", err)
	}

	if _, err := svc.Salvar(ctx, ce.usuario, ce.slug, ConfigCommand{
		Reforcos: map[string]float64{segunda: 3},
	}); err != nil {
		t.Fatalf("salvando o segundo: %v", err)
	}

	p := ce.obter(t)

	if got := p.Config.Reforcos[primeira]; got != 2 {
		t.Errorf("reforço de %s = %v, quer 2 — a segunda gravação apagou a primeira", primeira, got)
	}

	if got := p.Config.Reforcos[segunda]; got != 3 {
		t.Errorf("reforço de %s = %v, quer 3", segunda, got)
	}
}
