package upload

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
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
