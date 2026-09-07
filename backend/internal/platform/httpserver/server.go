package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type Server struct {
	httpServer      *http.Server
	shutdownTimeout time.Duration
}

type Option func(*Server)

func WithHandler(h http.Handler) Option {
	return func(s *Server) { s.httpServer.Handler = h }
}

func WithReadTimeout(d time.Duration) Option {
	return func(s *Server) { s.httpServer.ReadTimeout = d }
}

func WithWriteTimeout(d time.Duration) Option {
	return func(s *Server) { s.httpServer.WriteTimeout = d }
}

func WithShutdownTimeout(d time.Duration) Option {
	return func(s *Server) { s.shutdownTimeout = d }
}

func New(addr string, opts ...Option) *Server {
	s := &Server{
		httpServer: &http.Server{
			Addr:        addr,
			ReadTimeout: 15 * time.Second,
			// Suficiente para uma requisição normal. O handler de importação de
			// edital, que espera uma chamada externa de IA, estica o próprio prazo
			// pelo http.ResponseController.
			WriteTimeout: 30 * time.Second,
		},
		shutdownTimeout: 10 * time.Second,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Run sobe o servidor e bloqueia até o contexto ser cancelado, então desliga
// com calma. Devolve nil quando o desligamento foi limpo.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen and serve: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}

		return nil
	}
}
