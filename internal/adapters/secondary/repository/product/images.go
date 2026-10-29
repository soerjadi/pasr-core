package product

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *productRepository) Images(ctx context.Context, productIDs []int64) (map[int64][]domain.ImageKeys, error) {
	ctx, span := tracer.Start(ctx, "repository.product.Images")
	defer span.End()

	out := make(map[int64][]domain.ImageKeys, len(productIDs))
	if len(productIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, imagesQuery, productIDs)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("product.Images query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id   int64
			keys domain.ImageKeys
		)
		if err := rows.Scan(&id, &keys); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("product.Images scan: %w", err)
		}
		out[id] = append(out[id], keys)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("product.Images rows: %w", err)
	}
	return out, nil
}
