package otp

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *otpRepository) GetActive(ctx context.Context, phone string) (domain.OTPCode, error) {
	ctx, span := tracer.Start(ctx, "repository.otp.GetActive")
	defer span.End()

	var c domain.OTPCode
	err := r.db.QueryRow(ctx, getActiveQuery, phone).Scan(&c.ID, &c.Phone, &c.CodeHash, &c.ExpiresAt, &c.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OTPCode{}, domain.ErrNotFound
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.OTPCode{}, fmt.Errorf("otp.GetActive query: %w", err)
	}
	return c, nil
}
