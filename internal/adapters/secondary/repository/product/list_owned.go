package product

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *productRepository) ListOwned(ctx context.Context, q domain.SellerProductQuery) ([]domain.SellerProductRecord, error) {
	ctx, span := tracer.Start(ctx, "repository.product.ListOwned")
	defer span.End()

	sql, args := buildListOwnedQuery(q)
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("product.ListOwned query: %w", err)
	}
	defer rows.Close()
	items := []domain.SellerProductRecord{}
	for rows.Next() {
		var p domain.SellerProductRecord
		if err := scanOwned(rows, &p); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("product.ListOwned scan: %w", err)
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("product.ListOwned rows: %w", err)
	}
	return items, nil
}
