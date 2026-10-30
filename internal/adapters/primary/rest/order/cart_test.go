package order_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

func setup(t *testing.T) (*testapp.Env, *pgxpool.Pool) {
	t.Helper()
	env := testapp.New(t)
	return env, pgtest.Pool(t)
}

func validate(env *testapp.Env, body any, opts ...testapp.Opt) (*testapp.Resp, domain.CartValidation) {
	r := env.Do("POST", "/v1/cart/validate", body, opts...)
	var v domain.CartValidation
	if r.Status == http.StatusOK {
		r.JSON(&v)
	}
	return r, v
}

type line = map[string]any

func TestValidateCart_Anonymous200(t *testing.T) {
	env, db := setup(t)
	store, _ := seed.Store(t, db, seed.StoreOpts{Name: "Bu Siti", Photo: true})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Name: "Tempe", Price: 15000, Stock: 25, Images: 1})

	r, v := validate(env, line{"lines": []line{{"productId": p, "quantity": 2}}})
	if r.Status != http.StatusOK {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "validateCart", r.Body)
	if v.Total != 30000 || v.Groups[0].Seller.Name != "Bu Siti" || v.Groups[0].Lines[0].AvailableStock != 25 {
		t.Fatalf("got %+v", v)
	}
}

func TestValidateCart_PricesFromServerNotClient(t *testing.T) {
	env, db := setup(t)
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store, Price: 15000, Stock: 5})

	r, v := validate(env, line{"lines": []line{{"productId": p, "quantity": 2, "unitPrice": 1, "lineTotal": 2}}, "total": 3})
	if r.Status != http.StatusOK {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	if l := v.Groups[0].Lines[0]; l.UnitPrice != 15000 || l.LineTotal != 30000 || v.Total != 30000 {
		t.Fatalf("got %+v total %d", l, v.Total)
	}
}

func TestValidateCart_GroupsBySellerWithPickupInfoAndIssues(t *testing.T) {
	env, db := setup(t)
	a, _ := seed.Store(t, db, seed.StoreOpts{Name: "A", Photo: true, Address: "Jl. A 1", WhatsApp: "+6281111111111", Hours: allDays("07:00", "17:00")})
	b, _ := seed.Store(t, db, seed.StoreOpts{Name: "B"})
	closed, _ := seed.Store(t, db, seed.StoreOpts{Name: "C"})
	pending, _ := seed.Store(t, db, seed.StoreOpts{Name: "D", Status: "PENDING"})
	if _, err := db.Exec(context.Background(), `UPDATE stores SET is_open = false WHERE id = $1`, closed); err != nil {
		t.Fatal(err)
	}
	pa := seed.Product(t, db, seed.ProductOpts{StoreID: a, Price: 1000, Stock: 5})
	pb := seed.Product(t, db, seed.ProductOpts{StoreID: b, Price: 2000, Stock: 2})
	pa2 := seed.Product(t, db, seed.ProductOpts{StoreID: a, Price: 500, Stock: 5})
	pclosed := seed.Product(t, db, seed.ProductOpts{StoreID: closed})
	ppending := seed.Product(t, db, seed.ProductOpts{StoreID: pending})
	hidden := seed.Product(t, db, seed.ProductOpts{StoreID: a, Status: "HIDDEN"})
	deleted := seed.Product(t, db, seed.ProductOpts{StoreID: a, Deleted: true})
	oos := seed.Product(t, db, seed.ProductOpts{StoreID: a, Status: "OUT_OF_STOCK"})

	r, v := validate(env, line{"lines": []line{
		{"productId": pa, "quantity": 1}, {"productId": pb, "quantity": 3}, {"productId": pa2, "quantity": 2},
		{"productId": pclosed, "quantity": 1}, {"productId": ppending, "quantity": 1},
		{"productId": hidden, "quantity": 1}, {"productId": deleted, "quantity": 1}, {"productId": oos, "quantity": 1},
		{"productId": 987654, "quantity": 1}, {"productId": pa, "quantity": 1},
	}})
	if r.Status != http.StatusOK {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "validateCart", r.Body)
	// groups: A, B, closed C, then the bare pseudo-group (seller 0) for pending/hidden/deleted/missing
	if len(v.Groups) != 4 || v.Groups[0].Seller.ID != a || v.Groups[1].Seller.ID != b ||
		v.Groups[2].Seller.ID != closed || v.Groups[3].Seller.ID != 0 {
		t.Fatalf("groups: %s", r.Body)
	}
	s := v.Groups[0].Seller
	if s.PickupAddress != "Jl. A 1" || s.WhatsApp != "+6281111111111" || s.OpeningHours != "Setiap hari 07.00–17.00" {
		t.Fatalf("seller = %+v", s)
	}
	issue := func(l domain.ValidatedLine) string {
		if l.Issue == nil {
			return ""
		}
		return string(*l.Issue)
	}
	la := v.Groups[0].Lines // pa (merged qty 2), pa2, oos
	if len(la) != 3 || la[0].Quantity != 2 || issue(la[0]) != "" || issue(la[1]) != "" || issue(la[2]) != "OUT_OF_STOCK" {
		t.Fatalf("store A lines: %s", r.Body)
	}
	if issue(v.Groups[1].Lines[0]) != "INSUFFICIENT_STOCK" || v.Groups[1].Lines[0].AvailableStock != 2 {
		t.Errorf("B line = %+v", v.Groups[1].Lines[0])
	}
	// a closed but verified store is public: details stay, the issue is PRODUCT_UNAVAILABLE
	cl := v.Groups[2]
	if cl.Seller.Name != "C" || cl.Seller.PickupAddress == "" || cl.Lines[0].Name == "" || cl.Lines[0].UnitPrice == 0 ||
		issue(cl.Lines[0]) != "PRODUCT_UNAVAILABLE" {
		t.Errorf("closed store: %+v", cl)
	}
	bare := v.Groups[3]
	if len(bare.Lines) != 4 || bare.Seller != (domain.SellerSummary{}) {
		t.Fatalf("bare group: %s", r.Body)
	}
	for _, l := range bare.Lines {
		if issue(l) != "PRODUCT_UNAVAILABLE" || l.Name != "" || l.UnitPrice != 0 || l.LineTotal != 0 ||
			l.AvailableStock != 0 || l.Image != (domain.ImageSet{}) {
			t.Errorf("leaked: %+v", l)
		}
	}
	if v.Total != 2*1000+2*500 { // pa x2 + pa2 x2; everything else has an issue
		t.Errorf("total = %d", v.Total)
	}
}

func allDays(open, closeAt string) domain.Hours {
	days := []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu", "Minggu"}
	h := make(domain.Hours, 7)
	for i, d := range days {
		h[i] = domain.DayHours{Day: d, Open: open, Close: closeAt}
	}
	return h
}

func TestValidateCart_Over50Lines400(t *testing.T) {
	env, _ := setup(t)
	mk := func(n int) []line {
		ls := make([]line, n)
		for i := range ls {
			ls[i] = line{"productId": i + 1, "quantity": 1}
		}
		return ls
	}
	if r, _ := validate(env, line{"lines": mk(51)}); r.Status != http.StatusBadRequest {
		t.Fatalf("51 lines: %d %s", r.Status, r.Body)
	}
	if r, _ := validate(env, line{"lines": mk(50)}); r.Status != http.StatusOK {
		t.Fatalf("50 lines: %d %s", r.Status, r.Body)
	}
}

func TestValidateCart_ZeroOrNegativeQuantity400(t *testing.T) {
	env, _ := setup(t)
	for _, q := range []int{0, -3} {
		r, _ := validate(env, line{"lines": []line{{"productId": 1, "quantity": q}}})
		if r.Status != http.StatusBadRequest || !strings.Contains(string(r.Body), "VALIDATION_FAILED") {
			t.Fatalf("qty %d: %d %s", q, r.Status, r.Body)
		}
	}
}

func TestValidateCart_EmptyOrMalformedBody400(t *testing.T) {
	env, _ := setup(t)
	for _, body := range []any{line{"lines": []line{}}, line{}, "nope"} {
		if r, _ := validate(env, body); r.Status != http.StatusBadRequest {
			t.Fatalf("%v: %d %s", body, r.Status, r.Body)
		}
	}
}
