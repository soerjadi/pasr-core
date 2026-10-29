package storemgmt

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) CreateProduct(ctx context.Context, ownerID int64, in domain.ProductInput) (domain.SellerProduct, error) {
	ctx, span := tracer.Start(ctx, "service.storemgmt.CreateProduct")
	defer span.End()

	name, err := validName(in.Name)
	if err == nil {
		err = validPrice(in.Price)
	}
	if err == nil {
		err = validStock(in.Stock)
	}
	if err == nil {
		err = validDescription(in.Description)
	}
	if err == nil {
		err = validPhotos(in.PhotoIDs)
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProduct{}, err
	}
	store, err := s.storeOf(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProduct{}, err
	}

	rec := domain.SellerProductRecord{
		StoreID: store.ID, CategoryID: store.CategoryID, Name: name, Price: in.Price, Stock: in.Stock,
		Description: in.Description, Status: domain.ProductAvailable,
	}
	if rec.Stock == 0 {
		rec.Status = domain.ProductOutOfStock
	}

	var (
		created domain.SellerProductRecord
		keys    []domain.ImageKeys
	)
	// One transaction: the photos are claimed only if the product row is created.
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		if keys, err = s.claimPhotos(ctx, ownerID, in.PhotoIDs); err != nil {
			return err
		}
		if created, err = s.products.Create(ctx, rec); err != nil {
			return err
		}
		if len(in.PhotoIDs) > 0 {
			return s.products.ReplaceImages(ctx, created.ID, in.PhotoIDs)
		}
		return nil
	})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProduct{}, err
	}
	return s.view(created, keys), nil
}
