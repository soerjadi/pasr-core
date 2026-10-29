package storemgmt

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) UpdateMyProduct(ctx context.Context, ownerID, productID int64, patch domain.ProductPatch) (domain.SellerProduct, error) {
	ctx, span := tracer.Start(ctx, "service.storemgmt.UpdateMyProduct")
	defer span.End()

	if err := validatePatch(patch); err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProduct{}, err
	}
	store, err := s.storeOf(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProduct{}, err
	}

	var updated domain.SellerProductRecord
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.products.GetOwned(ctx, store.ID, productID, true)
		if err != nil {
			return err
		}
		if cur == nil {
			return notFound()
		}
		updated, err = applyPatch(*cur, patch)
		if err != nil {
			return err
		}
		if err := s.products.Update(ctx, updated); err != nil {
			return err
		}
		if patch.PhotoIDs != nil {
			if _, err := s.claimPhotos(ctx, ownerID, *patch.PhotoIDs); err != nil {
				return err
			}
			return s.products.ReplaceImages(ctx, updated.ID, *patch.PhotoIDs)
		}
		return nil
	})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProduct{}, err
	}
	out, err := s.load(ctx, updated)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProduct{}, err
	}
	return out, nil
}

func validatePatch(p domain.ProductPatch) error {
	if p.Name != nil {
		if _, err := validName(*p.Name); err != nil {
			return err
		}
	}
	if p.Price != nil {
		if err := validPrice(*p.Price); err != nil {
			return err
		}
	}
	if p.Stock != nil {
		if err := validStock(*p.Stock); err != nil {
			return err
		}
	}
	if p.Description != nil {
		if err := validDescription(*p.Description); err != nil {
			return err
		}
	}
	if p.PhotoIDs != nil {
		if err := validPhotos(*p.PhotoIDs); err != nil {
			return err
		}
	}
	if p.Status != nil && !p.Status.Valid() {
		return invalid("status must be AVAILABLE, OUT_OF_STOCK or HIDDEN")
	}
	return nil
}

// applyPatch returns cur with the patch applied and the status rules enforced:
//   - an explicit status is honoured, except AVAILABLE with stock 0 (rejected);
//   - otherwise stock 0 turns an AVAILABLE product OUT_OF_STOCK, and a restock above 0 turns
//     OUT_OF_STOCK back to AVAILABLE. A HIDDEN product stays hidden either way.
func applyPatch(cur domain.SellerProductRecord, p domain.ProductPatch) (domain.SellerProductRecord, error) {
	if p.Name != nil {
		cur.Name, _ = validName(*p.Name)
	}
	if p.Price != nil {
		cur.Price = *p.Price
	}
	if p.Description != nil {
		cur.Description = *p.Description
	}
	if p.Stock != nil {
		cur.Stock = *p.Stock
	}
	switch {
	case p.Status != nil:
		cur.Status = *p.Status
		if cur.Status == domain.ProductAvailable && cur.Stock == 0 {
			return cur, invalid("a product with stock 0 cannot be AVAILABLE")
		}
	case cur.Stock == 0 && cur.Status == domain.ProductAvailable:
		cur.Status = domain.ProductOutOfStock
	case p.Stock != nil && cur.Stock > 0 && cur.Status == domain.ProductOutOfStock:
		cur.Status = domain.ProductAvailable
	}
	return cur, nil
}
