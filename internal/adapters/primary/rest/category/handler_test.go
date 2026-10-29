package category

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type stubCatalog struct{ items []domain.Category }

func (s stubCatalog) ListCategories(context.Context) ([]domain.Category, error) { return s.items, nil }

func TestList(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(stubCatalog{}).List(rec, httptest.NewRequest(http.MethodGet, "/v1/categories", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}
