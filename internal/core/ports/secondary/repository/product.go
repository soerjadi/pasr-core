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
}
