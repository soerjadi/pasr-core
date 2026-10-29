package otp

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *otpRepository) CountByIPSince(ctx context.Context, ip string, since time.Time) (int, error) {
	ctx, span := tracer.Start(ctx, "repository.otp.CountByIPSince")
	defer span.End()

	var n int
	if err := r.db.QueryRow(ctx, countByIPSinceQuery, ip, since).Scan(&n); err != nil {
		tracer.RecordError(span, err)
		return 0, fmt.Errorf("otp.CountByIPSince query: %w", err)
	}
	return n, nil
}
