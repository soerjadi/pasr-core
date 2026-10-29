package catalog

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) GetProduct(ctx context.Context, id, viewerID int64) (domain.ProductDetail, error) {
	ctx, span := tracer.Start(ctx, "service.catalog.GetProduct")
	defer span.End()

	p, err := s.products.GetVisible(ctx, id, viewerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.ProductDetail{}, err
	}
	if p == nil {
		return domain.ProductDetail{}, notFound("product")
	}
	seller, err := s.storefront.GetVisible(ctx, p.SellerID, 0)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.ProductDetail{}, err
	}
	if seller == nil {
		return domain.ProductDetail{}, notFound("product")
	}
	images, err := s.products.Images(ctx, []int64{p.ID})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.ProductDetail{}, err
	}

	out := domain.ProductDetail{
		Product:     s.productSummary(*p, images[p.ID]),
		Description: p.Description,
		Images:      make([]domain.ImageSet, 0, len(images[p.ID])),
		Seller:      s.sellerSummary(*seller),
	}
	for _, k := range images[p.ID] {
		out.Images = append(out.Images, s.imageSet(k))
	}
	if viewerID > 0 {
		out.IsFavorite = &p.IsFavorite
	}
	return out, nil
}
