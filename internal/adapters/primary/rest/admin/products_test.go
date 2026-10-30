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

func (f *fx) patchProduct(t *testing.T, id int64, body any) domain.AdminProduct {
	t.Helper()
	r := f.do(http.MethodPatch, path("/v1/admin/products/%d", id), body)
	assertStatus(t, r, http.StatusOK)
	contract.AssertShape(t, "updateAdminProduct", r.Body)
	var p domain.AdminProduct
	r.JSON(&p)
	return p
}

func featuredIDs(t *testing.T, f *fx) []int64 {
	t.Helper()
	r := f.env.Do(http.MethodGet, "/v1/products?featured=true", nil)
	assertStatus(t, r, http.StatusOK)
	var page domain.Page[domain.ProductSummary]
	r.JSON(&page)
	var ids []int64
	for _, p := range page.Items {
		ids = append(ids, p.ID)
	}
	return ids
}

func TestPatchProduct_FeaturedAppearsInFeaturedList(t *testing.T) {
	f := setup(t)
	p := seed.Product(t, f.db, seed.ProductOpts{StoreID: f.sellerStore, Name: "Keripik"})
	seed.Product(t, f.db, seed.ProductOpts{StoreID: f.sellerStore})
	if ids := featuredIDs(t, f); len(ids) != 0 {
		t.Fatalf("featured before: %v", ids)
	}

	got := f.patchProduct(t, p, map[string]any{"featured": true, "unknownField": 1})
	if got.ID != p || !got.Featured || got.StoreID != f.sellerStore || got.Name != "Keripik" || got.Status != domain.ProductAvailable {
		t.Fatalf("patch: %+v", got)
	}
	if ids := featuredIDs(t, f); len(ids) != 1 || ids[0] != p {
		t.Fatalf("featured after: %v", ids)
	}

	// Fields left out stay as they are; featured off removes it again.
	if got := f.patchProduct(t, p, map[string]any{"featured": false}); got.Featured || got.HalalVerified {
		t.Fatalf("unfeature: %+v", got)
	}
	if ids := featuredIDs(t, f); len(ids) != 0 {
		t.Fatalf("featured after off: %v", ids)
	}
}

func TestPatchProduct_HalalVerifiedShowsBadgeFlag(t *testing.T) {
	f := setup(t)
	p := seed.Product(t, f.db, seed.ProductOpts{StoreID: f.sellerStore})
	detail := func() domain.ProductDetail {
		r := f.env.Do(http.MethodGet, path("/v1/products/%d", p), nil)
		assertStatus(t, r, http.StatusOK)
		var d domain.ProductDetail
		r.JSON(&d)
		return d
	}
	if detail().Product.HalalVerified {
		t.Fatal("halal before verification")
	}

	got := f.patchProduct(t, p, map[string]any{"halalVerified": true, "halalCertNo": "  ID00110012345678  "})
	if !got.HalalVerified || got.HalalCertNo == nil || *got.HalalCertNo != "ID00110012345678" {
		t.Fatalf("patch: %+v", got)
	}
	if !detail().Product.HalalVerified {
		t.Fatal("badge flag not public after verification")
	}

	// An empty certificate number clears it.
	if got := f.patchProduct(t, p, map[string]any{"halalCertNo": "", "halalVerified": false}); got.HalalCertNo != nil || got.HalalVerified {
		t.Fatalf("clear: %+v", got)
	}
	if detail().Product.HalalVerified {
		t.Fatal("badge still shown")
	}
}

func TestPatchProduct_StatusAndValidation(t *testing.T) {
	f := setup(t)
	p := seed.Product(t, f.db, seed.ProductOpts{StoreID: f.sellerStore, Stock: 3})
	empty := seed.Product(t, f.db, seed.ProductOpts{StoreID: f.sellerStore, Status: "OUT_OF_STOCK"})
	deleted := seed.Product(t, f.db, seed.ProductOpts{StoreID: f.sellerStore, Deleted: true})

	if got := f.patchProduct(t, p, map[string]any{"status": "HIDDEN"}); got.Status != domain.ProductHidden {
		t.Fatalf("hide: %+v", got)
	}
	assertStatus(t, f.env.Do(http.MethodGet, path("/v1/products/%d", p), nil), http.StatusNotFound)
	if got := f.patchProduct(t, p, map[string]any{"status": "AVAILABLE"}); got.Status != domain.ProductAvailable || got.Stock != 3 {
		t.Fatalf("show: %+v", got)
	}

	for name, body := range map[string]any{
		"unknown status":           map[string]any{"status": "SOLD"},
		"lowercase status":         map[string]any{"status": "hidden"},
		"cert too long":            map[string]any{"halalCertNo": strings.Repeat("9", 101)},
		"cert with NUL":            map[string]any{"halalCertNo": "ID\u0000"},
		"wrong type":               map[string]any{"featured": "yes"},
		"available, none in stock": nil,
	} {
		id := p
		if body == nil {
			id, body = empty, map[string]any{"status": "AVAILABLE"}
		}
		r := f.do(http.MethodPatch, path("/v1/admin/products/%d", id), body)
		if r.Status != http.StatusBadRequest {
			t.Fatalf("%s: got %d %s", name, r.Status, r.Body)
		}
	}
	r := f.env.Do(http.MethodPatch, path("/v1/admin/products/%d", p), nil, auth(f.op))
	assertErr(t, r, http.StatusBadRequest, "VALIDATION_FAILED")

	assertErr(t, f.do(http.MethodPatch, path("/v1/admin/products/%d", deleted), map[string]any{"featured": true}), http.StatusNotFound, "NOT_FOUND")
	assertErr(t, f.do(http.MethodPatch, "/v1/admin/products/999999", map[string]any{"featured": true}), http.StatusNotFound, "NOT_FOUND")

	var featured bool
	if err := f.db.QueryRow(context.Background(), `SELECT featured FROM products WHERE id = $1`, deleted).Scan(&featured); err != nil || featured {
		t.Fatalf("deleted product changed: %v %v", featured, err)
	}
}
