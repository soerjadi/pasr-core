package notify

import (
	"context"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/push"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const (
	batchSize   = 50
	maxAttempts = 5
	// sendTimeout bounds one Pusher.Send so a stuck provider cannot hold the claim transaction open.
	sendTimeout = 10 * time.Second
)

type service struct {
	notifs  repository.NotificationRepository
	devices repository.DeviceRepository
	pusher  push.Pusher
	tx      repository.TxManager

	batch       int
	sendTimeout time.Duration
}

func NewService(
	notifs repository.NotificationRepository,
	devices repository.DeviceRepository,
	pusher push.Pusher,
	tx repository.TxManager,
) services.NotifyService {
	return &service{notifs: notifs, devices: devices, pusher: pusher, tx: tx, batch: batchSize, sendTimeout: sendTimeout}
}

func (s *service) Enqueue(ctx context.Context, n domain.Notification) error {
	ctx, span := tracer.Start(ctx, "service.notify.Enqueue")
	defer span.End()

	if n.UserID <= 0 || !domain.ValidNotificationKind(n.Kind) {
		err := domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "notification needs a user and a known kind")
		tracer.RecordError(span, err)
		return err
	}
	if n.Title == "" || n.Body == "" {
		t, b := Message(n.Kind, n.Data)
		if n.Title == "" {
			n.Title = t
		}
		if n.Body == "" {
			n.Body = b
		}
	}
	if _, err := s.notifs.Insert(ctx, n); err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}
