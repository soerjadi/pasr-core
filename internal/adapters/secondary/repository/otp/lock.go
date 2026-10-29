package otp

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *otpRepository) Lock(ctx context.Context, key string) error {
	ctx, span := tracer.Start(ctx, "repository.otp.Lock")
	defer span.End()

	if _, err := r.db.Exec(ctx, lockQuery, key); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("otp.Lock exec: %w", err)
	}
	return nil
}
