package order_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

func reviewPath(item int64) string { return fmt.Sprintf("/v1/order-items/%d/reviews", item) }

func TestSubmitReview_HTTP(t *testing.T) {
	env, db := setup(t)
	sess := env.SignIn("081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	item := seed.OrderItem(t, db, sess.User.ID, store, p)
	auth := testapp.Bearer(sess.AccessToken)

	if r := env.Do(http.MethodPost, reviewPath(item), map[string]any{"rating": 5}); r.Status != http.StatusUnauthorized {
		t.Fatalf("no token: %d %s", r.Status, r.Body)
	}
	for name, body := range map[string]any{
		"rating 0": map[string]any{"rating": 0}, "rating 6": map[string]any{"rating": 6},
		"missing rating": map[string]any{"text": "x"},
		"long text":      map[string]any{"rating": 5, "text": strings.Repeat("a", 501)},
		"bad body":       "nope",
	} {
		if r := env.Do(http.MethodPost, reviewPath(item), body, auth); r.Status != http.StatusBadRequest || code(t, r) != "VALIDATION_FAILED" {
			t.Fatalf("%s: %d %s", name, r.Status, r.Body)
		}
	}
	for _, path := range []string{reviewPath(item + 9999), "/v1/order-items/abc/reviews"} {
		if r := env.Do(http.MethodPost, path, map[string]any{"rating": 5}, auth); r.Status != http.StatusNotFound {
			t.Fatalf("%s: %d %s", path, r.Status, r.Body)
		}
	}
	r := env.Do(http.MethodPost, reviewPath(item), map[string]any{"rating": 5, "text": "Mantap!"}, auth)
	if r.Status != http.StatusCreated {
		t.Fatalf("create: %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "submitReview", r.Body)
	r = env.Do(http.MethodPost, reviewPath(item), map[string]any{"rating": 5}, auth)
	if r.Status != http.StatusConflict || code(t, r) != "ALREADY_REVIEWED" {
		t.Fatalf("duplicate: %d %s", r.Status, r.Body)
	}
}

func TestSellerReviewsEndpoint_ShowsNewReviewAndAverage(t *testing.T) {
	env, db := setup(t)
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	var items []int64
	var tokens []string
	for i, phone := range []string{"081234500001", "081234500002"} {
		sess := env.SignIn(phone)
		if _, err := db.Exec(t.Context(), `UPDATE users SET name = $2 WHERE id = $1`, sess.User.ID, []string{"Agus", ""}[i]); err != nil {
			t.Fatal(err)
		}
		items = append(items, seed.OrderItem(t, db, sess.User.ID, store, p))
		tokens = append(tokens, sess.AccessToken)
	}
	for i, rating := range []int{5, 4} {
		r := env.Do(http.MethodPost, reviewPath(items[i]), map[string]any{"rating": rating, "text": fmt.Sprint("t", i)}, testapp.Bearer(tokens[i]))
		if r.Status != http.StatusCreated {
			t.Fatalf("submit %d: %d %s", i, r.Status, r.Body)
		}
	}

	path := fmt.Sprintf("/v1/sellers/%d", store)
	r := env.Do(http.MethodGet, path+"/reviews", nil)
	contract.AssertShape(t, "listSellerReviews", r.Body)
	var page domain.Page[domain.Review]
	r.JSON(&page)
	if len(page.Items) != 2 || page.Items[0].AuthorName != "Pembeli" || page.Items[0].Rating != 4 ||
		page.Items[1].AuthorName != "Agus" || page.Items[1].Text != "t0" {
		t.Fatalf("reviews: %s", r.Body)
	}
	var got struct {
		Seller struct {
			Rating      float64 `json:"rating"`
			RatingCount int     `json:"ratingCount"`
		} `json:"seller"`
	}
	env.Do(http.MethodGet, path, nil).JSON(&got)
	if got.Seller.Rating != 4.5 || got.Seller.RatingCount != 2 {
		t.Fatalf("seller = %+v", got.Seller)
	}
}

func TestOrderItem_ReviewedFlagTrueAfterReview_HTTP(t *testing.T) {
	env, db := setup(t)
	sess := env.SignIn("081234500001")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	item := seed.OrderItem(t, db, sess.User.ID, store, p)
	auth := testapp.Bearer(sess.AccessToken)
	var orderID int64
	if err := db.QueryRow(t.Context(), `SELECT so.order_id FROM seller_orders so JOIN order_items oi ON oi.seller_order_id = so.id WHERE oi.id = $1`, item).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	reviewed := func(path string, list bool) bool {
		r := env.Do(http.MethodGet, path, nil, auth)
		if list {
			var pg domain.Page[domain.Order]
			r.JSON(&pg)
			return pg.Items[0].SellerOrders[0].Items[0].Reviewed
		}
		return decodeOrder(t, r).SellerOrders[0].Items[0].Reviewed
	}
	single := fmt.Sprintf("/v1/orders/%d", orderID)
	if reviewed(single, false) || reviewed("/v1/orders", true) {
		t.Fatal("reviewed before review")
	}
	if r := env.Do(http.MethodPost, reviewPath(item), map[string]any{"rating": 3}, auth); r.Status != http.StatusCreated {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	if !reviewed(single, false) || !reviewed("/v1/orders", true) {
		t.Fatal("not reviewed after review")
	}
}
