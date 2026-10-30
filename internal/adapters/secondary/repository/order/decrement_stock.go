package order

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) DecrementStock(ctx context.Context, productID int64, qty int32) (bool, error) {
	ctx, span := tracer.Start(ctx, "repository.order.DecrementStock")
	defer span.End()

	tag, err := r.db.Exec(ctx, decrementStockQuery, productID, qty)
	if err != nil {
		tracer.RecordError(span, err)
		return false, fmt.Errorf("order.DecrementStock exec: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
