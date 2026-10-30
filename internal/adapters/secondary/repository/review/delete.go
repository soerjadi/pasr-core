package review

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *reviewRepository) Delete(ctx context.Context, id int64) (domain.DeletedReview, error) {
	ctx, span := tracer.Start(ctx, "repository.review.Delete")
	defer span.End()

	var d domain.DeletedReview
	err := r.db.QueryRow(ctx, deleteQuery, id).Scan(&d.ProductID, &d.StoreID, &d.Rating)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DeletedReview{}, domain.NewError(domain.ErrNotFound, "NOT_FOUND", "review not found")
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.DeletedReview{}, fmt.Errorf("review.Delete exec: %w", err)
	}
	return d, nil
}

func (r *reviewRepository) SubtractRating(ctx context.Context, productID, storeID int64, rating int32) error {
	ctx, span := tracer.Start(ctx, "repository.review.SubtractRating")
	defer span.End()

	var touched int
	if err := r.db.QueryRow(ctx, subtractRatingQuery, productID, storeID, rating).Scan(&touched); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("review.SubtractRating update: %w", err)
	}
	if touched != 2 {
		err := fmt.Errorf("review.SubtractRating touched %d rows, want 2 (product %d, store %d)", touched, productID, storeID)
		tracer.RecordError(span, err)
		return err
	}
	return nil
}
