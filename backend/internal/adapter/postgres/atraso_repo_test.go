//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// ComAtraso decide quem a varredura diária vai carregar. O LEFT JOIN com
// registros_atividade é o que separa "não estudou" de "estudou": errar aqui
// significa replanejar o cronograma de quem está em dia.
func TestPlanoRepo_ComAtraso(t *testing.T) {
	t.Parallel()

	r := novoRepos(t)
	u := r.criarUsuario(t, "atraso@b.c")
	c := r.criarConcurso(t, u, "tce-go")
	p := r.criarPlano(t, u, c)

	ontem := dia(2026, time.September, 1)
	hoje := dia(2026, time.September, 2)
	amanha := dia(2026, time.September, 3)
	d := c.Disciplinas[0]

	lidas := r.criarAtividades(t, p, c, []plano.Atividade{
		umDia(d, ontem, 0, "vencida"),
		umDia(d, hoje, 0, "de hoje"),
		umDia(d, amanha, 0, "futura"),
	})

	// Sem nada registrado, o dia de ontem deixa o plano atrasado.
	if got := r.slugsComAtraso(t, hoje); len(got) != 1 || got[0] != c.Slug {
		t.Fatalf("com atraso = %v, quer [%s]", got, c.Slug)
	}

	// Registrar SEM concluir não quita o dia: continuar em aberto é continuar
	// atrasado.
	r.registrarAtividade(t, p, lidas[0].ID, false)

	if got := r.slugsComAtraso(t, hoje); len(got) != 1 {
		t.Errorf("com atraso = %v, quer o plano ainda atrasado com registro aberto", got)
	}

	// Concluída, o atraso acaba — hoje e amanhã não contam.
	r.registrarAtividade(t, p, lidas[0].ID, true)

	if got := r.slugsComAtraso(t, hoje); len(got) != 0 {
		t.Errorf("com atraso = %v, quer vazio depois de concluir o dia vencido", got)
	}
}

func (r *repos) slugsComAtraso(t *testing.T, hoje time.Time) []string {
	t.Helper()

	ps, err := r.planos.ComAtraso(t.Context(), hoje)
	if err != nil {
		t.Fatalf("ComAtraso: %v", err)
	}

	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Slug)
	}

	return out
}

// registrarAtividade grava (ou regrava) o registro de uma atividade. Concluido
// é o que ComAtraso lê para decidir se o dia aconteceu.
func (r *repos) registrarAtividade(
	t *testing.T,
	p plano.Plano,
	atividadeID uuid.UUID,
	concluido bool,
) {
	t.Helper()

	if err := r.cronograma.SalvarRegistro(t.Context(), p.ID, plano.RegistroAtividade{
		AtividadeID: atividadeID,
		Concluido:   concluido,
	}); err != nil {
		t.Fatalf("SalvarRegistro: %v", err)
	}
}
