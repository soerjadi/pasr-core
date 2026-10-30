package order

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) RestoreStock(ctx context.Context, sellerOrderID int64) error {
	ctx, span := tracer.Start(ctx, "repository.order.RestoreStock")
	defer span.End()

	rows, err := r.db.Query(ctx, lockRestoreProductsQuery, sellerOrderID)
	if err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("order.RestoreStock lock: %w", err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("order.RestoreStock lock rows: %w", err)
	}
	if _, err := r.db.Exec(ctx, restoreStockQuery, sellerOrderID); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("order.RestoreStock exec: %w", err)
	}
	return nil
}
