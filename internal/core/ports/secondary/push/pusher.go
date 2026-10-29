package push

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// Pusher delivers a notification to one device token.
type Pusher interface {
	// Send returns unregistered=true (with a nil error) when the provider says the token is no longer valid,
	// so the caller can delete it. Any other failure is returned as err.
	Send(ctx context.Context, token string, n domain.Notification) (unregistered bool, err error)
}
