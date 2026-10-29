package repository

import "context"

type DeviceRepository interface {
	// Upsert registers the push token for the user; a token already held by another user moves to this one.
	Upsert(ctx context.Context, userID int64, fcmToken string) error
	// ListTokens returns the user's push tokens.
	ListTokens(ctx context.Context, userID int64) ([]string, error)
	// DeleteToken removes a token the provider reported as unregistered.
	DeleteToken(ctx context.Context, fcmToken string) error
}
