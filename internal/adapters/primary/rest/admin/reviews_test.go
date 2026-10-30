package admin_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

type aggregates struct{ productSum, productCount, storeSum, storeCount int64 }

func (f *fx) aggregates(t *testing.T, product, store int64) aggregates {
	t.Helper()
	var a aggregates
	err := f.db.QueryRow(context.Background(), `
		SELECT p.rating_sum, p.rating_count, s.rating_sum, s.rating_count
		FROM products p, stores s WHERE p.id = $1 AND s.id = $2`, product, store).
		Scan(&a.productSum, &a.productCount, &a.storeSum, &a.storeCount)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestDeleteReview_RecomputesAggregates(t *testing.T) {
	f := setup(t)
	// The store also has a review of another product, so its aggregate differs from the product's.
	store, _ := seed.Store(t, f.db, seed.StoreOpts{RatingSum: 11, RatingCount: 3})
	p := seed.Product(t, f.db, seed.ProductOpts{StoreID: store, RatingSum: 7, RatingCount: 2})
	other := seed.Product(t, f.db, seed.ProductOpts{StoreID: store, RatingSum: 4, RatingCount: 1})
	seed.Review(t, f.db, seed.ReviewOpts{StoreID: store, ProductID: p, Rating: 5, Text: "Enak"})
	bad := seed.Review(t, f.db, seed.ReviewOpts{StoreID: store, ProductID: p, Rating: 2, Text: "spam"})
	seed.Review(t, f.db, seed.ReviewOpts{StoreID: store, ProductID: other, Rating: 4})

	assertStatus(t, f.do(http.MethodDelete, path("/v1/admin/reviews/%d", bad), nil), http.StatusNoContent)
	if got := f.aggregates(t, p, store); got != (aggregates{productSum: 5, productCount: 1, storeSum: 9, storeCount: 2}) {
		t.Fatalf("aggregates: %+v", got)
	}
	if n := f.scalar(t, `SELECT count(*) FROM reviews WHERE id = $1`, bad); n != 0 {
		t.Fatal("review not deleted")
	}

	r := f.env.Do(http.MethodGet, path("/v1/products/%d", p), nil)
	var d domain.ProductDetail
	r.JSON(&d)
	if d.Product.Rating != 5 || d.Product.RatingCount != 1 {
		t.Fatalf("public product rating: %+v", d.Product)
	}
	r = f.env.Do(http.MethodGet, path("/v1/sellers/%d/reviews", store), nil)
	var page domain.Page[domain.Review]
	r.JSON(&page)
	if len(page.Items) != 2 {
		t.Fatalf("seller reviews: %+v", page.Items)
	}
	for _, rv := range page.Items {
		if rv.ID == bad {
			t.Fatalf("deleted review still listed: %+v", rv)
		}
	}

	assertErr(t, f.do(http.MethodDelete, path("/v1/admin/reviews/%d", bad), nil), http.StatusNotFound, "NOT_FOUND")
	assertErr(t, f.do(http.MethodDelete, "/v1/admin/reviews/999999", nil), http.StatusNotFound, "NOT_FOUND")
	if got := f.aggregates(t, p, store); got.productCount != 1 || got.storeCount != 2 {
		t.Fatalf("second delete changed aggregates: %+v", got)
	}
}

func TestDeleteReview_AggregatesNeverGoNegative(t *testing.T) {
	f := setup(t)
	// Aggregates that are already out of step with the reviews (here: empty) must not go below zero.
	store, _ := seed.Store(t, f.db, seed.StoreOpts{})
	p := seed.Product(t, f.db, seed.ProductOpts{StoreID: store})
	rv := seed.Review(t, f.db, seed.ReviewOpts{StoreID: store, ProductID: p, Rating: 4})

	assertStatus(t, f.do(http.MethodDelete, path("/v1/admin/reviews/%d", rv), nil), http.StatusNoContent)
	if got := f.aggregates(t, p, store); got != (aggregates{}) {
		t.Fatalf("aggregates: %+v", got)
	}
}
