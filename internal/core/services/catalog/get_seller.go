package catalog

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) GetSeller(ctx context.Context, id, viewerID int64) (domain.SellerDetail, error) {
	ctx, span := tracer.Start(ctx, "service.catalog.GetSeller")
	defer span.End()

	r, err := s.storefront.GetVisible(ctx, id, viewerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerDetail{}, err
	}
	if r == nil {
		return domain.SellerDetail{}, notFound("seller")
	}
	out := domain.SellerDetail{Seller: s.sellerSummary(*r), Description: r.Description, FollowerCount: r.FollowerCount}
	if viewerID > 0 {
		out.IsFollowing = &r.IsFollowing
	}
	return out, nil
}
