package order

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *orderRepository) SetStatementTimeout(ctx context.Context, d time.Duration) error {
	ctx, span := tracer.Start(ctx, "repository.order.SetStatementTimeout")
	defer span.End()

	if _, err := r.db.Exec(ctx, statementTimeoutQuery, fmt.Sprintf("%dms", d.Milliseconds())); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("order.SetStatementTimeout exec: %w", err)
	}
	return nil
}
