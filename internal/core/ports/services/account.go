package services

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type AccountService interface {
	GetMe(ctx context.Context, userID int64) (domain.User, error)
	UpdateMe(ctx context.Context, userID int64, name string) (domain.User, error)
	// RegisterDevice upserts the push token and reassigns it to the current user.
	RegisterDevice(ctx context.Context, userID int64, fcmToken string) error
	// CreateStore opens a PENDING store for the user; a second store is 409 STORE_EXISTS.
	CreateStore(ctx context.Context, userID int64, in domain.CreateStoreInput) (domain.StoreSummary, error)
}
