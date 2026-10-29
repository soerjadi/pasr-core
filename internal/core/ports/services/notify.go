package services

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type NotifyService interface {
	// Enqueue stores the notification through the executor in ctx, so inside a caller's transaction it commits
	// or rolls back with the state change. Empty Title and Body are filled from notify.Message.
	Enqueue(ctx context.Context, n domain.Notification) error
	// Dispatch sends one batch of queued notifications to all of each user's devices.
	Dispatch(ctx context.Context) error
}
