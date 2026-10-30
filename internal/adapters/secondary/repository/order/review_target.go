package order

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) GetReviewTarget(ctx context.Context, buyerID, orderItemID int64) (domain.ReviewTarget, error) {
	ctx, span := tracer.Start(ctx, "repository.order.GetReviewTarget")
	defer span.End()

	var t domain.ReviewTarget
	err := r.db.QueryRow(ctx, reviewTargetQuery, buyerID, orderItemID).Scan(&t.ItemID, &t.ProductID, &t.StoreID, &t.SellerOrderStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ReviewTarget{}, domain.NewError(domain.ErrNotFound, "NOT_FOUND", "order item not found")
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.ReviewTarget{}, fmt.Errorf("order.GetReviewTarget scan: %w", err)
	}
	return t, nil
}
