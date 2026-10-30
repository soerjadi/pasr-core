package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/category"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

type stubCatalog struct{ services.CatalogService }

func (stubCatalog) ListCategories(context.Context) ([]domain.Category, error) {
	return []domain.Category{{ID: 1, Name: "Makanan", IconKey: "food"}}, nil
}

func TestRoutes(t *testing.T) {
	h := New(Handlers{Category: category.NewHandler(stubCatalog{})}, []string{"*"}, func(*http.Request) error { return nil })

	for path, want := range map[string]int{"/healthz": 200, "/v1/categories": 200, "/v1/nope": 404} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("%s: got %d want %d", path, rec.Code, want)
		}
	}
}

func TestHealthzFailsWhenDBDown(t *testing.T) {
	down := func(*http.Request) error { return errors.New("connection refused") }
	h := New(Handlers{Category: category.NewHandler(stubCatalog{})}, []string{"*"}, down)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("healthz with the database down: %d, want 503", rec.Code)
	}
}
