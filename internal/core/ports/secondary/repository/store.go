package repository

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type StoreRepository interface {
	// Create inserts a PENDING store. A second store for the owner fails with a coded
	// domain.ErrConflict "STORE_EXISTS"; an unknown category or photo with domain.ErrValidationFailed.
	Create(ctx context.Context, s domain.Store) (domain.StoreSummary, error)
	// GetSummaryByOwner returns nil, nil when the user has no store. NewOrderCount and AcceptBy
	// come from the owner's seller orders in status CREATED.
	GetSummaryByOwner(ctx context.Context, ownerID int64) (*domain.StoreSummary, error)
}
