package stats

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

const zone = "Asia/Jakarta"

func setup(t *testing.T) (*pgxpool.Pool, repository.StatsRepository) {
	t.Helper()
	pool := pgtest.Pool(t)
	return pool, NewStatsRepository(db.NewExecutor(pool))
}

// sale inserts a seller-order with one item; completedAt non-zero adds the COMPLETED history row.
func sale(t *testing.T, pool *pgxpool.Pool, buyer, store, product int64, name string, qty int32, subtotal int64, status string, created, completedAt time.Time) {
	t.Helper()
	ctx := context.Background()
	var orderID, soID int64
	n := time.Now().UnixNano()
	if err := pool.QueryRow(ctx, `INSERT INTO orders (number, buyer_id, total, status, created_at) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		fmt.Sprintf("NM-T-%d", n), buyer, subtotal, status, created).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO seller_orders (order_id, store_id, status, subtotal, store_name, pickup_address, whatsapp, created_at)
		VALUES ($1,$2,$3,$4,'Toko','Jl',  '+6281234567889',$5) RETURNING id`, orderID, store, status, subtotal, created).Scan(&soID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO order_items (seller_order_id, product_id, name, quantity, unit_price, line_total) VALUES ($1,$2,$3,$4,$5,$5)`,
		soID, product, name, qty, subtotal); err != nil {
		t.Fatal(err)
	}
	if !completedAt.IsZero() {
		if _, err := pool.Exec(ctx, `INSERT INTO order_status_history (order_id, seller_order_id, status, at) VALUES ($1,$2,'COMPLETED',$3)`,
			orderID, soID, completedAt); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCompletedByDay_GroupsByWIBDate(t *testing.T) {
	pool, r := setup(t)
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	buyer := seed.User(t, pool, "", "B")
	p := seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	d := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	// 16:59 UTC is 23:59 WIB on the 15th; 17:00 UTC is 00:00 WIB on the 16th.
	sale(t, pool, buyer, store, p, "A", 1, 1000, "COMPLETED", d, d.Add(16*time.Hour+59*time.Minute))
	sale(t, pool, buyer, store, p, "A", 1, 2000, "COMPLETED", d, d.Add(17*time.Hour))
	sale(t, pool, buyer, store, p, "A", 1, 4000, "COMPLETED", d, d.Add(18*time.Hour))
	sale(t, pool, buyer, store, p, "A", 1, 8000, "CANCELLED", d, time.Time{})
	got, err := r.CompletedByDay(context.Background(), store, d, d.Add(48*time.Hour), zone)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.DayTotal{{Date: "2026-10-15", Sales: 1000, Orders: 1}, {Date: "2026-10-16", Sales: 6000, Orders: 2}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// The window is half-open on the completion time.
	got, _ = r.CompletedByDay(context.Background(), store, d.Add(17*time.Hour), d.Add(18*time.Hour), zone)
	if len(got) != 1 || got[0].Sales != 2000 {
		t.Fatalf("window = %v", got)
	}
}

func TestCompletedByDay_OnlyOwnStoreAndCompletedStatus(t *testing.T) {
	pool, r := setup(t)
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	other, _ := seed.Store(t, pool, seed.StoreOpts{})
	buyer := seed.User(t, pool, "", "B")
	p := seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	q := seed.Product(t, pool, seed.ProductOpts{StoreID: other})
	d := time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC)
	sale(t, pool, buyer, store, p, "A", 1, 1000, "COMPLETED", d, d)
	sale(t, pool, buyer, other, q, "B", 1, 5000, "COMPLETED", d, d)
	sale(t, pool, buyer, store, p, "A", 1, 7000, "READY_FOR_PICKUP", d, time.Time{})
	got, err := r.CompletedByDay(context.Background(), store, d.Add(-time.Hour), d.Add(time.Hour), zone)
	if err != nil || len(got) != 1 || got[0].Sales != 1000 {
		t.Fatalf("got %v err %v", got, err)
	}
	n, err := r.CountCreated(context.Background(), store, d.Add(-time.Hour), d.Add(time.Hour))
	if err != nil || n != 2 {
		t.Fatalf("created = %d err %v", n, err)
	}
}

func TestCountCreated_ExcludesCancelledAndOutOfWindow(t *testing.T) {
	pool, r := setup(t)
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	buyer := seed.User(t, pool, "", "B")
	p := seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	d := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	sale(t, pool, buyer, store, p, "A", 1, 1, "CREATED", d, time.Time{})
	sale(t, pool, buyer, store, p, "A", 1, 1, "CANCELLED", d, time.Time{})
	sale(t, pool, buyer, store, p, "A", 1, 1, "CREATED", d.Add(24*time.Hour), time.Time{}) // `to` is exclusive
	n, err := r.CountCreated(context.Background(), store, d, d.Add(24*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("created = %d err %v", n, err)
	}
}

func TestTopProducts_RankedByUnitsWithSnapshotName(t *testing.T) {
	pool, r := setup(t)
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	buyer := seed.User(t, pool, "", "B")
	var ps [4]int64
	for i := range ps {
		ps[i] = seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	}
	d := time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC)
	sale(t, pool, buyer, store, ps[0], "Old name", 2, 100, "COMPLETED", d, d)
	sale(t, pool, buyer, store, ps[0], "New name", 2, 100, "COMPLETED", d, d.Add(time.Minute)) // 4 units
	sale(t, pool, buyer, store, ps[1], "B", 9, 100, "COMPLETED", d, d)
	sale(t, pool, buyer, store, ps[2], "C", 4, 100, "COMPLETED", d, d) // ties with ps[0]: lower id first
	sale(t, pool, buyer, store, ps[3], "D", 1, 100, "COMPLETED", d, d)
	sale(t, pool, buyer, store, ps[3], "D", 50, 100, "CANCELLED", d, time.Time{})
	got, err := r.TopProducts(context.Background(), store, d.Add(-time.Hour), d.Add(time.Hour), 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.TopProduct{{ProductID: ps[1], Name: "B", UnitsSold: 9}, {ProductID: ps[0], Name: "New name", UnitsSold: 4}, {ProductID: ps[2], Name: "C", UnitsSold: 4}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
