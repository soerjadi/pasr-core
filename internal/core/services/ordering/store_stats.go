package ordering

import (
	"context"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const (
	statsDays  = 7
	topProduct = 3
)

func (s *service) StoreStats(ctx context.Context, ownerID int64, rng string) (domain.StoreStats, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.StoreStats")
	defer span.End()

	if rng != domain.StatsToday && rng != domain.StatsWeek {
		err := invalid("range must be TODAY or WEEK")
		tracer.RecordError(span, err)
		return domain.StoreStats{}, err
	}
	storeID, err := s.storeOf(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreStats{}, err
	}
	now := s.clock.Now().In(s.cfg.Location)
	todayFrom, to := dayBounds(now)
	weekFrom := todayFrom.AddDate(0, 0, -(statsDays - 1))

	// Daily always covers the seven days; the totals and the ranking cover the requested range.
	zone := s.cfg.Location.String()
	days, err := s.stats.CompletedByDay(ctx, storeID, weekFrom, to, zone)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreStats{}, err
	}
	from := weekFrom
	if rng == domain.StatsToday {
		from = todayFrom
	}
	top, err := s.stats.TopProducts(ctx, storeID, from, to, topProduct)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreStats{}, err
	}

	byDate := make(map[string]domain.DayTotal, len(days))
	for _, d := range days {
		byDate[d.Date] = d
	}
	out := domain.StoreStats{Range: rng, Daily: make([]domain.DailyValue, 0, statsDays), TopProducts: top}
	todayKey := todayFrom.Format(time.DateOnly)
	for i := 0; i < statsDays; i++ {
		key := weekFrom.AddDate(0, 0, i).Format(time.DateOnly)
		d := byDate[key]
		out.Daily = append(out.Daily, domain.DailyValue{Date: key, Value: d.Sales})
		if rng == domain.StatsWeek || key == todayKey {
			out.SalesValue += d.Sales
			out.CompletedOrders += d.Orders
		}
	}
	return out, nil
}
