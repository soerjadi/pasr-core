package repository

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type ReviewRepository interface {
	// ListByStore returns the store's reviews newest first (id DESC); beforeID > 0 keeps only
	// reviews with a smaller id. limit rows are fetched.
	ListByStore(ctx context.Context, storeID, beforeID int64, limit int) ([]domain.Review, error)
}
