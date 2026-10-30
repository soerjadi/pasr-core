package admin_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

func (f *fx) storeAction(t *testing.T, id int64, action string) domain.AdminStore {
	t.Helper()
	r := f.do(http.MethodPost, path("/v1/admin/stores/%d/%s", id, action), nil)
	assertStatus(t, r, http.StatusOK)
	contract.AssertShape(t, action+"AdminStore", r.Body)
	var s domain.AdminStore
	r.JSON(&s)
	return s
}

func (f *fx) storeStatus(t *testing.T, id int64) string {
	t.Helper()
	var s string
	if err := f.db.QueryRow(context.Background(), `SELECT status FROM stores WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *fx) verifiedNotifs(t *testing.T, owner int64) []map[string]string {
	t.Helper()
	rows, err := f.db.Query(context.Background(), `SELECT data FROM notifications WHERE user_id = $1 AND kind = 'STORE_VERIFIED' ORDER BY id`, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]string
	for rows.Next() {
		var d map[string]string
		if err := rows.Scan(&d); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

func TestVerifyStore_MakesProductsPublicAndNotifiesOwner(t *testing.T) {
	f := setup(t)
	store, owner := seed.Store(t, f.db, seed.StoreOpts{Name: "Warung Bu Tini", Status: "PENDING"})
	p := seed.Product(t, f.db, seed.ProductOpts{StoreID: store})
	productPath := path("/v1/products/%d", p)
	assertStatus(t, f.env.Do(http.MethodGet, productPath, nil), http.StatusNotFound)

	got := f.storeAction(t, store, "verify")
	if got.ID != store || got.Status != domain.StoreVerified || got.Name != "Warung Bu Tini" || got.OwnerID != owner {
		t.Fatalf("verify: %+v", got)
	}
	assertStatus(t, f.env.Do(http.MethodGet, productPath, nil), http.StatusOK)
	assertStatus(t, f.env.Do(http.MethodGet, path("/v1/sellers/%d", store), nil), http.StatusOK)
	n := f.verifiedNotifs(t, owner)
	if len(n) != 1 || n[0]["store"] != "Warung Bu Tini" {
		t.Fatalf("notifications: %+v", n)
	}

	// Verifying again is a no-op: still VERIFIED, no second push.
	if again := f.storeAction(t, store, "verify"); again.Status != domain.StoreVerified {
		t.Fatalf("again: %+v", again)
	}
	if n := f.verifiedNotifs(t, owner); len(n) != 1 {
		t.Fatalf("second verify notified again: %+v", n)
	}

	assertErr(t, f.do(http.MethodPost, "/v1/admin/stores/999999/verify", nil), http.StatusNotFound, "NOT_FOUND")
	assertErr(t, f.do(http.MethodPost, "/v1/admin/stores/abc/verify", nil), http.StatusNotFound, "NOT_FOUND")
}

func TestVerifyStore_SuspendedIs409(t *testing.T) {
	f := setup(t)
	store, owner := seed.Store(t, f.db, seed.StoreOpts{Status: "SUSPENDED"})
	assertErr(t, f.do(http.MethodPost, path("/v1/admin/stores/%d/verify", store), nil), http.StatusConflict, "STORE_STATE_INVALID")
	if s := f.storeStatus(t, store); s != "SUSPENDED" {
		t.Fatalf("status %s", s)
	}
	if n := f.verifiedNotifs(t, owner); len(n) != 0 {
		t.Fatalf("notified: %+v", n)
	}
}

func TestSuspendStore_HidesFromPublicAndBlocksOrders(t *testing.T) {
	f := setup(t)
	store, _ := seed.Store(t, f.db, seed.StoreOpts{Status: "VERIFIED"})
	p := seed.Product(t, f.db, seed.ProductOpts{StoreID: store, Stock: 5})
	productPath := path("/v1/products/%d", p)
	assertStatus(t, f.env.Do(http.MethodGet, productPath, nil), http.StatusOK)

	if got := f.storeAction(t, store, "suspend"); got.Status != domain.StoreSuspended {
		t.Fatalf("suspend: %+v", got)
	}
	assertStatus(t, f.env.Do(http.MethodGet, productPath, nil), http.StatusNotFound)
	assertStatus(t, f.env.Do(http.MethodGet, path("/v1/sellers/%d", store), nil), http.StatusNotFound)
	r := f.env.Do(http.MethodPost, "/v1/orders", map[string]any{"lines": []map[string]any{{"productId": p, "quantity": 1}}},
		auth(f.buyer), testapp.Header("Idempotency-Key", "suspended-1"))
	assertErr(t, r, http.StatusConflict, "PRODUCT_UNAVAILABLE")
	if n := f.scalar(t, `SELECT count(*) FROM orders`); n != 0 {
		t.Fatalf("orders created: %d", n)
	}

	// Suspending twice is a no-op, and a PENDING store can be suspended too.
	if got := f.storeAction(t, store, "suspend"); got.Status != domain.StoreSuspended {
		t.Fatalf("again: %+v", got)
	}
	pending, _ := seed.Store(t, f.db, seed.StoreOpts{Status: "PENDING"})
	if got := f.storeAction(t, pending, "suspend"); got.Status != domain.StoreSuspended {
		t.Fatalf("pending: %+v", got)
	}
	assertErr(t, f.do(http.MethodPost, "/v1/admin/stores/999999/suspend", nil), http.StatusNotFound, "NOT_FOUND")
}

func TestReinstateStore(t *testing.T) {
	f := setup(t)
	store, owner := seed.Store(t, f.db, seed.StoreOpts{Status: "SUSPENDED"})
	p := seed.Product(t, f.db, seed.ProductOpts{StoreID: store})

	if got := f.storeAction(t, store, "reinstate"); got.Status != domain.StoreVerified {
		t.Fatalf("reinstate: %+v", got)
	}
	assertStatus(t, f.env.Do(http.MethodGet, path("/v1/products/%d", p), nil), http.StatusOK)
	if n := f.verifiedNotifs(t, owner); len(n) != 0 {
		t.Fatalf("reinstate notified: %+v", n)
	}
	// Already VERIFIED: a no-op.
	if got := f.storeAction(t, store, "reinstate"); got.Status != domain.StoreVerified {
		t.Fatalf("again: %+v", got)
	}
	// A PENDING store was never verified: reinstating it must not skip verification.
	pending, _ := seed.Store(t, f.db, seed.StoreOpts{Status: "PENDING"})
	assertErr(t, f.do(http.MethodPost, path("/v1/admin/stores/%d/reinstate", pending), nil), http.StatusConflict, "STORE_STATE_INVALID")
	if s := f.storeStatus(t, pending); s != "PENDING" {
		t.Fatalf("pending became %s", s)
	}
	assertErr(t, f.do(http.MethodPost, "/v1/admin/stores/999999/reinstate", nil), http.StatusNotFound, "NOT_FOUND")
}

func TestListAdminStores_FilterAndPaging(t *testing.T) {
	f := setup(t) // the seller's store is VERIFIED
	p1, _ := seed.Store(t, f.db, seed.StoreOpts{Status: "PENDING"})
	p2, _ := seed.Store(t, f.db, seed.StoreOpts{Status: "PENDING"})
	seed.Store(t, f.db, seed.StoreOpts{Status: "SUSPENDED"})

	r := f.do(http.MethodGet, "/v1/admin/stores?status=PENDING&limit=1", nil)
	assertStatus(t, r, http.StatusOK)
	contract.AssertShape(t, "listAdminStores", r.Body)
	var page domain.Page[domain.AdminStore]
	r.JSON(&page)
	if len(page.Items) != 1 || page.Items[0].ID != p2 || page.NextCursor == nil {
		t.Fatalf("page 1: %+v", page)
	}
	r = f.do(http.MethodGet, "/v1/admin/stores?status=PENDING&limit=1&cursor="+*page.NextCursor, nil)
	assertStatus(t, r, http.StatusOK)
	page = domain.Page[domain.AdminStore]{}
	r.JSON(&page)
	if len(page.Items) != 1 || page.Items[0].ID != p1 || page.NextCursor != nil {
		t.Fatalf("page 2: %+v", page)
	}

	r = f.do(http.MethodGet, "/v1/admin/stores", nil)
	page = domain.Page[domain.AdminStore]{}
	r.JSON(&page)
	if len(page.Items) != 4 {
		t.Fatalf("all: %d items", len(page.Items))
	}

	assertErr(t, f.do(http.MethodGet, "/v1/admin/stores?status=OPEN", nil), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, f.do(http.MethodGet, "/v1/admin/stores?cursor=@@", nil), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, f.do(http.MethodGet, "/v1/admin/stores?limit=x", nil), http.StatusBadRequest, "VALIDATION_FAILED")
}
