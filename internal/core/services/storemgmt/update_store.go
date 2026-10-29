package storemgmt

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const (
	maxStoreNameRunes    = 50
	maxOwnerNameRunes    = 50
	maxAddressRunes      = 200
	maxKelurahanRunes    = 100
	maxStoreDescriptions = 2000
)

func (s *service) UpdateMyStore(ctx context.Context, ownerID int64, patch domain.StorePatch) (domain.MyStore, error) {
	ctx, span := tracer.Start(ctx, "service.storemgmt.UpdateMyStore")
	defer span.End()

	upd, err := storeUpdate(patch)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.MyStore{}, err
	}
	store, err := s.storeOf(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.MyStore{}, err
	}

	// One transaction: the new photo is claimed only if the store row is updated.
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		if patch.PhotoID != nil {
			if _, err := s.claimPhotos(ctx, ownerID, []int64{*patch.PhotoID}); err != nil {
				return err
			}
			upd.PhotoUploadID = patch.PhotoID
		}
		return s.stores.Update(ctx, store.ID, upd)
	})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.MyStore{}, err
	}
	out, err := s.loadStore(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.MyStore{}, err
	}
	return out, nil
}

// required trims v and checks it holds 1..max characters.
func required(field, v string, max int) (string, error) {
	v = strings.TrimSpace(v)
	if n := utf8.RuneCountInString(v); n < 1 || n > max {
		return "", invalid(field + " must be between 1 and " + strconv.Itoa(max) + " characters")
	}
	return v, nil
}

// storeUpdate validates the patch and maps it to the repository update.
func storeUpdate(p domain.StorePatch) (domain.StoreUpdate, error) {
	var (
		u   domain.StoreUpdate
		err error
	)
	set := func(dst **string, field string, v *string, max int) {
		if v == nil || err != nil {
			return
		}
		var t string
		if t, err = required(field, *v, max); err == nil {
			*dst = &t
		}
	}
	set(&u.Name, "name", p.Name, maxStoreNameRunes)
	set(&u.OwnerName, "ownerName", p.OwnerName, maxOwnerNameRunes)
	set(&u.Kelurahan, "kelurahan", p.Kelurahan, maxKelurahanRunes)
	set(&u.Address, "address", p.Address, maxAddressRunes)
	if err != nil {
		return domain.StoreUpdate{}, err
	}
	if p.CategoryID != nil {
		if *p.CategoryID < 1 {
			return domain.StoreUpdate{}, invalid("categoryId must be a category id")
		}
		u.CategoryID = p.CategoryID
	}
	if p.Description != nil {
		if utf8.RuneCountInString(*p.Description) > maxStoreDescriptions {
			return domain.StoreUpdate{}, invalid("description must be at most " + strconv.Itoa(maxStoreDescriptions) + " characters")
		}
		u.Description = p.Description
	}
	if p.PhotoID != nil && *p.PhotoID < 1 {
		return domain.StoreUpdate{}, invalid("photoId must be an upload id")
	}
	if p.WhatsApp != nil {
		w, err := domain.NormalizePhone(*p.WhatsApp)
		if err != nil {
			return domain.StoreUpdate{}, invalid("whatsapp must be a valid Indonesian mobile number")
		}
		u.WhatsApp = &w
	}
	if p.Hours != nil {
		if err := p.Hours.Validate(); err != nil {
			return domain.StoreUpdate{}, invalid("invalid hours: " + err.Error())
		}
		u.Hours = *p.Hours
	}
	return u, nil
}
