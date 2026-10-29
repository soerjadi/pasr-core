package identity

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time { return c.t }

var epoch = time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)

func TestTokens_RoundTrip(t *testing.T) {
	tk := NewTokens("secret", 15*time.Minute, &fixedClock{epoch})
	want := domain.Claims{UserID: 42, Role: domain.RoleOperator}
	tok, exp, err := tk.Issue(want)
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(epoch.Add(15 * time.Minute)) {
		t.Fatalf("expiresAt = %v", exp)
	}
	got, err := tk.Parse(tok)
	if err != nil || got != want {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestTokens_Expired(t *testing.T) {
	clock := &fixedClock{epoch}
	tk := NewTokens("secret", time.Minute, clock)
	tok, _, _ := tk.Issue(domain.Claims{UserID: 1, Role: domain.RoleBuyer})
	clock.t = epoch.Add(2 * time.Minute)
	if _, err := tk.Parse(tok); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestTokens_WrongSecret(t *testing.T) {
	tok, _, _ := NewTokens("a", time.Minute, &fixedClock{epoch}).Issue(domain.Claims{UserID: 1, Role: domain.RoleBuyer})
	if _, err := NewTokens("b", time.Minute, &fixedClock{epoch}).Parse(tok); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestTokens_AlgNoneRejected(t *testing.T) {
	tk := NewTokens("secret", time.Minute, &fixedClock{epoch})
	claims := jwt.MapClaims{"sub": "1", "role": "operator", "exp": epoch.Add(time.Minute).Unix()}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tk.Parse(tok); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("none: want ErrUnauthorized, got %v", err)
	}
	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte("secret"))
	if _, err := tk.Parse(hs512); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("hs512: want ErrUnauthorized, got %v", err)
	}
	if _, err := tk.Parse("garbage"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("garbage: want ErrUnauthorized, got %v", err)
	}
}
