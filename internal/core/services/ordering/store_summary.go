package ordering

import (
	"context"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

// latestOrderCount is how many recent orders the summary lists.
const latestOrderCount = 3

var allStatuses = []domain.OrderStatus{
	domain.OrderCreated, domain.OrderSellerProcessing, domain.OrderReadyForPickup, domain.OrderCompleted, domain.OrderCancelled,
}

func (s *service) StoreSummary(ctx context.Context, ownerID int64) (domain.StoreSummaryResponse, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.StoreSummary")
	defer span.End()

	st, err := s.stores.FindByOwner(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreSummaryResponse{}, err
	}
	if st == nil {
		return domain.StoreSummaryResponse{}, noStore()
	}
	sum, err := s.stores.GetSummaryByOwner(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreSummaryResponse{}, err
	}
	if sum == nil {
		return domain.StoreSummaryResponse{}, noStore()
	}
	now := s.clock.Now().In(s.cfg.Location)
	from, to := dayBounds(now)
	created, err := s.stats.CountCreated(ctx, st.ID, from, to)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreSummaryResponse{}, err
	}
	days, err := s.stats.CompletedByDay(ctx, st.ID, from, to, s.cfg.Location.String())
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreSummaryResponse{}, err
	}
	var sales int64
	for _, d := range days {
		sales += d.Sales
	}
	latest, err := s.orders.ListStoreOrders(ctx, st.ID, allStatuses, 0, latestOrderCount)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreSummaryResponse{}, err
	}
	for i := range latest {
		s.presentStore(&latest[i])
	}
	return domain.StoreSummaryResponse{
		IsOpen:        st.IsOpen,
		ClosesAt:      closesAt(st.Hours, now),
		NewOrderCount: sum.NewOrderCount,
		AcceptBy:      sum.AcceptBy,
		OrdersToday:   created,
		SalesToday:    sales,
		LatestOrders:  latest,
	}, nil
}

func noStore() error {
	return domain.NewError(domain.ErrConflict, "NO_STORE", "you do not have a store yet")
}

// dayBounds returns the instants [start of now's calendar day, start of the next day) in now's zone.
func dayBounds(now time.Time) (time.Time, time.Time) {
	y, m, d := now.Date()
	from := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	return from, time.Date(y, m, d+1, 0, 0, 0, 0, now.Location())
}

// closesAt is today's closing time ("HH:MM") from hours (Monday first), nil when the hours are
// unset or the store is closed today.
func closesAt(h domain.Hours, now time.Time) *string {
	if len(h) != 7 {
		return nil
	}
	day := h[(int(now.Weekday())+6)%7]
	if day.Closed || day.Close == "" {
		return nil
	}
	c := day.Close
	return &c
}
