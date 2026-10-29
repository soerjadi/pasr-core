package product

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *productRepository) SoftDelete(ctx context.Context, storeID, id int64) (bool, error) {
	ctx, span := tracer.Start(ctx, "repository.product.SoftDelete")
	defer span.End()

	tag, err := r.db.Exec(ctx, softDeleteQuery, id, storeID)
	if err != nil {
		tracer.RecordError(span, err)
		return false, fmt.Errorf("product.SoftDelete exec: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *productRepository) RemoveFavorites(ctx context.Context, productID int64) error {
	ctx, span := tracer.Start(ctx, "repository.product.RemoveFavorites")
	defer span.End()

	if _, err := r.db.Exec(ctx, removeFavoritesQuery, productID); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("product.RemoveFavorites exec: %w", err)
	}
	return nil
}
