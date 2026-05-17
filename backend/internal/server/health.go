package server

import (
	"context"
	"net/http"
	"time"

	"github.com/i-jurian/legavi/backend/internal/respond"
)

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		respond.JSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "db unavailable",
			"error":  err.Error(),
		})
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
