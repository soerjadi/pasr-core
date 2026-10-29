package product

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *productRepository) CountOwned(ctx context.Context, storeID int64) (domain.ProductCounts, error) {
	ctx, span := tracer.Start(ctx, "repository.product.CountOwned")
	defer span.End()

	var c domain.ProductCounts
	rows, err := r.db.Query(ctx, countOwnedQuery, storeID)
	if err != nil {
		tracer.RecordError(span, err)
		return c, fmt.Errorf("product.CountOwned query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			status string
			n      int32
		)
		if err := rows.Scan(&status, &n); err != nil {
			tracer.RecordError(span, err)
			return c, fmt.Errorf("product.CountOwned scan: %w", err)
		}
		c.All += n
		switch domain.ProductStatus(status) {
		case domain.ProductAvailable:
			c.Available = n
		case domain.ProductOutOfStock:
			c.OutOfStock = n
		case domain.ProductHidden:
			c.Hidden = n
		}
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return c, fmt.Errorf("product.CountOwned rows: %w", err)
	}
	return c, nil
}
