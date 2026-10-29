package storemgmt

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) GetMyProduct(ctx context.Context, ownerID, productID int64) (domain.SellerProduct, error) {
	ctx, span := tracer.Start(ctx, "service.storemgmt.GetMyProduct")
	defer span.End()

	store, err := s.storeOf(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProduct{}, err
	}
	rec, err := s.products.GetOwned(ctx, store.ID, productID, false)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProduct{}, err
	}
	if rec == nil {
		return domain.SellerProduct{}, notFound()
	}
	out, err := s.load(ctx, *rec)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProduct{}, err
	}
	return out, nil
}
