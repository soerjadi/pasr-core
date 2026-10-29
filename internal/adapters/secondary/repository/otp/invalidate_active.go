package otp

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *otpRepository) InvalidateActive(ctx context.Context, phone string, now time.Time) error {
	ctx, span := tracer.Start(ctx, "repository.otp.InvalidateActive")
	defer span.End()

	if _, err := r.db.Exec(ctx, invalidateActiveQuery, phone, now); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("otp.InvalidateActive exec: %w", err)
	}
	return nil
}
