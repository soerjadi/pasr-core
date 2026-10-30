// Package admin serves the operator API under /v1/admin. Every handler must run behind the Auth
// and RequireOperator middleware; router.AdminRoutes registers them that way.
package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

const maxBodyBytes = 16 << 10

type Handler struct {
	admin services.AdminService
}

func NewHandler(admin services.AdminService) *Handler {
	return &Handler{admin: admin}
}

func (h *Handler) ListStores(w http.ResponseWriter, r *http.Request) {
	limit, ok := limitParam(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	out, err := h.admin.ListStores(r.Context(), q.Get("status"), q.Get("cursor"), limit)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) VerifyStore(w http.ResponseWriter, r *http.Request) {
	h.storeAction(w, r, h.admin.VerifyStore)
}

func (h *Handler) SuspendStore(w http.ResponseWriter, r *http.Request) {
	h.storeAction(w, r, h.admin.SuspendStore)
}

func (h *Handler) ReinstateStore(w http.ResponseWriter, r *http.Request) {
	h.storeAction(w, r, h.admin.ReinstateStore)
}

func (h *Handler) storeAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64) (domain.AdminStore, error)) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	out, err := fn(r.Context(), id)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req domain.AdminProductPatch
	if !decode(w, r, &req) {
		return
	}
	out, err := h.admin.UpdateProduct(r.Context(), id, req)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
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

// limitParam parses the optional limit query parameter; the service clamps it to 1..50.
func limitParam(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		rest.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "limit must be a positive integer")
		return 0, false
	}
	return min(n, 1000), true
}

// decode reads a JSON body; unknown fields are ignored like everywhere else in the API.
func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		rest.WriteServiceError(w, r, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "invalid request body"))
		return false
	}
	return true
}
