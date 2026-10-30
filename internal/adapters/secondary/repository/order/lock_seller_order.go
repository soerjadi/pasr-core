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

func (r *orderRepository) LockSellerOrder(ctx context.Context, id, storeID int64) (domain.SellerOrderLock, error) {
	ctx, span := tracer.Start(ctx, "repository.order.LockSellerOrder")
	defer span.End()

	var l domain.SellerOrderLock
	var acceptBy *time.Time
	err := r.db.QueryRow(ctx, lockSellerOrderQuery, id, storeID).Scan(&l.ID, &l.OrderID, &l.OrderNumber, &l.BuyerID,
		&l.StoreID, &l.StoreName, &l.OwnerID, &l.Status, &acceptBy, &l.CancelReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SellerOrderLock{}, notFound()
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerOrderLock{}, fmt.Errorf("order.LockSellerOrder scan: %w", err)
	}
	l.AcceptBy = utc(acceptBy)
	return l, nil
}

func notFound() error {
	return domain.NewError(domain.ErrNotFound, "NOT_FOUND", "order not found")
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
