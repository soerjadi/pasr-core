package order

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

var wib = time.FixedZone("WIB", 7*3600)

func setup(t *testing.T) (*pgxpool.Pool, *db.Executor, *orderRepository) {
	t.Helper()
	pool := pgtest.Pool(t)
	exec := db.NewExecutor(pool)
	return pool, exec, NewOrderRepository(exec).(*orderRepository)
}

func TestNextOrderNumber_SequentialPerWIBDay(t *testing.T) {
	_, _, r := setup(t)
	ctx := context.Background()
	// 23:30 UTC on the 14th is already the 15th in WIB.
	day := time.Date(2026, 10, 14, 23, 30, 0, 0, time.UTC).In(wib)
	for _, want := range []string{"NM-20261015-001", "NM-20261015-002"} {
		got, err := r.NextOrderNumber(ctx, day)
		if err != nil || got != want {
			t.Fatalf("got %q %v, want %q", got, err, want)
		}
	}
	if got, err := r.NextOrderNumber(ctx, day.Add(24*time.Hour)); err != nil || got != "NM-20261016-001" {
		t.Fatalf("next day: %q %v", got, err)
	}
}

func TestNextOrderNumber_ConcurrentDistinct(t *testing.T) {
	_, exec, r := setup(t)
	day := time.Date(2026, 10, 15, 9, 0, 0, 0, wib)
	const n = 30
	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := exec.WithTx(context.Background(), func(ctx context.Context) error {
				got, err := r.NextOrderNumber(ctx, day)
				if err != nil {
					return err
				}
				mu.Lock()
				seen[got] = true
				mu.Unlock()
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for i := 1; i <= n; i++ {
		if !seen[fmt.Sprintf("NM-20261015-%03d", i)] {
			t.Fatalf("missing %d in %v", i, seen)
		}
	}
}

func TestDecrementStock_Conditions(t *testing.T) {
	pool, _, r := setup(t)
	ctx := context.Background()
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	p := seed.Product(t, pool, seed.ProductOpts{StoreID: store, Stock: 3})
	hidden := seed.Product(t, pool, seed.ProductOpts{StoreID: store, Status: "HIDDEN", Stock: 3})
	gone := seed.Product(t, pool, seed.ProductOpts{StoreID: store, Deleted: true, Stock: 3})

	stock := func(id int64) (n int32, status string) {
		if err := pool.QueryRow(ctx, `SELECT stock, status FROM products WHERE id = $1`, id).Scan(&n, &status); err != nil {
			t.Fatal(err)
		}
		return n, status
	}
	if ok, err := r.DecrementStock(ctx, p, 4); err != nil || ok {
		t.Fatalf("more than stock: %v %v", ok, err)
	}
	if ok, err := r.DecrementStock(ctx, p, 2); err != nil || !ok {
		t.Fatalf("decrement: %v %v", ok, err)
	}
	if n, st := stock(p); n != 1 || st != "AVAILABLE" {
		t.Fatalf("after 2: %d %s", n, st)
	}
	if ok, err := r.DecrementStock(ctx, p, 1); err != nil || !ok {
		t.Fatalf("last unit: %v %v", ok, err)
	}
	if n, st := stock(p); n != 0 || st != "OUT_OF_STOCK" {
		t.Fatalf("sold out: %d %s", n, st)
	}
	for _, id := range []int64{hidden, gone} {
		if ok, err := r.DecrementStock(ctx, id, 1); err != nil || ok {
			t.Fatalf("product %d: %v %v", id, ok, err)
		}
	}
}

func newOrder(buyer, store, product int64, number string, at time.Time) domain.NewOrder {
	return domain.NewOrder{
		Number: number, BuyerID: buyer, Total: 30000, Status: domain.OrderCreated, CreatedAt: at,
		SellerOrders: []domain.NewSellerOrder{{
			StoreID: store, StoreName: "Bu Siti", WhatsApp: "+6281", PickupAddress: "Jl. A", OpeningHours: "Setiap hari",
			Status: domain.OrderCreated, Subtotal: 30000, AcceptBy: at.Add(10 * time.Minute),
			Items: []domain.NewOrderItem{
				{ProductID: product, Name: "Tempe", Image: &domain.ImageKeys{Small: "s", Medium: "m", Large: "l"}, Quantity: 2, UnitPrice: 10000, LineTotal: 20000},
				{ProductID: product, Name: "Tempe lagi", Quantity: 1, UnitPrice: 10000, LineTotal: 10000},
			},
		}},
	}
}

func TestInsertThenGet_RoundTrip(t *testing.T) {
	pool, exec, r := setup(t)
	ctx := context.Background()
	buyer := seed.User(t, pool, "", "B")
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	p := seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	at := time.Date(2026, 10, 15, 10, 2, 3, 0, wib)

	var id int64
	err := exec.WithTx(ctx, func(ctx context.Context) error {
		var err error
		id, err = r.Insert(ctx, newOrder(buyer, store, p, "NM-20261015-001", at))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	o, err := r.Get(ctx, buyer, id)
	if err != nil {
		t.Fatal(err)
	}
	if o.Number != "NM-20261015-001" || o.Total != 30000 || o.Status != domain.OrderCreated ||
		o.PaymentMethod != domain.PaymentCashAtPickup || !o.CreatedAt.Equal(at) || o.CreatedAt.Location() != time.UTC {
		t.Fatalf("order = %+v", o)
	}
	if len(o.StatusHistory) != 1 || o.StatusHistory[0].Status != domain.OrderCreated || !o.StatusHistory[0].At.Equal(at) ||
		o.StatusHistory[0].At.Location() != time.UTC {
		t.Fatalf("history = %+v", o.StatusHistory)
	}
	if len(o.SellerOrders) != 1 {
		t.Fatalf("seller orders = %+v", o.SellerOrders)
	}
	so := o.SellerOrders[0]
	if so.SellerID != store || so.SellerName != "Bu Siti" || so.PickupAddress != "Jl. A" || so.WhatsApp != "+6281" ||
		so.OpeningHours != "Setiap hari" || so.Subtotal != 30000 || so.Status != domain.OrderCreated || len(so.Items) != 2 {
		t.Fatalf("seller order = %+v", so)
	}
	it := so.Items[0]
	if it.ProductID != p || it.Name != "Tempe" || it.Quantity != 2 || it.UnitPrice != 10000 || it.LineTotal != 20000 ||
		it.ImageKeys == nil || it.ImageKeys.Large != "l" || it.Reviewed || so.Items[1].ImageKeys != nil {
		t.Fatalf("items = %+v", so.Items)
	}
	var acceptBy time.Time
	var perSeller int
	if err := pool.QueryRow(ctx, `SELECT accept_by FROM seller_orders WHERE id = $1`, so.ID).Scan(&acceptBy); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM order_status_history WHERE seller_order_id = $1 AND status = 'CREATED'`, so.ID).Scan(&perSeller); err != nil {
		t.Fatal(err)
	}
	if !acceptBy.Equal(at.Add(10*time.Minute)) || perSeller != 1 {
		t.Fatalf("accept_by = %v, seller history = %d", acceptBy, perSeller)
	}

	if _, err := r.Get(ctx, seed.User(t, pool, "", "X"), id); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other buyer: %v", err)
	}
}

func TestList_FilterCursorAndOwnership(t *testing.T) {
	pool, _, r := setup(t)
	ctx := context.Background()
	buyer, other := seed.User(t, pool, "", "B"), seed.User(t, pool, "", "O")
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	p := seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	at := time.Now()
	var ids []int64
	for i, st := range []domain.OrderStatus{domain.OrderCreated, domain.OrderCompleted, domain.OrderCreated, domain.OrderCancelled} {
		o := newOrder(buyer, store, p, fmt.Sprintf("NM-L-%d", i), at)
		o.Status = st
		id, err := r.Insert(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if _, err := r.Insert(ctx, newOrder(other, store, p, "NM-L-X", at)); err != nil {
		t.Fatal(err)
	}

	all, err := r.List(ctx, buyer, nil, 0, 10)
	if err != nil || len(all) != 4 || all[0].ID != ids[3] || all[3].ID != ids[0] {
		t.Fatalf("all: %v %+v", err, all)
	}
	if len(all[0].SellerOrders) != 1 || len(all[0].SellerOrders[0].Items) != 2 || len(all[0].StatusHistory) != 1 {
		t.Fatalf("details missing: %+v", all[0])
	}
	page, err := r.List(ctx, buyer, nil, ids[2], 10)
	if err != nil || len(page) != 2 || page[0].ID != ids[1] {
		t.Fatalf("after cursor: %v %+v", err, page)
	}
	done, err := r.List(ctx, buyer, []domain.OrderStatus{domain.OrderCompleted, domain.OrderCancelled}, 0, 10)
	if err != nil || len(done) != 2 || done[0].ID != ids[3] || done[1].ID != ids[1] {
		t.Fatalf("done: %v %+v", err, done)
	}
	if empty, err := r.List(ctx, seed.User(t, pool, "", "N"), nil, 0, 10); err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("no orders: %v %+v", err, empty)
	}
}

// The buyer's list walks orders_buyer_idx in id DESC order instead of sorting.
func TestListQuery_UsesBuyerIndex(t *testing.T) {
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
	for _, filter := range [][]string{nil, {"COMPLETED", "CANCELLED"}} {
		rows, err := conn.Query(ctx, "EXPLAIN "+listQuery, int64(1), filter, int64(0), 21)
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
		if !strings.Contains(plan.String(), "orders_buyer_idx") || strings.Contains(plan.String(), "Sort") {
			t.Errorf("filter %v: plan:\n%s", filter, plan.String())
		}
	}
}

func TestSetStatementTimeout_LocalToTransaction(t *testing.T) {
	pool, exec, r := setup(t)
	ctx := context.Background()
	err := exec.WithTx(ctx, func(ctx context.Context) error {
		if err := r.SetStatementTimeout(ctx, 5*time.Second); err != nil {
			return err
		}
		var v string
		if err := exec.QueryRow(ctx, `SHOW statement_timeout`).Scan(&v); err != nil {
			return err
		}
		if v != "5s" {
			t.Errorf("inside tx: %q", v)
		}
		_, err := exec.Exec(ctx, `SELECT pg_sleep(0.2)`) // well under the limit
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	err = exec.WithTx(ctx, func(ctx context.Context) error {
		if err := r.SetStatementTimeout(ctx, 50*time.Millisecond); err != nil {
			return err
		}
		_, err := exec.Exec(ctx, `SELECT pg_sleep(1)`)
		return err
	})
	if err == nil {
		t.Fatal("statement over the timeout must fail")
	}
	var v string
	if err := pool.QueryRow(ctx, `SHOW statement_timeout`).Scan(&v); err != nil || v != "0" {
		t.Fatalf("leaked to the pool: %q %v", v, err)
	}
}
