package otp

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *otpRepository) Create(ctx context.Context, phone, ip, codeHash string, expiresAt, now time.Time) error {
	ctx, span := tracer.Start(ctx, "repository.otp.Create")
	defer span.End()

	if _, err := r.db.Exec(ctx, createQuery, phone, ip, codeHash, expiresAt, now); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("otp.Create exec: %w", err)
	}
	return nil
}
