// Package social serves the signed-in user's favorites and follows.
package social

import (
	"net/http"
	"strconv"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

type Handler struct {
	catalog services.CatalogService
}

func NewHandler(catalog services.CatalogService) *Handler {
	return &Handler{catalog: catalog}
}

// Every method must run behind the Auth middleware.

func principal(w http.ResponseWriter, r *http.Request) (domain.Claims, bool) {
	p, ok := rest.Principal(r.Context())
	if !ok {
		rest.WriteServiceError(w, r, domain.ErrUnauthorized)
	}
	return p, ok
}

// pathID parses a path id; a malformed one can never match a row, so it is a 404.
func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		rest.WriteError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return 0, false
	}
	return id, true
}

func (h *Handler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	limit, ok := rest.Limit(w, r)
	if !ok {
		return
	}
	page, err := h.catalog.ListFavorites(r.Context(), p.UserID, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, page)
}

// act runs a 204 action on the user and the path id.
func (h *Handler) act(param string, do func(r *http.Request, userID, id int64) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := principal(w, r)
		if !ok {
			return
		}
		id, ok := pathID(w, r, param)
		if !ok {
			return
		}
		if err := do(r, p.UserID, id); err != nil {
			rest.WriteServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	h.act("productId", func(r *http.Request, u, id int64) error { return h.catalog.AddFavorite(r.Context(), u, id) })(w, r)
}

func (h *Handler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {
	h.act("productId", func(r *http.Request, u, id int64) error { return h.catalog.RemoveFavorite(r.Context(), u, id) })(w, r)
}

func (h *Handler) Follow(w http.ResponseWriter, r *http.Request) {
	h.act("sellerId", func(r *http.Request, u, id int64) error { return h.catalog.Follow(r.Context(), u, id) })(w, r)
}

func (h *Handler) Unfollow(w http.ResponseWriter, r *http.Request) {
	h.act("sellerId", func(r *http.Request, u, id int64) error { return h.catalog.Unfollow(r.Context(), u, id) })(w, r)
}
