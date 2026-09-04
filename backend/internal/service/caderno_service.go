package service

import (
	"context"
	"strings"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/port"

	"github.com/google/uuid"
)

// CadernoService cuida do caderno de erros: o que o app deduz dos resultados
// ruins, e as anotações que o estudante escreve.
type CadernoService struct {
	carregador

	repo port.CadernoRepository
}

func NewCadernoService(deps Dependencias) *CadernoService {
	return &CadernoService{carregador: deps.carregador(), repo: deps.Caderno}
}

func (s *CadernoService) Caderno(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
) (Caderno, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return Caderno{}, err
	}

	anotacoes, err := s.repo.Anotacoes(ctx, c.Plano.ID)
	if err != nil {
		return Caderno{}, err
	}

	res := plano.Gerar(c.Plano.Config, &c.Concurso)
	plano.AplicarNosDias(res.Dias, c.Atividades)

	comNota, fracos := diasComNotaEFracos(res.Dias, c)

	return Caderno{
		PorDisciplina: cadernoPorDisciplina(c.Concurso, res.Dias, c),
		Anotacoes:     anotacoesDoCaderno(c.Concurso, anotacoes),
		DiasComNota:   comNota,
		DiasFracos:    fracos,
	}, nil
}

// cadernoPorDisciplina segue a ordem das disciplinas do concurso, para que a
// tela fique estável em vez de seguir a iteração de um map.
func cadernoPorDisciplina(
	cur concurso.Concurso,
	dias []plano.Dia,
	c contexto,
) []CadernoDaDisciplina {
	cadernos := plano.Caderno(resultadosDoPlano(dias, c))
	out := make([]CadernoDaDisciplina, 0, len(cadernos))

	for i, d := range cur.Disciplinas {
		itens, ok := cadernos[d.Codigo]
		if !ok || len(itens) == 0 {
			continue
		}

		temas := make([]ItemDoCaderno, 0, len(itens))
		for _, it := range itens {
			temas = append(temas, ItemDoCaderno{
				Tema:       it.Tema,
				Questoes:   it.Questoes,
				Acertos:    it.Acertos,
				Erros:      it.Erros,
				Aprov:      it.Aproveitamento(),
				UltimaData: it.UltimaData,
			})
		}

		out = append(out, CadernoDaDisciplina{
			Codigo: d.Codigo,
			Nome:   d.Nome,
			Cor:    i % concurso.TotalCoresDisciplinas,
			Itens:  temas,
		})
	}

	return out
}

func anotacoesDoCaderno(
	cur concurso.Concurso,
	anotacoes []plano.Anotacao,
) []AnotacaoDoCaderno {
	codigoPorID := make(map[uuid.UUID]string, len(cur.Disciplinas))
	for _, d := range cur.Disciplinas {
		codigoPorID[d.ID] = d.Codigo
	}

	out := make([]AnotacaoDoCaderno, 0, len(anotacoes))

	for _, a := range anotacoes {
		ac := AnotacaoDoCaderno{
			ID:        a.ID,
			Tema:      a.Tema,
			Texto:     a.Texto,
			Origem:    string(a.Origem),
			URL:       a.URL,
			Resolvido: a.Resolvido,
		}

		if a.Data != nil {
			s := plano.DayOf(*a.Data).Format(formatoISO)
			ac.Data = &s
		}

		if a.DisciplinaID != nil {
			ac.Disciplina = codigoPorID[*a.DisciplinaID]
		}

		out = append(out, ac)
	}

	return out
}

// diasComNotaEFracos percorre o plano uma vez só e devolve as duas listas que a
// tela do caderno mostra: os dias em que o estudante escreveu algo, e aqueles
// cujo aproveitamento ficou abaixo do limiar — os que valem uma segunda passada.
func diasComNotaEFracos(dias []plano.Dia, c contexto) ([]DiaComNota, []DiaFraco) {
	limiar := c.Plano.Config.Normalizar().LimiarFraco
	comNota := []DiaComNota{}
	fracos := []DiaFraco{}

	for _, d := range dias {
		dt := plano.DayOf(d.Data)

		if reg, ok := c.Dias[dt]; ok && reg.Nota != "" {
			discs := []string{}
			for _, a := range plano.AtividadesDoDia(c.Atividades, dt) {
				if a.Disciplina != "" {
					discs = append(discs, a.Disciplina)
				}
			}

			comNota = append(comNota, DiaComNota{
				Data: dt.Format(formatoISO), N: d.N, Disciplinas: discs, Nota: reg.Nota,
			})
		}

		_, questoes, acertos := plano.TotaisDoDia(c.Atividades, c.Registros, dt)
		if questoes == nil || *questoes <= 0 {
			continue
		}

		a := valorInt(acertos)

		if pct := a * 100 / *questoes; pct < limiar {
			fracos = append(fracos, DiaFraco{
				Data:     dt.Format(formatoISO),
				N:        d.N,
				Questoes: *questoes,
				Acertos:  a,
				Aprov:    pct,
			})
		}
	}

	return comNota, fracos
}

// AnotacaoCommand é uma anotação como o formulário a envia.
type AnotacaoCommand struct {
	Data       string
	Disciplina string
	Tema       string
	Texto      string
	URL        string
	Resolvido  bool
}

func (s *CadernoService) Criar(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	cmd AnotacaoCommand,
) (Caderno, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return Caderno{}, err
	}

	a, err := anotacaoDoComando(c.Concurso, plano.Anotacao{Origem: plano.OrigemManual}, cmd)
	if err != nil {
		return Caderno{}, err
	}

	if _, err := s.repo.CriarAnotacao(ctx, c.Plano.ID, a); err != nil {
		return Caderno{}, err
	}

	return s.Caderno(ctx, usuarioID, slug)
}

func (s *CadernoService) Atualizar(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	id uuid.UUID,
	cmd AnotacaoCommand,
) (Caderno, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return Caderno{}, err
	}

	anotacoes, err := s.repo.Anotacoes(ctx, c.Plano.ID)
	if err != nil {
		return Caderno{}, err
	}

	base := plano.Anotacao{ID: id, Origem: plano.OrigemManual}

	for _, a := range anotacoes {
		if a.ID == id {
			base = a

			break
		}
	}

	a, err := anotacaoDoComando(c.Concurso, base, cmd)
	if err != nil {
		return Caderno{}, err
	}

	a.ID = id

	if _, err := s.repo.AtualizarAnotacao(ctx, c.Plano.ID, a); err != nil {
		return Caderno{}, err
	}

	return s.Caderno(ctx, usuarioID, slug)
}

func (s *CadernoService) Remover(
	ctx context.Context,
	usuarioID uuid.UUID,
	slug string,
	id uuid.UUID,
) (Caderno, error) {
	c, err := s.carregar(ctx, usuarioID, slug)
	if err != nil {
		return Caderno{}, err
	}

	if err := s.repo.RemoverAnotacao(ctx, c.Plano.ID, id); err != nil {
		return Caderno{}, err
	}

	return s.Caderno(ctx, usuarioID, slug)
}

func anotacaoDoComando(
	cur concurso.Concurso,
	base plano.Anotacao,
	cmd AnotacaoCommand,
) (plano.Anotacao, error) {
	texto := strings.TrimSpace(cmd.Texto)
	if texto == "" {
		return plano.Anotacao{}, erroDeValidacao("escreva alguma coisa na anotação")
	}

	base.Texto = texto
	base.Tema = strings.TrimSpace(cmd.Tema)
	base.URL = strings.TrimSpace(cmd.URL)
	base.Resolvido = cmd.Resolvido
	base.Origem = plano.OrigemValida(base.Origem)

	if cmd.Data != "" {
		data, err := dataISO(cmd.Data)
		if err != nil {
			return plano.Anotacao{}, err
		}

		base.Data = &data
	} else {
		base.Data = nil
	}

	base.DisciplinaID = nil

	if cmd.Disciplina != "" {
		d := cur.DisciplinaPorCodigo(cmd.Disciplina)
		if d == nil {
			return plano.Anotacao{}, erroDeValidacao("matéria não encontrada")
		}

		id := d.ID
		base.DisciplinaID = &id
	}

	return base, nil
}
