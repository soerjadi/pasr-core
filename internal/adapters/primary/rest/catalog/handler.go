// Package catalog serves the public product, seller and review read endpoints.
package catalog

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

// viewerID is the signed-in user's id, or 0 for anonymous callers (OptionalAuth leaves no principal).
func viewerID(r *http.Request) int64 {
	if c, ok := rest.Principal(r.Context()); ok {
		return c.UserID
	}
	return 0
}

func badRequest(w http.ResponseWriter, msg string) {
	rest.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", msg)
}

// queryInt reads an optional non-negative integer query parameter.
func queryInt(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		badRequest(w, name+" must be a positive integer")
		return 0, false
	}
	return n, true
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

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := domain.ProductFilter{Q: q.Get("q"), Cursor: q.Get("cursor"), ViewerID: viewerID(r)}
	var ok bool
	if f.CategoryID, ok = queryInt(w, r, "categoryId"); !ok {
		return
	}
	if f.SellerID, ok = queryInt(w, r, "sellerId"); !ok {
		return
	}
	if f.Limit, ok = rest.Limit(w, r); !ok {
		return
	}
	if raw := q.Get("featured"); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			badRequest(w, "featured must be a boolean")
			return
		}
		f.Featured = b
	}
	page, err := h.catalog.ListProducts(r.Context(), f)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, page)
}

// GetProduct runs behind OptionalAuth.
func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "productId")
	if !ok {
		return
	}
	p, err := h.catalog.GetProduct(r.Context(), id, viewerID(r))
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, p)
}

// GetSeller runs behind OptionalAuth.
func (h *Handler) GetSeller(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "sellerId")
	if !ok {
		return
	}
	s, err := h.catalog.GetSeller(r.Context(), id, viewerID(r))
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, s)
}

func (h *Handler) ListSellerReviews(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "sellerId")
	if !ok {
		return
	}
	limit, ok := rest.Limit(w, r)
	if !ok {
		return
	}
	page, err := h.catalog.ListSellerReviews(r.Context(), id, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, page)
}
