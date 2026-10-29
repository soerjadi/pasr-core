package cart

import (
	"context"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

func TestLoadProducts_ReturnsDeletedHiddenAndStoreState(t *testing.T) {
	pool := pgtest.Pool(t)
	r := NewCartRepository(db.NewExecutor(pool))
	store, _ := seed.Store(t, pool, seed.StoreOpts{Name: "Bu Siti", Photo: true, WhatsApp: "+6281234567889"})
	live := seed.Product(t, pool, seed.ProductOpts{StoreID: store, Name: "Tempe", Price: 15000, Stock: 7, Images: 2})
	gone := seed.Product(t, pool, seed.ProductOpts{StoreID: store, Deleted: true})
	hidden := seed.Product(t, pool, seed.ProductOpts{StoreID: store, Status: "HIDDEN"})
	if _, err := pool.Exec(context.Background(), `UPDATE stores SET is_open = false WHERE id = $1`, store); err != nil {
		t.Fatal(err)
	}

	got, err := r.LoadProducts(context.Background(), []int64{live, gone, hidden, 999999}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 rows (unknown id absent), got %d", len(got))
	}
	p := got[live]
	if p.Name != "Tempe" || p.Price != 15000 || p.Stock != 7 || p.Image == nil || p.Deleted {
		t.Fatalf("live = %+v", p)
	}
	if p.Store.ID != store || p.Store.Name != "Bu Siti" || p.Store.Avatar == nil || p.Store.IsOpen ||
		p.Store.Status != "VERIFIED" || p.Store.WhatsApp != "+6281234567889" || p.Store.PickupAddress == "" {
		t.Fatalf("store = %+v", p.Store)
	}
	if !got[gone].Deleted || got[hidden].Status != domain.ProductHidden {
		t.Fatalf("classification lost: gone=%+v hidden=%+v", got[gone], got[hidden])
	}
}

func TestLoadProducts_LockBlocksOtherLockersButNotStoreWrites(t *testing.T) {
	pool := pgtest.Pool(t)
	exec := db.NewExecutor(pool)
	r := NewCartRepository(exec)
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	b := seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	a := seed.Product(t, pool, seed.ProductOpts{StoreID: store})
	ctx := context.Background()

	err := exec.WithTx(ctx, func(ctx context.Context) error {
		got, err := r.LoadProducts(ctx, []int64{b, a}, true)
		if err != nil || len(got) != 2 {
			t.Fatalf("locked load: %d rows, %v", len(got), err)
		}
		// the store row is not locked: a write to it from another connection must not wait
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '300ms'`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE stores SET is_open = false WHERE id = $1`, store); err != nil {
			t.Fatalf("store row must not be locked: %v", err)
		}
		// but the product rows are locked
		if _, err := tx.Exec(ctx, `UPDATE products SET stock = 1 WHERE id = $1`, a); err == nil {
			t.Fatal("product row must be locked")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
