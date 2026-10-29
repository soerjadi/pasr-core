package catalog_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

const phone = "+6281234567890"

func setup(t *testing.T) (*testapp.Env, *pgxpool.Pool) {
	t.Helper()
	env := testapp.New(t)
	return env, pgtest.Pool(t) // after New: both reset the shared DB, so no data exists yet
}

func auth(s domain.Session) testapp.Opt { return testapp.Bearer(s.AccessToken) }

type errBody struct {
	Code string `json:"code"`
}

func assertErr(t *testing.T, r *testapp.Resp, status int, code string) {
	t.Helper()
	var e errBody
	r.JSON(&e)
	if r.Status != status || e.Code != code {
		t.Fatalf("got %d %s, want %d %s", r.Status, r.Body, status, code)
	}
}

func list(t *testing.T, env *testapp.Env, query string) domain.Page[domain.ProductSummary] {
	t.Helper()
	r := env.Do(http.MethodGet, "/v1/products"+query, nil)
	if r.Status != http.StatusOK {
		t.Fatalf("list %s: %d %s", query, r.Status, r.Body)
	}
	var p domain.Page[domain.ProductSummary]
	r.JSON(&p)
	return p
}

func ids(items []domain.ProductSummary) []int64 {
	out := make([]int64, len(items))
	for i, p := range items {
		out[i] = p.ID
	}
	return out
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestListProducts_OnlyVerifiedStoresVisibleProducts(t *testing.T) {
	env, db := setup(t)
	verified, _ := seed.Store(t, db, seed.StoreOpts{})
	pending, _ := seed.Store(t, db, seed.StoreOpts{Status: "PENDING"})
	suspended, _ := seed.Store(t, db, seed.StoreOpts{Status: "SUSPENDED"})
	want := seed.Product(t, db, seed.ProductOpts{StoreID: verified, Images: 1, HalalVerified: true, RatingSum: 24, RatingCount: 5})
	seed.Product(t, db, seed.ProductOpts{StoreID: pending, Images: 1})
	seed.Product(t, db, seed.ProductOpts{StoreID: suspended, Images: 1})

	r := env.Do(http.MethodGet, "/v1/products", nil)
	contract.AssertShape(t, "listProducts", r.Body)
	var p domain.Page[domain.ProductSummary]
	r.JSON(&p)
	if !equalIDs(ids(p.Items), []int64{want}) || p.NextCursor != nil {
		t.Fatalf("got %+v", p)
	}
	got := p.Items[0]
	if got.Rating != 4.8 || got.RatingCount != 5 || !got.HalalVerified || got.Image.Small != "http://media.test/seed/product-1-1-small.jpg" {
		t.Fatalf("unexpected summary %+v", got)
	}
}

func TestListProducts_HiddenAndDeletedExcluded(t *testing.T) {
	env, db := setup(t)
	s, _ := seed.Store(t, db, seed.StoreOpts{})
	seed.Product(t, db, seed.ProductOpts{StoreID: s, Status: "HIDDEN"})
	seed.Product(t, db, seed.ProductOpts{StoreID: s, Deleted: true})
	ok := seed.Product(t, db, seed.ProductOpts{StoreID: s})
	if got := ids(list(t, env, "").Items); !equalIDs(got, []int64{ok}) {
		t.Fatalf("got %v, want [%d]", got, ok)
	}
}

func TestListProducts_OutOfStockStillListed(t *testing.T) {
	env, db := setup(t)
	s, _ := seed.Store(t, db, seed.StoreOpts{})
	id := seed.Product(t, db, seed.ProductOpts{StoreID: s, Status: "OUT_OF_STOCK"})
	p := list(t, env, "")
	if !equalIDs(ids(p.Items), []int64{id}) || p.Items[0].Stock != 0 {
		t.Fatalf("got %+v", p.Items)
	}
}

func TestListProducts_FilterByCategorySellerFeatured(t *testing.T) {
	env, db := setup(t)
	a, _ := seed.Store(t, db, seed.StoreOpts{})
	b, _ := seed.Store(t, db, seed.StoreOpts{})
	p1 := seed.Product(t, db, seed.ProductOpts{StoreID: a, CategoryID: 1})
	p2 := seed.Product(t, db, seed.ProductOpts{StoreID: a, CategoryID: 2, Featured: true})
	p3 := seed.Product(t, db, seed.ProductOpts{StoreID: b, CategoryID: 2})

	cases := []struct {
		query string
		want  []int64
	}{
		{"?categoryId=2", []int64{p3, p2}},
		{fmt.Sprintf("?sellerId=%d", a), []int64{p2, p1}},
		{"?featured=true", []int64{p2}},
		{fmt.Sprintf("?categoryId=2&sellerId=%d", b), []int64{p3}},
		{"?featured=false", []int64{p3, p2, p1}},
	}
	for _, c := range cases {
		if got := ids(list(t, env, c.query).Items); !equalIDs(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.query, got, c.want)
		}
	}
	assertErr(t, env.Do(http.MethodGet, "/v1/products?categoryId=abc", nil), 400, "VALIDATION_FAILED")
}

func TestListProducts_SearchCaseInsensitiveAndEscapesWildcards(t *testing.T) {
	env, db := setup(t)
	s, _ := seed.Store(t, db, seed.StoreOpts{})
	tempe := seed.Product(t, db, seed.ProductOpts{StoreID: s, Name: "Keripik Tempe"})
	percent := seed.Product(t, db, seed.ProductOpts{StoreID: s, Name: "Diskon 50% Madu"})
	under := seed.Product(t, db, seed.ProductOpts{StoreID: s, Name: "Kue_Kering"})
	seed.Product(t, db, seed.ProductOpts{StoreID: s, Name: "Kue Kering"})
	slash := seed.Product(t, db, seed.ProductOpts{StoreID: s, Name: `Es\Kopi`})

	cases := map[string][]int64{
		"TEMPE":  {tempe},
		"tEmPe":  {tempe},
		"50%":    {percent},
		"e_k":    {under},
		"s\\k":   {slash},
		"%%":     nil,
		"__":     nil,
		"ue_ke":  {under},
		"zzzzzz": nil,
	}
	for q, want := range cases {
		got := ids(list(t, env, "?q="+url.QueryEscape(q)).Items)
		if !equalIDs(got, want) {
			t.Errorf("q=%q: got %v, want %v", q, got, want)
		}
	}
}

func TestListProducts_ShortQuery400(t *testing.T) {
	env, _ := setup(t)
	for _, q := range []string{"a", "%", " a "} {
		assertErr(t, env.Do(http.MethodGet, "/v1/products?q="+url.QueryEscape(q), nil), 400, "VALIDATION_FAILED")
	}
	if r := env.Do(http.MethodGet, "/v1/products?q=", nil); r.Status != http.StatusOK {
		t.Fatalf("empty q should list: %d %s", r.Status, r.Body)
	}
}

func TestListProducts_CursorPagesWithoutOverlapOrGapWhileInserting(t *testing.T) {
	env, db := setup(t)
	s, _ := seed.Store(t, db, seed.StoreOpts{})
	var all []int64
	for i := 0; i < 5; i++ {
		all = append([]int64{seed.Product(t, db, seed.ProductOpts{StoreID: s, Images: 1})}, all...)
	}

	first := list(t, env, "?limit=2")
	if first.NextCursor == nil || !equalIDs(ids(first.Items), all[:2]) {
		t.Fatalf("page 1: %+v", first)
	}
	// New products arrive between page requests; keyset pagination must not shift page 2.
	seed.Product(t, db, seed.ProductOpts{StoreID: s})
	seed.Product(t, db, seed.ProductOpts{StoreID: s})

	second := list(t, env, "?limit=2&cursor="+url.QueryEscape(*first.NextCursor))
	if second.NextCursor == nil || !equalIDs(ids(second.Items), all[2:4]) {
		t.Fatalf("page 2: got %v, want %v", ids(second.Items), all[2:4])
	}
	third := list(t, env, "?limit=2&cursor="+url.QueryEscape(*second.NextCursor))
	if third.NextCursor != nil || !equalIDs(ids(third.Items), all[4:]) {
		t.Fatalf("page 3: %+v", third)
	}
	assertErr(t, env.Do(http.MethodGet, "/v1/products?cursor=%21%21", nil), 400, "VALIDATION_FAILED")
}

func TestListProducts_LimitCappedAt50(t *testing.T) {
	env, db := setup(t)
	s, _ := seed.Store(t, db, seed.StoreOpts{})
	for i := 0; i < 55; i++ {
		seed.Product(t, db, seed.ProductOpts{StoreID: s})
	}
	p := list(t, env, "?limit=500")
	if len(p.Items) != 50 || p.NextCursor == nil {
		t.Fatalf("got %d items, cursor %v", len(p.Items), p.NextCursor)
	}
	if def := list(t, env, ""); len(def.Items) != 20 {
		t.Fatalf("default limit: got %d", len(def.Items))
	}
}

func TestGetProduct_AnonymousIsFavoriteNull(t *testing.T) {
	env, db := setup(t)
	s, _ := seed.Store(t, db, seed.StoreOpts{Photo: true, RatingSum: 9, RatingCount: 2, Hours: sevenDays()})
	id := seed.Product(t, db, seed.ProductOpts{StoreID: s, Images: 3, Description: "Enak"})

	r := env.Do(http.MethodGet, "/v1/products/"+strconv.FormatInt(id, 10), nil)
	if r.Status != http.StatusOK {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
	var d struct {
		domain.ProductDetail
		IsFavorite any `json:"isFavorite"`
	}
	r.JSON(&d)
	if d.IsFavorite != nil || len(d.Images) != 3 || d.Description != "Enak" || d.Seller.OpeningHours != "Setiap hari 07.00–15.00" || d.Seller.Rating != 4.5 {
		t.Fatalf("unexpected %s", r.Body)
	}

	if bad := env.Do(http.MethodGet, "/v1/products/"+strconv.FormatInt(id, 10), nil, testapp.Bearer("garbage")); bad.Status != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", bad.Status)
	}
}

func TestGetProduct_AuthenticatedIsFavoriteTrue(t *testing.T) {
	env, db := setup(t)
	s, _ := seed.Store(t, db, seed.StoreOpts{Photo: true})
	fav := seed.Product(t, db, seed.ProductOpts{StoreID: s, Images: 3})
	plain := seed.Product(t, db, seed.ProductOpts{StoreID: s, Images: 1})
	sess := env.SignIn(phone)
	var me struct {
		ID int64 `json:"id"`
	}
	env.Do(http.MethodGet, "/v1/me", nil, auth(sess)).JSON(&me)
	seed.Favorite(t, db, me.ID, fav)

	r := env.Do(http.MethodGet, "/v1/products/"+strconv.FormatInt(fav, 10), nil, auth(sess))
	contract.AssertShape(t, "getProduct", r.Body)
	var d domain.ProductDetail
	r.JSON(&d)
	if d.IsFavorite == nil || !*d.IsFavorite {
		t.Fatalf("want favorite true: %s", r.Body)
	}
	r = env.Do(http.MethodGet, "/v1/products/"+strconv.FormatInt(plain, 10), nil, auth(sess))
	r.JSON(&d)
	if d.IsFavorite == nil || *d.IsFavorite {
		t.Fatalf("want favorite false: %s", r.Body)
	}
}

func TestGetProduct_HiddenOrPendingStore404(t *testing.T) {
	env, db := setup(t)
	ok, _ := seed.Store(t, db, seed.StoreOpts{})
	pending, _ := seed.Store(t, db, seed.StoreOpts{Status: "PENDING"})
	for _, id := range map[string]int64{
		"hidden":  seed.Product(t, db, seed.ProductOpts{StoreID: ok, Status: "HIDDEN"}),
		"deleted": seed.Product(t, db, seed.ProductOpts{StoreID: ok, Deleted: true}),
		"pending": seed.Product(t, db, seed.ProductOpts{StoreID: pending}),
		"missing": 99999,
	} {
		assertErr(t, env.Do(http.MethodGet, "/v1/products/"+strconv.FormatInt(id, 10), nil), 404, "NOT_FOUND")
	}
	assertErr(t, env.Do(http.MethodGet, "/v1/products/abc", nil), 404, "NOT_FOUND")
}

func TestGetSeller_FollowerCountAndIsFollowing(t *testing.T) {
	env, db := setup(t)
	s, _ := seed.Store(t, db, seed.StoreOpts{Photo: true, Description: "Toko enak", RatingSum: 14, RatingCount: 3})
	sess := env.SignIn(phone)
	var me struct {
		ID int64 `json:"id"`
	}
	env.Do(http.MethodGet, "/v1/me", nil, auth(sess)).JSON(&me)
	seed.Follow(t, db, me.ID, s)
	seed.Follow(t, db, seed.User(t, db, "", "Lain"), s)
	path := "/v1/sellers/" + strconv.FormatInt(s, 10)

	anon := env.Do(http.MethodGet, path, nil)
	var d domain.SellerDetail
	anon.JSON(&d)
	if anon.Status != 200 || d.FollowerCount != 2 || d.IsFollowing != nil || d.Seller.Rating != 4.7 || d.Seller.OpeningHours != "Belum diatur" || d.Seller.Avatar == nil {
		t.Fatalf("anonymous: %s", anon.Body)
	}

	r := env.Do(http.MethodGet, path, nil, auth(sess))
	contract.AssertShape(t, "getSeller", r.Body)
	d = domain.SellerDetail{}
	r.JSON(&d)
	if d.IsFollowing == nil || !*d.IsFollowing || d.Seller.Location != "Menteng" {
		t.Fatalf("follower: %s", r.Body)
	}
}

func TestGetSeller_PendingAndSuspended404(t *testing.T) {
	env, db := setup(t)
	pending, _ := seed.Store(t, db, seed.StoreOpts{Status: "PENDING"})
	suspended, _ := seed.Store(t, db, seed.StoreOpts{Status: "SUSPENDED"})
	for _, id := range []int64{pending, suspended, 99999} {
		p := "/v1/sellers/" + strconv.FormatInt(id, 10)
		assertErr(t, env.Do(http.MethodGet, p, nil), 404, "NOT_FOUND")
		assertErr(t, env.Do(http.MethodGet, p+"/reviews", nil), 404, "NOT_FOUND")
	}
}

func TestListSellerReviews_NewestFirst(t *testing.T) {
	env, db := setup(t)
	s, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: s})
	agus := seed.User(t, db, "", "Agus")
	base := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	r1 := seed.Review(t, db, seed.ReviewOpts{BuyerID: agus, StoreID: s, ProductID: p, Rating: 5, Text: "Enak", CreatedAt: base})
	r2 := seed.Review(t, db, seed.ReviewOpts{StoreID: s, ProductID: p, Rating: 4, CreatedAt: base.Add(time.Hour)})
	r3 := seed.Review(t, db, seed.ReviewOpts{BuyerID: agus, StoreID: s, ProductID: p, Rating: 3, CreatedAt: base.Add(2 * time.Hour)})
	other, _ := seed.Store(t, db, seed.StoreOpts{})
	op := seed.Product(t, db, seed.ProductOpts{StoreID: other})
	seed.Review(t, db, seed.ReviewOpts{StoreID: other, ProductID: op})

	path := "/v1/sellers/" + strconv.FormatInt(s, 10) + "/reviews"
	r := env.Do(http.MethodGet, path, nil)
	contract.AssertShape(t, "listSellerReviews", r.Body)
	var page domain.Page[domain.Review]
	r.JSON(&page)
	if len(page.Items) != 3 || page.Items[0].ID != r3 || page.Items[1].ID != r2 || page.Items[2].ID != r1 || page.NextCursor != nil {
		t.Fatalf("order: %s", r.Body)
	}
	if page.Items[1].AuthorName != "Pembeli" || page.Items[2].AuthorName != "Agus" || page.Items[2].Text != "Enak" ||
		!page.Items[2].CreatedAt.Equal(base) {
		t.Fatalf("fields: %s", r.Body)
	}

	r = env.Do(http.MethodGet, path+"?limit=2", nil)
	r.JSON(&page)
	if len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatalf("page 1: %s", r.Body)
	}
	r = env.Do(http.MethodGet, path+"?limit=2&cursor="+url.QueryEscape(*page.NextCursor), nil)
	r.JSON(&page)
	if len(page.Items) != 1 || page.Items[0].ID != r1 || page.NextCursor != nil {
		t.Fatalf("page 2: %s", r.Body)
	}
}

func sevenDays() domain.Hours {
	names := []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}
	h := make(domain.Hours, 7)
	for i, n := range names {
		h[i] = domain.DayHours{Day: n, Open: "07:00", Close: "15:00"}
	}
	return h
}
