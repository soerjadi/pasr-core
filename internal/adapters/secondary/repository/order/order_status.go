package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) LockOrderStatuses(ctx context.Context, orderID int64) (domain.OrderStatus, []domain.OrderStatus, error) {
	ctx, span := tracer.Start(ctx, "repository.order.LockOrderStatuses")
	defer span.End()

	var current domain.OrderStatus
	err := r.db.QueryRow(ctx, lockOrderQuery, orderID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, notFound()
	}
	if err != nil {
		tracer.RecordError(span, err)
		return "", nil, fmt.Errorf("order.LockOrderStatuses lock: %w", err)
	}
	// A separate statement: under READ COMMITTED it sees sibling changes committed while it waited.
	rows, err := r.db.Query(ctx, sellerStatusesQuery, orderID)
	if err != nil {
		tracer.RecordError(span, err)
		return "", nil, fmt.Errorf("order.LockOrderStatuses statuses: %w", err)
	}
	defer rows.Close()
	var statuses []domain.OrderStatus
	for rows.Next() {
		var s domain.OrderStatus
		if err := rows.Scan(&s); err != nil {
			tracer.RecordError(span, err)
			return "", nil, fmt.Errorf("order.LockOrderStatuses scan: %w", err)
		}
		statuses = append(statuses, s)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return "", nil, fmt.Errorf("order.LockOrderStatuses rows: %w", err)
	}
	return current, statuses, nil
}

func (r *orderRepository) SetOrderStatus(ctx context.Context, orderID int64, status domain.OrderStatus, at time.Time) error {
	ctx, span := tracer.Start(ctx, "repository.order.SetOrderStatus")
	defer span.End()

	tag, err := r.db.Exec(ctx, setOrderStatusQuery, orderID, status, at)
	if err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("order.SetOrderStatus exec: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return notFound()
	}
	return nil
}
