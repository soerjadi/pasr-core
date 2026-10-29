package product

import (
	"context"
	"strings"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

// The keyset list must walk an index in id DESC order for each filter instead of sorting.
// Sequential scans are disabled so the plan shows whether a usable index exists at all.
func TestBuildListQuery_FiltersHaveOrderedIndexes(t *testing.T) {
	pool := pgtest.Pool(t)
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	seed.Product(t, pool, seed.ProductOpts{StoreID: store})

	cases := []struct {
		name  string
		query domain.ProductQuery
		index string
	}{
		{"category", domain.ProductQuery{CategoryID: 1, Limit: 21}, "products_category_id_idx"},
		{"seller", domain.ProductQuery{SellerID: store, Limit: 21}, "products_store_id_idx"},
		{"featured", domain.ProductQuery{Featured: true, Limit: 21}, "products_featured_id_idx"},
	}
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SET enable_seqscan = off; SET enable_bitmapscan = off`); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		sql, args := buildListQuery(c.query)
		rows, err := conn.Query(ctx, "EXPLAIN "+sql, args...)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
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
		if !strings.Contains(plan.String(), c.index) || strings.Contains(plan.String(), "Sort") {
			t.Errorf("%s: want %s without Sort, plan:\n%s", c.name, c.index, plan.String())
		}
	}
}

// The seller's own list walks (store_id, id DESC) with and without a status filter.
func TestBuildListOwnedQuery_UsesStoreIndex(t *testing.T) {
	pool := pgtest.Pool(t)
	store, _ := seed.Store(t, pool, seed.StoreOpts{})
	seed.Product(t, pool, seed.ProductOpts{StoreID: store})

	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SET enable_seqscan = off; SET enable_bitmapscan = off`); err != nil {
		t.Fatal(err)
	}
	for _, status := range []domain.ProductStatus{"", domain.ProductHidden} {
		sql, args := buildListOwnedQuery(domain.SellerProductQuery{StoreID: store, Status: status, BeforeID: 100, Limit: 21})
		rows, err := conn.Query(ctx, "EXPLAIN "+sql, args...)
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
		if !strings.Contains(plan.String(), "products_store_id_idx") || strings.Contains(plan.String(), "Sort") {
			t.Errorf("status %q: plan:\n%s", status, plan.String())
		}
	}
}
