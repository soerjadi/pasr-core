package category

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

type stubCatalog struct {
	services.CatalogService
	items []domain.Category
}

func (s stubCatalog) ListCategories(context.Context) ([]domain.Category, error) { return s.items, nil }

func TestList(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(stubCatalog{}).List(rec, httptest.NewRequest(http.MethodGet, "/v1/categories", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}
