package testapp

import (
	"net/http"
	"testing"

	"github.com/oemah/nusantara/core/internal/testutil/contract"
)

func TestCategoriesEndpoint(t *testing.T) {
	env := New(t)

	resp := env.Do(http.MethodGet, "/v1/categories", nil)

	if resp.Status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Status, resp.Body)
	}
	contract.AssertShape(t, "listCategories", resp.Body)
}
