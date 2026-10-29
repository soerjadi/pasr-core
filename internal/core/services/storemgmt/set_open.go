package storemgmt

import (
	"context"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) SetMyStoreOpen(ctx context.Context, ownerID int64, open bool) error {
	ctx, span := tracer.Start(ctx, "service.storemgmt.SetMyStoreOpen")
	defer span.End()

	store, err := s.storeOf(ctx, ownerID)
	if err == nil {
		err = s.stores.SetOpen(ctx, store.ID, open)
	}
	if err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}
