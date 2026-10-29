package services

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// OrderingService prices carts and, from Task 16 on, places and tracks orders.
type OrderingService interface {
	// ValidateCart prices and stock-checks a cart without reserving anything. It is public.
	// 1 to 50 lines, quantity >= 1 each, otherwise domain.ErrValidationFailed. Duplicate product
	// ids are merged by summing quantity. Every requested line comes back, grouped by store in
	// request order; prices come from the database. Lines of products that do not exist at all
	// have no store and are grouped under a seller with id 0. Total sums only lines without an issue.
	ValidateCart(ctx context.Context, lines []domain.CartLineInput) (domain.CartValidation, error)
}
