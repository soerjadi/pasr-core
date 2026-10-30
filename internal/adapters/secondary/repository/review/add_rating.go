package review

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *reviewRepository) AddRating(ctx context.Context, productID, storeID int64, rating int32) error {
	ctx, span := tracer.Start(ctx, "repository.review.AddRating")
	defer span.End()

	var touched int
	if err := r.db.QueryRow(ctx, addRatingQuery, productID, storeID, rating).Scan(&touched); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("review.AddRating update: %w", err)
	}
	if touched != 2 {
		err := fmt.Errorf("review.AddRating touched %d rows, want 2 (product %d, store %d)", touched, productID, storeID)
		tracer.RecordError(span, err)
		return err
	}
	return nil
}
