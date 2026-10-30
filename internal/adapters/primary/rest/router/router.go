package router

import (
	"net/http"
	"time"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest/admin"
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

// UploadTimeout is how long POST /v1/me/images may take to read the request and write the response.
const UploadTimeout = 60 * time.Second

type Handlers struct {
	Admin    *admin.Handler
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
		mux.Handle("GET /v1/me/store/summary", h.Authenticate(http.HandlerFunc(h.MyStore.GetSummary)))
		mux.Handle("GET /v1/me/store/stats", h.Authenticate(http.HandlerFunc(h.MyStore.GetStats)))
		mux.Handle("GET /v1/me/store/orders", h.Authenticate(http.HandlerFunc(h.MyStore.ListOrders)))
		mux.Handle("GET /v1/me/store/orders/{id}", h.Authenticate(http.HandlerFunc(h.MyStore.GetOrder)))
		mux.Handle("POST /v1/me/store/orders/{id}/accept", h.Authenticate(http.HandlerFunc(h.MyStore.AcceptOrder)))
		mux.Handle("POST /v1/me/store/orders/{id}/reject", h.Authenticate(http.HandlerFunc(h.MyStore.RejectOrder)))
		mux.Handle("POST /v1/me/store/orders/{id}/ready", h.Authenticate(http.HandlerFunc(h.MyStore.ReadyOrder)))
		mux.Handle("POST /v1/me/store/orders/{id}/complete", h.Authenticate(http.HandlerFunc(h.MyStore.CompleteOrder)))
	}

	if h.Order != nil {
		mux.HandleFunc("POST /v1/cart/validate", h.Order.ValidateCart)
		mux.Handle("POST /v1/orders", h.Authenticate(http.HandlerFunc(h.Order.CreateOrder)))
		mux.Handle("GET /v1/orders", h.Authenticate(http.HandlerFunc(h.Order.ListOrders)))
		mux.Handle("GET /v1/orders/{orderId}", h.Authenticate(http.HandlerFunc(h.Order.GetOrder)))
		mux.Handle("POST /v1/order-items/{orderItemId}/reviews", h.Authenticate(http.HandlerFunc(h.Order.SubmitReview)))
	}

	if h.Social != nil {
		mux.Handle("GET /v1/me/favorites", h.Authenticate(http.HandlerFunc(h.Social.ListFavorites)))
		mux.Handle("PUT /v1/me/favorites/{productId}", h.Authenticate(http.HandlerFunc(h.Social.AddFavorite)))
		mux.Handle("DELETE /v1/me/favorites/{productId}", h.Authenticate(http.HandlerFunc(h.Social.RemoveFavorite)))
		mux.Handle("PUT /v1/me/following/{sellerId}", h.Authenticate(http.HandlerFunc(h.Social.Follow)))
		mux.Handle("DELETE /v1/me/following/{sellerId}", h.Authenticate(http.HandlerFunc(h.Social.Unfollow)))
	}

	if h.Admin != nil {
		// Every operator route is registered here and only here, behind Authenticate and then
		// RequireOperator: anonymous callers get 401, everyone but an operator 403.
		for _, rt := range AdminRoutes(h.Admin) {
			mux.Handle(rt.Pattern, h.Authenticate(middleware.RequireOperator(rt.Handler)))
		}
	}

	if h.Media != nil {
		// Uploads get their own deadline: the server's 15s Read/WriteTimeout is too short for a
		// multi-megabyte photo over a slow mobile connection.
		mux.Handle("POST /v1/me/images", middleware.ExtendDeadline(UploadTimeout)(h.Authenticate(http.HandlerFunc(h.Media.Upload))))
		if h.Media.HasFiles() {
			mux.HandleFunc("GET /media/{key...}", h.Media.Serve)
		}
	}

	return middleware.Tracing(middleware.Logger(middleware.Recover(middleware.CORS(allowedOrigins)(mux))))
}

// Route is a method pattern and the handler it serves.
type Route struct {
	Pattern string
	Handler http.HandlerFunc
}

// AdminRoutes lists every /v1/admin route. New wraps each one in Authenticate and RequireOperator,
// so an operator route cannot be added without the role check. a may be nil to list the patterns.
func AdminRoutes(a *admin.Handler) []Route {
	if a == nil {
		a = &admin.Handler{}
	}
	return []Route{
		{"GET /v1/admin/stores", a.ListStores},
		{"POST /v1/admin/stores/{id}/verify", a.VerifyStore},
		{"POST /v1/admin/stores/{id}/suspend", a.SuspendStore},
		{"POST /v1/admin/stores/{id}/reinstate", a.ReinstateStore},
		{"PATCH /v1/admin/products/{id}", a.UpdateProduct},
		{"POST /v1/admin/categories", a.CreateCategory},
		{"PATCH /v1/admin/categories/{id}", a.UpdateCategory},
		{"DELETE /v1/admin/categories/{id}", a.DeleteCategory},
		{"GET /v1/admin/ad-campaigns", a.ListAdCampaigns},
		{"POST /v1/admin/ad-campaigns", a.CreateAdCampaign},
		{"PATCH /v1/admin/ad-campaigns/{id}", a.UpdateAdCampaign},
		{"DELETE /v1/admin/ad-campaigns/{id}", a.DeleteAdCampaign},
		{"GET /v1/admin/orders", a.ListOrders},
		{"POST /v1/admin/seller-orders/{id}/accept", a.AcceptOrder},
		{"POST /v1/admin/seller-orders/{id}/reject", a.RejectOrder},
		{"POST /v1/admin/seller-orders/{id}/ready", a.ReadyOrder},
		{"POST /v1/admin/seller-orders/{id}/complete", a.CompleteOrder},
		{"POST /v1/admin/seller-orders/{id}/cancel", a.CancelOrder},
		{"DELETE /v1/admin/reviews/{id}", a.DeleteReview},
	}
}
