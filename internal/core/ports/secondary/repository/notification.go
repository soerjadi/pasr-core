package repository

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type NotificationRepository interface {
	// Insert queues the notification (through the executor, so it joins a transaction in ctx) and returns its id.
	Insert(ctx context.Context, n domain.Notification) (int64, error)
	// ClaimUnsent returns up to limit unsent notifications with fewer than maxAttempts failed attempts,
	// oldest first, locking them with FOR UPDATE SKIP LOCKED. It must run inside a transaction.
	ClaimUnsent(ctx context.Context, limit, maxAttempts int) ([]domain.Notification, error)
	MarkSent(ctx context.Context, id int64) error
	// MarkFailed increments attempts and stores the error text.
	MarkFailed(ctx context.Context, id int64, lastError string) error
}
