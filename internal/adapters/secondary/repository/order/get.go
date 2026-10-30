package order

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) Get(ctx context.Context, buyerID, orderID int64) (domain.Order, error) {
	ctx, span := tracer.Start(ctx, "repository.order.Get")
	defer span.End()

	o, err := scanOrder(r.db.QueryRow(ctx, getQuery, buyerID, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.NewError(domain.ErrNotFound, "NOT_FOUND", "order not found")
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Order{}, fmt.Errorf("order.Get scan: %w", err)
	}
	orders := []domain.Order{o}
	if err := r.loadDetails(ctx, orders); err != nil {
		tracer.RecordError(span, err)
		return domain.Order{}, err
	}
	return orders[0], nil
}
