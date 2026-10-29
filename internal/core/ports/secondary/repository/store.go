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
	// GetRefByOwner returns the owner's store id and category, or nil, nil without a store.
	GetRefByOwner(ctx context.Context, ownerID int64) (*domain.StoreRef, error)
	// FindByOwner returns the owner's full store row with its photo keys, or nil, nil without a store.
	FindByOwner(ctx context.Context, ownerID int64) (*domain.MyStoreRecord, error)
	// Update changes only the non-nil fields in one statement, so concurrent patches of different
	// fields do not overwrite each other. An unknown category is domain.ErrValidationFailed.
	Update(ctx context.Context, storeID int64, u domain.StoreUpdate) error
	// SetOpen stores the open flag.
	SetOpen(ctx context.Context, storeID int64, open bool) error
}
