package mystore_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
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

type errBody struct {
	Code string `json:"code"`
}

func setup(t *testing.T) (*testapp.Env, *pgxpool.Pool) {
	t.Helper()
	env := testapp.New(t)
	return env, pgtest.Pool(t) // after New: both reset the shared DB, so no data exists yet
}

func auth(s domain.Session) testapp.Opt { return testapp.Bearer(s.AccessToken) }

func assertErr(t *testing.T, r *testapp.Resp, status int, code string) {
	t.Helper()
	var e errBody
	r.JSON(&e)
	if r.Status != status || e.Code != code {
		t.Fatalf("got %d %s, want %d %s", r.Status, r.Body, status, code)
	}
}

// seller signs in and gives the user a store with the given status.
func seller(t *testing.T, env *testapp.Env, db *pgxpool.Pool, status string) (domain.Session, int64) {
	t.Helper()
	s := env.SignIn(phone)
	storeID, _ := seed.Store(t, db, seed.StoreOpts{OwnerID: s.User.ID, Status: status})
	return s, storeID
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func uploadImage(t *testing.T, env *testapp.Env, s domain.Session) int64 {
	t.Helper()
	r := env.DoFile(http.MethodPost, "/v1/me/images", "file", "a.jpg", "image/jpeg", jpegBytes(t), auth(s))
	if r.Status != http.StatusCreated {
		t.Fatalf("upload: %d %s", r.Status, r.Body)
	}
	var up domain.ImageUpload
	r.JSON(&up)
	return up.ID
}

func create(t *testing.T, env *testapp.Env, s domain.Session, body map[string]any) domain.SellerProduct {
	t.Helper()
	r := env.Do(http.MethodPost, "/v1/me/store/products", body, auth(s))
	if r.Status != http.StatusCreated {
		t.Fatalf("create: %d %s", r.Status, r.Body)
	}
	var p domain.SellerProduct
	r.JSON(&p)
	return p
}

func patch(env *testapp.Env, s domain.Session, id int64, body map[string]any) *testapp.Resp {
	return env.Do(http.MethodPatch, fmt.Sprintf("/v1/me/store/products/%d", id), body, auth(s))
}

func publicIDs(t *testing.T, env *testapp.Env) []int64 {
	t.Helper()
	r := env.Do(http.MethodGet, "/v1/products", nil)
	var p domain.Page[domain.ProductSummary]
	r.JSON(&p)
	out := []int64{}
	for _, it := range p.Items {
		out = append(out, it.ID)
	}
	return out
}

func TestCreateProduct_WhilePendingStoreAllowed(t *testing.T) {
	env, db := setup(t)
	s, storeID := seller(t, env, db, "PENDING")
	photo := uploadImage(t, env, s)

	r := env.Do(http.MethodPost, "/v1/me/store/products",
		map[string]any{"name": "Nasi pecel", "price": 15000, "stock": 10, "description": "Pedas", "photoIds": []int64{photo}}, auth(s))
	if r.Status != http.StatusCreated {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "createMyProduct", r.Body)
	var p domain.SellerProduct
	r.JSON(&p)
	if p.Name != "Nasi pecel" || p.Price != 15000 || p.Stock != 10 || p.Status != domain.ProductAvailable ||
		p.Description != "Pedas" || len(p.Images) != 1 || p.Images[0].Small == "" {
		t.Fatalf("got %+v", p)
	}
	var owner int64
	if err := db.QueryRow(context.Background(), `SELECT store_id FROM products WHERE id = $1`, p.ID).Scan(&owner); err != nil || owner != storeID {
		t.Fatalf("store %d err %v", owner, err)
	}
	var attached bool
	if err := db.QueryRow(context.Background(), `SELECT attached FROM uploads WHERE id = $1`, photo).Scan(&attached); err != nil || !attached {
		t.Fatalf("photo attached %v err %v", attached, err)
	}
	// Without photos the product is still created.
	if q := create(t, env, s, map[string]any{"name": "Tanpa foto", "price": 1000, "stock": 0}); q.Status != domain.ProductOutOfStock || len(q.Images) != 0 {
		t.Fatalf("got %+v", q)
	}
}

func TestCreateProduct_NoStore409NoStore(t *testing.T) {
	env, _ := setup(t)
	s := env.SignIn(phone)
	r := env.Do(http.MethodPost, "/v1/me/store/products", map[string]any{"name": "A", "price": 1000, "stock": 1}, auth(s))
	assertErr(t, r, http.StatusConflict, "NO_STORE")
	assertErr(t, env.Do(http.MethodGet, "/v1/me/store/products", nil, auth(s)), http.StatusConflict, "NO_STORE")
	if r := env.Do(http.MethodPost, "/v1/me/store/products", map[string]any{}); r.Status != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", r.Status)
	}
}

func TestCreateProduct_ValidationTable(t *testing.T) {
	env, db := setup(t)
	s, _ := seller(t, env, db, "PENDING")
	long := func(n int) string { return string(bytes.Repeat([]byte("a"), n)) }
	cases := []struct {
		name string
		body map[string]any
	}{
		{"empty name", map[string]any{"name": "", "price": 1000, "stock": 1}},
		{"blank name", map[string]any{"name": "   ", "price": 1000, "stock": 1}},
		{"name too long", map[string]any{"name": long(81), "price": 1000, "stock": 1}},
		{"zero price", map[string]any{"name": "A", "price": 0, "stock": 1}},
		{"negative price", map[string]any{"name": "A", "price": -5, "stock": 1}},
		{"negative stock", map[string]any{"name": "A", "price": 1000, "stock": -1}},
		{"description too long", map[string]any{"name": "A", "price": 1000, "stock": 1, "description": long(2001)}},
		{"foreign photo", map[string]any{"name": "A", "price": 1000, "stock": 1, "photoIds": []int64{999999}}},
		{"duplicate photo", map[string]any{"name": "A", "price": 1000, "stock": 1, "photoIds": []int64{1, 1}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertErr(t, env.Do(http.MethodPost, "/v1/me/store/products", c.body, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
		})
	}
	// Boundaries are accepted.
	create(t, env, s, map[string]any{"name": long(80), "price": 1, "stock": 0, "description": long(2000)})
}

func TestCreateProduct_MoreThanFivePhotos400(t *testing.T) {
	env, db := setup(t)
	s, _ := seller(t, env, db, "VERIFIED")
	var ids []int64
	for i := 0; i < 6; i++ {
		ids = append(ids, uploadImage(t, env, s))
	}
	r := env.Do(http.MethodPost, "/v1/me/store/products", map[string]any{"name": "A", "price": 1000, "stock": 1, "photoIds": ids}, auth(s))
	assertErr(t, r, http.StatusBadRequest, "VALIDATION_FAILED")
	// Nothing was claimed.
	var n int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM uploads WHERE attached`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("attached %d err %v", n, err)
	}
	// Exactly five is fine and keeps the order.
	p := create(t, env, s, map[string]any{"name": "A", "price": 1000, "stock": 1, "photoIds": ids[:5]})
	if len(p.Images) != 5 {
		t.Fatalf("got %d images", len(p.Images))
	}
}

func TestListMyProducts_IncludesHiddenAndReturnsCounts(t *testing.T) {
	env, db := setup(t)
	s, storeID := seller(t, env, db, "PENDING")
	other, _ := seed.Store(t, db, seed.StoreOpts{})
	seed.Product(t, db, seed.ProductOpts{StoreID: other})
	a := seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Images: 1})
	seed.Product(t, db, seed.ProductOpts{StoreID: storeID})
	h := seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Status: "HIDDEN"})
	o := seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Status: "OUT_OF_STOCK"})
	seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Deleted: true})

	r := env.Do(http.MethodGet, "/v1/me/store/products", nil, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "listMyProducts", r.Body)
	var p domain.SellerProductPage
	r.JSON(&p)
	if len(p.Items) != 4 || p.NextCursor != nil {
		t.Fatalf("got %+v", p)
	}
	if p.Counts != (domain.ProductCounts{All: 4, Available: 2, OutOfStock: 1, Hidden: 1}) {
		t.Fatalf("counts %+v", p.Counts)
	}
	have := map[int64]domain.SellerProduct{}
	for _, it := range p.Items {
		have[it.ID] = it
	}
	if have[h].Status != domain.ProductHidden || have[o].Status != domain.ProductOutOfStock || len(have[a].Images) != 1 {
		t.Fatalf("got %+v", have)
	}
	// Newest first, paginated; counts stay the totals.
	r = env.Do(http.MethodGet, "/v1/me/store/products?limit=3", nil, auth(s))
	r.JSON(&p)
	if len(p.Items) != 3 || p.NextCursor == nil || p.Counts.All != 4 {
		t.Fatalf("page 1 %+v", p)
	}
	r = env.Do(http.MethodGet, "/v1/me/store/products?limit=3&cursor="+*p.NextCursor, nil, auth(s))
	var p2 domain.SellerProductPage
	r.JSON(&p2)
	if len(p2.Items) != 1 || p2.NextCursor != nil || p2.Items[0].ID != a {
		t.Fatalf("page 2 %+v", p2)
	}
}

func TestListMyProducts_FilterByStatus(t *testing.T) {
	env, db := setup(t)
	s, storeID := seller(t, env, db, "VERIFIED")
	seed.Product(t, db, seed.ProductOpts{StoreID: storeID})
	h := seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Status: "HIDDEN"})

	var p domain.SellerProductPage
	env.Do(http.MethodGet, "/v1/me/store/products?status=HIDDEN", nil, auth(s)).JSON(&p)
	if len(p.Items) != 1 || p.Items[0].ID != h || p.Counts.All != 2 {
		t.Fatalf("got %+v", p)
	}
	assertErr(t, env.Do(http.MethodGet, "/v1/me/store/products?status=BOGUS", nil, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestGetMyProduct(t *testing.T) {
	env, db := setup(t)
	s, storeID := seller(t, env, db, "PENDING")
	id := seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Images: 2, Description: "Enak"})
	r := env.Do(http.MethodGet, fmt.Sprintf("/v1/me/store/products/%d", id), nil, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "getMyProduct", r.Body)
	var p domain.SellerProduct
	r.JSON(&p)
	if p.ID != id || p.Description != "Enak" || len(p.Images) != 2 {
		t.Fatalf("got %+v", p)
	}
	assertErr(t, env.Do(http.MethodGet, "/v1/me/store/products/abc", nil, auth(s)), http.StatusNotFound, "NOT_FOUND")
}

func TestUpdateProduct_StockZeroBecomesOutOfStock(t *testing.T) {
	env, db := setup(t)
	s, storeID := seller(t, env, db, "VERIFIED")
	id := seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Stock: 5})
	r := patch(env, s, id, map[string]any{"stock": 0})
	var p domain.SellerProduct
	r.JSON(&p)
	if r.Status != http.StatusOK || p.Status != domain.ProductOutOfStock || p.Stock != 0 {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
}

func TestUpdateProduct_RestockClearsOutOfStock(t *testing.T) {
	env, db := setup(t)
	s, storeID := seller(t, env, db, "VERIFIED")
	id := seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Status: "OUT_OF_STOCK"})
	var p domain.SellerProduct
	patch(env, s, id, map[string]any{"stock": 7}).JSON(&p)
	if p.Status != domain.ProductAvailable || p.Stock != 7 {
		t.Fatalf("got %+v", p)
	}
	// An explicit status in the same patch wins over the automatic change.
	id2 := seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Status: "OUT_OF_STOCK"})
	patch(env, s, id2, map[string]any{"stock": 7, "status": "HIDDEN"}).JSON(&p)
	if p.Status != domain.ProductHidden {
		t.Fatalf("got %+v", p)
	}
	// Other fields change nothing about status; name and price are updated.
	patch(env, s, id2, map[string]any{"name": "Baru", "price": 2500}).JSON(&p)
	if p.Status != domain.ProductHidden || p.Name != "Baru" || p.Price != 2500 || p.Stock != 7 {
		t.Fatalf("got %+v", p)
	}
}

func TestUpdateProduct_AvailableWithZeroStockRejected(t *testing.T) {
	env, db := setup(t)
	s, storeID := seller(t, env, db, "VERIFIED")
	id := seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Status: "HIDDEN", Stock: 3})
	assertErr(t, patch(env, s, id, map[string]any{"status": "AVAILABLE", "stock": 0}), http.StatusBadRequest, "VALIDATION_FAILED")
	out := seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Status: "OUT_OF_STOCK"})
	assertErr(t, patch(env, s, out, map[string]any{"status": "AVAILABLE"}), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, patch(env, s, id, map[string]any{"status": "BOGUS"}), http.StatusBadRequest, "VALIDATION_FAILED")
	assertErr(t, patch(env, s, id, map[string]any{"price": 0}), http.StatusBadRequest, "VALIDATION_FAILED")
	// Rejected patches change nothing.
	var st string
	if err := db.QueryRow(context.Background(), `SELECT status FROM products WHERE id = $1`, id).Scan(&st); err != nil || st != "HIDDEN" {
		t.Fatalf("status %s err %v", st, err)
	}
}

func TestUpdateProduct_ReplacesPhotos(t *testing.T) {
	env, db := setup(t)
	s, storeID := seller(t, env, db, "VERIFIED")
	id := seed.Product(t, db, seed.ProductOpts{StoreID: storeID, Images: 2})
	a, b := uploadImage(t, env, s), uploadImage(t, env, s)
	var p domain.SellerProduct
	r := patch(env, s, id, map[string]any{"photoIds": []int64{a, b}})
	r.JSON(&p)
	if r.Status != http.StatusOK || len(p.Images) != 2 {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	patch(env, s, id, map[string]any{"photoIds": []int64{}}).JSON(&p)
	if len(p.Images) != 0 {
		t.Fatalf("got %+v", p)
	}
}

func TestUpdateProduct_HiddenDisappearsFromPublicList(t *testing.T) {
	env, db := setup(t)
	s, storeID := seller(t, env, db, "VERIFIED")
	id := seed.Product(t, db, seed.ProductOpts{StoreID: storeID})
	if got := publicIDs(t, env); len(got) != 1 || got[0] != id {
		t.Fatalf("before: %v", got)
	}
	if r := patch(env, s, id, map[string]any{"status": "HIDDEN"}); r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	if got := publicIDs(t, env); len(got) != 0 {
		t.Fatalf("after hide: %v", got)
	}
	patch(env, s, id, map[string]any{"status": "AVAILABLE"})
	if got := publicIDs(t, env); len(got) != 1 {
		t.Fatalf("after unhide: %v", got)
	}
}

func TestUpdateProduct_OtherStoresProduct404(t *testing.T) {
	env, db := setup(t)
	s, _ := seller(t, env, db, "VERIFIED")
	other, _ := seed.Store(t, db, seed.StoreOpts{})
	id := seed.Product(t, db, seed.ProductOpts{StoreID: other, Stock: 4})
	assertErr(t, patch(env, s, id, map[string]any{"stock": 0}), http.StatusNotFound, "NOT_FOUND")
	path := fmt.Sprintf("/v1/me/store/products/%d", id)
	assertErr(t, env.Do(http.MethodGet, path, nil, auth(s)), http.StatusNotFound, "NOT_FOUND")
	assertErr(t, env.Do(http.MethodDelete, path, nil, auth(s)), http.StatusNotFound, "NOT_FOUND")
	var stock int
	var deleted bool
	if err := db.QueryRow(context.Background(), `SELECT stock, deleted_at IS NOT NULL FROM products WHERE id = $1`, id).Scan(&stock, &deleted); err != nil || stock != 4 || deleted {
		t.Fatalf("stock %d deleted %v err %v", stock, deleted, err)
	}
}

func TestDeleteProduct_GonePublicly_OldOrderItemSurvives(t *testing.T) {
	env, db := setup(t)
	s, storeID := seller(t, env, db, "VERIFIED")
	id := seed.Product(t, db, seed.ProductOpts{StoreID: storeID})
	buyer := seed.User(t, db, "", "Pembeli")
	item := seed.OrderItem(t, db, buyer, storeID, id)

	path := fmt.Sprintf("/v1/me/store/products/%d", id)
	if r := env.Do(http.MethodDelete, path, nil, auth(s)); r.Status != http.StatusNoContent {
		t.Fatalf("delete: %d %s", r.Status, r.Body)
	}
	if got := publicIDs(t, env); len(got) != 0 {
		t.Fatalf("still public: %v", got)
	}
	if r := env.Do(http.MethodGet, fmt.Sprintf("/v1/products/%d", id), nil); r.Status != http.StatusNotFound {
		t.Fatalf("public get: %d", r.Status)
	}
	assertErr(t, env.Do(http.MethodGet, path, nil, auth(s)), http.StatusNotFound, "NOT_FOUND")
	assertErr(t, env.Do(http.MethodDelete, path, nil, auth(s)), http.StatusNotFound, "NOT_FOUND")
	var p domain.SellerProductPage
	env.Do(http.MethodGet, "/v1/me/store/products", nil, auth(s)).JSON(&p)
	if len(p.Items) != 0 || p.Counts.All != 0 {
		t.Fatalf("seller list %+v", p)
	}
	var n int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM order_items WHERE id = $1 AND product_id = $2`, item, id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("order item survives: %d %v", n, err)
	}
}

func TestDeleteProduct_RemovesFromFavorites(t *testing.T) {
	env, db := setup(t)
	s, storeID := seller(t, env, db, "VERIFIED")
	id := seed.Product(t, db, seed.ProductOpts{StoreID: storeID})
	keep := seed.Product(t, db, seed.ProductOpts{StoreID: storeID})
	fan := seed.User(t, db, "", "Fan")
	seed.Favorite(t, db, fan, id)
	seed.Favorite(t, db, fan, keep)

	if r := env.Do(http.MethodDelete, fmt.Sprintf("/v1/me/store/products/%d", id), nil, auth(s)); r.Status != http.StatusNoContent {
		t.Fatalf("delete: %d %s", r.Status, r.Body)
	}
	var n int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM favorites WHERE product_id = $1`, id).Scan(&n); err != nil || n != 0 {
		t.Fatalf("favorites left %d err %v", n, err)
	}
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM favorites WHERE product_id = $1`, keep).Scan(&n); err != nil || n != 1 {
		t.Fatalf("other favorite %d err %v", n, err)
	}
}
