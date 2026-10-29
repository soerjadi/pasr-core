package otp

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *otpRepository) CountByPhoneSince(ctx context.Context, phone string, since time.Time) (int, error) {
	ctx, span := tracer.Start(ctx, "repository.otp.CountByPhoneSince")
	defer span.End()

	var n int
	if err := r.db.QueryRow(ctx, countByPhoneSinceQuery, phone, since).Scan(&n); err != nil {
		tracer.RecordError(span, err)
		return 0, fmt.Errorf("otp.CountByPhoneSince query: %w", err)
	}
	return n, nil
}
