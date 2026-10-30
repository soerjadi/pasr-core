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

type twoStoreOrder struct {
	storeA, storeB     int64
	productA, productB int64
	soA, soB           int64 // seller-order ids
	order              domain.Order
}

// placeTwoStoreOrder has the buyer order 2 of a product from each of two VERIFIED stores.
func (f *fx) placeTwoStoreOrder(t *testing.T, key string) twoStoreOrder {
	t.Helper()
	var o twoStoreOrder
	o.storeA, _ = seed.Store(t, f.db, seed.StoreOpts{Name: "Toko A"})
	o.storeB, _ = seed.Store(t, f.db, seed.StoreOpts{Name: "Toko B"})
	o.productA = seed.Product(t, f.db, seed.ProductOpts{StoreID: o.storeA, Stock: 10, Images: 1})
	o.productB = seed.Product(t, f.db, seed.ProductOpts{StoreID: o.storeB, Stock: 10})
	r := f.env.Do(http.MethodPost, "/v1/orders", map[string]any{"lines": []map[string]any{
		{"productId": o.productA, "quantity": 2}, {"productId": o.productB, "quantity": 2},
	}}, auth(f.buyer), testapp.Header("Idempotency-Key", key))
	assertStatus(t, r, http.StatusCreated)
	r.JSON(&o.order)
	for _, so := range o.order.SellerOrders {
		switch so.SellerID {
		case o.storeA:
			o.soA = so.ID
		case o.storeB:
			o.soB = so.ID
		}
	}
	if o.soA == 0 || o.soB == 0 {
		t.Fatalf("seller-orders: %+v", o.order.SellerOrders)
	}
	return o
}

func (f *fx) orderAction(t *testing.T, id int64, action string, body any) domain.StoreOrder {
	t.Helper()
	r := f.do(http.MethodPost, path("/v1/admin/seller-orders/%d/%s", id, action), body)
	assertStatus(t, r, http.StatusOK)
	contract.AssertShape(t, action+"AdminSellerOrder", r.Body)
	var so domain.StoreOrder
	r.JSON(&so)
	return so
}

func (f *fx) adminOrders(t *testing.T, query string) domain.Page[domain.AdminOrder] {
	t.Helper()
	r := f.do(http.MethodGet, "/v1/admin/orders"+query, nil)
	assertStatus(t, r, http.StatusOK)
	var page domain.Page[domain.AdminOrder]
	r.JSON(&page)
	return page
}

func (f *fx) stock(t *testing.T, product int64) int64 {
	t.Helper()
	return f.scalar(t, `SELECT stock FROM products WHERE id = $1`, product)
}

func TestAdminOrders_ListAndOperatorCancelRestoresStock(t *testing.T) {
	f := setup(t)
	o := f.placeTwoStoreOrder(t, "two-1")

	r := f.do(http.MethodGet, "/v1/admin/orders", nil)
	assertStatus(t, r, http.StatusOK)
	contract.AssertShape(t, "listAdminOrders", r.Body)
	all := f.adminOrders(t, "")
	if len(all.Items) != 2 || all.Items[0].ID != o.soB || all.Items[0].StoreID != o.storeB || all.Items[0].StoreName != "Toko B" ||
		all.Items[1].ID != o.soA || all.Items[1].Number != o.order.Number || len(all.Items[1].Items) != 1 ||
		all.Items[1].Items[0].Image.Small == "" {
		t.Fatalf("all: %+v", all)
	}
	page := f.adminOrders(t, "?limit=1")
	if len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatalf("limit 1: %+v", page)
	}
	if next := f.adminOrders(t, "?limit=1&cursor="+*page.NextCursor); len(next.Items) != 1 || next.Items[0].ID != o.soA || next.NextCursor != nil {
		t.Fatalf("page 2: %+v", next)
	}

	// The operator accepts A through the seller code path, then cancels it: stock comes back.
	if so := f.orderAction(t, o.soA, "accept", nil); so.Status != domain.OrderSellerProcessing {
		t.Fatalf("accept: %+v", so)
	}
	if got := f.adminOrders(t, "?status=SELLER_PROCESSING"); len(got.Items) != 1 || got.Items[0].ID != o.soA {
		t.Fatalf("processing filter: %+v", got)
	}
	if got := f.adminOrders(t, "?status=CREATED"); len(got.Items) != 1 || got.Items[0].ID != o.soB {
		t.Fatalf("created filter: %+v", got)
	}
	if s := f.stock(t, o.productA); s != 8 {
		t.Fatalf("stock before cancel: %d", s)
	}
	so := f.orderAction(t, o.soA, "cancel", map[string]any{"reason": "OTHER", "note": "Toko tutup mendadak"})
	if so.Status != domain.OrderCancelled {
		t.Fatalf("cancel: %+v", so)
	}
	if s := f.stock(t, o.productA); s != 10 {
		t.Fatalf("stock after cancel: %d", s)
	}
	var by, reason string
	if err := f.db.QueryRow(context.Background(), `SELECT cancelled_by, cancel_reason FROM seller_orders WHERE id = $1`, o.soA).Scan(&by, &reason); err != nil ||
		by != domain.ActorOperator || reason != "OTHER" {
		t.Fatalf("cancelled_by %q reason %q err %v", by, reason, err)
	}
	if n := f.scalar(t, `SELECT count(*) FROM notifications WHERE user_id = $1 AND kind = 'ORDER_CANCELLED'`, f.buyer.User.ID); n != 1 {
		t.Fatalf("buyer cancel pushes: %d", n)
	}
	// Repeating the cancel is a no-op: stock is not restored twice.
	f.orderAction(t, o.soA, "cancel", map[string]any{"reason": "BUSY_OR_CLOSED"})
	if s := f.stock(t, o.productA); s != 10 {
		t.Fatalf("stock after second cancel: %d", s)
	}

	// B goes all the way: accept, ready with a note, complete.
	f.orderAction(t, o.soB, "accept", nil)
	if so := f.orderAction(t, o.soB, "ready", map[string]any{"note": "Ambil di depan"}); so.Status != domain.OrderReadyForPickup || so.Note != "Ambil di depan" {
		t.Fatalf("ready: %+v", so)
	}
	if so := f.orderAction(t, o.soB, "complete", nil); so.Status != domain.OrderCompleted {
		t.Fatalf("complete: %+v", so)
	}
	r = f.env.Do(http.MethodGet, path("/v1/orders/%d", o.order.ID), nil, auth(f.buyer))
	var buyerView domain.Order
	r.JSON(&buyerView)
	if buyerView.Status != domain.OrderCompleted {
		t.Fatalf("order status: %s", buyerView.Status)
	}
}

func TestAdminOrders_ActionErrors(t *testing.T) {
	f := setup(t)
	o := f.placeTwoStoreOrder(t, "two-2")

	post := func(id int64, action string, body any) *testapp.Resp {
		return f.do(http.MethodPost, path("/v1/admin/seller-orders/%d/%s", id, action), body)
	}
	assertErr(t, post(o.soA, "complete", nil), http.StatusConflict, "ORDER_NOT_ACTIONABLE")
	assertErr(t, post(o.soA, "cancel", map[string]any{"reason": "OTHER", "note": "x"}), http.StatusConflict, "ORDER_NOT_ACTIONABLE")
	assertErr(t, post(o.soA, "reject", map[string]any{"reason": "OTHER"}), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, post(o.soA, "reject", map[string]any{"reason": "EXPIRED"}), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, post(o.soA, "reject", map[string]any{"reason": "OTHER", "note": "a\u0000b"}), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, post(o.soA, "reject", nil), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, post(999999, "accept", nil), http.StatusNotFound, "NOT_FOUND")
	assertErr(t, post(999999, "reject", map[string]any{"reason": "CANNOT_SERVE"}), http.StatusNotFound, "NOT_FOUND")

	if so := f.orderAction(t, o.soA, "reject", map[string]any{"reason": "CANNOT_SERVE"}); so.Status != domain.OrderCancelled {
		t.Fatalf("reject: %+v", so)
	}
	if s := f.stock(t, o.productA); s != 10 {
		t.Fatalf("stock after reject: %d", s)
	}

	assertErr(t, f.do(http.MethodGet, "/v1/admin/orders?status=NEW", nil), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, f.do(http.MethodGet, "/v1/admin/orders?cursor=zz!", nil), http.StatusBadRequest, "VALIDATION_FAILED")
}
