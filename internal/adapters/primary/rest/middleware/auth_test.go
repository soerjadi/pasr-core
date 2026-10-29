package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
)

func parse(token string) (domain.Claims, error) {
	switch token {
	case "buyer":
		return domain.Claims{UserID: 1, Role: domain.RoleBuyer}, nil
	case "op":
		return domain.Claims{UserID: 2, Role: domain.RoleOperator}, nil
	}
	return domain.Claims{}, domain.ErrUnauthorized
}

var probe = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	c, ok := rest.Principal(r.Context())
	if !ok {
		w.Header().Set("X-Anon", "1")
	}
	w.Header().Set("X-User", string(rune('0'+c.UserID)))
	w.WriteHeader(http.StatusOK)
})

func do(h http.Handler, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuth_MissingHeader401(t *testing.T) {
	rec := do(Auth(parse)(probe), "")
	if rec.Code != 401 || !strings.Contains(rec.Body.String(), `"UNAUTHORIZED"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if rec := do(Auth(parse)(probe), "Bearer nope"); rec.Code != 401 {
		t.Fatalf("bad token: got %d", rec.Code)
	}
}

func TestAuth_ValidSetsPrincipal(t *testing.T) {
	rec := do(Auth(parse)(probe), "Bearer buyer")
	if rec.Code != 200 || rec.Header().Get("X-User") != "1" || rec.Header().Get("X-Anon") != "" {
		t.Fatalf("got %d %v", rec.Code, rec.Header())
	}
}

func TestOptionalAuth_AnonymousPasses(t *testing.T) {
	rec := do(OptionalAuth(parse)(probe), "")
	if rec.Code != 200 || rec.Header().Get("X-Anon") != "1" {
		t.Fatalf("got %d %v", rec.Code, rec.Header())
	}
	if rec := do(OptionalAuth(parse)(probe), "Bearer buyer"); rec.Header().Get("X-User") != "1" {
		t.Fatalf("principal not set: %v", rec.Header())
	}
}

func TestOptionalAuth_InvalidToken401(t *testing.T) {
	if rec := do(OptionalAuth(parse)(probe), "Bearer nope"); rec.Code != 401 {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRequireOperator_Buyer403(t *testing.T) {
	h := Auth(parse)(RequireOperator(probe))
	rec := do(h, "Bearer buyer")
	if rec.Code != 403 || !strings.Contains(rec.Body.String(), `"FORBIDDEN"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "Bearer op"); rec.Code != 200 {
		t.Fatalf("operator: got %d", rec.Code)
	}
}
