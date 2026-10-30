package services

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// AdminService is the operator's toolset: one method per /v1/admin route. Callers must have
// checked the operator role; the service does not know who is calling unless an actor is passed.
type AdminService interface {
	// ListStores pages every store newest first. status "" means all, otherwise PENDING, VERIFIED
	// or SUSPENDED; another value or a malformed cursor is domain.ErrValidationFailed.
	ListStores(ctx context.Context, status, cursor string, limit int) (domain.Page[domain.AdminStore], error)
	// VerifyStore moves a PENDING store to VERIFIED and queues STORE_VERIFIED to its owner in the
	// same transaction. An already VERIFIED store is returned unchanged (no push); a SUSPENDED one
	// is 409 STORE_STATE_INVALID (use ReinstateStore). An unknown id is domain.ErrNotFound.
	VerifyStore(ctx context.Context, id int64) (domain.AdminStore, error)
	// SuspendStore moves a PENDING or VERIFIED store to SUSPENDED, hiding it and its products and
	// blocking new orders. An already SUSPENDED store is returned unchanged.
	SuspendStore(ctx context.Context, id int64) (domain.AdminStore, error)
	// ReinstateStore moves a SUSPENDED store back to VERIFIED. An already VERIFIED store is returned
	// unchanged; a PENDING one is 409 STORE_STATE_INVALID, since it was never verified.
	ReinstateStore(ctx context.Context, id int64) (domain.AdminStore, error)
}
