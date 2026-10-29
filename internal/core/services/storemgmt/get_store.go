package storemgmt

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) GetMyStore(ctx context.Context, ownerID int64) (domain.MyStore, error) {
	ctx, span := tracer.Start(ctx, "service.storemgmt.GetMyStore")
	defer span.End()

	out, err := s.loadStore(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.MyStore{}, err
	}
	return out, nil
}

// loadStore reads the owner's store as the API view, or fails with 409 NO_STORE.
func (s *service) loadStore(ctx context.Context, ownerID int64) (domain.MyStore, error) {
	r, err := s.stores.FindByOwner(ctx, ownerID)
	if err != nil {
		return domain.MyStore{}, err
	}
	if r == nil {
		return domain.MyStore{}, errNoStore()
	}
	out := domain.MyStore{
		ID: r.ID, Name: r.Name, CategoryID: r.CategoryID, OwnerName: r.OwnerName, Kelurahan: r.Kelurahan,
		Address: r.Address, Description: r.Description, Status: r.Status, IsOpen: r.IsOpen, WhatsApp: r.WhatsApp,
		Hours: r.Hours,
	}
	if r.PhotoKeys != nil {
		set := s.imageSet(*r.PhotoKeys)
		out.Photo = &set
	}
	return out, nil
}
