package otp

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *otpRepository) IncrementAttempts(ctx context.Context, id int64) (int, error) {
	ctx, span := tracer.Start(ctx, "repository.otp.IncrementAttempts")
	defer span.End()

	var n int
	if err := r.db.QueryRow(ctx, incrementAttemptsQuery, id).Scan(&n); err != nil {
		tracer.RecordError(span, err)
		return 0, fmt.Errorf("otp.IncrementAttempts query: %w", err)
	}
	return n, nil
}
