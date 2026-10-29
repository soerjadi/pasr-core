package product

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *productRepository) GetVisible(ctx context.Context, id, viewerID int64) (*domain.ProductRecord, error) {
	ctx, span := tracer.Start(ctx, "repository.product.GetVisible")
	defer span.End()

	var p domain.ProductRecord
	err := r.db.QueryRow(ctx, getVisibleQuery, id, viewerID).Scan(&p.ID, &p.Name, &p.SellerID, &p.SellerName,
		&p.Description, &p.Price, &p.Rating, &p.RatingCount, &p.Stock, &p.HalalVerified, &p.IsFavorite)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("product.GetVisible query: %w", err)
	}
	return &p, nil
}
