package me_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
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

func storeBody() map[string]any {
	return map[string]any{
		"name": "Warung Bu Sari", "categoryId": 1, "ownerName": "Sari",
		"kelurahan": "Menteng", "address": "RT 01/RW 02, Jl. Mawar 3",
	}
}

func assertErr(t *testing.T, r *testapp.Resp, status int, code string) {
	t.Helper()
	var e errBody
	r.JSON(&e)
	if r.Status != status || e.Code != code {
		t.Fatalf("got %d %s, want %d %s", r.Status, r.Body, status, code)
	}
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

func TestGetMe_WithoutStore(t *testing.T) {
	env, _ := setup(t)
	s := env.SignIn(phone)
	r := env.Do(http.MethodGet, "/v1/me", nil, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
	var raw map[string]any
	r.JSON(&raw)
	if _, has := raw["store"]; has || raw["phone"] != phone || raw["id"] == nil {
		t.Fatalf("body = %s", r.Body)
	}
}

func TestGetMe_WithStoreShowsPendingAndNewOrderCount(t *testing.T) {
	env, pool := setup(t)
	ctx := context.Background()
	s := env.SignIn(phone)
	cr := env.Do(http.MethodPost, "/v1/me/store", storeBody(), auth(s))
	if cr.Status != http.StatusCreated {
		t.Fatalf("create store: %d %s", cr.Status, cr.Body)
	}
	var st domain.StoreSummary
	cr.JSON(&st)

	var orderID int64
	if err := pool.QueryRow(ctx, `INSERT INTO orders (number, buyer_id, total, status) VALUES ('NM-20990101-001', $1, 1000, 'CREATED') RETURNING id`, s.User.ID).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	early := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	late := early.Add(time.Hour)
	for _, at := range []time.Time{late, early} {
		if _, err := pool.Exec(ctx, `INSERT INTO seller_orders (order_id, store_id, status, subtotal, accept_by, store_name, pickup_address, whatsapp)
			VALUES ($1, $2, 'CREATED', 1000, $3, 'x', 'y', 'z')`, orderID, st.ID, at); err != nil {
			t.Fatal(err)
		}
	}
	// Orders in other statuses must not count.
	if _, err := pool.Exec(ctx, `INSERT INTO seller_orders (order_id, store_id, status, subtotal, accept_by, store_name, pickup_address, whatsapp)
		VALUES ($1, $2, 'COMPLETED', 1000, $3, 'x', 'y', 'z')`, orderID, st.ID, early.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	r := env.Do(http.MethodGet, "/v1/me", nil, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "getMe", r.Body)
	var u domain.User
	r.JSON(&u)
	if u.Store == nil || u.Store.Status != domain.StorePending || u.Store.NewOrderCount != 2 ||
		u.Store.AcceptBy == nil || !u.Store.AcceptBy.Equal(early) {
		t.Fatalf("store = %+v", u.Store)
	}
}

func TestUpdateMe_TrimsAndValidatesLength(t *testing.T) {
	env, _ := setup(t)
	s := env.SignIn(phone)

	r := env.Do(http.MethodPatch, "/v1/me", map[string]string{"name": "  Suryadi Sulaksono  "}, auth(s))
	if r.Status != http.StatusOK {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "updateMe", mustWithStore(t, r.Body))
	var u domain.User
	r.JSON(&u)
	if u.Name != "Suryadi Sulaksono" {
		t.Fatalf("name = %q", u.Name)
	}

	for _, name := range []string{"", "   ", strings.Repeat("a", 51)} {
		assertErr(t, env.Do(http.MethodPatch, "/v1/me", map[string]string{"name": name}, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
	}
	// 50 multi-byte runes are allowed.
	if r := env.Do(http.MethodPatch, "/v1/me", map[string]string{"name": strings.Repeat("é", 50)}, auth(s)); r.Status != http.StatusOK {
		t.Fatalf("50 runes: %d %s", r.Status, r.Body)
	}
}

// mustWithStore adds the fixture's store block so the shape check covers the whole User shape.
func mustWithStore(t *testing.T, body []byte) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	m["store"] = map[string]any{"id": 1, "name": "n", "status": "PENDING", "newOrderCount": 0, "acceptBy": "2099-01-01T00:00:00Z"}
	out, _ := json.Marshal(m)
	return out
}

func deviceCount(t *testing.T, pool *pgxpool.Pool, token string) (n int, owner int64) {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT user_id FROM devices WHERE fcm_token = $1`, token)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		n++
		if err := rows.Scan(&owner); err != nil {
			t.Fatal(err)
		}
	}
	return n, owner
}

func TestRegisterDevice_UpsertIsIdempotent(t *testing.T) {
	env, pool := setup(t)
	s := env.SignIn(phone)
	for i := 0; i < 2; i++ {
		if r := env.Do(http.MethodPut, "/v1/me/devices", map[string]string{"fcmToken": "tok-1"}, auth(s)); r.Status != http.StatusNoContent {
			t.Fatalf("status %d %s", r.Status, r.Body)
		}
	}
	if n, owner := deviceCount(t, pool, "tok-1"); n != 1 || owner != s.User.ID {
		t.Fatalf("rows = %d owner = %d", n, owner)
	}
	assertErr(t, env.Do(http.MethodPut, "/v1/me/devices", map[string]string{"fcmToken": "  "}, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestRegisterDevice_TokenMovesToNewUser(t *testing.T) {
	env, pool := setup(t)
	a := env.SignIn(phone)
	b := env.SignIn("+6281234567891")
	env.Do(http.MethodPut, "/v1/me/devices", map[string]string{"fcmToken": "tok-1"}, auth(a))
	if r := env.Do(http.MethodPut, "/v1/me/devices", map[string]string{"fcmToken": "tok-1"}, auth(b)); r.Status != http.StatusNoContent {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
	if n, owner := deviceCount(t, pool, "tok-1"); n != 1 || owner != b.User.ID {
		t.Fatalf("rows = %d owner = %d, want 1 owned by %d", n, owner, b.User.ID)
	}
}

func TestCreateStore_StartsPending(t *testing.T) {
	env, pool := setup(t)
	s := env.SignIn(phone)
	photo := uploadImage(t, env, s)
	body := storeBody()
	body["photoId"] = photo
	body["name"] = "  Warung Bu Sari "

	r := env.Do(http.MethodPost, "/v1/me/store", body, auth(s))
	if r.Status != http.StatusCreated {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "createMyStore", r.Body)
	var st domain.StoreSummary
	r.JSON(&st)
	if st.Status != domain.StorePending || st.NewOrderCount != 0 || st.AcceptBy != nil || st.Name != "Warung Bu Sari" {
		t.Fatalf("store = %+v", st)
	}

	var (
		wa       string
		hoursNil bool
		attached bool
	)
	if err := pool.QueryRow(context.Background(), `SELECT s.whatsapp, s.hours IS NULL, u.attached FROM stores s JOIN uploads u ON u.id = s.photo_upload_id WHERE s.id = $1`, st.ID).
		Scan(&wa, &hoursNil, &attached); err != nil {
		t.Fatal(err)
	}
	if wa != phone || !hoursNil || !attached {
		t.Fatalf("whatsapp=%q hoursNil=%v attached=%v", wa, hoursNil, attached)
	}
}

func TestCreateStore_SecondStoreIs409StoreExists(t *testing.T) {
	env, _ := setup(t)
	s := env.SignIn(phone)
	if r := env.Do(http.MethodPost, "/v1/me/store", storeBody(), auth(s)); r.Status != http.StatusCreated {
		t.Fatalf("first: %d %s", r.Status, r.Body)
	}
	assertErr(t, env.Do(http.MethodPost, "/v1/me/store", storeBody(), auth(s)), http.StatusConflict, "STORE_EXISTS")
}

func TestCreateStore_ConcurrentOnlyOneSucceeds(t *testing.T) {
	env, pool := setup(t)
	s := env.SignIn(phone)
	var created, conflict atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch r := env.Do(http.MethodPost, "/v1/me/store", storeBody(), auth(s)); r.Status {
			case http.StatusCreated:
				created.Add(1)
			case http.StatusConflict:
				conflict.Add(1)
			default:
				t.Errorf("unexpected %d %s", r.Status, r.Body)
			}
		}()
	}
	wg.Wait()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM stores`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if created.Load() != 1 || conflict.Load() != 7 || n != 1 {
		t.Fatalf("created=%d conflict=%d stores=%d", created.Load(), conflict.Load(), n)
	}
}

func TestCreateStore_InvalidCategory400(t *testing.T) {
	env, _ := setup(t)
	s := env.SignIn(phone)
	body := storeBody()
	body["categoryId"] = 99999
	assertErr(t, env.Do(http.MethodPost, "/v1/me/store", body, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestCreateStore_ValidatesFields(t *testing.T) {
	env, _ := setup(t)
	s := env.SignIn(phone)
	for field, v := range map[string]any{
		"name": strings.Repeat("a", 51), "ownerName": "", "address": strings.Repeat("a", 201), "kelurahan": "  ",
	} {
		body := storeBody()
		body[field] = v
		assertErr(t, env.Do(http.MethodPost, "/v1/me/store", body, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")
	}
}

func TestCreateStore_PhotoOfAnotherUserRejected(t *testing.T) {
	env, pool := setup(t)
	other := env.SignIn("+6281234567891")
	s := env.SignIn(phone)
	photo := uploadImage(t, env, other)
	body := storeBody()
	body["photoId"] = photo
	assertErr(t, env.Do(http.MethodPost, "/v1/me/store", body, auth(s)), http.StatusBadRequest, "VALIDATION_FAILED")

	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM stores`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("stores = %d, %v", n, err)
	}
}

func TestMe_RequiresAuth(t *testing.T) {
	env, _ := setup(t)
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/v1/me"}, {http.MethodPatch, "/v1/me"}, {http.MethodPut, "/v1/me/devices"}, {http.MethodPost, "/v1/me/store"},
	} {
		if r := env.Do(c.method, c.path, map[string]string{}); r.Status != http.StatusUnauthorized {
			t.Errorf("%s %s = %d", c.method, c.path, r.Status)
		}
	}
}
