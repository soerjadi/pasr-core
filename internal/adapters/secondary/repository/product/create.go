package product

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *productRepository) Create(ctx context.Context, p domain.SellerProductRecord) (domain.SellerProductRecord, error) {
	ctx, span := tracer.Start(ctx, "repository.product.Create")
	defer span.End()

	err := r.db.QueryRow(ctx, createQuery, p.StoreID, p.CategoryID, p.Name, p.Description, p.Price, p.Stock, string(p.Status)).Scan(&p.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "products_category_id_fkey" {
			return domain.SellerProductRecord{}, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "unknown category")
		}
		tracer.RecordError(span, err)
		return domain.SellerProductRecord{}, fmt.Errorf("product.Create insert: %w", err)
	}
	return p, nil
}
