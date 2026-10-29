package repository

import (
	"context"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type OTPRepository interface {
	// Lock takes a transaction-scoped advisory lock on key; call it inside TxManager.WithTx.
	Lock(ctx context.Context, key string) error
	Create(ctx context.Context, phone, ip, codeHash string, expiresAt, now time.Time) error
	// InvalidateActive consumes every unconsumed code of the phone.
	InvalidateActive(ctx context.Context, phone string, now time.Time) error
	CountByPhoneSince(ctx context.Context, phone string, since time.Time) (int, error)
	CountByIPSince(ctx context.Context, ip string, since time.Time) (int, error)
	// GetActive returns the newest unconsumed code, or domain.ErrNotFound.
	GetActive(ctx context.Context, phone string) (domain.OTPCode, error)
	// IncrementAttempts atomically adds one attempt and returns the new count.
	IncrementAttempts(ctx context.Context, id int64) (int, error)
	// Consume marks the code used; it reports false when it was already consumed.
	Consume(ctx context.Context, id int64, now time.Time) (bool, error)
}
