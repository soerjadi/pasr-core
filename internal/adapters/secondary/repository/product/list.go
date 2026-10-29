package product

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *productRepository) List(ctx context.Context, q domain.ProductQuery) ([]domain.ProductRecord, error) {
	ctx, span := tracer.Start(ctx, "repository.product.List")
	defer span.End()

	sql, args := buildListQuery(q)
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("product.List query: %w", err)
	}
	defer rows.Close()

	items := []domain.ProductRecord{}
	for rows.Next() {
		var p domain.ProductRecord
		if err := rows.Scan(&p.ID, &p.Name, &p.SellerID, &p.SellerName, &p.Price, &p.Rating,
			&p.RatingCount, &p.Stock, &p.HalalVerified); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("product.List scan: %w", err)
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("product.List rows: %w", err)
	}
	return items, nil
}
