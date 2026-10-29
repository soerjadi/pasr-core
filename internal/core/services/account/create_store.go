package account

import (
	"context"
	"strings"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) CreateStore(ctx context.Context, userID int64, in domain.CreateStoreInput) (domain.StoreSummary, error) {
	ctx, span := tracer.Start(ctx, "service.account.CreateStore")
	defer span.End()

	store, err := validateStore(userID, in)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreSummary{}, err
	}
	owner, err := s.users.GetByID(ctx, userID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreSummary{}, err
	}
	store.WhatsApp = owner.Phone

	var out domain.StoreSummary
	// One transaction: the photo is claimed only if the store row is created.
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		if in.PhotoID != nil {
			if _, err := s.uploads.ResolveOwned(ctx, userID, []int64{*in.PhotoID}); err != nil {
				return err
			}
			store.PhotoUploadID = in.PhotoID
		}
		created, err := s.stores.Create(ctx, store)
		if err != nil {
			return err
		}
		if in.PhotoID != nil {
			if err := s.uploads.MarkAttached(ctx, []int64{*in.PhotoID}); err != nil {
				return err
			}
		}
		out = created
		return nil
	})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.StoreSummary{}, err
	}
	return out, nil
}

func validateStore(ownerID int64, in domain.CreateStoreInput) (domain.Store, error) {
	var (
		st  = domain.Store{OwnerID: ownerID, CategoryID: in.CategoryID}
		err error
	)
	if st.Name, err = requireRunes("name", in.Name, 50); err != nil {
		return st, err
	}
	if st.OwnerName, err = requireRunes("ownerName", in.OwnerName, 50); err != nil {
		return st, err
	}
	if st.Address, err = requireRunes("address", in.Address, 200); err != nil {
		return st, err
	}
	if st.Kelurahan = strings.TrimSpace(in.Kelurahan); st.Kelurahan == "" {
		return st, invalid("kelurahan is required")
	}
	if in.CategoryID <= 0 {
		return st, invalid("categoryId is required")
	}
	return st, nil
}
