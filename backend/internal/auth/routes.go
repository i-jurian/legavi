package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/i-jurian/legavi/backend/internal/respond"
)

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/register/start", respond.Handle(h.RegisterStart))
	r.Post("/register/verify", respond.Handle(h.RegisterVerify))
	r.Post("/login/start", respond.Handle(h.LoginStart))
	r.Post("/login/verify", respond.Handle(h.LoginVerify))
	r.Group(func(r chi.Router) {
		r.Use(h.RequireSession)
		r.Post("/logout", respond.Handle(h.Logout))
		r.Get("/me", respond.Handle(h.Me))
		r.Post("/unlock/start", respond.Handle(h.UnlockStart))
		r.Post("/unlock/verify", respond.Handle(h.UnlockVerify))
	})
	return r
}
