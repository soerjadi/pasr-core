package services

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// StoreManagementService lets a seller manage the products of their own store. The store is
// found from the owner's user id: without one every method fails with a coded
// domain.ErrConflict "NO_STORE", and a product of another store is domain.ErrNotFound.
type StoreManagementService interface {
	// CreateProduct adds a product; it is allowed while the store is still PENDING. Stock 0
	// creates it OUT_OF_STOCK. photoIds (max 5, first is primary) must be the caller's unattached uploads.
	CreateProduct(ctx context.Context, ownerID int64, in domain.ProductInput) (domain.SellerProduct, error)
	// ListMyProducts pages the store's products newest first, hidden ones included. status ""
	// means all. Counts are the store totals regardless of status and cursor.
	ListMyProducts(ctx context.Context, ownerID int64, status string, cursor string, limit int) (domain.SellerProductPage, error)
	GetMyProduct(ctx context.Context, ownerID, productID int64) (domain.SellerProduct, error)
	// UpdateMyProduct applies the non-nil fields. Stock reaching 0 sets OUT_OF_STOCK; a restock
	// above 0 returns an OUT_OF_STOCK product to AVAILABLE unless the patch sets status itself;
	// AVAILABLE with stock 0 is rejected. photoIds replace the whole image list.
	UpdateMyProduct(ctx context.Context, ownerID, productID int64, patch domain.ProductPatch) (domain.SellerProduct, error)
	// DeleteMyProduct soft-deletes the product and removes it from every favorites list.
	DeleteMyProduct(ctx context.Context, ownerID, productID int64) error
}
