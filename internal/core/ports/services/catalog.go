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
	// PurgeOrphanUploads deletes unattached uploads older than the retention period from the
	// database and storage, returning how many uploads were removed.
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
}
