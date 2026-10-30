package order

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) NextOrderNumber(ctx context.Context, day time.Time) (string, error) {
	ctx, span := tracer.Start(ctx, "repository.order.NextOrderNumber")
	defer span.End()

	// The calendar date in day's own location, passed as text so no time zone conversion applies.
	date := day.Format(time.DateOnly)
	var seq int
	if err := r.db.QueryRow(ctx, nextNumberQuery, date).Scan(&seq); err != nil {
		tracer.RecordError(span, err)
		return "", fmt.Errorf("order.NextOrderNumber upsert: %w", err)
	}
	return domain.FormatOrderNumber(day, seq), nil
}
