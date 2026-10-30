package ordering

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) ListOrders(ctx context.Context, buyerID int64, tab, cursor string, limit int) (domain.Page[domain.Order], error) {
	ctx, span := tracer.Start(ctx, "service.ordering.ListOrders")
	defer span.End()

	if tab == "" {
		tab = "ALL"
	}
	statuses, err := domain.TabStatuses(tab)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.Order]{}, err
	}
	afterID, err := domain.DecodeCursor(cursor)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.Order]{}, err
	}
	limit = domain.ClampLimit(limit)
	orders, err := s.orders.List(ctx, buyerID, statuses, afterID, limit+1)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.Order]{}, err
	}
	for i := range orders {
		s.present(&orders[i])
	}
	return domain.BuildPage(orders, limit, func(o domain.Order) int64 { return o.ID }), nil
}
