package category

import (
	"net/http"

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

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.catalog.ListCategories(r.Context())
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	if items == nil {
		items = []domain.Category{}
	}
	rest.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
