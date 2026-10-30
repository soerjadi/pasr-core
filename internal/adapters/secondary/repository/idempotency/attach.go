package idempotency

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *idempotencyRepository) Attach(ctx context.Context, userID int64, key string, orderID int64) error {
	ctx, span := tracer.Start(ctx, "repository.idempotency.Attach")
	defer span.End()

	tag, err := r.db.Exec(ctx, attachQuery, userID, key, orderID)
	if err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("idempotency.Attach exec: %w", err)
	}
	if tag.RowsAffected() != 1 {
		err := fmt.Errorf("idempotency.Attach: key not claimed")
		tracer.RecordError(span, err)
		return err
	}
	return nil
}
