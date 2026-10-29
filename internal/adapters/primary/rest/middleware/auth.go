package middleware

import (
	"net/http"
	"strings"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
)

type parseFunc = func(string) (domain.Claims, error)

// Auth requires a valid bearer token; otherwise the request gets 401.
func Auth(parse parseFunc) func(http.Handler) http.Handler {
	return authenticate(parse, true)
}

// OptionalAuth lets anonymous requests through, but a present invalid token is still 401
// so the client knows to refresh.
func OptionalAuth(parse parseFunc) func(http.Handler) http.Handler {
	return authenticate(parse, false)
}

func authenticate(parse parseFunc, required bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" && !required {
				next.ServeHTTP(w, r)
				return
			}
			scheme, token, ok := strings.Cut(header, " ")
			if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
				rest.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or malformed bearer token")
				return
			}
			claims, err := parse(strings.TrimSpace(token))
			if err != nil {
				rest.WriteServiceError(w, r, domain.ErrUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(rest.WithPrincipal(r.Context(), claims)))
		})
	}
}

// RequireOperator must run after Auth; non-operators get 403.
func RequireOperator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := rest.Principal(r.Context())
		if !ok {
			rest.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}
		if c.Role != domain.RoleOperator {
			rest.WriteError(w, http.StatusForbidden, "FORBIDDEN", "operator role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
