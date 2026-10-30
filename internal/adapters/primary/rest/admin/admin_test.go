package admin_test

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/router"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/internal/testutil/seed"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

const (
	buyerPhone  = "+6281234500001"
	sellerPhone = "+6281234500002"
)

// fx is the app with an operator, a buyer without a store and a seller (a buyer who owns a store).
type fx struct {
	env               *testapp.Env
	db                *pgxpool.Pool
	op, buyer, seller domain.Session
	sellerStore       int64
}

func setup(t *testing.T) *fx {
	t.Helper()
	env := testapp.New(t)
	db := pgtest.Pool(t) // after New: both reset the shared DB, so no rows exist yet
	f := &fx{env: env, db: db}
	f.op = env.SignInOperator(testapp.OperatorPhone)
	f.buyer = env.SignIn(buyerPhone)
	f.seller = env.SignIn(sellerPhone)
	f.sellerStore, _ = seed.Store(t, db, seed.StoreOpts{OwnerID: f.seller.User.ID, Status: "VERIFIED"})
	return f
}

func auth(s domain.Session) testapp.Opt { return testapp.Bearer(s.AccessToken) }

func (f *fx) do(method, path string, body any) *testapp.Resp {
	return f.env.Do(method, path, body, auth(f.op))
}

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func assertErr(t *testing.T, r *testapp.Resp, status int, code string) {
	t.Helper()
	var e errBody
	r.JSON(&e)
	if r.Status != status || e.Code != code {
		t.Fatalf("got %d %s, want %d %s", r.Status, r.Body, status, code)
	}
}

func assertStatus(t *testing.T, r *testapp.Resp, status int) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("got %d %s, want %d", r.Status, r.Body, status)
	}
}

func (f *fx) scalar(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := f.db.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// adminRoutes is the authoritative list of operator endpoints. TestAdmin_RequiresOperatorRole
// checks it against what the router registers, so a route added in one place only fails.
var adminRoutes = []struct{ method, path, pattern string }{
	{http.MethodGet, "/v1/admin/stores", "GET /v1/admin/stores"},
	{http.MethodPost, "/v1/admin/stores/1/verify", "POST /v1/admin/stores/{id}/verify"},
	{http.MethodPost, "/v1/admin/stores/1/suspend", "POST /v1/admin/stores/{id}/suspend"},
	{http.MethodPost, "/v1/admin/stores/1/reinstate", "POST /v1/admin/stores/{id}/reinstate"},
	{http.MethodPatch, "/v1/admin/products/1", "PATCH /v1/admin/products/{id}"},
	{http.MethodPost, "/v1/admin/categories", "POST /v1/admin/categories"},
	{http.MethodPatch, "/v1/admin/categories/1", "PATCH /v1/admin/categories/{id}"},
	{http.MethodDelete, "/v1/admin/categories/999999", "DELETE /v1/admin/categories/{id}"},
	{http.MethodGet, "/v1/admin/ad-campaigns", "GET /v1/admin/ad-campaigns"},
	{http.MethodPost, "/v1/admin/ad-campaigns", "POST /v1/admin/ad-campaigns"},
	{http.MethodPatch, "/v1/admin/ad-campaigns/1", "PATCH /v1/admin/ad-campaigns/{id}"},
	{http.MethodDelete, "/v1/admin/ad-campaigns/1", "DELETE /v1/admin/ad-campaigns/{id}"},
	{http.MethodGet, "/v1/admin/orders", "GET /v1/admin/orders"},
	{http.MethodPost, "/v1/admin/seller-orders/1/accept", "POST /v1/admin/seller-orders/{id}/accept"},
	{http.MethodPost, "/v1/admin/seller-orders/1/reject", "POST /v1/admin/seller-orders/{id}/reject"},
	{http.MethodPost, "/v1/admin/seller-orders/1/ready", "POST /v1/admin/seller-orders/{id}/ready"},
	{http.MethodPost, "/v1/admin/seller-orders/1/complete", "POST /v1/admin/seller-orders/{id}/complete"},
	{http.MethodPost, "/v1/admin/seller-orders/1/cancel", "POST /v1/admin/seller-orders/{id}/cancel"},
}

func TestAdmin_RequiresOperatorRole(t *testing.T) {
	f := setup(t)

	var want []string
	for _, rt := range adminRoutes {
		want = append(want, rt.pattern)
	}
	var got []string
	for _, rt := range router.AdminRoutes(nil) {
		got = append(got, rt.Pattern)
	}
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Fatalf("router admin routes differ from the test table\nrouter:\n%s\ntable:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	for _, rt := range adminRoutes {
		name := rt.method + " " + rt.path
		t.Run(name, func(t *testing.T) {
			r := f.env.Do(rt.method, rt.path, map[string]any{})
			assertErr(t, r, http.StatusUnauthorized, "UNAUTHORIZED")
			r = f.env.Do(rt.method, rt.path, map[string]any{}, testapp.Bearer("not-a-token"))
			assertErr(t, r, http.StatusUnauthorized, "UNAUTHORIZED")
			for who, s := range map[string]domain.Session{"buyer": f.buyer, "seller": f.seller} {
				r = f.env.Do(rt.method, rt.path, map[string]any{}, auth(s))
				if r.Status != http.StatusForbidden || !strings.Contains(string(r.Body), `"FORBIDDEN"`) {
					t.Fatalf("%s: got %d %s, want 403 FORBIDDEN", who, r.Status, r.Body)
				}
			}
			// The operator reaches the handler: a JSON answer that is not an auth or routing failure.
			r = f.do(rt.method, rt.path, map[string]any{})
			if r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden || r.Status == http.StatusMethodNotAllowed ||
				!strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				t.Fatalf("operator: got %d %q %s", r.Status, r.Header.Get("Content-Type"), r.Body)
			}
		})
	}
}

func path(format string, args ...any) string { return fmt.Sprintf(format, args...) }
