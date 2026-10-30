package services

import (
	"context"
	"io"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type CatalogService interface {
	ListCategories(ctx context.Context) ([]domain.Category, error)
	// UploadImage validates r as a JPEG, PNG or WebP image, stores three JPEG variants and
	// records them as an unattached upload owned by ownerID.
	UploadImage(ctx context.Context, ownerID int64, r io.Reader) (domain.ImageUpload, error)
	// PurgeOrphanUploads deletes unattached uploads, and attached ones nothing refers to any more,
	// older than the retention period from the database and storage, returning how many were removed.
	PurgeOrphanUploads(ctx context.Context) (int, error)
	// ListProducts pages publicly visible products newest first. A non-empty q shorter than
	// 2 characters fails with domain.ErrValidationFailed.
	ListProducts(ctx context.Context, f domain.ProductFilter) (domain.Page[domain.ProductSummary], error)
	// GetProduct returns a visible product, else domain.ErrNotFound. viewerID 0 is anonymous
	// and leaves IsFavorite nil.
	GetProduct(ctx context.Context, id, viewerID int64) (domain.ProductDetail, error)
	// GetSeller returns a verified seller, else domain.ErrNotFound. viewerID 0 is anonymous
	// and leaves IsFollowing nil.
	GetSeller(ctx context.Context, id, viewerID int64) (domain.SellerDetail, error)
	// ListSellerReviews pages a verified seller's reviews newest first.
	ListSellerReviews(ctx context.Context, sellerID int64, cursor string, limit int) (domain.Page[domain.Review], error)

	// ListFavorites pages the user's favorites newest first, omitting products that are no
	// longer publicly visible.
	ListFavorites(ctx context.Context, userID int64, cursor string, limit int) (domain.Page[domain.ProductSummary], error)
	// AddFavorite is idempotent; a product that is not publicly visible is domain.ErrNotFound.
	AddFavorite(ctx context.Context, userID, productID int64) error
	// RemoveFavorite is idempotent.
	RemoveFavorite(ctx context.Context, userID, productID int64) error
	// Follow is idempotent; a store that is not verified is domain.ErrNotFound and the user's
	// own store is a coded domain.ErrConflict "CANNOT_FOLLOW_OWN_STORE".
	Follow(ctx context.Context, userID, storeID int64) error
	// Unfollow is idempotent.
	Unfollow(ctx context.Context, userID, storeID int64) error
}
