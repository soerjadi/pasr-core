package social_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

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

func assertErr(t *testing.T, r *testapp.Resp, status int, code string) {
	t.Helper()
	var e struct {
		Code string `json:"code"`
	}
	r.JSON(&e)
	if r.Status != status || e.Code != code {
		t.Fatalf("got %d %s, want %d %s", r.Status, r.Body, status, code)
	}
}

func assert204(t *testing.T, r *testapp.Resp) {
	t.Helper()
	if r.Status != http.StatusNoContent {
		t.Fatalf("got %d %s, want 204", r.Status, r.Body)
	}
}

func favPath(id int64) string { return fmt.Sprintf("/v1/me/favorites/%d", id) }
func followPath(id int64) string {
	return fmt.Sprintf("/v1/me/following/%d", id)
}

func favCount(t *testing.T, db *pgxpool.Pool, userID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM favorites WHERE user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func listFavs(t *testing.T, env *testapp.Env, s domain.Session, query string) domain.Page[domain.ProductSummary] {
	t.Helper()
	r := env.Do(http.MethodGet, "/v1/me/favorites"+query, nil, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("list: %d %s", r.Status, r.Body)
	}
	var p domain.Page[domain.ProductSummary]
	r.JSON(&p)
	return p
}

func TestAddFavorite_Idempotent(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})

	assert204(t, env.Do(http.MethodPut, favPath(p), nil, auth(s)))
	assert204(t, env.Do(http.MethodPut, favPath(p), nil, auth(s)))
	if n := favCount(t, db, s.User.ID); n != 1 {
		t.Fatalf("favorites = %d, want 1", n)
	}
}

func TestAddFavorite_HiddenProduct404(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	pending, _ := seed.Store(t, db, seed.StoreOpts{Status: "PENDING"})
	for name, id := range map[string]int64{
		"hidden":  seed.Product(t, db, seed.ProductOpts{StoreID: store, Status: "HIDDEN"}),
		"deleted": seed.Product(t, db, seed.ProductOpts{StoreID: store, Deleted: true}),
		"pending": seed.Product(t, db, seed.ProductOpts{StoreID: pending}),
		"missing": 999999,
	} {
		t.Run(name, func(t *testing.T) {
			assertErr(t, env.Do(http.MethodPut, favPath(id), nil, auth(s)), http.StatusNotFound, "NOT_FOUND")
		})
	}
	assertErr(t, env.Do(http.MethodPut, "/v1/me/favorites/abc", nil, auth(s)), http.StatusNotFound, "NOT_FOUND")
	if n := favCount(t, db, s.User.ID); n != 0 {
		t.Fatalf("favorites = %d, want 0", n)
	}
}

func TestRemoveFavorite_Idempotent(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	seed.Favorite(t, db, s.User.ID, p)

	assert204(t, env.Do(http.MethodDelete, favPath(p), nil, auth(s)))
	assert204(t, env.Do(http.MethodDelete, favPath(p), nil, auth(s)))
	assert204(t, env.Do(http.MethodDelete, favPath(424242), nil, auth(s)))
	if n := favCount(t, db, s.User.ID); n != 0 {
		t.Fatalf("favorites = %d, want 0", n)
	}
}

func TestListFavorites_PagedNewestFirstAndSkipsHidden(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	other := env.SignIn("+6281234567891")
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	var ids []int64
	for i := 0; i < 5; i++ {
		ids = append(ids, seed.Product(t, db, seed.ProductOpts{StoreID: store, Images: 1}))
	}
	hidden := seed.Product(t, db, seed.ProductOpts{StoreID: store, Status: "HIDDEN"})
	gone := seed.Product(t, db, seed.ProductOpts{StoreID: store, Deleted: true})
	// Favorite in this order: 0, 1, hidden, 2, gone, 3, 4. Newest favorite first is 4, 3, 2, 1, 0.
	for _, p := range []int64{ids[0], ids[1], hidden, ids[2], gone, ids[3], ids[4]} {
		seed.Favorite(t, db, s.User.ID, p)
	}
	seed.Favorite(t, db, other.User.ID, ids[0])

	first := listFavs(t, env, s, "?limit=2")
	contract.AssertShape(t, "listFavorites", mustJSON(t, first))
	if got := []int64{first.Items[0].ID, first.Items[1].ID}; got[0] != ids[4] || got[1] != ids[3] || first.NextCursor == nil {
		t.Fatalf("page 1 = %v cursor %v", got, first.NextCursor)
	}
	second := listFavs(t, env, s, "?limit=2&cursor="+*first.NextCursor)
	if len(second.Items) != 2 || second.Items[0].ID != ids[2] || second.Items[1].ID != ids[1] || second.NextCursor == nil {
		t.Fatalf("page 2 = %+v", second)
	}
	third := listFavs(t, env, s, "?limit=2&cursor="+*second.NextCursor)
	if len(third.Items) != 1 || third.Items[0].ID != ids[0] || third.NextCursor != nil {
		t.Fatalf("page 3 = %+v", third)
	}
	if third.Items[0].Image.Small == "" {
		t.Fatalf("image missing: %+v", third.Items[0])
	}
	if p := listFavs(t, env, env.SignIn("+6281234567892"), ""); len(p.Items) != 0 || p.NextCursor != nil {
		t.Fatalf("empty = %+v", p)
	}
	assertErr(t, env.Do(http.MethodGet, "/v1/me/favorites?cursor=!!", nil, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestGetProduct_IsFavoriteReflectsChange(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	p := seed.Product(t, db, seed.ProductOpts{StoreID: store})
	isFav := func() bool {
		r := env.Do(http.MethodGet, fmt.Sprintf("/v1/products/%d", p), nil, auth(s))
		var d domain.ProductDetail
		r.JSON(&d)
		if d.IsFavorite == nil {
			t.Fatalf("isFavorite nil: %s", r.Body)
		}
		return *d.IsFavorite
	}
	if isFav() {
		t.Fatal("favorite before add")
	}
	assert204(t, env.Do(http.MethodPut, favPath(p), nil, auth(s)))
	if !isFav() {
		t.Fatal("not favorite after add")
	}
	assert204(t, env.Do(http.MethodDelete, favPath(p), nil, auth(s)))
	if isFav() {
		t.Fatal("favorite after remove")
	}
}

func sellerDetail(t *testing.T, env *testapp.Env, s domain.Session, id int64) domain.SellerDetail {
	t.Helper()
	r := env.Do(http.MethodGet, fmt.Sprintf("/v1/sellers/%d", id), nil, auth(s))
	var d domain.SellerDetail
	r.JSON(&d)
	return d
}

func TestFollow_Idempotent_UpdatesFollowerCount(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	other := env.SignIn("+6281234567891")
	store, _ := seed.Store(t, db, seed.StoreOpts{})

	assert204(t, env.Do(http.MethodPut, followPath(store), nil, auth(s)))
	assert204(t, env.Do(http.MethodPut, followPath(store), nil, auth(s)))
	d := sellerDetail(t, env, s, store)
	if d.FollowerCount != 1 || d.IsFollowing == nil || !*d.IsFollowing {
		t.Fatalf("got %+v", d)
	}
	assert204(t, env.Do(http.MethodPut, followPath(store), nil, auth(other)))
	if d := sellerDetail(t, env, other, store); d.FollowerCount != 2 {
		t.Fatalf("followers = %d, want 2", d.FollowerCount)
	}
}

func TestFollow_OwnStore409(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	store, _ := seed.Store(t, db, seed.StoreOpts{OwnerID: s.User.ID})
	assertErr(t, env.Do(http.MethodPut, followPath(store), nil, auth(s)), http.StatusConflict, "CANNOT_FOLLOW_OWN_STORE")
}

func TestFollow_NonVisibleStore404(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	pending, _ := seed.Store(t, db, seed.StoreOpts{Status: "PENDING"})
	suspended, _ := seed.Store(t, db, seed.StoreOpts{Status: "SUSPENDED"})
	for _, id := range []int64{pending, suspended, 999999} {
		assertErr(t, env.Do(http.MethodPut, followPath(id), nil, auth(s)), http.StatusNotFound, "NOT_FOUND")
	}
}

func TestUnfollow_Idempotent(t *testing.T) {
	env, db := setup(t)
	s := env.SignIn(phone)
	store, _ := seed.Store(t, db, seed.StoreOpts{})
	seed.Follow(t, db, s.User.ID, store)

	assert204(t, env.Do(http.MethodDelete, followPath(store), nil, auth(s)))
	assert204(t, env.Do(http.MethodDelete, followPath(store), nil, auth(s)))
	assert204(t, env.Do(http.MethodDelete, followPath(999999), nil, auth(s)))
	if d := sellerDetail(t, env, s, store); d.FollowerCount != 0 || d.IsFollowing == nil || *d.IsFollowing {
		t.Fatalf("got %+v", d)
	}
}

func TestSocial_RequiresAuth(t *testing.T) {
	env, _ := setup(t)
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/v1/me/favorites"},
		{http.MethodPut, "/v1/me/favorites/1"},
		{http.MethodDelete, "/v1/me/favorites/1"},
		{http.MethodPut, "/v1/me/following/1"},
		{http.MethodDelete, "/v1/me/following/1"},
	} {
		if r := env.Do(c.method, c.path, nil); r.Status != http.StatusUnauthorized {
			t.Fatalf("%s %s: %d", c.method, c.path, r.Status)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
