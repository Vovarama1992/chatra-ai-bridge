package chatra

import "github.com/go-chi/chi/v5"

func RegisterRoutes(r chi.Router, h *Handler) {
	r.Post("/chatra/webhook", h.HandleWebhook)
}
