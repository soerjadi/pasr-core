package repository

import (
	"context"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type OrderRepository interface {
	// NextOrderNumber atomically takes the next sequence of day's calendar date (in day's location,
	// pass a WIB time) and formats it as NM-YYYYMMDD-NNN. Inside a transaction the counter row
	// stays locked until it ends, so numbers are gap-free per committed day.
	NextOrderNumber(ctx context.Context, day time.Time) (string, error)
	// Insert stores the order, its seller-orders and items, and the CREATED history entries (one at
	// the order level, one per seller-order). It runs several statements: call it inside a transaction.
	Insert(ctx context.Context, o domain.NewOrder) (int64, error)
	// DecrementStock takes qty from an AVAILABLE, not deleted product with enough stock and reports
	// whether it did. Stock reaching 0 turns the product OUT_OF_STOCK.
	DecrementStock(ctx context.Context, productID int64, qty int32) (bool, error)
	// Get returns the buyer's order, or domain.ErrNotFound when it does not exist or is someone else's.
	Get(ctx context.Context, buyerID, orderID int64) (domain.Order, error)
	// List returns up to limit of the buyer's orders with id < afterID (0 for the first page), newest
	// first, filtered to statuses unless nil. Reads load the whole page's details in three queries.
	List(ctx context.Context, buyerID int64, statuses []domain.OrderStatus, afterID int64, limit int) ([]domain.Order, error)
}
