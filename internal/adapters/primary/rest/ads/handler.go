// Package ads serves the public ad placements endpoint.
package ads

import (
	"net/http"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

type Handler struct {
	ads services.AdsService
}

func NewHandler(ads services.AdsService) *Handler { return &Handler{ads: ads} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.ads.List(r.Context(), r.URL.Query().Get("placement"))
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	rest.WriteJSON(w, http.StatusOK, struct {
		Items any `json:"items"`
	}{items})
}
