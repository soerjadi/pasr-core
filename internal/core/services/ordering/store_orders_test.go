package ordering_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

var operator = domain.Actor{Role: domain.ActorOperator, UserID: 1}

func sellerActor(owner int64) domain.Actor {
	return domain.Actor{Role: domain.ActorSeller, UserID: owner}
}

// shop is a VERIFIED store with one product of 10 in stock.
type shop struct {
	store, owner, product int64
}

func newShop(t *testing.T, f *ofx, name string) shop {
	t.Helper()
	store, owner := seed.Store(t, f.pool, seed.StoreOpts{Name: name})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 10, Price: 5000})
	return shop{store: store, owner: owner, product: p}
}

// place orders qty of each shop's product in one order and returns it.
func (f *ofx) place(t *testing.T, buyer int64, qty int32, shops ...shop) domain.Order {
	t.Helper()
	var lines []domain.CartLineInput
	for _, s := range shops {
		lines = append(lines, ln(s.product, qty))
	}
	o, err := f.svc.CreateOrder(context.Background(), buyer, fmt.Sprintf("so-%d", seedN()), lines)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// soOf returns the seller-order id of store in o.
func soOf(t *testing.T, o domain.Order, store int64) int64 {
	t.Helper()
	for _, so := range o.SellerOrders {
		if so.SellerID == store {
			return so.ID
		}
	}
	t.Fatalf("store %d not in order %d", store, o.ID)
	return 0
}

type notif struct {
	Kind string
	Data map[string]string
}

func (f *ofx) notifs(t *testing.T, user int64, kind string) []notif {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `SELECT kind, data FROM notifications WHERE user_id = $1 AND kind = $2 ORDER BY id`, user, kind)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []notif
	for rows.Next() {
		var n notif
		if err := rows.Scan(&n.Kind, &n.Data); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}

func (f *ofx) orderStatus(t *testing.T, id int64) domain.OrderStatus {
	t.Helper()
	o, err := f.svc.GetOrder(context.Background(), f.buyerOf(t, id), id)
	if err != nil {
		t.Fatal(err)
	}
	return o.Status
}

func (f *ofx) buyerOf(t *testing.T, orderID int64) int64 {
	t.Helper()
	var b int64
	if err := f.pool.QueryRow(context.Background(), `SELECT buyer_id FROM orders WHERE id = $1`, orderID).Scan(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestListStoreOrders_TabsAndCounts(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "Budi")
	s := newShop(t, f, "Bu Siti")
	var ids []int64
	for range 5 {
		ids = append(ids, soOf(t, f.place(t, buyer, 1, s), s.store))
	}
	a := sellerActor(s.owner)
	if _, err := f.svc.Accept(ctx, a, ids[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Accept(ctx, a, ids[2]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Ready(ctx, a, ids[2], ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Reject(ctx, a, ids[3], "BUSY_OR_CLOSED", ""); err != nil {
		t.Fatal(err)
	}

	want := map[string][]int64{"NEW": {ids[4], ids[0]}, "PROCESSING": {ids[2], ids[1]}, "DONE": {ids[3]}}
	for tab, wantIDs := range want {
		page, err := f.svc.ListStoreOrders(ctx, s.owner, tab, "", 20)
		if err != nil {
			t.Fatalf("%s: %v", tab, err)
		}
		if page.Counts != (domain.StoreOrderCounts{New: 2, Processing: 2, Done: 1}) {
			t.Fatalf("%s counts = %+v", tab, page.Counts)
		}
		if len(page.Items) != len(wantIDs) || page.NextCursor != nil {
			t.Fatalf("%s items = %+v", tab, page.Items)
		}
		for i, so := range page.Items {
			if so.ID != wantIDs[i] {
				t.Fatalf("%s order = %d, want %d", tab, so.ID, wantIDs[i])
			}
		}
	}
	first, err := f.svc.ListStoreOrders(ctx, s.owner, "NEW", "", 1)
	if err != nil || len(first.Items) != 1 || first.NextCursor == nil {
		t.Fatalf("first page: %v %+v", err, first)
	}
	so := first.Items[0]
	if so.BuyerName != "Budi" || so.Number == "" || so.Subtotal != 5000 || len(so.Items) != 1 || so.AcceptBy == nil ||
		!so.AcceptBy.Equal(t0.Add(10*time.Minute)) || len(so.StatusHistory) != 1 || so.CreatedAt.Location() != time.UTC {
		t.Fatalf("store order = %+v", so)
	}
	second, err := f.svc.ListStoreOrders(ctx, s.owner, "NEW", *first.NextCursor, 1)
	if err != nil || len(second.Items) != 1 || second.Items[0].ID != ids[0] || second.NextCursor != nil {
		t.Fatalf("second page: %v %+v", err, second)
	}
	for _, bad := range []string{"ALL", "x"} {
		if _, err := f.svc.ListStoreOrders(ctx, s.owner, bad, "", 0); !errors.Is(err, domain.ErrValidationFailed) {
			t.Fatalf("tab %q: %v", bad, err)
		}
	}
	if _, err := f.svc.ListStoreOrders(ctx, s.owner, "NEW", "!!", 0); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("bad cursor: %v", err)
	}
	if def, err := f.svc.ListStoreOrders(ctx, s.owner, "", "", 0); err != nil || len(def.Items) != 2 {
		t.Fatalf("default tab: %v %+v", err, def)
	}
}

func TestListStoreOrders_OnlyOwnStore(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	a, b := newShop(t, f, "A"), newShop(t, f, "B")
	o := f.place(t, buyer, 1, a, b)
	page, err := f.svc.ListStoreOrders(ctx, a.owner, "NEW", "", 0)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != soOf(t, o, a.store) || page.Counts.New != 1 {
		t.Fatalf("own: %v %+v", err, page)
	}
	if len(page.Items[0].Items) != 1 || page.Items[0].Items[0].ProductID != a.product {
		t.Fatalf("items of another store leaked: %+v", page.Items[0].Items)
	}
	_, err = f.svc.ListStoreOrders(ctx, buyer, "NEW", "", 0)
	if codeOf(err) != "NO_STORE" || !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("no store: %v", err)
	}
}

func TestGetStoreOrder_OtherStore404(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	a, b := newShop(t, f, "A"), newShop(t, f, "B")
	o := f.place(t, buyer, 1, a, b)
	so, err := f.svc.GetStoreOrder(ctx, a.owner, soOf(t, o, a.store))
	if err != nil || so.Number != o.Number || so.Status != domain.OrderCreated {
		t.Fatalf("own: %v %+v", err, so)
	}
	for _, id := range []int64{soOf(t, o, b.store), 987654} {
		if _, err := f.svc.GetStoreOrder(ctx, a.owner, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("seller-order %d: %v", id, err)
		}
	}
	if _, err := f.svc.GetStoreOrder(ctx, buyer, soOf(t, o, a.store)); codeOf(err) != "NO_STORE" {
		t.Fatalf("no store: %v", err)
	}
}

func TestAccept_MovesToProcessingAndNotifiesBuyer(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "Bu Siti")
	o := f.place(t, buyer, 2, s)
	id := soOf(t, o, s.store)
	f.clock.Advance(3 * time.Minute)

	got, err := f.svc.Accept(ctx, sellerActor(s.owner), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.Status != domain.OrderSellerProcessing || len(got.StatusHistory) != 2 ||
		got.StatusHistory[1].Status != domain.OrderSellerProcessing || !got.StatusHistory[1].At.Equal(t0.Add(3*time.Minute)) {
		t.Fatalf("store order = %+v", got)
	}
	bo, err := f.svc.GetOrder(ctx, buyer, o.ID)
	if err != nil || bo.Status != domain.OrderSellerProcessing || len(bo.StatusHistory) != 2 ||
		bo.StatusHistory[1].Status != domain.OrderSellerProcessing {
		t.Fatalf("buyer order = %v %+v", err, bo)
	}
	n := f.notifs(t, buyer, domain.NotifOrderAccepted)
	if len(n) != 1 || n[0].Data["orderNumber"] != o.Number || n[0].Data["orderId"] != fmt.Sprint(o.ID) ||
		n[0].Data["sellerOrderId"] != fmt.Sprint(id) || n[0].Data["store"] != "Bu Siti" {
		t.Fatalf("notifications = %+v", n)
	}
	if f.stock(t, s.product) != 8 {
		t.Fatalf("stock changed on accept: %d", f.stock(t, s.product))
	}
}

func TestAccept_Twice_Idempotent(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "A")
	id := soOf(t, f.place(t, buyer, 1, s), s.store)
	first, err := f.svc.Accept(ctx, sellerActor(s.owner), id)
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(time.Minute)
	second, err := f.svc.Accept(ctx, sellerActor(s.owner), id)
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != domain.OrderSellerProcessing || len(second.StatusHistory) != len(first.StatusHistory) {
		t.Fatalf("second = %+v", second)
	}
	if n := f.notifs(t, buyer, domain.NotifOrderAccepted); len(n) != 1 {
		t.Fatalf("notifications = %d", len(n))
	}
	if n := f.count(t, `SELECT count(*) FROM order_status_history WHERE status = 'SELLER_PROCESSING'`); n != 2 {
		t.Fatalf("history rows = %d (one seller-order, one order level)", n)
	}
}

func TestAccept_AfterDeadline409OrderExpired(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "A")
	id := soOf(t, f.place(t, buyer, 1, s), s.store)
	f.clock.Advance(10*time.Minute + time.Second)
	_, err := f.svc.Accept(ctx, sellerActor(s.owner), id)
	if codeOf(err) != "ORDER_EXPIRED" || !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("late accept: %v", err)
	}
	// Once the job has expired it, accepting still answers ORDER_EXPIRED.
	if n, err := f.svc.ExpireOverdue(ctx); err != nil || n != 1 {
		t.Fatalf("expire: %d %v", n, err)
	}
	if _, err := f.svc.Accept(ctx, sellerActor(s.owner), id); codeOf(err) != "ORDER_EXPIRED" {
		t.Fatalf("accept after expiry: %v", err)
	}
}

func TestReject_RestoresStockAndStoresReason(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "A")
	o := f.place(t, buyer, 3, s)
	id := soOf(t, o, s.store)
	if f.stock(t, s.product) != 7 {
		t.Fatal("stock not taken")
	}
	got, err := f.svc.Reject(ctx, sellerActor(s.owner), id, "OUT_OF_STOCK", "habis")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.OrderCancelled || f.stock(t, s.product) != 10 {
		t.Fatalf("status %s stock %d", got.Status, f.stock(t, s.product))
	}
	var reason, note, by string
	if err := f.pool.QueryRow(ctx, `SELECT cancel_reason, cancel_note, cancelled_by FROM seller_orders WHERE id = $1`, id).
		Scan(&reason, &note, &by); err != nil {
		t.Fatal(err)
	}
	if reason != "OUT_OF_STOCK" || note != "habis" || by != "seller" {
		t.Fatalf("stored %q %q %q", reason, note, by)
	}
	n := f.notifs(t, buyer, domain.NotifOrderRejected)
	if len(n) != 1 || n[0].Data["reason"] != "OUT_OF_STOCK" {
		t.Fatalf("notifications = %+v", n)
	}
	if f.orderStatus(t, o.ID) != domain.OrderCancelled {
		t.Fatalf("order status = %s", f.orderStatus(t, o.ID))
	}
	// Repeating the reject neither restores again nor notifies again.
	if _, err := f.svc.Reject(ctx, sellerActor(s.owner), id, "OUT_OF_STOCK", ""); err != nil {
		t.Fatal(err)
	}
	if f.stock(t, s.product) != 10 || len(f.notifs(t, buyer, domain.NotifOrderRejected)) != 1 {
		t.Fatalf("repeat changed state: stock %d", f.stock(t, s.product))
	}
}

func TestReject_OtherRequiresNote400(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "A")
	id := soOf(t, f.place(t, buyer, 1, s), s.store)
	for _, tc := range []struct{ reason, note string }{{"OTHER", ""}, {"OTHER", "   "}, {"NOPE", "x"}, {"", ""}, {"EXPIRED", ""}} {
		if _, err := f.svc.Reject(ctx, sellerActor(s.owner), id, tc.reason, tc.note); !errors.Is(err, domain.ErrValidationFailed) {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
	if got, err := f.svc.Reject(ctx, sellerActor(s.owner), id, "OTHER", "libur"); err != nil || got.Status != domain.OrderCancelled {
		t.Fatalf("with note: %v %+v", err, got)
	}
}

func TestNotes_WithNULAre400(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "A")
	id := soOf(t, f.place(t, buyer, 1, s), s.store)
	if _, err := f.svc.Reject(ctx, operator, id, "OTHER", "a\x00b"); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("reject: %v", err)
	}
	if _, err := f.svc.Accept(ctx, operator, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Ready(ctx, operator, id, "ambil\x00"); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("ready: %v", err)
	}
	if _, err := f.svc.Cancel(ctx, operator, id, "OTHER", "\x00"); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestReady_SetsNoteAndNotifies(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "A")
	o := f.place(t, buyer, 1, s)
	id := soOf(t, o, s.store)
	if _, err := f.svc.Accept(ctx, sellerActor(s.owner), id); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Ready(ctx, sellerActor(s.owner), id, "Siap diambil mulai jam 14.00")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.OrderReadyForPickup || got.Note != "Siap diambil mulai jam 14.00" {
		t.Fatalf("store order = %+v", got)
	}
	bo, _ := f.svc.GetOrder(ctx, buyer, o.ID)
	if bo.Status != domain.OrderReadyForPickup || bo.SellerOrders[0].Note != "Siap diambil mulai jam 14.00" {
		t.Fatalf("buyer view = %+v", bo)
	}
	if n := f.notifs(t, buyer, domain.NotifOrderReady); len(n) != 1 {
		t.Fatalf("notifications = %+v", n)
	}
}

func TestComplete_OnlyFromReady(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "A")
	o := f.place(t, buyer, 1, s)
	id := soOf(t, o, s.store)
	a := sellerActor(s.owner)
	if _, err := f.svc.Accept(ctx, a, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Complete(ctx, a, id); codeOf(err) != "ORDER_NOT_ACTIONABLE" {
		t.Fatalf("complete from processing: %v", err)
	}
	if _, err := f.svc.Ready(ctx, a, id, ""); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Complete(ctx, a, id)
	if err != nil || got.Status != domain.OrderCompleted {
		t.Fatalf("complete: %v %+v", err, got)
	}
	if f.orderStatus(t, o.ID) != domain.OrderCompleted || len(f.notifs(t, buyer, domain.NotifOrderCompleted)) != 1 {
		t.Fatal("order not completed or buyer not notified")
	}
	if f.stock(t, s.product) != 9 {
		t.Fatalf("stock = %d", f.stock(t, s.product))
	}
}

func TestTransition_InvalidIs409OrderNotActionable(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "A")
	id := soOf(t, f.place(t, buyer, 1, s), s.store)
	a := sellerActor(s.owner)
	_, err := f.svc.Complete(ctx, a, id)
	if codeOf(err) != "ORDER_NOT_ACTIONABLE" || !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("complete from CREATED: %v", err)
	}
	if _, err := f.svc.Cancel(ctx, a, id, "BUSY_OR_CLOSED", ""); codeOf(err) != "ORDER_NOT_ACTIONABLE" {
		t.Fatalf("cancel from CREATED: %v", err)
	}
	if _, err := f.svc.Ready(ctx, a, id, ""); codeOf(err) != "ORDER_NOT_ACTIONABLE" {
		t.Fatalf("ready from CREATED: %v", err)
	}
	if n := f.count(t, `SELECT count(*) FROM order_status_history WHERE seller_order_id = $1`, id); n != 1 {
		t.Fatalf("history rows = %d", n)
	}
}

func TestCancel_FromReadyRestoresStock(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "A")
	o := f.place(t, buyer, 4, s)
	id := soOf(t, o, s.store)
	a := sellerActor(s.owner)
	if _, err := f.svc.Accept(ctx, a, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Ready(ctx, a, id, ""); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Cancel(ctx, a, id, "OTHER", "pembeli tidak datang")
	if err != nil || got.Status != domain.OrderCancelled {
		t.Fatalf("cancel: %v %+v", err, got)
	}
	if f.stock(t, s.product) != 10 {
		t.Fatalf("stock = %d", f.stock(t, s.product))
	}
	n := f.notifs(t, buyer, domain.NotifOrderCancelled)
	if len(n) != 1 || n[0].Data["reason"] != "OTHER" {
		t.Fatalf("notifications = %+v", n)
	}
	if _, err := f.svc.Cancel(ctx, a, id, "BUSY_OR_CLOSED", ""); err != nil { // repeat: idempotent
		t.Fatalf("repeat cancel: %v", err)
	}
	if f.stock(t, s.product) != 10 {
		t.Fatalf("stock after repeat = %d", f.stock(t, s.product))
	}
}

func TestRestoreStock_ClearsAutoOutOfStock(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	store, owner := seed.Store(t, f.pool, seed.StoreOpts{})
	p := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store, Stock: 2})
	o := f.place(t, buyer, 2, shop{store: store, owner: owner, product: p})
	var status string
	if err := f.pool.QueryRow(ctx, `SELECT status FROM products WHERE id = $1`, p).Scan(&status); err != nil || status != "OUT_OF_STOCK" {
		t.Fatalf("after selling out: %s %v", status, err)
	}
	if _, err := f.svc.Reject(ctx, sellerActor(owner), soOf(t, o, store), "CANNOT_SERVE", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT status FROM products WHERE id = $1`, p).Scan(&status); err != nil || status != "AVAILABLE" || f.stock(t, p) != 2 {
		t.Fatalf("after restore: %s %d %v", status, f.stock(t, p), err)
	}
}

func TestMultiSeller_OrderStatusFollowsLeastAdvanced(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	a, b := newShop(t, f, "A"), newShop(t, f, "B")
	o := f.place(t, buyer, 1, a, b)
	soA, soB := soOf(t, o, a.store), soOf(t, o, b.store)

	steps := []struct {
		do   func() error
		want domain.OrderStatus
	}{
		{func() error { _, err := f.svc.Accept(ctx, sellerActor(a.owner), soA); return err }, domain.OrderCreated},
		{func() error { _, err := f.svc.Ready(ctx, sellerActor(a.owner), soA, ""); return err }, domain.OrderCreated},
		{func() error { _, err := f.svc.Accept(ctx, sellerActor(b.owner), soB); return err }, domain.OrderSellerProcessing},
		{func() error { _, err := f.svc.Ready(ctx, sellerActor(b.owner), soB, ""); return err }, domain.OrderReadyForPickup},
		{func() error { _, err := f.svc.Complete(ctx, sellerActor(a.owner), soA); return err }, domain.OrderReadyForPickup},
		{func() error { _, err := f.svc.Complete(ctx, sellerActor(b.owner), soB); return err }, domain.OrderCompleted},
	}
	for i, st := range steps {
		f.clock.Advance(time.Minute)
		if err := st.do(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if got := f.orderStatus(t, o.ID); got != st.want {
			t.Fatalf("step %d: order status %s, want %s", i, got, st.want)
		}
	}
	bo, _ := f.svc.GetOrder(ctx, buyer, o.ID)
	var hist []domain.OrderStatus
	for _, h := range bo.StatusHistory {
		hist = append(hist, h.Status)
	}
	want := []domain.OrderStatus{domain.OrderCreated, domain.OrderSellerProcessing, domain.OrderReadyForPickup, domain.OrderCompleted}
	if fmt.Sprint(hist) != fmt.Sprint(want) {
		t.Fatalf("order history = %v, want %v", hist, want)
	}
}

func TestMultiSeller_OneRejectedOneCompleted_OrderCompleted(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	a, b := newShop(t, f, "A"), newShop(t, f, "B")
	o := f.place(t, buyer, 1, a, b)
	soA, soB := soOf(t, o, a.store), soOf(t, o, b.store)
	if _, err := f.svc.Reject(ctx, sellerActor(a.owner), soA, "BUSY_OR_CLOSED", ""); err != nil {
		t.Fatal(err)
	}
	if got := f.orderStatus(t, o.ID); got != domain.OrderCreated {
		t.Fatalf("after reject: %s", got)
	}
	for _, step := range []func() error{
		func() error { _, err := f.svc.Accept(ctx, sellerActor(b.owner), soB); return err },
		func() error { _, err := f.svc.Ready(ctx, sellerActor(b.owner), soB, ""); return err },
		func() error { _, err := f.svc.Complete(ctx, sellerActor(b.owner), soB); return err },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.orderStatus(t, o.ID); got != domain.OrderCompleted {
		t.Fatalf("order status = %s", got)
	}
}

func TestExpireOverdue_CancelsRestoresStockNotifiesBothSides(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "Bu Siti")
	o := f.place(t, buyer, 3, s)
	id := soOf(t, o, s.store)

	f.clock.Advance(9 * time.Minute)
	if n, err := f.svc.ExpireOverdue(ctx); err != nil || n != 0 {
		t.Fatalf("before deadline: %d %v", n, err)
	}
	f.clock.Advance(time.Minute) // exactly accept_by
	if n, err := f.svc.ExpireOverdue(ctx); err != nil || n != 1 {
		t.Fatalf("at deadline: %d %v", n, err)
	}
	if f.stock(t, s.product) != 10 || f.orderStatus(t, o.ID) != domain.OrderCancelled {
		t.Fatalf("stock %d status %s", f.stock(t, s.product), f.orderStatus(t, o.ID))
	}
	var reason, by string
	if err := f.pool.QueryRow(ctx, `SELECT cancel_reason, cancelled_by FROM seller_orders WHERE id = $1`, id).Scan(&reason, &by); err != nil {
		t.Fatal(err)
	}
	if reason != "EXPIRED" || by != "system" {
		t.Fatalf("stored %q %q", reason, by)
	}
	for user, label := range map[int64]string{buyer: "buyer", s.owner: "seller"} {
		n := f.notifs(t, user, domain.NotifOrderCancelled)
		if len(n) != 1 || n[0].Data["reason"] != "EXPIRED" || n[0].Data["sellerOrderId"] != fmt.Sprint(id) {
			t.Fatalf("%s notifications = %+v", label, n)
		}
	}
	if n, err := f.svc.ExpireOverdue(ctx); err != nil || n != 0 {
		t.Fatalf("second run: %d %v", n, err)
	}
	if f.stock(t, s.product) != 10 {
		t.Fatalf("stock after second run = %d", f.stock(t, s.product))
	}
}

func TestExpireOverdue_SkipsAcceptedOrders(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	a, b := newShop(t, f, "A"), newShop(t, f, "B")
	o := f.place(t, buyer, 1, a, b)
	if _, err := f.svc.Accept(ctx, sellerActor(a.owner), soOf(t, o, a.store)); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(time.Hour)
	if n, err := f.svc.ExpireOverdue(ctx); err != nil || n != 1 {
		t.Fatalf("expire: %d %v", n, err)
	}
	accepted, _ := f.svc.GetStoreOrder(ctx, a.owner, soOf(t, o, a.store))
	expired, _ := f.svc.GetStoreOrder(ctx, b.owner, soOf(t, o, b.store))
	if accepted.Status != domain.OrderSellerProcessing || expired.Status != domain.OrderCancelled {
		t.Fatalf("accepted %s expired %s", accepted.Status, expired.Status)
	}
	if f.stock(t, a.product) != 9 || f.stock(t, b.product) != 10 {
		t.Fatalf("stock a %d b %d", f.stock(t, a.product), f.stock(t, b.product))
	}
	if got := f.orderStatus(t, o.ID); got != domain.OrderSellerProcessing {
		t.Fatalf("order status = %s", got)
	}
	if n := f.notifs(t, a.owner, domain.NotifOrderCancelled); len(n) != 0 {
		t.Fatalf("accepted seller notified: %+v", n)
	}
}

// The seller accepts at the very deadline while the job expires the order. Exactly one of them
// wins, and the stock comes back at most once.
func TestAcceptRacesExpire_ExactlyOneOutcome_StockRestoredAtMostOnce(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "A")
	if _, err := f.pool.Exec(ctx, `UPDATE products SET stock = 1000 WHERE id = $1`, s.product); err != nil {
		t.Fatal(err)
	}
	outcomes := map[string]int{}
	for i := range 20 {
		f.clock.Set(t0)
		o := f.place(t, buyer, 1, s)
		id := soOf(t, o, s.store)
		before := f.stock(t, s.product)
		f.clock.Set(t0.Add(10 * time.Minute)) // exactly accept_by: both moves are legal

		var acceptErr, expireErr error
		var expired int
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; _, acceptErr = f.svc.Accept(ctx, sellerActor(s.owner), id) }()
		go func() { defer wg.Done(); <-start; expired, expireErr = f.svc.ExpireOverdue(ctx) }()
		close(start)
		wg.Wait()

		if expireErr != nil {
			t.Fatalf("run %d: expire: %v", i, expireErr)
		}
		got, err := f.svc.GetStoreOrder(ctx, s.owner, id)
		if err != nil {
			t.Fatal(err)
		}
		after := f.stock(t, s.product)
		switch {
		case acceptErr == nil && expired == 0:
			if got.Status != domain.OrderSellerProcessing || after != before {
				t.Fatalf("run %d: accept won but status %s stock %d->%d", i, got.Status, before, after)
			}
			outcomes["accepted"]++
		case acceptErr != nil && expired == 1:
			if codeOf(acceptErr) != "ORDER_EXPIRED" || got.Status != domain.OrderCancelled || after != before+1 {
				t.Fatalf("run %d: expire won but accept %v status %s stock %d->%d", i, acceptErr, got.Status, before, after)
			}
			outcomes["expired"]++
		default:
			t.Fatalf("run %d: both or neither: accept %v expired %d", i, acceptErr, expired)
		}
		if n := f.count(t, `SELECT count(*) FROM order_status_history WHERE seller_order_id = $1`, id); n != 2 {
			t.Fatalf("run %d: seller-order history rows = %d", i, n)
		}
	}
	t.Logf("outcomes: %v", outcomes)
}

func TestOperatorOverride_CanActOnAnyStore(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	s := newShop(t, f, "A")
	o := f.place(t, buyer, 2, s)
	id := soOf(t, o, s.store)
	if _, err := f.svc.Accept(ctx, operator, id); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Cancel(ctx, operator, id, "OTHER", "laporan penipuan")
	if err != nil || got.Status != domain.OrderCancelled || f.stock(t, s.product) != 10 {
		t.Fatalf("operator cancel: %v %+v stock %d", err, got, f.stock(t, s.product))
	}
	var by string
	if err := f.pool.QueryRow(ctx, `SELECT cancelled_by FROM seller_orders WHERE id = $1`, id).Scan(&by); err != nil || by != "operator" {
		t.Fatalf("cancelled_by = %q %v", by, err)
	}
	if _, err := f.svc.Accept(ctx, operator, 987654); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := f.svc.Accept(ctx, domain.Actor{Role: domain.ActorBuyer, UserID: buyer}, id); err == nil {
		t.Fatal("buyer acted on a seller-order")
	}
}

func TestSellerCannotActOnPendingOrderOfAnotherStore404(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	a, b := newShop(t, f, "A"), newShop(t, f, "B")
	o := f.place(t, buyer, 1, b)
	id := soOf(t, o, b.store)
	acts := map[string]func() error{
		"accept":   func() error { _, err := f.svc.Accept(ctx, sellerActor(a.owner), id); return err },
		"reject":   func() error { _, err := f.svc.Reject(ctx, sellerActor(a.owner), id, "BUSY_OR_CLOSED", ""); return err },
		"ready":    func() error { _, err := f.svc.Ready(ctx, sellerActor(a.owner), id, ""); return err },
		"complete": func() error { _, err := f.svc.Complete(ctx, sellerActor(a.owner), id); return err },
		"cancel":   func() error { _, err := f.svc.Cancel(ctx, sellerActor(a.owner), id, "OTHER", "x"); return err },
	}
	for name, act := range acts {
		if err := act(); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := f.svc.Accept(ctx, sellerActor(buyer), id); codeOf(err) != "NO_STORE" {
		t.Fatalf("no store: %v", err)
	}
	if got, _ := f.svc.GetStoreOrder(ctx, b.owner, id); got.Status != domain.OrderCreated {
		t.Fatalf("status changed: %s", got.Status)
	}
	if f.stock(t, b.product) != 9 {
		t.Fatalf("stock = %d", f.stock(t, b.product))
	}
}

// Two sellers accept their parts of one order at the same moment; the order row lock makes the
// later recomputation see the earlier change, so the order ends SELLER_PROCESSING.
func TestMultiSeller_ConcurrentAccepts_OrderStatusConsistent(t *testing.T) {
	f := setupOrders(t)
	ctx := context.Background()
	buyer := seed.User(t, f.pool, "", "")
	a, b := newShop(t, f, "A"), newShop(t, f, "B")
	if _, err := f.pool.Exec(ctx, `UPDATE products SET stock = 1000`); err != nil {
		t.Fatal(err)
	}
	for i := range 10 {
		o := f.place(t, buyer, 1, a, b)
		start := make(chan struct{})
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for j, s := range []shop{a, b} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[j] = f.svc.Accept(ctx, sellerActor(s.owner), soOf(t, o, s.store))
			}()
		}
		close(start)
		wg.Wait()
		if errs[0] != nil || errs[1] != nil {
			t.Fatalf("run %d: %v", i, errs)
		}
		if got := f.orderStatus(t, o.ID); got != domain.OrderSellerProcessing {
			t.Fatalf("run %d: order status %s", i, got)
		}
		if n := f.count(t, `SELECT count(*) FROM order_status_history WHERE order_id = $1 AND seller_order_id IS NULL`, o.ID); n != 2 {
			t.Fatalf("run %d: order-level history rows = %d", i, n)
		}
	}
}
