package services

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// AdminService is the operator's toolset: one method per /v1/admin route. Callers must have
// checked the operator role; the service does not know who is calling unless an actor is passed.
type AdminService interface {
	// ListStores pages every store newest first. status "" means all, otherwise PENDING, VERIFIED
	// or SUSPENDED; another value or a malformed cursor is domain.ErrValidationFailed.
	ListStores(ctx context.Context, status, cursor string, limit int) (domain.Page[domain.AdminStore], error)
	// VerifyStore moves a PENDING store to VERIFIED and queues STORE_VERIFIED to its owner in the
	// same transaction. An already VERIFIED store is returned unchanged (no push); a SUSPENDED one
	// is 409 STORE_STATE_INVALID (use ReinstateStore). An unknown id is domain.ErrNotFound.
	VerifyStore(ctx context.Context, id int64) (domain.AdminStore, error)
	// SuspendStore moves a PENDING or VERIFIED store to SUSPENDED, hiding it and its products and
	// blocking new orders. An already SUSPENDED store is returned unchanged.
	SuspendStore(ctx context.Context, id int64) (domain.AdminStore, error)
	// ReinstateStore moves a SUSPENDED store back to VERIFIED. An already VERIFIED store is returned
	// unchanged; a PENDING one is 409 STORE_STATE_INVALID, since it was never verified.
	ReinstateStore(ctx context.Context, id int64) (domain.AdminStore, error)

	// UpdateProduct applies the non-nil fields to a non-deleted product of any store (else
	// domain.ErrNotFound). status must be AVAILABLE, OUT_OF_STOCK or HIDDEN, and AVAILABLE needs
	// stock; halalCertNo is trimmed, at most 100 characters, and "" clears it.
	UpdateProduct(ctx context.Context, id int64, patch domain.AdminProductPatch) (domain.AdminProduct, error)

	// CreateCategory adds a category: name (at most 60 characters) and iconKey (at most 40) are
	// required, trimmed and free of NUL characters, else domain.ErrValidationFailed.
	CreateCategory(ctx context.Context, in domain.CategoryInput) (domain.Category, error)
	// UpdateCategory applies the non-nil fields under the CreateCategory rules; unknown ids are
	// domain.ErrNotFound.
	UpdateCategory(ctx context.Context, id int64, in domain.CategoryInput) (domain.Category, error)
	// DeleteCategory removes a category no store or product (deleted ones included) uses; one in use
	// is 409 CATEGORY_IN_USE.
	DeleteCategory(ctx context.Context, id int64) error

	// ListAdCampaigns pages every campaign newest first, inactive and out-of-window ones included.
	ListAdCampaigns(ctx context.Context, cursor string, limit int) (domain.Page[domain.AdCampaign], error)
	// CreateAdCampaign stores a campaign; active defaults to true and priority to 0. placement must
	// be a known key, advertiser, title and label non-blank, targetUrl and the three image URLs
	// absolute https, and startsAt before endsAt when both are set, else domain.ErrValidationFailed.
	CreateAdCampaign(ctx context.Context, in domain.AdCampaignInput) (domain.AdCampaign, error)
	// UpdateAdCampaign merges the provided fields into the campaign and validates the result under
	// the CreateAdCampaign rules; an unknown id is domain.ErrNotFound.
	UpdateAdCampaign(ctx context.Context, id int64, in domain.AdCampaignInput) (domain.AdCampaign, error)
	// DeleteAdCampaign removes a campaign; an unknown id is domain.ErrNotFound.
	DeleteAdCampaign(ctx context.Context, id int64) error

	// ListOrders pages the seller-orders of every store newest first; see OrderingService.ListAllStoreOrders.
	ListOrders(ctx context.Context, status, cursor string, limit int) (domain.Page[domain.AdminOrder], error)
	// AcceptOrder, RejectOrder, ReadyOrder, CompleteOrder and CancelOrder act on any store's
	// seller-order as operatorID through the OrderingService (Actor{Role: operator}), so they follow
	// exactly the seller rules, notifications and stock handling.
	AcceptOrder(ctx context.Context, operatorID, id int64) (domain.StoreOrder, error)
	RejectOrder(ctx context.Context, operatorID, id int64, reason, note string) (domain.StoreOrder, error)
	ReadyOrder(ctx context.Context, operatorID, id int64, note string) (domain.StoreOrder, error)
	CompleteOrder(ctx context.Context, operatorID, id int64) (domain.StoreOrder, error)
	CancelOrder(ctx context.Context, operatorID, id int64, reason, note string) (domain.StoreOrder, error)

	// DeleteReview removes a review and subtracts its rating from the product and store aggregates
	// in one transaction; an unknown id is domain.ErrNotFound.
	DeleteReview(ctx context.Context, id int64) error
}
