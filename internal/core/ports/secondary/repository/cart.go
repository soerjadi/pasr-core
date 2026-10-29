package repository

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type CartRepository interface {
	// LoadProducts loads the products by exact id, deleted and hidden ones included; unknown ids
	// are absent from the map. With lock the rows are read FOR UPDATE ordered by product id (product
	// rows only, never the stores), so callers must be inside a transaction.
	LoadProducts(ctx context.Context, ids []int64, lock bool) (map[int64]domain.CartProduct, error)
}
