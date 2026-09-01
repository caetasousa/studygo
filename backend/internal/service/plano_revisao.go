package service

import (
	"context"
	"strings"
	"time"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"

	"github.com/google/uuid"
)

// montarRevisao builds one day's review-tail view: what has already been
// logged for it (RegistroRevisao) and the observação already saved, if any.
func montarRevisao(
	disciplina string,
	dt time.Time,
	registros map[time.Time]plano.RegistroRevisao,
	anotacoes []plano.Anotacao,
) *RevisaoResposta {
	out := &RevisaoResposta{Disciplina: disciplina}

	if r, ok := registros[dt]; ok {
		out.Questoes = r.Questoes
		out.Acertos = r.Acertos
	}

	if a := anotacaoDaRevisao(anotacoes, dt); a != nil {
		out.AnotacaoID = a.ID.String()
		out.Observacao = a.Texto
	}

	return out
}

// anotacaoDaRevisao finds the notebook entry a review's observação lives in —
// the one written from this exact review, not just any note that happens to
// share its date.
func anotacaoDaRevisao(anotacoes []plano.Anotacao, dt time.Time) *plano.Anotacao {
	for i := range anotacoes {
		a := anotacoes[i]
		if a.Origem == plano.OrigemRevisao && a.Data != nil && plano.DayOf(*a.Data).Equal(dt) {
			return &anotacoes[i]
		}
	}

	return nil
}

// RevisaoInput is what one review-tail log carries: the battery's result, and
// an optional note. An empty Observacao on a review that already had one
// clears it rather than leaving the old text behind.
type RevisaoInput struct {
	Questoes   *int   `json:"questoes"`
	Acertos    *int   `json:"acertos"`
	Observacao string `json:"observacao"`
}

// RegistrarRevisao logs one day's review-tail result: the battery (questões
// e acertos), and — reusing the same notebook the rest of the app writes to —
// an observação, filed under the discipline the review actually covered that
// day. Editing the observação again updates the same entry rather than piling
// up a new one per save.
func (s *PlanoService) RegistrarRevisao(
	ctx context.Context,
	userID uuid.UUID,
	slug, dataISO string,
	in RevisaoInput,
) (PlanoResposta, error) {
	c, salvo, err := s.carregar(ctx, userID, slug)
	if err != nil {
		return PlanoResposta{}, err
	}

	data, ok := parseISODate(dataISO)
	if !ok {
		return PlanoResposta{}, ErrValidacao{Msg: "data inválida"}
	}

	res := plano.Gerar(salvo.Config, &c)
	plano.AplicarReordenacoes(res.Dias, salvo.Reordenacoes)

	d := diaDoResultado(res.Dias, data)
	if d == nil {
		return PlanoResposta{}, ErrValidacao{Msg: "esse dia não faz parte do plano"}
	}

	revisao := plano.FilaRevisao(res.Dias, temasPorRevisao(salvo.Config), foiEstudado(salvo))[d.N]
	if len(revisao) == 0 {
		return PlanoResposta{}, ErrValidacao{
			Msg: "esse dia ainda não tem revisão — ela começa a partir do segundo dia de estudo",
		}
	}

	if in.Questoes != nil && (in.Acertos == nil || *in.Acertos > *in.Questoes) {
		return PlanoResposta{}, ErrValidacao{Msg: "acertos não pode passar de questões"}
	}

	reg := plano.RegistroRevisao{Data: plano.DayOf(data), Questoes: in.Questoes, Acertos: in.Acertos}
	if err := s.planos.UpsertRevisaoRegistro(ctx, salvo.ID, reg); err != nil {
		return PlanoResposta{}, err
	}

	salvo.Revisoes[reg.Data] = reg

	if err := s.salvarObservacaoRevisao(ctx, c, salvo, revisao[0].Disciplina, data, in.Observacao); err != nil {
		return PlanoResposta{}, err
	}

	return s.montar(ctx, c, salvo)
}

// salvarObservacaoRevisao writes the review's note to the notebook: creates it
// the first time, edits the same entry after, and removes it once the text is
// cleared rather than leaving an empty note behind.
func (s *PlanoService) salvarObservacaoRevisao(
	ctx context.Context,
	c concurso.Concurso,
	salvo plano.Salvo,
	disciplina string,
	data time.Time,
	texto string,
) error {
	existentes, err := s.planos.ListAnotacoes(ctx, salvo.ID)
	if err != nil {
		return err
	}

	dt := plano.DayOf(data)
	existente := anotacaoDaRevisao(existentes, dt)
	texto = strings.TrimSpace(texto)

	switch {
	case texto == "" && existente != nil:
		return s.planos.DeleteAnotacao(ctx, salvo.ID, existente.ID)
	case texto == "":
		return nil
	}

	a := plano.Anotacao{Data: &dt, Texto: texto, Origem: plano.OrigemRevisao}

	if disc := c.DisciplinaByCodigo(disciplina); disc != nil {
		a.DisciplinaID = &disc.ID
	}

	if existente != nil {
		a.ID = existente.ID

		_, err := s.planos.UpdateAnotacao(ctx, salvo.ID, a)

		return err
	}

	_, err = s.planos.CreateAnotacao(ctx, salvo.ID, a)

	return err
}

// diaDoResultado finds one generated day by date.
func diaDoResultado(dias []plano.Dia, dt time.Time) *plano.Dia {
	dt = plano.DayOf(dt)

	for i := range dias {
		if plano.DayOf(dias[i].Data).Equal(dt) {
			return &dias[i]
		}
	}

	return nil
}
