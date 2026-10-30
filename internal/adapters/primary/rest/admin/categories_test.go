package admin_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

// categories are reference data the test database never truncates, so every test removes what it adds.
func (f *fx) createCategory(t *testing.T, body map[string]any) domain.Category {
	t.Helper()
	r := f.do(http.MethodPost, "/v1/admin/categories", body)
	assertStatus(t, r, http.StatusCreated)
	contract.AssertShape(t, "createAdminCategory", r.Body)
	var c domain.Category
	r.JSON(&c)
	t.Cleanup(func() {
		_, _ = f.db.Exec(context.Background(), `DELETE FROM products WHERE category_id = $1`, c.ID)
		_, _ = f.db.Exec(context.Background(), `DELETE FROM stores WHERE category_id = $1`, c.ID)
		_, _ = f.db.Exec(context.Background(), `DELETE FROM categories WHERE id = $1`, c.ID)
	})
	return c
}

func publicCategories(t *testing.T, f *fx) map[int64]domain.Category {
	t.Helper()
	r := f.env.Do(http.MethodGet, "/v1/categories", nil)
	assertStatus(t, r, http.StatusOK)
	var list struct {
		Items []domain.Category `json:"items"`
	}
	r.JSON(&list)
	out := map[int64]domain.Category{}
	for _, c := range list.Items {
		out[c.ID] = c
	}
	return out
}

func TestCategories_CRUDAndDeleteBlockedWhenInUse(t *testing.T) {
	f := setup(t)
	before := len(publicCategories(t, f))

	c := f.createCategory(t, map[string]any{"name": "  Jamu & Herbal ", "iconKey": "herbal", "sortOrder": 20})
	if c.ID == 0 || c.Name != "Jamu & Herbal" || c.IconKey != "herbal" {
		t.Fatalf("create: %+v", c)
	}
	if got := publicCategories(t, f); len(got) != before+1 || got[c.ID] != c {
		t.Fatalf("public list: %+v", got)
	}

	r := f.do(http.MethodPatch, path("/v1/admin/categories/%d", c.ID), map[string]any{"name": "Jamu"})
	assertStatus(t, r, http.StatusOK)
	contract.AssertShape(t, "updateAdminCategory", r.Body)
	var patched domain.Category
	r.JSON(&patched)
	if patched.ID != c.ID || patched.Name != "Jamu" || patched.IconKey != "herbal" {
		t.Fatalf("patch: %+v", patched)
	}
	if got := publicCategories(t, f)[c.ID]; got.Name != "Jamu" {
		t.Fatalf("public after patch: %+v", got)
	}

	// A store or a product (even a deleted one) still refers to the category: delete is refused.
	usedByProduct := f.createCategory(t, map[string]any{"name": "Dipakai produk", "iconKey": "other"})
	seed.Product(t, f.db, seed.ProductOpts{StoreID: f.sellerStore, CategoryID: usedByProduct.ID, Deleted: true})
	assertErr(t, f.do(http.MethodDelete, path("/v1/admin/categories/%d", usedByProduct.ID), nil), http.StatusConflict, "CATEGORY_IN_USE")
	usedByStore := f.createCategory(t, map[string]any{"name": "Dipakai toko", "iconKey": "other"})
	seed.Store(t, f.db, seed.StoreOpts{CategoryID: usedByStore.ID})
	assertErr(t, f.do(http.MethodDelete, path("/v1/admin/categories/%d", usedByStore.ID), nil), http.StatusConflict, "CATEGORY_IN_USE")
	if got := publicCategories(t, f); got[usedByProduct.ID].ID == 0 || got[usedByStore.ID].ID == 0 {
		t.Fatalf("in-use category deleted: %+v", got)
	}

	r = f.do(http.MethodDelete, path("/v1/admin/categories/%d", c.ID), nil)
	assertStatus(t, r, http.StatusNoContent)
	if _, ok := publicCategories(t, f)[c.ID]; ok {
		t.Fatal("deleted category still listed")
	}
	assertErr(t, f.do(http.MethodDelete, path("/v1/admin/categories/%d", c.ID), nil), http.StatusNotFound, "NOT_FOUND")
	assertErr(t, f.do(http.MethodPatch, path("/v1/admin/categories/%d", c.ID), map[string]any{"name": "x"}), http.StatusNotFound, "NOT_FOUND")
}

func TestCategories_Validation(t *testing.T) {
	f := setup(t)
	before := len(publicCategories(t, f))
	for name, body := range map[string]any{
		"missing name":    map[string]any{"iconKey": "x"},
		"blank name":      map[string]any{"name": "   ", "iconKey": "x"},
		"missing iconKey": map[string]any{"name": "Baru"},
		"long name":       map[string]any{"name": strings.Repeat("a", 61), "iconKey": "x"},
		"NUL in name":     map[string]any{"name": "Ba\u0000ru", "iconKey": "x"},
		"NUL in iconKey":  map[string]any{"name": "Baru", "iconKey": "x\u0000"},
		"not an object":   []int{1},
	} {
		t.Run(name, func(t *testing.T) {
			assertErr(t, f.do(http.MethodPost, "/v1/admin/categories", body), http.StatusBadRequest, "VALIDATION_FAILED")
		})
	}
	if got := len(publicCategories(t, f)); got != before {
		t.Fatalf("invalid creates stored rows: %d -> %d", before, got)
	}

	c := f.createCategory(t, map[string]any{"name": "Baru", "iconKey": "x"})
	for _, body := range []map[string]any{{"name": ""}, {"iconKey": " "}, {"name": "\u0000"}} {
		assertErr(t, f.do(http.MethodPatch, path("/v1/admin/categories/%d", c.ID), body), http.StatusBadRequest, "VALIDATION_FAILED")
	}
}
