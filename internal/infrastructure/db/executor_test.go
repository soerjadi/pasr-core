package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
)

func count(t *testing.T, ex *db.Executor, name string) int {
	t.Helper()
	var n int
	if err := ex.QueryRow(context.Background(), `SELECT count(*) FROM categories WHERE name = $1`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func insert(ctx context.Context, ex *db.Executor, name string) error {
	_, err := ex.Exec(ctx, `INSERT INTO categories (name, icon_key) VALUES ($1, 'k')`, name)
	return err
}

func TestExecutor_CommitsAndRollsBack(t *testing.T) {
	ex := db.NewExecutor(pgtest.Pool(t))
	ctx := context.Background()

	if err := ex.WithTx(ctx, func(ctx context.Context) error { return insert(ctx, ex, "kept") }); err != nil {
		t.Fatal(err)
	}
	if count(t, ex, "kept") != 1 {
		t.Fatal("commit lost row")
	}

	boom := errors.New("boom")
	err := ex.WithTx(ctx, func(ctx context.Context) error {
		if err := insert(ctx, ex, "gone"); err != nil {
			return err
		}
		// invisible outside the tx until commit
		if count(t, ex, "gone") != 0 {
			t.Error("uncommitted row visible outside tx")
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if count(t, ex, "gone") != 0 {
		t.Fatal("rollback failed")
	}
}

func TestExecutor_NestedJoinsOuter(t *testing.T) {
	ex := db.NewExecutor(pgtest.Pool(t))
	ctx := context.Background()
	boom := errors.New("boom")

	err := ex.WithTx(ctx, func(ctx context.Context) error {
		if err := ex.WithTx(ctx, func(ctx context.Context) error { return insert(ctx, ex, "inner") }); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if count(t, ex, "inner") != 0 {
		t.Fatal("nested tx committed independently of outer")
	}
}

func TestExecutor_RollsBackOnPanic(t *testing.T) {
	ex := db.NewExecutor(pgtest.Pool(t))
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic was swallowed")
			}
		}()
		_ = ex.WithTx(context.Background(), func(ctx context.Context) error {
			_ = insert(ctx, ex, "panicked")
			panic("kaboom")
		})
	}()
	if count(t, ex, "panicked") != 0 {
		t.Fatal("panic did not roll back")
	}
}
