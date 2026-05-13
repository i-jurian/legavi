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

	"github.com/i-jurian/legavi/backend/internal/auth"
	"github.com/i-jurian/legavi/backend/internal/config"
	"github.com/i-jurian/legavi/backend/internal/pool"
)

type Server struct {
	cfg    *config.Config
	db     *pool.DB
	log    *slog.Logger
	auth   *auth.Handler
	apiSrv *http.Server
}

func New(cfg *config.Config, database *pool.DB, log *slog.Logger, authH *auth.Handler) *Server {
	s := &Server{cfg: cfg, db: database, log: log, auth: authH}
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

	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register/start", s.auth.RegisterStart)
		r.Post("/register/verify", s.auth.RegisterVerify)
		r.Post("/login/start", s.auth.LoginStart)
		r.Post("/login/verify", s.auth.LoginVerify)
	})

	r.Group(func(r chi.Router) {
		r.Use(s.auth.RequireSession)
		r.Post("/api/v1/auth/logout", s.auth.Logout)
		r.Get("/api/v1/auth/me", s.auth.Me)
	})

	return r
}

func (s *Server) Start(ctx context.Context) error {
	apiErr := make(chan error, 1)

	go func() {
		s.log.Info("api listening", "addr", s.cfg.APIListen)
		if err := s.apiSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			apiErr <- err
			return
		}
		apiErr <- nil
	}()

	select {
	case <-ctx.Done():
		return s.shutdown()
	case err := <-apiErr:
		return fmt.Errorf("api server: %w", err)
	}
}

func (s *Server) shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.apiSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("api shutdown: %w", err)
	}
	return nil
}
