package router

import (
	"net/http"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/ads"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/auth"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/catalog"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/category"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/me"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/media"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/middleware"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/mystore"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/order"
	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/social"
)

type Handlers struct {
	Ads      *ads.Handler
	Auth     *auth.Handler
	Category *category.Handler
	Catalog  *catalog.Handler
	Media    *media.Handler
	Me       *me.Handler
	MyStore  *mystore.Handler
	Order    *order.Handler
	Social   *social.Handler
	// Authenticate wraps a handler so it only runs for a valid bearer token.
	Authenticate func(http.Handler) http.Handler
	// OptionalAuth lets anonymous requests through but rejects a present invalid token.
	OptionalAuth func(http.Handler) http.Handler
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

	if h.Catalog != nil {
		mux.HandleFunc("GET /v1/products", h.Catalog.ListProducts)
		mux.Handle("GET /v1/products/{productId}", h.OptionalAuth(http.HandlerFunc(h.Catalog.GetProduct)))
		mux.Handle("GET /v1/sellers/{sellerId}", h.OptionalAuth(http.HandlerFunc(h.Catalog.GetSeller)))
		mux.HandleFunc("GET /v1/sellers/{sellerId}/reviews", h.Catalog.ListSellerReviews)
	}

	if h.Ads != nil {
		mux.HandleFunc("GET /v1/ads", h.Ads.List)
	}

	if h.Me != nil {
		mux.Handle("GET /v1/me", h.Authenticate(http.HandlerFunc(h.Me.Get)))
		mux.Handle("PATCH /v1/me", h.Authenticate(http.HandlerFunc(h.Me.Update)))
		mux.Handle("PUT /v1/me/devices", h.Authenticate(http.HandlerFunc(h.Me.RegisterDevice)))
		mux.Handle("POST /v1/me/store", h.Authenticate(http.HandlerFunc(h.Me.CreateStore)))
	}

	if h.MyStore != nil {
		mux.Handle("GET /v1/me/store", h.Authenticate(http.HandlerFunc(h.MyStore.GetStore)))
		mux.Handle("PATCH /v1/me/store", h.Authenticate(http.HandlerFunc(h.MyStore.UpdateStore)))
		mux.Handle("PUT /v1/me/store/open", h.Authenticate(http.HandlerFunc(h.MyStore.SetOpen)))
		mux.Handle("POST /v1/me/store/products", h.Authenticate(http.HandlerFunc(h.MyStore.CreateProduct)))
		mux.Handle("GET /v1/me/store/products", h.Authenticate(http.HandlerFunc(h.MyStore.ListProducts)))
		mux.Handle("GET /v1/me/store/products/{id}", h.Authenticate(http.HandlerFunc(h.MyStore.GetProduct)))
		mux.Handle("PATCH /v1/me/store/products/{id}", h.Authenticate(http.HandlerFunc(h.MyStore.UpdateProduct)))
		mux.Handle("DELETE /v1/me/store/products/{id}", h.Authenticate(http.HandlerFunc(h.MyStore.DeleteProduct)))
	}

	if h.Order != nil {
		mux.HandleFunc("POST /v1/cart/validate", h.Order.ValidateCart)
	}

	if h.Social != nil {
		mux.Handle("GET /v1/me/favorites", h.Authenticate(http.HandlerFunc(h.Social.ListFavorites)))
		mux.Handle("PUT /v1/me/favorites/{productId}", h.Authenticate(http.HandlerFunc(h.Social.AddFavorite)))
		mux.Handle("DELETE /v1/me/favorites/{productId}", h.Authenticate(http.HandlerFunc(h.Social.RemoveFavorite)))
		mux.Handle("PUT /v1/me/following/{sellerId}", h.Authenticate(http.HandlerFunc(h.Social.Follow)))
		mux.Handle("DELETE /v1/me/following/{sellerId}", h.Authenticate(http.HandlerFunc(h.Social.Unfollow)))
	}

	if h.Media != nil {
		mux.Handle("POST /v1/me/images", h.Authenticate(http.HandlerFunc(h.Media.Upload)))
		if h.Media.HasFiles() {
			mux.HandleFunc("GET /media/{key...}", h.Media.Serve)
		}
	}

	return middleware.Tracing(middleware.Logger(middleware.Recover(middleware.CORS(allowedOrigins)(mux))))
}
