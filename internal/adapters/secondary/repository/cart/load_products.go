package cart

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *cartRepository) LoadProducts(ctx context.Context, ids []int64, lock bool) (map[int64]domain.CartProduct, error) {
	ctx, span := tracer.Start(ctx, "repository.cart.LoadProducts")
	defer span.End()

	out := make(map[int64]domain.CartProduct, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	query := loadProductsQuery
	if lock {
		query = loadProductsForUpdateQuery
	}
	rows, err := r.db.Query(ctx, query, ids)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("cart.LoadProducts query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p domain.CartProduct
		s := &p.Store
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.Status, &p.Deleted, &p.Image,
			&s.ID, &s.Name, &s.Avatar, &s.Location, &s.Rating, &s.RatingCount, &s.PickupAddress,
			&s.Hours, &s.WhatsApp, &s.Status, &s.IsOpen, &s.OwnerID); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("cart.LoadProducts scan: %w", err)
		}
		out[p.ID] = p
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("cart.LoadProducts rows: %w", err)
	}
	return out, nil
}
