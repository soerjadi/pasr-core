package order

import (
	"net/http"
	"strconv"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
)

// SubmitReview runs behind the Auth middleware. A malformed id can never match, so it is a 404.
func (h *Handler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("orderItemId"), 10, 64)
	if err != nil || id <= 0 {
		rest.WriteError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	var req struct {
		Rating int32  `json:"rating"`
		Text   string `json:"text"`
	}
	if !decode(w, r, &req) {
		return
	}
	review, err := h.ordering.SubmitReview(r.Context(), p.UserID, id, req.Rating, req.Text)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusCreated, review)
}
