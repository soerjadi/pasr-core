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

	// GetOrder returns one of the buyer's orders; another user's order is domain.ErrNotFound.
	GetOrder(ctx context.Context, buyerID, orderID int64) (domain.Order, error)
	// ListOrders pages the buyer's orders newest first. tab is ALL (also for ""), PROCESSING,
	// READY or DONE (domain.TabStatuses); other values and malformed cursors are ErrValidationFailed.
	ListOrders(ctx context.Context, buyerID int64, tab, cursor string, limit int) (domain.Page[domain.Order], error)

	// SubmitReview reviews an item of one of the buyer's orders, once. Items of other buyers or that
	// do not exist are domain.ErrNotFound; a seller-order that is not COMPLETED is 409
	// ORDER_NOT_COMPLETED and a second review 409 ALREADY_REVIEWED. rating is 1 to 5 and text, trimmed,
	// at most 500 characters, else domain.ErrValidationFailed. The review and the product and store
	// rating aggregates are written in one transaction.
	SubmitReview(ctx context.Context, buyerID, orderItemID int64, rating int32, text string) (domain.Review, error)

	// ListStoreOrders pages the owner's store's seller-orders newest first. tab is NEW (also for ""),
	// PROCESSING or DONE (domain.SellerTabStatuses); Counts cover every tab. Without a store 409 NO_STORE.
	ListStoreOrders(ctx context.Context, ownerID int64, tab, cursor string, limit int) (domain.StoreOrderPage, error)
	// StoreSummary is the owner's home screen: open flag, today's closing time, orders waiting to be
	// accepted, today's order count and completed sales (WIB day) and the three newest orders.
	// Without a store 409 NO_STORE.
	StoreSummary(ctx context.Context, ownerID int64) (domain.StoreSummaryResponse, error)
	// StoreStats reports completed sales and best sellers for rng (domain.StatsToday or StatsWeek, else
	// domain.ErrValidationFailed). Daily always holds the last seven WIB days, zero-filled; the other
	// figures cover today or all seven days. Without a store 409 NO_STORE.
	StoreStats(ctx context.Context, ownerID int64, rng string) (domain.StoreStats, error)
	// GetStoreOrder returns a seller-order of the owner's store; another store's is domain.ErrNotFound.
	GetStoreOrder(ctx context.Context, ownerID, id int64) (domain.StoreOrder, error)

	// The actions below move one seller-order through the state machine (domain.Transition). A
	// seller acts on their own store's seller-orders only (others are domain.ErrNotFound, no store
	// is 409 NO_STORE); an operator acts on any. Repeating a move whose target is the current
	// status returns the current state without side effects. Illegal moves are 409 ORDER_NOT_ACTIONABLE.

	// Accept moves CREATED to SELLER_PROCESSING; after accept_by it is 409 ORDER_EXPIRED.
	Accept(ctx context.Context, actor domain.Actor, id int64) (domain.StoreOrder, error)
	// Reject cancels a CREATED seller-order and restores its stock. reason is OUT_OF_STOCK,
	// BUSY_OR_CLOSED, CANNOT_SERVE or OTHER; OTHER requires a note.
	Reject(ctx context.Context, actor domain.Actor, id int64, reason, note string) (domain.StoreOrder, error)
	// Ready moves SELLER_PROCESSING to READY_FOR_PICKUP; a non-empty note becomes the pickup note.
	Ready(ctx context.Context, actor domain.Actor, id int64, note string) (domain.StoreOrder, error)
	// Complete moves READY_FOR_PICKUP to COMPLETED.
	Complete(ctx context.Context, actor domain.Actor, id int64) (domain.StoreOrder, error)
	// Cancel cancels a SELLER_PROCESSING or READY_FOR_PICKUP seller-order and restores its stock;
	// reason and note follow the Reject rules.
	Cancel(ctx context.Context, actor domain.Actor, id int64, reason, note string) (domain.StoreOrder, error)
	// ExpireOverdue cancels up to 100 CREATED seller-orders past accept_by as the system, each in
	// its own transaction, and reports how many it cancelled.
	ExpireOverdue(ctx context.Context) (int, error)
}
