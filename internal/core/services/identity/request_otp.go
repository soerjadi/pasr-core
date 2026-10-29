package identity

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func errRateLimited() error {
	return domain.NewError(domain.ErrTooManyRequests, "OTP_RATE_LIMITED", "too many attempts, try again later")
}

func (s *service) RequestOTP(ctx context.Context, phone, ip string) error {
	ctx, span := tracer.Start(ctx, "service.identity.RequestOTP")
	defer span.End()

	norm, err := domain.NormalizePhone(phone)
	if err != nil {
		tracer.RecordError(span, err)
		return err
	}
	now := s.clock.Now()
	since := now.Add(-s.cfg.OTPWindow)

	code, err := newCode()
	if err != nil {
		tracer.RecordError(span, err)
		return err
	}
	limited := false
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		// Serialise concurrent requests per phone and per IP (fixed order: phone, then IP) so the
		// counts below cannot be raced past the limit.
		if err := s.otps.Lock(ctx, "otp:phone:"+norm); err != nil {
			return err
		}
		if err := s.otps.Lock(ctx, "otp:ip:"+ip); err != nil {
			return err
		}
		byPhone, err := s.otps.CountByPhoneSince(ctx, norm, since)
		if err != nil {
			return err
		}
		byIP, err := s.otps.CountByIPSince(ctx, ip, since)
		if err != nil {
			return err
		}
		if byPhone >= s.cfg.OTPPhoneLimit || byIP >= s.cfg.OTPIPLimit {
			limited = true
			return nil
		}
		if err := s.otps.InvalidateActive(ctx, norm, now); err != nil {
			return err
		}
		return s.otps.Create(ctx, norm, ip, hashHMAC(s.cfg.OTPSecret, norm+":"+code), now.Add(s.cfg.OTPTTL), now)
	})
	if err != nil {
		tracer.RecordError(span, err)
		return err
	}
	if limited {
		err := errRateLimited()
		tracer.RecordError(span, err)
		return err
	}
	// Sent after commit so a code is never delivered for a row that was rolled back.
	if err := s.sender.Send(ctx, norm, code); err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}
