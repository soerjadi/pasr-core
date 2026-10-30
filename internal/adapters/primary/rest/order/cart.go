package order

import (
	"net/http"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
)

// ValidateCart is public: a signed-in caller is treated like an anonymous one.
func (h *Handler) ValidateCart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lines []domain.CartLineInput `json:"lines"`
	}
	if !decode(w, r, &req) {
		return
	}
	out, err := h.ordering.ValidateCart(r.Context(), req.Lines)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, out)
}
