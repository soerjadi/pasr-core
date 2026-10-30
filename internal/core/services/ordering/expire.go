package ordering

import (
	"context"
	"errors"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

// expireBatch is how many overdue seller-orders one ExpireOverdue run handles.
const expireBatch = 100

var systemActor = domain.Actor{Role: domain.ActorSystem}

// ExpireOverdue picks overdue seller-orders with FOR UPDATE SKIP LOCKED (rows a seller is acting on
// right now are left for the next run), then expires each in its own transaction through apply,
// which re-checks status and deadline under the row lock. A seller-order accepted in between is
// skipped. One failing row does not stop the others.
func (s *service) ExpireOverdue(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.ExpireOverdue")
	defer span.End()

	var ids []int64
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := s.orders.SetStatementTimeout(ctx, txTimeout); err != nil {
			return err
		}
		var err error
		ids, err = s.orders.OverdueSellerOrders(ctx, s.clock.Now(), expireBatch)
		return err
	})
	if err != nil {
		tracer.RecordError(span, err)
		return 0, err
	}

	n := 0
	var errs []error
	for _, id := range ids {
		_, changed, err := s.apply(ctx, systemActor, id, domain.ActionExpire, params{reason: domain.ReasonExpired})
		var coded *domain.CodedError
		switch {
		case err == nil:
			if changed {
				n++
			}
		case errors.As(err, &coded) && coded.Code == "ORDER_NOT_ACTIONABLE":
			// accepted (or otherwise moved on) after it was picked
		default:
			errs = append(errs, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		tracer.RecordError(span, err)
		return n, err
	}
	return n, nil
}
