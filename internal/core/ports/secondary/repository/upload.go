package repository

import (
	"context"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type UploadRepository interface {
	Create(ctx context.Context, ownerID int64, keys domain.ImageKeys) (int64, error)
	// ResolveOwned returns the keys of the uploads; every id must be owned by ownerID and still
	// unattached, otherwise it fails with domain.ErrValidationFailed.
	ResolveOwned(ctx context.Context, ownerID int64, ids []int64) (map[int64]domain.ImageKeys, error)
	MarkAttached(ctx context.Context, ids []int64) error
	// DeleteOrphans removes unattached uploads created before olderThan and returns their keys.
	DeleteOrphans(ctx context.Context, olderThan time.Time) ([]domain.ImageKeys, error)
	// DeleteDetached removes attached uploads created before olderThan that no product image, store
	// photo or order-item image snapshot refers to any more, and returns their keys.
	DeleteDetached(ctx context.Context, olderThan time.Time) ([]domain.ImageKeys, error)
}
