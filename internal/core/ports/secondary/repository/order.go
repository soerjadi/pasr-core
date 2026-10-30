package repository

import (
	"context"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type OrderRepository interface {
	// SetStatementTimeout bounds each following statement of the current transaction (SET LOCAL
	// statement_timeout), lock waits included. Outside a transaction it has no lasting effect.
	SetStatementTimeout(ctx context.Context, d time.Duration) error
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

	// GetReviewTarget returns the item and its seller-order status when the item belongs to an order
	// of buyerID, otherwise domain.ErrNotFound.
	GetReviewTarget(ctx context.Context, buyerID, orderItemID int64) (domain.ReviewTarget, error)

	// LockSellerOrder reads a seller-order with FOR UPDATE (only that row is locked). storeID 0
	// matches any store; a missing seller-order or one of another store is domain.ErrNotFound.
	LockSellerOrder(ctx context.Context, id, storeID int64) (domain.SellerOrderLock, error)
	// UpdateSellerOrder stores a transition (status, updated_at and the non-nil fields) and appends
	// its seller-order history row.
	UpdateSellerOrder(ctx context.Context, c domain.SellerOrderChange) error
	// RestoreStock gives every item's quantity back to its product with a plain UPDATE, after
	// locking the products in ascending id order. An OUT_OF_STOCK product with stock again turns AVAILABLE.
	RestoreStock(ctx context.Context, sellerOrderID int64) error
	// LockOrderStatuses locks the order row (FOR NO KEY UPDATE) and returns its stored status and the
	// statuses of all its seller-orders, read after the lock was granted.
	LockOrderStatuses(ctx context.Context, orderID int64) (domain.OrderStatus, []domain.OrderStatus, error)
	// SetOrderStatus stores the order status and appends an order-level history row.
	SetOrderStatus(ctx context.Context, orderID int64, status domain.OrderStatus, at time.Time) error
	// GetStoreOrder returns the seller's view of a seller-order of storeID (0 matches any store),
	// or domain.ErrNotFound.
	GetStoreOrder(ctx context.Context, storeID, id int64) (domain.StoreOrder, error)
	// ListStoreOrders returns up to limit of the store's seller-orders in statuses with id < afterID
	// (0 for the first page), newest first, with items and history.
	ListStoreOrders(ctx context.Context, storeID int64, statuses []domain.OrderStatus, afterID int64, limit int) ([]domain.StoreOrder, error)
	// CountStoreOrders counts the store's seller-orders per seller tab in one query.
	CountStoreOrders(ctx context.Context, storeID int64) (domain.StoreOrderCounts, error)
	// OverdueSellerOrders returns up to limit CREATED seller-orders with accept_by <= now, locking
	// them FOR UPDATE SKIP LOCKED (rows locked by others are passed over).
	OverdueSellerOrders(ctx context.Context, now time.Time, limit int) ([]int64, error)
}
