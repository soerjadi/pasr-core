// Package order serves the ordering endpoints: cart validation, and later orders.
package order

import (
	"encoding/json"
	"net/http"

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

// ValidateCart is public: a signed-in caller is treated like an anonymous one.
func (h *Handler) ValidateCart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lines []domain.CartLineInput `json:"lines"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rest.WriteServiceError(w, r, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "invalid request body"))
		return
	}
	out, err := h.ordering.ValidateCart(r.Context(), req.Lines)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}
