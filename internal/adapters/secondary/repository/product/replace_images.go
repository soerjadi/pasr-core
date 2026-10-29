package product

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

// ReplaceImages is two statements, so callers run it inside a transaction.
func (r *productRepository) ReplaceImages(ctx context.Context, productID int64, uploadIDs []int64) error {
	ctx, span := tracer.Start(ctx, "repository.product.ReplaceImages")
	defer span.End()

	if _, err := r.db.Exec(ctx, deleteImagesQuery, productID); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("product.ReplaceImages delete: %w", err)
	}
	if len(uploadIDs) == 0 {
		return nil
	}
	if _, err := r.db.Exec(ctx, insertImagesQuery, productID, uploadIDs); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("product.ReplaceImages insert: %w", err)
	}
	return nil
}
