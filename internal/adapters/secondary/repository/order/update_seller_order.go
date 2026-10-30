package order

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) UpdateSellerOrder(ctx context.Context, c domain.SellerOrderChange) error {
	ctx, span := tracer.Start(ctx, "repository.order.UpdateSellerOrder")
	defer span.End()

	tag, err := r.db.Exec(ctx, updateSellerOrderQuery, c.ID, c.Status, c.At, c.Note, c.CancelReason, c.CancelNote, c.CancelledBy)
	if err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("order.UpdateSellerOrder exec: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return notFound()
	}
	return nil
}
