package review

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const pgUniqueViolation = "23505"

func (r *reviewRepository) Create(ctx context.Context, n domain.NewReview) (domain.Review, error) {
	ctx, span := tracer.Start(ctx, "repository.review.Create")
	defer span.End()

	var v domain.Review
	err := r.db.QueryRow(ctx, createQuery, n.OrderItemID, n.ProductID, n.StoreID, n.BuyerID, n.Rating, n.Text).
		Scan(&v.ID, &v.Rating, &v.Text, &v.AuthorName, &v.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.Review{}, domain.NewError(domain.ErrConflict, "ALREADY_REVIEWED", "this item was already reviewed")
		}
		tracer.RecordError(span, err)
		return domain.Review{}, fmt.Errorf("review.Create insert: %w", err)
	}
	v.CreatedAt = v.CreatedAt.UTC()
	return v, nil
}
