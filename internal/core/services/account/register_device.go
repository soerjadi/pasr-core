package account

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const maxFCMTokenRunes = 4096

func (s *service) RegisterDevice(ctx context.Context, userID int64, fcmToken string) error {
	ctx, span := tracer.Start(ctx, "service.account.RegisterDevice")
	defer span.End()

	fcmToken = strings.TrimSpace(fcmToken)
	if n := utf8.RuneCountInString(fcmToken); n < 1 || n > maxFCMTokenRunes {
		err := invalid("fcmToken is required")
		tracer.RecordError(span, err)
		return err
	}
	if err := s.devices.Upsert(ctx, userID, fcmToken); err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}
