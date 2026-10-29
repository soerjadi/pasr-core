package review

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *reviewRepository) ListByStore(ctx context.Context, storeID, beforeID int64, limit int) ([]domain.Review, error) {
	ctx, span := tracer.Start(ctx, "repository.review.ListByStore")
	defer span.End()

	rows, err := r.db.Query(ctx, listByStoreQuery, storeID, beforeID, limit)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("review.ListByStore query: %w", err)
	}
	defer rows.Close()

	items := []domain.Review{}
	for rows.Next() {
		var v domain.Review
		if err := rows.Scan(&v.ID, &v.Rating, &v.Text, &v.AuthorName, &v.CreatedAt); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("review.ListByStore scan: %w", err)
		}
		v.CreatedAt = v.CreatedAt.UTC()
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("review.ListByStore rows: %w", err)
	}
	return items, nil
}
