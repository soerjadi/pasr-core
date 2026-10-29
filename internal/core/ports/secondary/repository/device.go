package repository

import "context"

type DeviceRepository interface {
	// Upsert registers the push token for the user; a token already held by another user moves to this one.
	Upsert(ctx context.Context, userID int64, fcmToken string) error
}
