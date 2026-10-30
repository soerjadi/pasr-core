// Package order serves the ordering endpoints: cart validation and the buyer's orders.
package order

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

// 50 lines of JSON fit well under this; it bounds what an anonymous caller can make us read.
const maxBodyBytes = 64 << 10

type Handler struct {
	ordering services.OrderingService
}

func NewHandler(ordering services.OrderingService) *Handler {
	return &Handler{ordering: ordering}
}

// CreateOrder answers 201 with the order, also when the Idempotency-Key replays a stored one,
// so the response to a retry is byte-for-byte the first response. Runs behind the Auth middleware.
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	var req struct {
		Lines []domain.CartLineInput `json:"lines"`
	}
	if !decode(w, r, &req) {
		return
	}
	o, err := h.ordering.CreateOrder(r.Context(), p.UserID, r.Header.Get("Idempotency-Key"), req.Lines)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusCreated, o)
}

// ListOrders runs behind the Auth middleware.
func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit := 0
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			rest.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "limit must be a positive integer")
			return
		}
		limit = min(n, 1000)
	}
	page, err := h.ordering.ListOrders(r.Context(), p.UserID, q.Get("tab"), q.Get("cursor"), limit)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, page)
}

// GetOrder runs behind the Auth middleware. A malformed id can never match, so it is a 404.
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("orderId"), 10, 64)
	if err != nil || id <= 0 {
		rest.WriteError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	o, err := h.ordering.GetOrder(r.Context(), p.UserID, id)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, o)
}

func principal(w http.ResponseWriter, r *http.Request) (domain.Claims, bool) {
	p, ok := rest.Principal(r.Context())
	if !ok {
		rest.WriteServiceError(w, r, domain.ErrUnauthorized)
	}
	return p, ok
}

// decode reads a JSON body of at most maxBodyBytes; failures are written as 400.
func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		rest.WriteServiceError(w, r, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "invalid request body"))
		return false
	}
	return true
}
