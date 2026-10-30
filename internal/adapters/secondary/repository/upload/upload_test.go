package upload

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

func setup(t *testing.T) (*uploadRepository, *pgxpool.Pool, int64, int64) {
	t.Helper()
	pool := pgtest.Pool(t)
	ctx := context.Background()
	var a, b int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (phone) VALUES ('+6281000000001') RETURNING id`).Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (phone) VALUES ('+6281000000002') RETURNING id`).Scan(&b); err != nil {
		t.Fatal(err)
	}
	return NewUploadRepository(db.NewExecutor(pool)).(*uploadRepository), pool, a, b
}

func keys(n string) domain.ImageKeys {
	return domain.ImageKeys{Small: n + "-s", Medium: n + "-m", Large: n + "-l"}
}

func TestResolveOwned_RejectsOtherUsersUpload(t *testing.T) {
	r, _, alice, bob := setup(t)
	ctx := context.Background()
	id, err := r.Create(ctx, alice, keys("a"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.ResolveOwned(ctx, alice, []int64{id})
	if err != nil || got[id] != keys("a") {
		t.Fatalf("owner resolve = %v, %v", got, err)
	}
	if _, err := r.ResolveOwned(ctx, bob, []int64{id}); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("other user: err = %v, want ErrValidationFailed", err)
	}
	if _, err := r.ResolveOwned(ctx, alice, []int64{id, id + 999}); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("unknown id: err = %v", err)
	}
	if err := r.MarkAttached(ctx, []int64{id}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ResolveOwned(ctx, alice, []int64{id}); !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("attached upload must not resolve again: err = %v", err)
	}
}

func TestDeleteOrphans_OnlyOldUnattached(t *testing.T) {
	r, pool, alice, _ := setup(t)
	ctx := context.Background()
	oldOrphan, _ := r.Create(ctx, alice, keys("old-orphan"))
	oldAttached, _ := r.Create(ctx, alice, keys("old-attached"))
	fresh, _ := r.Create(ctx, alice, keys("fresh"))
	if err := r.MarkAttached(ctx, []int64{oldAttached}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE uploads SET created_at = now() - interval '48 hours' WHERE id = ANY($1)`, []int64{oldOrphan, oldAttached}); err != nil {
		t.Fatal(err)
	}

	got, err := r.DeleteOrphans(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != keys("old-orphan") {
		t.Fatalf("deleted keys = %v", got)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM uploads WHERE id = ANY($1)`, []int64{oldAttached, fresh}).Scan(&n); err != nil || n != 2 {
		t.Fatalf("remaining = %d, %v", n, err)
	}
}

// TestDeleteDetached_OnlyOldUnreferencedAttached seeds an upload for every kind of reference and
// checks that only the old, attached, unreferenced one is removed.
func TestDeleteDetached_OnlyOldUnreferencedAttached(t *testing.T) {
	r, pool, _, _ := setup(t)
	ctx := context.Background()

	store, owner := seed.Store(t, pool, seed.StoreOpts{Photo: true})
	product := seed.Product(t, pool, seed.ProductOpts{StoreID: store, Images: 1})
	seed.Product(t, pool, seed.ProductOpts{StoreID: store, Images: 1, Deleted: true})
	// a replaced product photo still shown in an order's line snapshot
	snapshotted, snapKeys := seed.Upload(t, pool, owner, "snapshotted")
	item := seed.OrderItem(t, pool, seed.User(t, pool, "", "Buyer"), store, product)
	b, _ := json.Marshal(snapKeys)
	if _, err := pool.Exec(ctx, `UPDATE order_items SET image = $1 WHERE id = $2`, b, item); err != nil {
		t.Fatal(err)
	}
	detached, detachedKeys := seed.Upload(t, pool, owner, "detached")
	freshDetached, _ := seed.Upload(t, pool, owner, "fresh-detached")
	unattached, _ := r.Create(ctx, owner, keys("unattached")) // DeleteOrphans' job, not this one's

	var inUse []int64
	if err := pool.QueryRow(ctx, `
		SELECT array_agg(id) FROM uploads
		WHERE id IN (SELECT photo_upload_id FROM stores) OR id IN (SELECT upload_id FROM product_images)`).Scan(&inUse); err != nil {
		t.Fatal(err)
	}
	if len(inUse) != 3 {
		t.Fatalf("seeded %d referenced uploads, want 3 (store photo, product, deleted product)", len(inUse))
	}
	if _, err := pool.Exec(ctx, `UPDATE uploads SET created_at = now() - interval '48 hours' WHERE id <> $1`, freshDetached); err != nil {
		t.Fatal(err)
	}

	got, err := r.DeleteDetached(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != detachedKeys {
		t.Fatalf("deleted keys = %v, want only %v", got, detachedKeys)
	}
	keep := append([]int64{snapshotted, freshDetached, unattached}, inUse...)
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM uploads WHERE id = ANY($1)`, keep).Scan(&n); err != nil || n != len(keep) {
		t.Fatalf("kept %d of %d, %v", n, len(keep), err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM uploads WHERE id = $1`, detached).Scan(&n); err != nil || n != 0 {
		t.Fatalf("detached upload still present: %d, %v", n, err)
	}
}
