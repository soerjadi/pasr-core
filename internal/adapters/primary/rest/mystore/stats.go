package mystore

import (
	"net/http"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
)

func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	out, err := h.ordering.StoreSummary(r.Context(), p.UserID)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	out, err := h.ordering.StoreStats(r.Context(), p.UserID, r.URL.Query().Get("range"))
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}
