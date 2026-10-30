package mystore_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

// t0 is 10:02 WIB on 15 October 2026.
var t0 = time.Date(2026, 10, 15, 3, 2, 0, 0, time.UTC)

const buyerPhone = "+6281234500001"

// storeOrder is a seller with a VERIFIED store, a product, and a buyer who ordered qty of it.
type storeOrder struct {
	env           *testapp.Env
	db            *pgxpool.Pool
	seller, buyer domain.Session
	product       int64
	order         domain.Order
	id            int64 // the seller-order id
}

func placeStoreOrder(t *testing.T, qty int32) *storeOrder {
	t.Helper()
	env, db := setup(t)
	env.Clock.Set(t0)
	s, store := seller(t, env, db, "VERIFIED")
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Name: "Nasi pecel", Price: 15000, Stock: 10, Images: 1})
	b := env.SignIn(buyerPhone)
	if r := env.Do(http.MethodPatch, "/v1/me", map[string]any{"name": "Budi"}, auth(b)); r.Status != http.StatusOK {
		t.Fatalf("name: %d %s", r.Status, r.Body)
	}
	r := env.Do(http.MethodPost, "/v1/orders", map[string]any{"lines": []map[string]any{{"productId": p, "quantity": qty}}},
		auth(b), testapp.Header("Idempotency-Key", "k1"))
	if r.Status != http.StatusCreated {
		t.Fatalf("create order: %d %s", r.Status, r.Body)
	}
	var o domain.Order
	r.JSON(&o)
	return &storeOrder{env: env, db: db, seller: s, buyer: b, product: p, order: o, id: o.SellerOrders[0].ID}
}

func (f *storeOrder) post(action string, body any, as domain.Session) *testapp.Resp {
	return f.env.Do(http.MethodPost, fmt.Sprintf("/v1/me/store/orders/%d/%s", f.id, action), body, auth(as))
}

func (f *storeOrder) mustPost(t *testing.T, action string, body any) domain.StoreOrder {
	t.Helper()
	r := f.post(action, body, f.seller)
	if r.Status != http.StatusOK {
		t.Fatalf("%s: %d %s", action, r.Status, r.Body)
	}
	contract.AssertShape(t, action+"MyStoreOrder", r.Body)
	var so domain.StoreOrder
	r.JSON(&so)
	return so
}

func (f *storeOrder) stock(t *testing.T) int32 {
	t.Helper()
	var n int32
	if err := f.db.QueryRow(context.Background(), `SELECT stock FROM products WHERE id = $1`, f.product).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestListMyStoreOrders_ShapeTabsAndCounts(t *testing.T) {
	f := placeStoreOrder(t, 2)
	r := f.env.Do(http.MethodGet, "/v1/me/store/orders", nil, auth(f.seller))
	if r.Status != http.StatusOK {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	var page domain.StoreOrderPage
	r.JSON(&page)
	if len(page.Items) != 1 || page.Items[0].ID != f.id || page.Items[0].BuyerName != "Budi" ||
		page.Counts != (domain.StoreOrderCounts{New: 1}) {
		t.Fatalf("default tab = %s", r.Body)
	}

	f.mustPost(t, "accept", nil)
	f.mustPost(t, "ready", map[string]any{"note": "Siap diambil mulai jam 14.00"})
	r = f.env.Do(http.MethodGet, "/v1/me/store/orders?tab=PROCESSING", nil, auth(f.seller))
	if r.Status != http.StatusOK {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "listMyStoreOrders", r.Body)
	r.JSON(&page)
	if len(page.Items) != 1 || page.Counts != (domain.StoreOrderCounts{Processing: 1}) || page.NextCursor != nil {
		t.Fatalf("processing tab = %s", r.Body)
	}
	if r := f.env.Do(http.MethodGet, "/v1/me/store/orders?tab=NEW", nil, auth(f.seller)); r.Status != http.StatusOK {
		t.Fatalf("new tab: %d", r.Status)
	} else {
		r.JSON(&page)
		if len(page.Items) != 0 {
			t.Fatalf("new tab = %s", r.Body)
		}
	}
	assertErr(t, f.env.Do(http.MethodGet, "/v1/me/store/orders?tab=ALL", nil, auth(f.seller)), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, f.env.Do(http.MethodGet, "/v1/me/store/orders?limit=x", nil, auth(f.seller)), http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestGetMyStoreOrder_ShapeAndOtherStore404(t *testing.T) {
	f := placeStoreOrder(t, 2)
	f.mustPost(t, "accept", nil)
	f.mustPost(t, "ready", map[string]any{"note": "Siap diambil mulai jam 14.00"})
	r := f.env.Do(http.MethodGet, fmt.Sprintf("/v1/me/store/orders/%d", f.id), nil, auth(f.seller))
	if r.Status != http.StatusOK {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "getMyStoreOrder", r.Body)
	var so domain.StoreOrder
	r.JSON(&so)
	if so.Number != f.order.Number || so.Subtotal != 30000 || so.Note != "Siap diambil mulai jam 14.00" ||
		len(so.StatusHistory) != 3 || so.AcceptBy == nil || so.Items[0].Image.Small == "" {
		t.Fatalf("store order = %s", r.Body)
	}

	other := f.env.SignIn("+6281234500002")
	seed.Store(t, f.db, seed.StoreOpts{OwnerID: other.User.ID})
	assertErr(t, f.env.Do(http.MethodGet, fmt.Sprintf("/v1/me/store/orders/%d", f.id), nil, auth(other)), http.StatusNotFound, "NOT_FOUND")
	assertErr(t, f.post("complete", nil, other), http.StatusNotFound, "NOT_FOUND")
	assertErr(t, f.env.Do(http.MethodGet, "/v1/me/store/orders/abc", nil, auth(f.seller)), http.StatusNotFound, "NOT_FOUND")
}

func TestStoreOrderActions_AcceptReadyCompleteNotifyBuyer(t *testing.T) {
	f := placeStoreOrder(t, 1)
	if so := f.mustPost(t, "accept", nil); so.Status != domain.OrderSellerProcessing {
		t.Fatalf("accept: %+v", so)
	}
	if so := f.mustPost(t, "accept", nil); so.Status != domain.OrderSellerProcessing { // idempotent
		t.Fatalf("accept again: %+v", so)
	}
	if so := f.mustPost(t, "ready", nil); so.Status != domain.OrderReadyForPickup || so.Note != "" { // no body
		t.Fatalf("ready: %+v", so)
	}
	if so := f.mustPost(t, "complete", nil); so.Status != domain.OrderCompleted {
		t.Fatalf("complete: %+v", so)
	}
	r := f.env.Do(http.MethodGet, fmt.Sprintf("/v1/orders/%d", f.order.ID), nil, auth(f.buyer))
	var o domain.Order
	r.JSON(&o)
	if o.Status != domain.OrderCompleted || len(o.StatusHistory) != 4 {
		t.Fatalf("buyer order = %s", r.Body)
	}
	var kinds []string
	rows, err := f.db.Query(context.Background(), `SELECT kind FROM notifications WHERE user_id = $1 ORDER BY id`, f.buyer.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, k)
	}
	rows.Close()
	if fmt.Sprint(kinds) != fmt.Sprint([]string{domain.NotifOrderAccepted, domain.NotifOrderReady, domain.NotifOrderCompleted}) {
		t.Fatalf("buyer notifications = %v", kinds)
	}
}

func TestRejectMyStoreOrder_ValidationAndStockRestore(t *testing.T) {
	f := placeStoreOrder(t, 3)
	assertErr(t, f.post("reject", map[string]any{"reason": "OTHER"}, f.seller), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, f.post("reject", map[string]any{"reason": "LAZY"}, f.seller), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, f.post("reject", nil, f.seller), http.StatusBadRequest, "VALIDATION_FAILED")
	if f.stock(t) != 7 {
		t.Fatalf("stock = %d", f.stock(t))
	}
	so := f.mustPost(t, "reject", map[string]any{"reason": "OTHER", "note": "Bahan habis"})
	if so.Status != domain.OrderCancelled || f.stock(t) != 10 {
		t.Fatalf("reject: %+v stock %d", so, f.stock(t))
	}
	assertErr(t, f.post("accept", nil, f.seller), http.StatusConflict, "ORDER_NOT_ACTIONABLE")
}

func TestStoreOrder_CompleteFromCreated409NotActionable(t *testing.T) {
	f := placeStoreOrder(t, 1)
	assertErr(t, f.post("complete", nil, f.seller), http.StatusConflict, "ORDER_NOT_ACTIONABLE")
	assertErr(t, f.post("ready", map[string]any{"note": "x"}, f.seller), http.StatusConflict, "ORDER_NOT_ACTIONABLE")
}

func TestStoreOrder_AcceptAfterDeadline409OrderExpired(t *testing.T) {
	f := placeStoreOrder(t, 1)
	f.env.Clock.Set(t0.Add(10*time.Minute + time.Second))
	assertErr(t, f.post("accept", nil, f.seller), http.StatusConflict, "ORDER_EXPIRED")
}

func TestStoreOrders_NoStore409AndAuth401(t *testing.T) {
	f := placeStoreOrder(t, 1)
	assertErr(t, f.env.Do(http.MethodGet, "/v1/me/store/orders", nil, auth(f.buyer)), http.StatusConflict, "NO_STORE")
	assertErr(t, f.post("accept", nil, f.buyer), http.StatusConflict, "NO_STORE")
	for _, req := range []struct{ method, path string }{
		{http.MethodGet, "/v1/me/store/orders"},
		{http.MethodGet, fmt.Sprintf("/v1/me/store/orders/%d", f.id)},
		{http.MethodPost, fmt.Sprintf("/v1/me/store/orders/%d/accept", f.id)},
		{http.MethodPost, fmt.Sprintf("/v1/me/store/orders/%d/reject", f.id)},
		{http.MethodPost, fmt.Sprintf("/v1/me/store/orders/%d/ready", f.id)},
		{http.MethodPost, fmt.Sprintf("/v1/me/store/orders/%d/complete", f.id)},
	} {
		if r := f.env.Do(req.method, req.path, nil); r.Status != http.StatusUnauthorized {
			t.Fatalf("%s %s: %d", req.method, req.path, r.Status)
		}
	}
}
