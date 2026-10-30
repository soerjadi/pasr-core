package repository

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type ReviewRepository interface {
	// ListByStore returns the store's reviews newest first (id DESC); beforeID > 0 keeps only
	// reviews with a smaller id. limit rows are fetched.
	ListByStore(ctx context.Context, storeID, beforeID int64, limit int) ([]domain.Review, error)

	// Create inserts the review and returns it with the buyer's name ("Pembeli" when empty). A second
	// review of the same order item is a coded domain.ErrConflict "ALREADY_REVIEWED"; concurrent
	// inserts of the same item wait for each other, so exactly one succeeds.
	Create(ctx context.Context, r domain.NewReview) (domain.Review, error)
	// AddRating adds rating to the rating_sum and 1 to the rating_count of the product and the store.
	AddRating(ctx context.Context, productID, storeID int64, rating int32) error
}
