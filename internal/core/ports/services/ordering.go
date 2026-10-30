package services

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// OrderingService prices carts and places and tracks the buyer's orders.
type OrderingService interface {
	// ValidateCart prices and stock-checks a cart without reserving anything. It is public.
	// 1 to 50 lines, quantity >= 1 each, otherwise domain.ErrValidationFailed. Duplicate product
	// ids are merged by summing quantity. Every requested line comes back, grouped by store in
	// request order; prices come from the database. Lines of products that do not exist at all
	// have no store and are grouped under a seller with id 0. Total sums only lines without an issue.
	ValidateCart(ctx context.Context, lines []domain.CartLineInput) (domain.CartValidation, error)

	// CreateOrder places a cash-at-pickup order in one transaction: one seller-order per store,
	// prices from the database, stock decremented, a NEW_ORDER push queued for each store owner.
	// key (the Idempotency-Key, 1 to 255 bytes) is required; the same key with the same merged
	// cart returns the stored order, with another cart 422 IDEMPOTENCY_KEY_REUSED. Lines that
	// cannot be bought fail the whole order with 409 PRODUCT_UNAVAILABLE, STORE_CLOSED or
	// INSUFFICIENT_STOCK naming the product ids.
	CreateOrder(ctx context.Context, buyerID int64, key string, lines []domain.CartLineInput) (domain.Order, error)
}
