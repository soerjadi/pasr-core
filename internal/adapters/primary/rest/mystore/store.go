package mystore

import (
	"net/http"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
)

func (h *Handler) GetStore(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	out, err := h.store.GetMyStore(r.Context(), p.UserID)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) UpdateStore(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	var req domain.StorePatch
	if !decode(w, r, &req) {
		return
	}
	out, err := h.store.UpdateMyStore(r.Context(), p.UserID, req)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) SetOpen(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	var req domain.SetOpenRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Open == nil {
		rest.WriteServiceError(w, r, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "open is required"))
		return
	}
	if err := h.store.SetMyStoreOpen(r.Context(), p.UserID, *req.Open); err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
