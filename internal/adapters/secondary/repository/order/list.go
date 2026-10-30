package order

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) List(ctx context.Context, buyerID int64, statuses []domain.OrderStatus, afterID int64, limit int) ([]domain.Order, error) {
	ctx, span := tracer.Start(ctx, "repository.order.List")
	defer span.End()

	var filter []string // nil means no filter
	for _, s := range statuses {
		filter = append(filter, string(s))
	}
	rows, err := r.db.Query(ctx, listQuery, buyerID, filter, afterID, limit)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("order.List query: %w", err)
	}
	defer rows.Close()
	orders := []domain.Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("order.List scan: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("order.List rows: %w", err)
	}
	if err := r.loadDetails(ctx, orders); err != nil {
		tracer.RecordError(span, err)
		return nil, err
	}
	return orders, nil
}
