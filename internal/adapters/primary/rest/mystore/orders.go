package mystore

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
)

// RejectRequest matches the RejectRequest schema.
type RejectRequest struct {
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

// ReadyRequest matches the ReadyRequest schema; the body is optional.
type ReadyRequest struct {
	Note string `json:"note"`
}

// Every method must run behind the Auth middleware. Handlers only parse and call the service.

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
	out, err := h.ordering.ListStoreOrders(r.Context(), p.UserID, q.Get("tab"), q.Get("cursor"), limit)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	out, err := h.ordering.GetStoreOrder(r.Context(), p.UserID, id)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) AcceptOrder(w http.ResponseWriter, r *http.Request) {
	h.act(w, r, func(a domain.Actor, id int64) (domain.StoreOrder, error) {
		return h.ordering.Accept(r.Context(), a, id)
	})
}

func (h *Handler) RejectOrder(w http.ResponseWriter, r *http.Request) {
	var req RejectRequest
	if !decode(w, r, &req) {
		return
	}
	h.act(w, r, func(a domain.Actor, id int64) (domain.StoreOrder, error) {
		return h.ordering.Reject(r.Context(), a, id, req.Reason, req.Note)
	})
}

func (h *Handler) ReadyOrder(w http.ResponseWriter, r *http.Request) {
	var req ReadyRequest
	if !decodeOptional(w, r, &req) {
		return
	}
	h.act(w, r, func(a domain.Actor, id int64) (domain.StoreOrder, error) {
		return h.ordering.Ready(r.Context(), a, id, req.Note)
	})
}

func (h *Handler) CompleteOrder(w http.ResponseWriter, r *http.Request) {
	h.act(w, r, func(a domain.Actor, id int64) (domain.StoreOrder, error) {
		return h.ordering.Complete(r.Context(), a, id)
	})
}

// act runs one seller action on the {id} seller-order and writes the resulting StoreOrder.
func (h *Handler) act(w http.ResponseWriter, r *http.Request, fn func(domain.Actor, int64) (domain.StoreOrder, error)) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	out, err := fn(domain.Actor{Role: domain.ActorSeller, UserID: p.UserID}, id)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}

// decodeOptional is decode for a body that may be absent: an empty body leaves into unchanged.
func decodeOptional(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil && !errors.Is(err, io.EOF) {
		rest.WriteServiceError(w, r, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "invalid request body"))
		return false
	}
	return true
}
