package storemgmt

import (
	"context"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) DeleteMyProduct(ctx context.Context, ownerID, productID int64) error {
	ctx, span := tracer.Start(ctx, "service.storemgmt.DeleteMyProduct")
	defer span.End()

	store, err := s.storeOf(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return err
	}
	// Order items keep pointing at the soft-deleted row; favorites would only dangle.
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := s.products.SoftDelete(ctx, store.ID, productID)
		if err != nil {
			return err
		}
		if !ok {
			return notFound()
		}
		return s.products.RemoveFavorites(ctx, productID)
	})
	if err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}
