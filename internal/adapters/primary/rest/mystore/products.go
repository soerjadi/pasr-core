// Package mystore serves the signed-in seller's store management endpoints.
package mystore

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

const maxBodyBytes = 16 << 10

type Handler struct {
	store    services.StoreManagementService
	ordering services.OrderingService
}

func NewHandler(store services.StoreManagementService, ordering services.OrderingService) *Handler {
	return &Handler{store: store, ordering: ordering}
}

// Every method must run behind the Auth middleware.

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	var req domain.ProductInput
	if !decode(w, r, &req) {
		return
	}
	out, err := h.store.CreateProduct(r.Context(), p.UserID, req)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusCreated, out)
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit, ok := rest.Limit(w, r)
	if !ok {
		return
	}
	out, err := h.store.ListMyProducts(r.Context(), p.UserID, q.Get("status"), q.Get("cursor"), limit)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	out, err := h.store.GetMyProduct(r.Context(), p.UserID, id)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req domain.ProductPatch
	if !decode(w, r, &req) {
		return
	}
	out, err := h.store.UpdateMyProduct(r.Context(), p.UserID, id, req)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteMyProduct(r.Context(), p.UserID, id); err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func principal(w http.ResponseWriter, r *http.Request) (domain.Claims, bool) {
	p, ok := rest.Principal(r.Context())
	if !ok {
		rest.WriteServiceError(w, r, domain.ErrUnauthorized)
	}
	return p, ok
}

// pathID parses the {id} path value; a malformed id can never match a row, so it is a 404.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		rest.WriteError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return 0, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		rest.WriteServiceError(w, r, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "invalid request body"))
		return false
	}
	return true
}
