package idempotency

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

func order(t *testing.T, pool *pgxpool.Pool, buyer int64, number string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO orders (number, buyer_id, total, status) VALUES ($1, $2, 1, 'CREATED') RETURNING id`, number, buyer).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestClaim_FirstClaimsThenReportsStoredRow(t *testing.T) {
	pool := pgtest.Pool(t)
	r := NewIdempotencyRepository(db.NewExecutor(pool))
	ctx := context.Background()
	u, other := seed.User(t, pool, "", "A"), seed.User(t, pool, "", "B")

	if _, claimed, _, err := r.Claim(ctx, u, "k", "h1"); err != nil || !claimed {
		t.Fatalf("first claim: claimed=%v err=%v", claimed, err)
	}
	oid := order(t, pool, u, "NM-1")
	if err := r.Attach(ctx, u, "k", oid); err != nil {
		t.Fatal(err)
	}
	got, claimed, hash, err := r.Claim(ctx, u, "k", "h2")
	if err != nil || claimed || got != oid || hash != "h1" {
		t.Fatalf("second claim: order=%d claimed=%v hash=%q err=%v", got, claimed, hash, err)
	}
	// keys are per user
	if _, claimed, _, err := r.Claim(ctx, other, "k", "h1"); err != nil || !claimed {
		t.Fatalf("other user: claimed=%v err=%v", claimed, err)
	}
	if err := r.Attach(ctx, u, "missing", oid); err == nil {
		t.Fatal("attach of an unclaimed key must fail")
	}
}

// A second transaction claiming the same key waits for the first. After a commit it sees the
// attached order; after a rollback it claims the key itself.
func TestClaim_WaitsForConcurrentHolder(t *testing.T) {
	for _, commit := range []bool{true, false} {
		name := map[bool]string{true: "commit", false: "rollback"}[commit]
		t.Run(name, func(t *testing.T) {
			pool := pgtest.Pool(t)
			exec := db.NewExecutor(pool)
			r := NewIdempotencyRepository(exec)
			ctx := context.Background()
			u := seed.User(t, pool, "", "A")
			oid := order(t, pool, u, "NM-1")

			holding, release := make(chan struct{}), make(chan struct{})
			errRollback := errors.New("rollback")
			first := make(chan error, 1)
			go func() {
				first <- exec.WithTx(ctx, func(ctx context.Context) error {
					if _, claimed, _, err := r.Claim(ctx, u, "k", "h"); err != nil || !claimed {
						t.Errorf("holder: claimed=%v err=%v", claimed, err)
					}
					if err := r.Attach(ctx, u, "k", oid); err != nil {
						return err
					}
					close(holding)
					<-release
					if !commit {
						return errRollback
					}
					return nil
				})
			}()
			<-holding

			type res struct {
				order   int64
				claimed bool
				err     error
			}
			second := make(chan res, 1)
			go func() {
				var out res
				out.err = exec.WithTx(ctx, func(ctx context.Context) error {
					var err error
					out.order, out.claimed, _, err = r.Claim(ctx, u, "k", "h")
					return err
				})
				second <- out
			}()
			select {
			case got := <-second:
				t.Fatalf("second claim did not wait: %+v", got)
			case <-time.After(200 * time.Millisecond):
			}
			close(release)
			if err := <-first; commit && err != nil || !commit && !errors.Is(err, errRollback) {
				t.Fatalf("holder: %v", err)
			}
			got := <-second
			if got.err != nil {
				t.Fatal(got.err)
			}
			if commit && (got.claimed || got.order != oid) || !commit && (!got.claimed || got.order != 0) {
				t.Fatalf("after %s: %+v", name, got)
			}
		})
	}
}
