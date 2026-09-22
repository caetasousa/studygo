package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"studygo/internal/domain/concurso"
	"studygo/internal/domain/plano"
	"studygo/internal/domain/usuario"
	"studygo/internal/port"
	"studygo/internal/service"
)

// writeJSON serializa v com o status dado. Erro de codificação é logado, não
// devolvido: a linha de status já foi enviada.
func writeJSON(w http.ResponseWriter, logger *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if v == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.Error("codificando resposta", slog.Any("error", err))
	}
}

// writeError traduz um erro de domínio ou de aplicação em status e mensagem
// segura. Erro inesperado é logado com o id da requisição e vira 500.
func writeError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	status, msg := classificar(err)

	switch {
	case errors.Is(err, port.ErrProvedorIndisponivel):
		// Não é bug nosso: o provedor de IA está sobrecarregado ou lento. Vale
		// uma linha para a operação, mas como aviso, não como erro.
		logger.WarnContext(
			r.Context(),
			"importação de edital: provedor indisponível",
			slog.String("path", r.URL.Path),
			slog.Any("error", err),
		)
	case status >= http.StatusInternalServerError:
		logger.ErrorContext(
			r.Context(),
			"requisição falhou",
			slog.String("path", r.URL.Path),
			slog.Any("error", err),
		)
	}

	writeJSON(w, logger, status, map[string]string{"erro": msg})
}

func classificar(err error) (int, string) {
	var validacao service.ErrValidacao
	if errors.As(err, &validacao) {
		return http.StatusUnprocessableEntity, validacao.Msg
	}

	// A tag repetida nomeia a tag, então é um tipo e não um sentinela.
	var tagRepetida concurso.ErrCodigoRepetido
	if errors.As(err, &tagRepetida) {
		return http.StatusUnprocessableEntity, tagRepetida.Error()
	}

	switch {
	case errors.Is(err, usuario.ErrEmailEmUso):
		return http.StatusConflict, usuario.ErrEmailEmUso.Error()

	// Só chega aqui se o serviço esgotou as tentativas de sortear outro sufixo.
	case errors.Is(err, concurso.ErrSlugEmUso):
		return http.StatusConflict, concurso.ErrSlugEmUso.Error()

	case errors.Is(err, usuario.ErrCredenciaisInvalidas):
		return http.StatusUnauthorized, usuario.ErrCredenciaisInvalidas.Error()

	case errors.Is(err, usuario.ErrEmailInvalido),
		errors.Is(err, usuario.ErrSenhaFraca),
		errors.Is(err, usuario.ErrNomeObrigatorio):
		return http.StatusUnprocessableEntity, err.Error()

	// As invariantes do cadastro de concurso são validação de entrada do ponto
	// de vista de quem chama, não falha do servidor.
	case errors.Is(err, concurso.ErrNomeObrigatorio),
		errors.Is(err, concurso.ErrProvaObrigatoria),
		errors.Is(err, concurso.ErrSemDisciplina),
		errors.Is(err, concurso.ErrDisciplinaSemNome),
		errors.Is(err, concurso.ErrBlocoInvalido),
		errors.Is(err, concurso.ErrSemPontos):
		return http.StatusUnprocessableEntity, err.Error()

	case errors.Is(err, usuario.ErrNaoEncontrado),
		errors.Is(err, concurso.ErrNaoEncontrado),
		errors.Is(err, plano.ErrNaoEncontrado),
		errors.Is(err, plano.ErrAnotacaoNaoEncontrada),
		errors.Is(err, plano.ErrAtividadeNaoEncontrada):
		return http.StatusNotFound, err.Error()

	case errors.Is(err, errRequisicaoInvalida):
		return http.StatusBadRequest, err.Error()

	case errors.Is(err, errCorpoGrandeDemais):
		return http.StatusRequestEntityTooLarge, err.Error()

	case errors.Is(err, errNaoAutenticado):
		return http.StatusUnauthorized, "não autenticado"

	case errors.Is(err, port.ErrImportacaoIndisponivel):
		return http.StatusServiceUnavailable, err.Error()

	case errors.Is(err, port.ErrProvedorIndisponivel):
		return http.StatusServiceUnavailable,
			"a IA está sobrecarregada agora — tente de novo em alguns minutos " +
				"ou cadastre o concurso manualmente"

	default:
		return http.StatusInternalServerError, "erro interno"
	}
}

var (
	errRequisicaoInvalida = errors.New("requisição inválida")
	errNaoAutenticado     = errors.New("não autenticado")
	errCorpoGrandeDemais  = errors.New("corpo da requisição grande demais")
)

// Tetos de corpo, por natureza da rota.
//
// Um corpo sem teto é um convite: o decoder lê o que vier, e um POST
// autenticado de um gigabyte derruba o processo antes de qualquer validação. O
// limite é por requisição e vale mesmo com Content-Length mentindo, porque quem
// conta os bytes é a leitura, não o cabeçalho.
const (
	// maxCorpoJSON cobre com folga o maior payload comum — o cadastro de um
	// concurso com todas as disciplinas, temas e fontes preenchidas.
	maxCorpoJSON = 1 << 20 // 1 MiB

	// maxCorpoPlanilha é o teto das rotas que trazem um CSV DENTRO do JSON. O
	// CSV tem o próprio limite (maxPlanilhaTEC); a folga aqui paga o custo de
	// escapá-lo como string JSON.
	maxCorpoPlanilha = 8 << 20 // 8 MiB
)

// decode lê um corpo JSON em v, recusando campos desconhecidos: um campo com
// nome errado é um bug do cliente, e falhar alto é melhor que ignorá-lo.
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	return decodeLimitado(w, r, v, maxCorpoJSON)
}

// decodeLimitado é decode com um teto explícito, para as rotas que legitimamente
// recebem mais que um formulário.
//
// O corpo é trocado por um que se recusa a crescer além de max ANTES de o
// decoder tocar nele: sem isso o limite seria checado depois de a memória já ter
// sido alocada, que é exatamente o que se quer evitar.
func decodeLimitado(w http.ResponseWriter, r *http.Request, v any, max int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, max)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(v); err != nil {
		return traduzirCorpo(err)
	}

	return nil
}

// traduzirCorpo separa "não coube" de "não presta": os dois são culpa de quem
// chamou, mas só o primeiro tem conserto do lado do cliente (mandar menos).
func traduzirCorpo(err error) error {
	var grande *http.MaxBytesError
	if errors.As(err, &grande) {
		return errCorpoGrandeDemais
	}

	return errRequisicaoInvalida
}
