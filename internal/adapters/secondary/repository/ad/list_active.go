package ad

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *adRepository) ListActive(ctx context.Context, placement string, now time.Time) ([]domain.Ad, error) {
	ctx, span := tracer.Start(ctx, "repository.ad.ListActive")
	defer span.End()

	rows, err := r.db.Query(ctx, listActiveQuery, placement, now)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("ad.ListActive query: %w", err)
	}
	defer rows.Close()

	items := []domain.Ad{}
	for rows.Next() {
		var a domain.Ad
		if err := rows.Scan(&a.ID, &a.Placement, &a.Title, &a.Subtitle, &a.Image, &a.TargetURL, &a.Label); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("ad.ListActive scan: %w", err)
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("ad.ListActive rows: %w", err)
	}
	return items, nil
}
