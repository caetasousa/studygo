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
			// Long enough for a normal request; the edital-import handler, which
			// waits on an external LLM call, extends its own deadline via
			// http.ResponseController.
			WriteTimeout: 30 * time.Second,
		},
		shutdownTimeout: 10 * time.Second,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Run starts the server and blocks until ctx is canceled, then shuts down
// gracefully. It returns nil on a clean shutdown.
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
