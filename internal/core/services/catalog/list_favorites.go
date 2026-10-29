package catalog

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) ListFavorites(ctx context.Context, userID int64, cursor string, limit int) (domain.Page[domain.ProductSummary], error) {
	ctx, span := tracer.Start(ctx, "service.catalog.ListFavorites")
	defer span.End()

	before, err := domain.DecodeCursor(cursor)
	if err != nil {
		return domain.Page[domain.ProductSummary]{}, err
	}
	limit = domain.ClampLimit(limit)

	rows, err := s.social.ListFavorites(ctx, userID, before, limit+1)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.ProductSummary]{}, err
	}
	page := domain.BuildPage(rows, limit, func(f domain.FavoriteRecord) int64 { return f.FavoriteID })

	ids := make([]int64, len(page.Items))
	for i, f := range page.Items {
		ids[i] = f.Product.ID
	}
	images, err := s.products.Images(ctx, ids)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.ProductSummary]{}, err
	}
	items := make([]domain.ProductSummary, len(page.Items))
	for i, f := range page.Items {
		items[i] = s.productSummary(f.Product, images[f.Product.ID])
	}
	return domain.Page[domain.ProductSummary]{Items: items, NextCursor: page.NextCursor}, nil
}
