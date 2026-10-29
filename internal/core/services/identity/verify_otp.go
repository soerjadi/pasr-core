package identity

import (
	"context"
	"errors"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func errOTPInvalid() error {
	return domain.NewError(domain.ErrValidationFailed, "OTP_INVALID", "the code is invalid or has expired")
}

func (s *service) VerifyOTP(ctx context.Context, phone, code string) (domain.Session, error) {
	ctx, span := tracer.Start(ctx, "service.identity.VerifyOTP")
	defer span.End()

	norm, err := domain.NormalizePhone(phone)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Session{}, err
	}
	stored, err := s.otps.GetActive(ctx, norm)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Session{}, errOTPInvalid()
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Session{}, err
	}
	now := s.clock.Now()
	if !now.Before(stored.ExpiresAt) {
		return domain.Session{}, errOTPInvalid()
	}

	// Count the attempt atomically before comparing, so concurrent guesses cannot exceed the cap.
	attempts, err := s.otps.IncrementAttempts(ctx, stored.ID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Session{}, err
	}
	if attempts > s.cfg.OTPMaxAttempts {
		return domain.Session{}, errRateLimited()
	}
	if !hashEqual(stored.CodeHash, hashHMAC(s.cfg.OTPSecret, norm+":"+code)) {
		return domain.Session{}, errOTPInvalid()
	}

	role := domain.RoleBuyer
	if s.operators[norm] {
		role = domain.RoleOperator
	}
	var session domain.Session
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := s.otps.Consume(ctx, stored.ID, now)
		if err != nil {
			return err
		}
		if !ok {
			return errOTPInvalid() // lost the race to a concurrent verification
		}
		user, err := s.users.UpsertByPhone(ctx, norm, role)
		if err != nil {
			return err
		}
		familyID, err := newFamilyID()
		if err != nil {
			return err
		}
		session, err = s.issueSession(ctx, user, familyID, now)
		return err
	})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Session{}, err
	}
	return session, nil
}
