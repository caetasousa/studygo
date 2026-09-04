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

	switch {
	case errors.Is(err, usuario.ErrEmailEmUso):
		return http.StatusConflict, usuario.ErrEmailEmUso.Error()

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
		errors.Is(err, plano.ErrAtividadeNaoEncontrada):
		return http.StatusNotFound, err.Error()

	case errors.Is(err, errRequisicaoInvalida):
		return http.StatusBadRequest, err.Error()

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
)

// decode lê um corpo JSON em v, recusando campos desconhecidos: um campo com
// nome errado é um bug do cliente, e falhar alto é melhor que ignorá-lo.
func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(v); err != nil {
		return errRequisicaoInvalida
	}

	return nil
}
