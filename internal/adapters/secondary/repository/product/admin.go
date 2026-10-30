package product

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *productRepository) LockAdmin(ctx context.Context, id int64) (*domain.AdminProduct, error) {
	ctx, span := tracer.Start(ctx, "repository.product.LockAdmin")
	defer span.End()

	var p domain.AdminProduct
	var status string
	err := r.db.QueryRow(ctx, getAdminForUpdateQuery, id).Scan(
		&p.ID, &p.StoreID, &p.Name, &p.Price, &p.Stock, &status, &p.Featured, &p.HalalVerified, &p.HalalCertNo)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("product.LockAdmin query: %w", err)
	}
	p.Status = domain.ProductStatus(status)
	return &p, nil
}

func (r *productRepository) UpdateAdmin(ctx context.Context, p domain.AdminProduct) error {
	ctx, span := tracer.Start(ctx, "repository.product.UpdateAdmin")
	defer span.End()

	if _, err := r.db.Exec(ctx, updateAdminQuery, p.ID, string(p.Status), p.Featured, p.HalalVerified, p.HalalCertNo); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("product.UpdateAdmin exec: %w", err)
	}
	return nil
}
