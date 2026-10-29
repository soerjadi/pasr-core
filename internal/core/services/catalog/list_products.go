package catalog

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const minQueryRunes = 2

func (s *service) ListProducts(ctx context.Context, f domain.ProductFilter) (domain.Page[domain.ProductSummary], error) {
	ctx, span := tracer.Start(ctx, "service.catalog.ListProducts")
	defer span.End()

	q := strings.TrimSpace(f.Q)
	if q != "" && utf8.RuneCountInString(q) < minQueryRunes {
		return domain.Page[domain.ProductSummary]{}, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "q must be at least 2 characters")
	}
	before, err := domain.DecodeCursor(f.Cursor)
	if err != nil {
		return domain.Page[domain.ProductSummary]{}, err
	}
	limit := domain.ClampLimit(f.Limit)

	rows, err := s.products.List(ctx, domain.ProductQuery{
		CategoryID: f.CategoryID, SellerID: f.SellerID, Q: q, Featured: f.Featured, BeforeID: before, Limit: limit + 1,
	})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.ProductSummary]{}, err
	}
	page := domain.BuildPage(rows, limit, func(p domain.ProductRecord) int64 { return p.ID })

	ids := make([]int64, len(page.Items))
	for i, p := range page.Items {
		ids[i] = p.ID
	}
	images, err := s.products.Images(ctx, ids)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.ProductSummary]{}, err
	}
	items := make([]domain.ProductSummary, len(page.Items))
	for i, p := range page.Items {
		items[i] = s.productSummary(p, images[p.ID])
	}
	return domain.Page[domain.ProductSummary]{Items: items, NextCursor: page.NextCursor}, nil
}
