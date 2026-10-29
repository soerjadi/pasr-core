package otp

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *otpRepository) Consume(ctx context.Context, id int64, now time.Time) (bool, error) {
	ctx, span := tracer.Start(ctx, "repository.otp.Consume")
	defer span.End()

	tag, err := r.db.Exec(ctx, consumeQuery, id, now)
	if err != nil {
		tracer.RecordError(span, err)
		return false, fmt.Errorf("otp.Consume exec: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
