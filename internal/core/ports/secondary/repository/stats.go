package repository

import (
	"context"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// StatsRepository aggregates a store's seller-orders. A seller-order counts as completed on the
// day (in the given zone) of its COMPLETED history row, and only while its status is COMPLETED.
type StatsRepository interface {
	// CountCreated counts the store's seller-orders created in [from, to) that are not CANCELLED.
	CountCreated(ctx context.Context, storeID int64, from, to time.Time) (int32, error)
	// CompletedByDay sums subtotals and counts seller-orders completed in [from, to), grouped by
	// calendar date in zone (an IANA name). Days without sales are absent.
	CompletedByDay(ctx context.Context, storeID int64, from, to time.Time, zone string) ([]domain.DayTotal, error)
	// TopProducts ranks products by units in seller-orders completed in [from, to), most units
	// first (ties by product id), at most limit, each with the name from its latest sale.
	TopProducts(ctx context.Context, storeID int64, from, to time.Time, limit int) ([]domain.TopProduct, error)
}
