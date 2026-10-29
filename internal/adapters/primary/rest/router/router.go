package router

import (
	"net/http"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/auth"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/category"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/media"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/middleware"
)

type Handlers struct {
	Auth     *auth.Handler
	Category *category.Handler
	Media    *media.Handler
	// Authenticate wraps a handler so it only runs for a valid bearer token.
	Authenticate func(http.Handler) http.Handler
}

// New wires routes on the stdlib mux (Go 1.22+ method patterns) and wraps them in the global middleware.
// Add the /v1 routes from docs/api/openapi.yaml here as their handlers land.
func New(h Handlers, allowedOrigins []string, ping func(*http.Request) error) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := ping(r); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /v1/auth/otp/request", h.Auth.RequestOTP)
	mux.HandleFunc("POST /v1/auth/otp/verify", h.Auth.VerifyOTP)
	mux.HandleFunc("POST /v1/auth/refresh", h.Auth.Refresh)
	mux.HandleFunc("POST /v1/auth/logout", h.Auth.Logout)

	mux.HandleFunc("GET /v1/categories", h.Category.List)

	if h.Media != nil {
		mux.Handle("POST /v1/me/images", h.Authenticate(http.HandlerFunc(h.Media.Upload)))
		if h.Media.HasFiles() {
			mux.HandleFunc("GET /media/{key...}", h.Media.Serve)
		}
	}

	return middleware.Tracing(middleware.Logger(middleware.Recover(middleware.CORS(allowedOrigins)(mux))))
}
