// Package server wraps net/http with the project's timeouts and graceful
// shutdown semantics.
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Server is a thin wrapper around *http.Server.
type Server struct {
	http     *http.Server
	shutdown time.Duration
	logger   shared.Logger
}

// New builds a server bound to the configured port serving the given handler.
func New(cfg config.HTTPConfig, handler http.Handler, logger shared.Logger) *Server {
	return &Server{
		http: &http.Server{
			Addr:        fmt.Sprintf(":%d", cfg.Port),
			Handler:     handler,
			ReadTimeout: cfg.ReadTimeout,
			// ReadHeaderTimeout bounds slow-header (Slowloris) attacks independently
			// of the body read timeout.
			ReadHeaderTimeout: cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       2 * cfg.ReadTimeout,
		},
		shutdown: cfg.ShutdownTimeout,
		logger:   logger,
	}
}

// Run serves until the context is cancelled, then drains in-flight requests
// within the shutdown timeout.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening", "addr", s.http.Addr)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("http server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdown)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	}
}
