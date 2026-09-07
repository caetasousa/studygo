// Package middleware reúne o que atravessa todas as requisições: id de
// correlação, recuperação de pânico, log estruturado, CORS e o teto de
// requisições. A autenticação NÃO mora aqui — ela precisa do emissor de token,
// e por isso fica no adapter httpapi, junto de quem conhece esse contrato.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ctxKey int

const requestIDKey ctxKey = iota

// Chain compõe os middlewares de modo que o PRIMEIRO argumento seja o mais
// externo — a ordem em que se lê a chamada é a ordem em que a requisição
// atravessa.
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}

	return h
}

// RequestID prende um identificador ao contexto e o devolve no cabeçalho.
//
// Reaproveita o X-Request-Id que veio, se veio: é o que liga a linha de log do
// backend à do edital-processor quando um problema atravessa os dois.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}

		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom devolve o id que RequestID guardou, ou "" se não houver.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)

	return id
}

// Recover transforma um pânico em 500 e uma linha de log, mantendo o servidor
// de pé. Um handler que quebra derruba a requisição dele, não o processo.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.ErrorContext(
						r.Context(),
						"panic recovered",
						slog.Any("panic", rec),
						slog.String("path", r.URL.Path),
						slog.String("request_id", RequestIDFrom(r.Context())),
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"erro":"erro interno"}`))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// Logger emite uma linha estruturada por requisição: método, rota, status e
// duração. A MENSAGEM é fixa e os identificadores são atributos — o contrário
// (id dentro da mensagem) produz uma mensagem distinta por requisição e torna
// impossível agrupar por tipo de evento.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			logger.InfoContext(
				r.Context(),
				"http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", RequestIDFrom(r.Context())),
			)
		})
	}
}

// CORS responde ao preflight e libera a origem configurada.
//
// Uma origem só, nunca "*": com o token no cabeçalho Authorization, um curinga
// aqui deixaria qualquer site chamar a API com a credencial de quem estivesse
// logado. Em produção o navegador nem passa por aqui — a SPA e a API são
// servidas da mesma origem —, mas o desenvolvimento local usa duas portas.
func CORS(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Vary", "Origin")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap deixa o http.ResponseController alcançar o writer de baixo — é o que
// permite ao handler de importação esticar o prazo de escrita enquanto espera a
// IA. Sem isto, embrulhar o writer para contar o status quebraria esse ajuste.
func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
