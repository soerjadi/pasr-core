package stats

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *statsRepository) CountCreated(ctx context.Context, storeID int64, from, to time.Time) (int32, error) {
	ctx, span := tracer.Start(ctx, "repository.stats.CountCreated")
	defer span.End()

	var n int32
	if err := r.db.QueryRow(ctx, countCreatedQuery, storeID, from, to).Scan(&n); err != nil {
		tracer.RecordError(span, err)
		return 0, fmt.Errorf("stats.CountCreated scan: %w", err)
	}
	return n, nil
}

func (r *statsRepository) CompletedByDay(ctx context.Context, storeID int64, from, to time.Time, zone string) ([]domain.DayTotal, error) {
	ctx, span := tracer.Start(ctx, "repository.stats.CompletedByDay")
	defer span.End()

	rows, err := r.db.Query(ctx, completedByDayQuery, storeID, from, to, zone)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("stats.CompletedByDay query: %w", err)
	}
	defer rows.Close()
	out := []domain.DayTotal{}
	for rows.Next() {
		var d domain.DayTotal
		if err := rows.Scan(&d.Date, &d.Sales, &d.Orders); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("stats.CompletedByDay scan: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("stats.CompletedByDay rows: %w", err)
	}
	return out, nil
}

func (r *statsRepository) TopProducts(ctx context.Context, storeID int64, from, to time.Time, limit int) ([]domain.TopProduct, error) {
	ctx, span := tracer.Start(ctx, "repository.stats.TopProducts")
	defer span.End()

	rows, err := r.db.Query(ctx, topProductsQuery, storeID, from, to, limit)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("stats.TopProducts query: %w", err)
	}
	defer rows.Close()
	out := []domain.TopProduct{}
	for rows.Next() {
		var p domain.TopProduct
		if err := rows.Scan(&p.ProductID, &p.Name, &p.UnitsSold); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("stats.TopProducts scan: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("stats.TopProducts rows: %w", err)
	}
	return out, nil
}
