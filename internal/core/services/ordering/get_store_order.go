package ordering

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) GetStoreOrder(ctx context.Context, ownerID, id int64) (domain.StoreOrder, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.GetStoreOrder")
	defer span.End()

	storeID, err := s.storeOf(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreOrder{}, err
	}
	out, err := s.orders.GetStoreOrder(ctx, storeID, id)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreOrder{}, err
	}
	s.presentStore(&out)
	return out, nil
}
