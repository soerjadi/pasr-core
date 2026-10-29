package catalog

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) ListSellerReviews(ctx context.Context, sellerID int64, cursor string, limit int) (domain.Page[domain.Review], error) {
	ctx, span := tracer.Start(ctx, "service.catalog.ListSellerReviews")
	defer span.End()

	before, err := domain.DecodeCursor(cursor)
	if err != nil {
		return domain.Page[domain.Review]{}, err
	}
	limit = domain.ClampLimit(limit)

	seller, err := s.storefront.GetVisible(ctx, sellerID, 0)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.Review]{}, err
	}
	if seller == nil {
		return domain.Page[domain.Review]{}, notFound("seller")
	}
	rows, err := s.reviews.ListByStore(ctx, sellerID, before, limit+1)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Page[domain.Review]{}, err
	}
	return domain.BuildPage(rows, limit, func(r domain.Review) int64 { return r.ID }), nil
}
