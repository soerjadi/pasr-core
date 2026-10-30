package ordering

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) GetOrder(ctx context.Context, buyerID, orderID int64) (domain.Order, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.GetOrder")
	defer span.End()

	o, err := s.orders.Get(ctx, buyerID, orderID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Order{}, err
	}
	s.present(&o)
	return o, nil
}
