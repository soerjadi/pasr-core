package identity

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// Tokens issues and verifies short-lived HS256 access tokens.
type Tokens struct {
	secret []byte
	ttl    time.Duration
	clock  domain.Clock
}

func NewTokens(secret string, ttl time.Duration, clock domain.Clock) *Tokens {
	return &Tokens{secret: []byte(secret), ttl: ttl, clock: clock}
}

type tokenClaims struct {
	Role domain.Role `json:"role"`
	jwt.RegisteredClaims
}

func (t *Tokens) Issue(c domain.Claims) (string, time.Time, error) {
	now := t.clock.Now()
	exp := now.Add(t.ttl)
	tc := tokenClaims{
		Role: c.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(c.UserID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, tc).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("identity.Tokens.Issue sign: %w", err)
	}
	return signed, exp, nil
}

// Parse accepts only unexpired HS256 tokens signed with the secret; every failure is ErrUnauthorized.
func (t *Tokens) Parse(token string) (domain.Claims, error) {
	var tc tokenClaims
	_, err := jwt.ParseWithClaims(token, &tc,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(t.clock.Now),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return domain.Claims{}, domain.ErrUnauthorized
	}
	id, err := strconv.ParseInt(tc.Subject, 10, 64)
	if err != nil || id <= 0 || (tc.Role != domain.RoleBuyer && tc.Role != domain.RoleOperator) {
		return domain.Claims{}, domain.ErrUnauthorized
	}
	return domain.Claims{UserID: id, Role: tc.Role}, nil
}
