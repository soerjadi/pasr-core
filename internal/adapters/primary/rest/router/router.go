package router

import (
	"net/http"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/category"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/middleware"
)

type Handlers struct {
	Category *category.Handler
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

	mux.HandleFunc("GET /v1/categories", h.Category.List)

	return middleware.Tracing(middleware.Logger(middleware.Recover(middleware.CORS(allowedOrigins)(mux))))
}
