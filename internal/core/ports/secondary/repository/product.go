package repository

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type ProductRepository interface {
	// List returns visible products newest first (id DESC); q.Limit rows are fetched.
	List(ctx context.Context, q domain.ProductQuery) ([]domain.ProductRecord, error)
	// GetVisible returns nil, nil unless the product is publicly visible. viewerID 0 is anonymous.
	GetVisible(ctx context.Context, id, viewerID int64) (*domain.ProductRecord, error)
	// Images returns the image keys of each product in display order, in one query.
	Images(ctx context.Context, productIDs []int64) (map[int64][]domain.ImageKeys, error)

	// Create inserts a product and returns the stored row.
	Create(ctx context.Context, r domain.SellerProductRecord) (domain.SellerProductRecord, error)
	// GetOwned returns nil, nil unless the non-deleted product belongs to storeID. lock takes a
	// row lock (FOR UPDATE) for a read-modify-write inside a transaction.
	GetOwned(ctx context.Context, storeID, id int64, lock bool) (*domain.SellerProductRecord, error)
	// ListOwned pages a store's non-deleted products, hidden ones included, newest first.
	ListOwned(ctx context.Context, q domain.SellerProductQuery) ([]domain.SellerProductRecord, error)
	// CountOwned counts a store's non-deleted products by status in one grouped query.
	CountOwned(ctx context.Context, storeID int64) (domain.ProductCounts, error)
	// Update writes name, price, stock, description and status of the product.
	Update(ctx context.Context, r domain.SellerProductRecord) error
	// ReplaceImages sets the product's images to the uploads, in order.
	ReplaceImages(ctx context.Context, productID int64, uploadIDs []int64) error
	// SoftDelete sets deleted_at on the store's product and reports whether a row was deleted.
	SoftDelete(ctx context.Context, storeID, id int64) (bool, error)
	// RemoveFavorites deletes every favorite of the product.
	RemoveFavorites(ctx context.Context, productID int64) error
}
