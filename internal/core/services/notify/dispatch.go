package notify

import (
	"context"
	"strings"

	log "github.com/oemahdev/logger"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

// Dispatch claims a batch inside one transaction (FOR UPDATE SKIP LOCKED, so concurrent dispatchers take
// disjoint rows), sends each notification to every device of its user and records the outcome.
// A notification counts as delivered when the user has no devices or at least one send succeeded;
// otherwise attempts is incremented and, after maxAttempts, it is no longer claimed.
func (s *service) Dispatch(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "service.notify.Dispatch")
	defer span.End()

	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		batch, err := s.notifs.ClaimUnsent(ctx, batchSize, maxAttempts)
		if err != nil {
			return err
		}
		for _, n := range batch {
			if err := s.deliver(ctx, n); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}

func (s *service) deliver(ctx context.Context, n domain.Notification) error {
	tokens, err := s.devices.ListTokens(ctx, n.UserID)
	if err != nil {
		return err
	}
	var sent int
	var sendErrs []string
	for _, tok := range tokens {
		unregistered, err := s.pusher.Send(ctx, tok, n)
		switch {
		case err != nil:
			sendErrs = append(sendErrs, err.Error())
		case unregistered:
			if derr := s.devices.DeleteToken(ctx, tok); derr != nil {
				return derr
			}
		default:
			sent++
		}
	}
	if len(sendErrs) > 0 && sent == 0 {
		msg := strings.Join(sendErrs, "; ")
		log.WarnCtx(ctx, "notification send failed", log.Field("notificationId", n.ID), log.Field("kind", n.Kind))
		return s.notifs.MarkFailed(ctx, n.ID, msg)
	}
	if err := s.notifs.MarkSent(ctx, n.ID); err != nil {
		return err
	}
	return nil
}
