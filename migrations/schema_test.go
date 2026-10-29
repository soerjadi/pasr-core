package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/migrations"
)

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func wantErr(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err == nil {
		t.Fatalf("expected constraint violation for %q", sql)
	}
}

func scalar[T any](t *testing.T, pool *pgxpool.Pool, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	return v
}

func newUser(t *testing.T, pool *pgxpool.Pool, phone string) int64 {
	t.Helper()
	return scalar[int64](t, pool, `INSERT INTO users (phone, name) VALUES ($1, 'Budi') RETURNING id`, phone)
}

func newStore(t *testing.T, pool *pgxpool.Pool, ownerID int64) int64 {
	t.Helper()
	return scalar[int64](t, pool, `INSERT INTO stores (owner_id, name, category_id, owner_name, kelurahan, address, whatsapp)
		VALUES ($1, 'Warung', 1, 'Budi', 'Menteng', 'Jl. A 1', '+628111') RETURNING id`, ownerID)
}

func newProduct(t *testing.T, pool *pgxpool.Pool, storeID int64) int64 {
	t.Helper()
	return scalar[int64](t, pool, `INSERT INTO products (store_id, category_id, name, price, stock)
		VALUES ($1, 1, 'Nasi', 10000, 5) RETURNING id`, storeID)
}

func TestSeedCategoriesAreNine(t *testing.T) {
	pool := pgtest.Pool(t)
	if n := scalar[int](t, pool, `SELECT count(*) FROM categories`); n != 9 {
		t.Fatalf("categories = %d, want 9", n)
	}
	if name := scalar[string](t, pool, `SELECT name FROM categories ORDER BY sort_order, id LIMIT 1`); name != "Makanan & Minuman" {
		t.Fatalf("first category = %q", name)
	}
}

func TestProductPriceMustBePositive(t *testing.T) {
	pool := pgtest.Pool(t)
	sid := newStore(t, pool, newUser(t, pool, "+6281"))
	wantErr(t, pool, `INSERT INTO products (store_id, category_id, name, price, stock) VALUES ($1, 1, 'X', 0, 1)`, sid)
	wantErr(t, pool, `INSERT INTO products (store_id, category_id, name, price, stock) VALUES ($1, 1, 'X', -5, 1)`, sid)
}

func TestStockCannotGoNegative(t *testing.T) {
	pool := pgtest.Pool(t)
	sid := newStore(t, pool, newUser(t, pool, "+6281"))
	pid := newProduct(t, pool, sid)
	wantErr(t, pool, `UPDATE products SET stock = -1 WHERE id = $1`, pid)
}

func TestOneStorePerOwner(t *testing.T) {
	pool := pgtest.Pool(t)
	uid := newUser(t, pool, "+6281")
	newStore(t, pool, uid)
	wantErr(t, pool, `INSERT INTO stores (owner_id, name, category_id, owner_name, kelurahan, address, whatsapp)
		VALUES ($1, 'Lagi', 1, 'Budi', 'K', 'A', '+628')`, uid)
}

func TestReviewUniquePerOrderItem(t *testing.T) {
	pool := pgtest.Pool(t)
	buyer := newUser(t, pool, "+6281")
	sid := newStore(t, pool, newUser(t, pool, "+6282"))
	pid := newProduct(t, pool, sid)
	oid := scalar[int64](t, pool, `INSERT INTO orders (number, buyer_id, total, status) VALUES ('NM-20261029-001', $1, 10000, 'CREATED') RETURNING id`, buyer)
	soid := scalar[int64](t, pool, `INSERT INTO seller_orders (order_id, store_id, status, subtotal, store_name, pickup_address, opening_hours, whatsapp)
		VALUES ($1, $2, 'COMPLETED', 10000, 'Warung', 'Jl. A 1', '', '+628') RETURNING id`, oid, sid)
	iid := scalar[int64](t, pool, `INSERT INTO order_items (seller_order_id, product_id, name, image, quantity, unit_price, line_total)
		VALUES ($1, $2, 'Nasi', '{}', 1, 10000, 10000) RETURNING id`, soid, pid)
	q := `INSERT INTO reviews (order_item_id, product_id, store_id, buyer_id, rating, text) VALUES ($1, $2, $3, $4, $5, 'ok')`
	exec(t, pool, q, iid, pid, sid, buyer, 5)
	wantErr(t, pool, q, iid, pid, sid, buyer, 4)
}

func TestIdempotencyKeyUniquePerUser(t *testing.T) {
	pool := pgtest.Pool(t)
	u1 := newUser(t, pool, "+6281")
	u2 := newUser(t, pool, "+6282")
	oid := scalar[int64](t, pool, `INSERT INTO orders (number, buyer_id, total, status) VALUES ('NM-20261029-001', $1, 1, 'CREATED') RETURNING id`, u1)
	q := `INSERT INTO idempotency_keys (user_id, key, request_hash, order_id) VALUES ($1, 'k1', 'h', $2)`
	exec(t, pool, q, u1, oid)
	wantErr(t, pool, q, u1, oid)
	exec(t, pool, q, u2, oid)
}

func TestAdPlacementCheck(t *testing.T) {
	pool := pgtest.Pool(t)
	wantErr(t, pool, `INSERT INTO ad_campaigns (placement, advertiser, title, subtitle, image, target_url, label)
		VALUES ('cart_banner', 'X', 'T', 'S', '{}', 'https://x.id', 'Iklan')`)
}

func TestMigrationsDownThenUp(t *testing.T) {
	url := pgtest.URL(t)
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	var db *sql.DB = stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	t.Cleanup(func() { _ = goose.UpContext(ctx, db, ".") })
	if err := goose.DownToContext(ctx, db, ".", 1); err != nil {
		t.Fatalf("down to 1: %v", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		t.Fatalf("up again: %v", err)
	}
	p := mustPool(t, url)
	// Reset truncates ad_campaigns between tests, so the seed is asserted right after migrating up.
	if n := scalar[int](t, p, `SELECT count(DISTINCT placement) FROM ad_campaigns`); n != 4 {
		t.Fatalf("seeded placements = %d, want 4", n)
	}
	if n := scalar[int](t, p, `SELECT count(*) FROM ad_campaigns WHERE target_url NOT LIKE 'https://%'`); n != 0 {
		t.Fatalf("%d non-https campaigns", n)
	}
	if n := scalar[int](t, p, `SELECT count(*) FROM categories`); n != 9 {
		t.Fatalf("categories after up = %d, want 9", n)
	}
}

func mustPool(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}
