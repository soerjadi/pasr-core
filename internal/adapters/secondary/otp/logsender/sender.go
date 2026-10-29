// Package logsender is the development OTP sender: it writes to the application log instead of sending an SMS.
package logsender

import (
	"context"

	log "github.com/oemahdev/logger"

	"github.com/oemah/nusantara/core/internal/core/ports/secondary/otp"
)

type sender struct {
	logCode bool
}

// New returns a sender that logs the masked phone. The code itself is logged only when env is "development".
func New(env string) otp.Sender {
	return &sender{logCode: env == "development"}
}

func (s *sender) Send(ctx context.Context, phone, code string) error {
	fields := []log.LogField{log.Field("phone", mask(phone))}
	if s.logCode {
		fields = append(fields, log.Field("code", code))
	}
	log.InfoCtx(ctx, "otp: code issued (log sender, nothing sent)", fields...)
	return nil
}

// mask keeps only the last four digits.
func mask(phone string) string {
	if len(phone) <= 4 {
		return "****"
	}
	return "****" + phone[len(phone)-4:]
}
