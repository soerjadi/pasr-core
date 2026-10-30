package ordering_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
)

type reviewable struct {
	buyer, store, product, item int64
}

// completedItem seeds a COMPLETED order item of a fresh store, bought by a buyer with the given name.
func completedItem(t *testing.T, f *ofx, buyerName string) reviewable {
	t.Helper()
	buyer := seed.User(t, f.pool, "", buyerName)
	store, _ := seed.Store(t, f.pool, seed.StoreOpts{})
	product := seed.Product(t, f.pool, seed.ProductOpts{StoreID: store})
	return reviewable{buyer, store, product, seed.OrderItem(t, f.pool, buyer, store, product)}
}

func (f *ofx) aggregates(t *testing.T, store, product int64) (storeSum, storeCount, prodSum, prodCount int) {
	t.Helper()
	ctx := context.Background()
	if err := f.pool.QueryRow(ctx, `SELECT rating_sum, rating_count FROM stores WHERE id = $1`, store).Scan(&storeSum, &storeCount); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT rating_sum, rating_count FROM products WHERE id = $1`, product).Scan(&prodSum, &prodCount); err != nil {
		t.Fatal(err)
	}
	return
}

func TestSubmitReview_CompletedItemCreatesReviewAndUpdatesAggregates(t *testing.T) {
	f := setupOrders(t)
	r := completedItem(t, f, "Suryadi")
	// a pre-existing rating so that the update adds rather than sets
	if _, err := f.pool.Exec(context.Background(), `UPDATE stores SET rating_sum = 8, rating_count = 2 WHERE id = $1`, r.store); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE products SET rating_sum = 3, rating_count = 1 WHERE id = $1`, r.product); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.SubmitReview(context.Background(), r.buyer, r.item, 4, "  Enak sekali  ")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == 0 || got.Rating != 4 || got.Text != "Enak sekali" || got.AuthorName != "Suryadi" || got.CreatedAt.IsZero() {
		t.Fatalf("review = %+v", got)
	}
	ss, sc, ps, pc := f.aggregates(t, r.store, r.product)
	if ss != 12 || sc != 3 || ps != 7 || pc != 2 {
		t.Fatalf("aggregates store %d/%d product %d/%d", ss, sc, ps, pc)
	}
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM reviews WHERE order_item_id = $1 AND buyer_id = $2 AND store_id = $3 AND product_id = $4`,
		r.item, r.buyer, r.store, r.product).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows %d %v", n, err)
	}
}

func TestSubmitReview_NotCompleted409(t *testing.T) {
	f := setupOrders(t)
	for _, status := range []string{"CREATED", "SELLER_PROCESSING", "READY_FOR_PICKUP", "CANCELLED"} {
		r := completedItem(t, f, "A")
		if _, err := f.pool.Exec(context.Background(), `UPDATE seller_orders SET status = $2 WHERE id = (SELECT seller_order_id FROM order_items WHERE id = $1)`, r.item, status); err != nil {
			t.Fatal(err)
		}
		_, err := f.svc.SubmitReview(context.Background(), r.buyer, r.item, 5, "")
		if !errors.Is(err, domain.ErrConflict) || codeOf(err) != "ORDER_NOT_COMPLETED" {
			t.Fatalf("%s: %v", status, err)
		}
		if _, sc, _, pc := f.aggregates(t, r.store, r.product); sc != 0 || pc != 0 {
			t.Fatalf("%s: aggregates changed", status)
		}
	}
}

func TestSubmitReview_Duplicate409(t *testing.T) {
	f := setupOrders(t)
	r := completedItem(t, f, "A")
	ctx := context.Background()
	if _, err := f.svc.SubmitReview(ctx, r.buyer, r.item, 5, "a"); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.SubmitReview(ctx, r.buyer, r.item, 1, "b")
	if !errors.Is(err, domain.ErrConflict) || codeOf(err) != "ALREADY_REVIEWED" {
		t.Fatalf("second: %v", err)
	}
	if ss, sc, ps, pc := f.aggregates(t, r.store, r.product); ss != 5 || sc != 1 || ps != 5 || pc != 1 {
		t.Fatalf("aggregates %d/%d %d/%d", ss, sc, ps, pc)
	}
}

func TestSubmitReview_ConcurrentDuplicate_OneCreated_AggregatesCountOnce(t *testing.T) {
	f := setupOrders(t)
	r := completedItem(t, f, "A")
	const n = 8
	var created, conflicts, other atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := f.svc.SubmitReview(context.Background(), r.buyer, r.item, 5, "x")
			switch {
			case err == nil:
				created.Add(1)
			case errors.Is(err, domain.ErrConflict) && codeOf(err) == "ALREADY_REVIEWED":
				conflicts.Add(1)
			default:
				t.Errorf("unexpected: %v", err)
				other.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if created.Load() != 1 || conflicts.Load() != n-1 {
		t.Fatalf("created %d conflicts %d other %d", created.Load(), conflicts.Load(), other.Load())
	}
	if got := f.count(t, `SELECT count(*) FROM reviews WHERE order_item_id = $1`, r.item); got != 1 {
		t.Fatalf("reviews = %d", got)
	}
	if ss, sc, ps, pc := f.aggregates(t, r.store, r.product); ss != 5 || sc != 1 || ps != 5 || pc != 1 {
		t.Fatalf("aggregates %d/%d %d/%d", ss, sc, ps, pc)
	}
}

func TestSubmitReview_OtherBuyersItem404(t *testing.T) {
	f := setupOrders(t)
	r := completedItem(t, f, "A")
	other := seed.User(t, f.pool, "", "B")
	for _, c := range []struct{ buyer, item int64 }{{other, r.item}, {r.buyer, r.item + 9999}} {
		_, err := f.svc.SubmitReview(context.Background(), c.buyer, c.item, 5, "")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("buyer %d item %d: %v", c.buyer, c.item, err)
		}
	}
	if got := f.count(t, `SELECT count(*) FROM reviews`); got != 0 {
		t.Fatalf("reviews = %d", got)
	}
}

func TestSubmitReview_RatingAndTextValidation(t *testing.T) {
	f := setupOrders(t)
	r := completedItem(t, f, "A")
	ctx := context.Background()
	for _, c := range []struct {
		name   string
		rating int32
		text   string
	}{
		{"zero", 0, ""}, {"six", 6, ""}, {"negative", -1, ""},
		{"501 ascii", 5, strings.Repeat("a", 501)},
		{"501 runes", 5, strings.Repeat("é", 501)},
		{"501 after trim is 500 plus", 5, " " + strings.Repeat("a", 500) + "b"},
	} {
		_, err := f.svc.SubmitReview(ctx, r.buyer, r.item, c.rating, c.text)
		if !errors.Is(err, domain.ErrValidationFailed) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if got := f.count(t, `SELECT count(*) FROM reviews`); got != 0 {
		t.Fatalf("reviews = %d", got)
	}
	// 500 multi-byte runes (1000 bytes) and padding that trims down to 500 are accepted.
	got, err := f.svc.SubmitReview(ctx, r.buyer, r.item, 5, "  "+strings.Repeat("é", 500)+"  ")
	if err != nil || len([]rune(got.Text)) != 500 {
		t.Fatalf("500 runes: %v", err)
	}
}

func TestSubmitReview_EmptyTextAllowed(t *testing.T) {
	f := setupOrders(t)
	r := completedItem(t, f, "A")
	got, err := f.svc.SubmitReview(context.Background(), r.buyer, r.item, 3, "   ")
	if err != nil || got.Text != "" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestOrderItem_ReviewedFlagTrueAfterReview(t *testing.T) {
	f := setupOrders(t)
	r := completedItem(t, f, "A")
	ctx := context.Background()
	var orderID int64
	if err := f.pool.QueryRow(ctx, `SELECT so.order_id FROM order_items oi JOIN seller_orders so ON so.id = oi.seller_order_id WHERE oi.id = $1`, r.item).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	flag := func() bool {
		o, err := f.svc.GetOrder(ctx, r.buyer, orderID)
		if err != nil {
			t.Fatal(err)
		}
		return o.SellerOrders[0].Items[0].Reviewed
	}
	if flag() {
		t.Fatal("reviewed before review")
	}
	if _, err := f.svc.SubmitReview(ctx, r.buyer, r.item, 5, ""); err != nil {
		t.Fatal(err)
	}
	if !flag() {
		t.Fatal("GetOrder: not reviewed after review")
	}
	page, err := f.svc.ListOrders(ctx, r.buyer, "ALL", "", 10)
	if err != nil || len(page.Items) != 1 || !page.Items[0].SellerOrders[0].Items[0].Reviewed {
		t.Fatalf("ListOrders: %+v %v", page, err)
	}
	owner := f.count(t, `SELECT owner_id FROM stores WHERE id = $1`, r.store)
	so, err := f.svc.ListStoreOrders(ctx, int64(owner), "DONE", "", 10)
	if err != nil || len(so.Items) != 1 || !so.Items[0].Items[0].Reviewed {
		t.Fatalf("ListStoreOrders: %+v %v", so, err)
	}
}

func TestSubmitReview_AuthorNameFallbackPembeli(t *testing.T) {
	f := setupOrders(t)
	for _, name := range []string{"", "   "} {
		r := completedItem(t, f, name)
		got, err := f.svc.SubmitReview(context.Background(), r.buyer, r.item, 5, "")
		if err != nil || got.AuthorName != "Pembeli" {
			t.Fatalf("name %q: %+v %v", name, got, err)
		}
	}
}
