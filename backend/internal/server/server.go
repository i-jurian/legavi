// Package server runs the HTTP API listener.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"

	"github.com/i-jurian/legavi/backend/internal/auth"
	"github.com/i-jurian/legavi/backend/internal/config"
	"github.com/i-jurian/legavi/backend/internal/database"
	"github.com/i-jurian/legavi/backend/internal/vault"
)

type Server struct {
	cfg     *config.Config
	db      *database.DB
	log     *slog.Logger
	auth    *auth.Handler
	vault   *vault.Handler
	apiSrv  *http.Server
	ipLimit *ipRateLimiter
}

func New(cfg *config.Config, database *database.DB, log *slog.Logger, authH *auth.Handler, vaultH *vault.Handler) *Server {
	s := &Server{
		cfg:     cfg,
		db:      database,
		log:     log,
		auth:    authH,
		vault:   vaultH,
		ipLimit: newIPRateLimiter(rate.Every(6*time.Second), 10), // burst 10 then refill 1 every 6s on all endpoints
	}
	s.apiSrv = &http.Server{
		Addr:              cfg.APIListen,
		Handler:           s.apiRoutes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) apiRoutes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(s.logRequests)

	r.Get("/healthz", s.healthz)
	r.Get("/readyz", s.readyz)

	r.Route("/api", func(r chi.Router) {
		r.Use(s.ipLimit.middleware)

		r.Route("/v1", func(r chi.Router) {
			r.Mount("/auth", s.auth.Routes())
			r.Mount("/vault", s.vault.Routes(s.auth.RequireSession))
		})
	})

	return r
}

func (s *Server) Start(ctx context.Context) error {
	apiErr := make(chan error, 1)

	go s.ipLimit.sweepLoop(ctx)

	go func() {
		s.log.Info("api listening", "addr", s.cfg.APIListen)
		if err := s.apiSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			apiErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.shutdown(shutdownCtx)
	case err := <-apiErr:
		return fmt.Errorf("api server: %w", err)
	}
}

func (s *Server) shutdown(ctx context.Context) error {
	if err := s.apiSrv.Shutdown(ctx); err != nil {
		return fmt.Errorf("api shutdown: %w", err)
	}
	return nil
}
