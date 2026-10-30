package order

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) OverdueSellerOrders(ctx context.Context, now time.Time, limit int) ([]int64, error) {
	ctx, span := tracer.Start(ctx, "repository.order.OverdueSellerOrders")
	defer span.End()

	rows, err := r.db.Query(ctx, overdueQuery, now, limit)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("order.OverdueSellerOrders query: %w", err)
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("order.OverdueSellerOrders scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("order.OverdueSellerOrders rows: %w", err)
	}
	return ids, nil
}
