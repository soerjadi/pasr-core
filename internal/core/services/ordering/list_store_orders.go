package ordering

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) ListStoreOrders(ctx context.Context, ownerID int64, tab, cursor string, limit int) (domain.StoreOrderPage, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.ListStoreOrders")
	defer span.End()

	if tab == "" {
		tab = "NEW"
	}
	statuses, err := domain.SellerTabStatuses(tab)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreOrderPage{}, err
	}
	afterID, err := domain.DecodeCursor(cursor)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreOrderPage{}, err
	}
	storeID, err := s.storeOf(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreOrderPage{}, err
	}
	limit = domain.ClampLimit(limit)
	rows, err := s.orders.ListStoreOrders(ctx, storeID, statuses, afterID, limit+1)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreOrderPage{}, err
	}
	counts, err := s.orders.CountStoreOrders(ctx, storeID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreOrderPage{}, err
	}
	for i := range rows {
		s.presentStore(&rows[i])
	}
	page := domain.BuildPage(rows, limit, func(o domain.StoreOrder) int64 { return o.ID })
	return domain.StoreOrderPage{Items: page.Items, NextCursor: page.NextCursor, Counts: counts}, nil
}
