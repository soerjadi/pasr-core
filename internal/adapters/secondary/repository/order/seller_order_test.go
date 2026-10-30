package order

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

var numberSeq atomic.Int64

func insertOne(t *testing.T, r *orderRepository, buyer, store, product int64, at time.Time) (orderID, sellerOrderID int64) {
	t.Helper()
	ctx := context.Background()
	id, err := r.Insert(ctx, newOrder(buyer, store, product, fmt.Sprintf("NM-S-%d", numberSeq.Add(1)), at))
	if err != nil {
		t.Fatal(err)
	}
	o, err := r.Get(ctx, buyer, id)
	if err != nil {
		t.Fatal(err)
	}
	return id, o.SellerOrders[0].ID
}

func TestLockSellerOrder_StoreScopedAndAnyStore(t *testing.T) {
	pool, exec, r := setup(t)
	ctx := context.Background()
	buyer := seed.User(t, pool, "", "B")
	store, owner := seed.Store(t, pool, seed.StoreOpts{Name: "Bu Siti"})
	other, _ := seed.Store(t, pool, seed.StoreOpts{})
	p := seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	at := time.Date(2026, 10, 15, 10, 0, 0, 0, wib)
	orderID, soID := insertOne(t, r, buyer, store, p, at)

	err := exec.WithTx(ctx, func(ctx context.Context) error {
		l, err := r.LockSellerOrder(ctx, soID, store)
		if err != nil {
			return err
		}
		if l.ID != soID || l.OrderID != orderID || l.BuyerID != buyer || l.StoreID != store || l.OwnerID != owner ||
			l.Status != domain.OrderCreated || l.AcceptBy == nil || !l.AcceptBy.Equal(at.Add(10*time.Minute)) ||
			l.AcceptBy.Location() != time.UTC || l.StoreName != "Bu Siti" || l.OrderNumber == "" || l.CancelReason != "" {
			t.Errorf("lock = %+v", l)
		}
		if _, err := r.LockSellerOrder(ctx, soID, 0); err != nil {
			t.Errorf("any store: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.LockSellerOrder(ctx, soID, other); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other store: %v", err)
	}
}

func TestUpdateSellerOrder_StoresFieldsAndHistory(t *testing.T) {
	pool, _, r := setup(t)
	ctx := context.Background()
	buyer := seed.User(t, pool, "", "Budi")
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	p := seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	at := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	_, soID := insertOne(t, r, buyer, store, p, at)

	note := "ambil jam 2"
	if err := r.UpdateSellerOrder(ctx, domain.SellerOrderChange{ID: soID, Status: domain.OrderReadyForPickup, At: at.Add(time.Hour), Note: &note}); err != nil {
		t.Fatal(err)
	}
	reason, by := "OTHER", "seller"
	if err := r.UpdateSellerOrder(ctx, domain.SellerOrderChange{ID: soID, Status: domain.OrderCancelled, At: at.Add(2 * time.Hour),
		CancelReason: &reason, CancelledBy: &by}); err != nil {
		t.Fatal(err)
	}
	so, err := r.GetStoreOrder(ctx, store, soID)
	if err != nil {
		t.Fatal(err)
	}
	if so.Status != domain.OrderCancelled || so.Note != note || so.BuyerName != "Budi" || len(so.Items) != 2 ||
		len(so.StatusHistory) != 3 || so.StatusHistory[2].Status != domain.OrderCancelled ||
		!so.StatusHistory[1].At.Equal(at.Add(time.Hour)) || so.CreatedAt.Location() != time.UTC {
		t.Fatalf("store order = %+v", so)
	}
	var gotReason, gotBy string
	var updated time.Time
	if err := pool.QueryRow(ctx, `SELECT cancel_reason, cancelled_by, updated_at FROM seller_orders WHERE id = $1`, soID).
		Scan(&gotReason, &gotBy, &updated); err != nil {
		t.Fatal(err)
	}
	if gotReason != reason || gotBy != by || !updated.Equal(at.Add(2*time.Hour)) {
		t.Fatalf("stored %q %q %v", gotReason, gotBy, updated)
	}
	if err := r.UpdateSellerOrder(ctx, domain.SellerOrderChange{ID: 999999, Status: domain.OrderCancelled, At: at}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestRestoreStock_AddsQuantitiesAndClearsOutOfStock(t *testing.T) {
	pool, _, r := setup(t)
	ctx := context.Background()
	buyer := seed.User(t, pool, "", "")
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	p := seed.Product(t, pool, seed.ProductOpts{StoreID: store, Status: "OUT_OF_STOCK", Stock: 0})
	hidden := seed.Product(t, pool, seed.ProductOpts{StoreID: store, Status: "HIDDEN", Stock: 0})
	_, soID := insertOne(t, r, buyer, store, p, time.Now())
	o := newOrder(buyer, store, hidden, "NM-H", time.Now())
	hid, err := r.Insert(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	ho, _ := r.Get(ctx, buyer, hid)

	for _, id := range []int64{soID, ho.SellerOrders[0].ID} {
		if err := r.RestoreStock(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	type want struct {
		stock  int32
		status string
	}
	// The order has two lines of the product (2 + 1); a hidden product stays hidden (seeded with 10).
	for id, w := range map[int64]want{p: {3, "AVAILABLE"}, hidden: {13, "HIDDEN"}} {
		var n int32
		var st string
		if err := pool.QueryRow(ctx, `SELECT stock, status FROM products WHERE id = $1`, id).Scan(&n, &st); err != nil {
			t.Fatal(err)
		}
		if n != w.stock || st != w.status {
			t.Fatalf("product %d: %d %s, want %+v", id, n, st, w)
		}
	}
}

func TestLockOrderStatusesAndSetOrderStatus(t *testing.T) {
	pool, exec, r := setup(t)
	ctx := context.Background()
	buyer := seed.User(t, pool, "", "")
	a, _ := seed.Store(t, pool, seed.StoreOpts{})
	b, _ := seed.Store(t, pool, seed.StoreOpts{})
	pa := seed.Product(t, pool, seed.ProductOpts{StoreID: a})
	created := time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC)
	o := newOrder(buyer, a, pa, "NM-M", created)
	second := o.SellerOrders[0]
	second.StoreID = b
	second.Status = domain.OrderCompleted
	o.SellerOrders = append(o.SellerOrders, second)
	id, err := r.Insert(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 15, 4, 0, 0, 0, time.UTC)
	err = exec.WithTx(ctx, func(ctx context.Context) error {
		cur, sts, err := r.LockOrderStatuses(ctx, id)
		if err != nil {
			return err
		}
		if cur != domain.OrderCreated || len(sts) != 2 || sts[0] != domain.OrderCreated || sts[1] != domain.OrderCompleted {
			t.Errorf("statuses %s %v", cur, sts)
		}
		return r.SetOrderStatus(ctx, id, domain.OrderSellerProcessing, at)
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := r.Get(ctx, buyer, id)
	if got.Status != domain.OrderSellerProcessing || len(got.StatusHistory) != 2 || !got.StatusHistory[1].At.Equal(at) {
		t.Fatalf("order = %+v", got)
	}
	if _, _, err := r.LockOrderStatuses(ctx, 999999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestListAndCountStoreOrders(t *testing.T) {
	pool, _, r := setup(t)
	ctx := context.Background()
	buyer := seed.User(t, pool, "", "")
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	other, _ := seed.Store(t, pool, seed.StoreOpts{})
	p := seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	po := seed.Product(t, pool, seed.ProductOpts{StoreID: other})
	var ids []int64
	for _, st := range []domain.OrderStatus{domain.OrderCreated, domain.OrderSellerProcessing, domain.OrderReadyForPickup,
		domain.OrderCompleted, domain.OrderCancelled, domain.OrderCreated} {
		_, so := insertOne(t, r, buyer, store, p, time.Now())
		if _, err := pool.Exec(ctx, `UPDATE seller_orders SET status = $2 WHERE id = $1`, so, st); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, so)
	}
	insertOne(t, r, buyer, other, po, time.Now())

	c, err := r.CountStoreOrders(ctx, store)
	if err != nil || c != (domain.StoreOrderCounts{New: 2, Processing: 2, Done: 2}) {
		t.Fatalf("counts %+v %v", c, err)
	}
	if c, err := r.CountStoreOrders(ctx, 999999); err != nil || c != (domain.StoreOrderCounts{}) {
		t.Fatalf("empty counts %+v %v", c, err)
	}
	newTab, err := r.ListStoreOrders(ctx, store, []domain.OrderStatus{domain.OrderCreated}, 0, 10)
	if err != nil || len(newTab) != 2 || newTab[0].ID != ids[5] || newTab[1].ID != ids[0] || len(newTab[0].Items) != 2 ||
		len(newTab[0].StatusHistory) != 1 {
		t.Fatalf("new: %v %+v", err, newTab)
	}
	page, err := r.ListStoreOrders(ctx, store, []domain.OrderStatus{domain.OrderCreated}, ids[5], 10)
	if err != nil || len(page) != 1 || page[0].ID != ids[0] {
		t.Fatalf("cursor: %v %+v", err, page)
	}
	if _, err := r.GetStoreOrder(ctx, other, ids[0]); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other store get: %v", err)
	}
}

func TestOverdueSellerOrders_SkipsLockedAndNotDue(t *testing.T) {
	pool, exec, r := setup(t)
	ctx := context.Background()
	buyer := seed.User(t, pool, "", "")
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	p := seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	at := time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC)
	_, due := insertOne(t, r, buyer, store, p, at)
	_, locked := insertOne(t, r, buyer, store, p, at)
	_, accepted := insertOne(t, r, buyer, store, p, at)
	_, later := insertOne(t, r, buyer, store, p, at.Add(time.Hour))
	if _, err := pool.Exec(ctx, `UPDATE seller_orders SET status = 'SELLER_PROCESSING' WHERE id = $1`, accepted); err != nil {
		t.Fatal(err)
	}
	now := at.Add(10 * time.Minute) // exactly accept_by of the first three

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT 1 FROM seller_orders WHERE id = $1 FOR UPDATE`, locked); err != nil {
		t.Fatal(err)
	}
	err = exec.WithTx(ctx, func(ctx context.Context) error {
		got, err := r.OverdueSellerOrders(ctx, now, 100)
		if err != nil {
			return err
		}
		if len(got) != 1 || got[0] != due {
			t.Errorf("overdue = %v, want [%d] (later %d)", got, due, later)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The seller's NEW tab walks an index in id DESC order instead of sorting.
func TestListStoreOrdersQuery_UsesIndex(t *testing.T) {
	pool, _, _ := setup(t)
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SET enable_seqscan = off; SET enable_bitmapscan = off`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = conn.Exec(ctx, `RESET enable_seqscan; RESET enable_bitmapscan`) }()
	for _, filter := range [][]string{{"CREATED"}, {"COMPLETED", "CANCELLED"}} {
		rows, err := conn.Query(ctx, "EXPLAIN "+listStoreOrdersQuery, int64(1), filter, int64(0), 21)
		if err != nil {
			t.Fatal(err)
		}
		var plan strings.Builder
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal(err)
			}
			plan.WriteString(line + "\n")
		}
		rows.Close()
		if !strings.Contains(plan.String(), "seller_orders_store_") {
			t.Errorf("filter %v: plan:\n%s", filter, plan.String())
		}
		if len(filter) == 1 && strings.Contains(plan.String(), "Sort") {
			t.Errorf("filter %v sorts: plan:\n%s", filter, plan.String())
		}
	}
}
