package product

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *productRepository) Update(ctx context.Context, p domain.SellerProductRecord) error {
	ctx, span := tracer.Start(ctx, "repository.product.Update")
	defer span.End()

	if _, err := r.db.Exec(ctx, updateQuery, p.ID, p.Name, p.Price, p.Stock, p.Description, string(p.Status)); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("product.Update exec: %w", err)
	}
	return nil
}
