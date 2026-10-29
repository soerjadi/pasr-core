package repository

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type StorefrontRepository interface {
	// GetVisible returns nil, nil unless the store is VERIFIED. viewerID 0 is anonymous.
	GetVisible(ctx context.Context, id, viewerID int64) (*domain.SellerRecord, error)
}
