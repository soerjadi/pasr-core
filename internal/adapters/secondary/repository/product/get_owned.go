package product

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func scanOwned(row pgx.Row, p *domain.SellerProductRecord) error {
	var status string
	if err := row.Scan(&p.ID, &p.StoreID, &p.CategoryID, &p.Name, &p.Price, &p.Stock, &p.Description, &status); err != nil {
		return err
	}
	p.Status = domain.ProductStatus(status)
	return nil
}

func (r *productRepository) GetOwned(ctx context.Context, storeID, id int64, lock bool) (*domain.SellerProductRecord, error) {
	ctx, span := tracer.Start(ctx, "repository.product.GetOwned")
	defer span.End()

	q := getOwnedQuery
	if lock {
		q = getOwnedForUpdateQuery
	}
	var p domain.SellerProductRecord
	err := scanOwned(r.db.QueryRow(ctx, q, id, storeID), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("product.GetOwned query: %w", err)
	}
	return &p, nil
}
