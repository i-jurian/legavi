package vault

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/i-jurian/legavi/backend/internal/respond"
)

func (h *Handler) Routes(requireSession func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(requireSession)
	r.Get("/entries", respond.Handle(h.List))
	r.Post("/entries", respond.Handle(h.Create))
	r.Post("/entries/reorder", respond.Handle(h.Reorder))
	r.Get("/entries/{id}", respond.Handle(h.Get))
	r.Put("/entries/{id}", respond.Handle(h.Update))
	r.Delete("/entries/{id}", respond.Handle(h.SoftDelete))
	r.Post("/entries/{id}/restore", respond.Handle(h.Restore))
	return r
}
