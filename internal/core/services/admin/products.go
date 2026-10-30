package admin

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const maxHalalCertRunes = 100

func (s *service) UpdateProduct(ctx context.Context, id int64, patch domain.AdminProductPatch) (domain.AdminProduct, error) {
	ctx, span := tracer.Start(ctx, "service.admin.UpdateProduct")
	defer span.End()

	if patch.Status != nil && !patch.Status.Valid() {
		err := invalid("status must be AVAILABLE, OUT_OF_STOCK or HIDDEN")
		tracer.RecordError(span, err)
		return domain.AdminProduct{}, err
	}
	var cert *string
	if patch.HalalCertNo != nil {
		c, err := cleanText("halalCertNo", *patch.HalalCertNo, maxHalalCertRunes, false)
		if err != nil {
			tracer.RecordError(span, err)
			return domain.AdminProduct{}, err
		}
		cert = &c
	}

	var out domain.AdminProduct
	err := s.Tx.WithTx(ctx, func(ctx context.Context) error {
		p, err := s.Products.LockAdmin(ctx, id)
		if err != nil {
			return err
		}
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "NOT_FOUND", "product not found")
		}
		if patch.Featured != nil {
			p.Featured = *patch.Featured
		}
		if patch.HalalVerified != nil {
			p.HalalVerified = *patch.HalalVerified
		}
		if cert != nil {
			p.HalalCertNo = cert
			if *cert == "" {
				p.HalalCertNo = nil
			}
		}
		if patch.Status != nil {
			if *patch.Status == domain.ProductAvailable && p.Stock == 0 {
				return invalid("a product without stock cannot be AVAILABLE")
			}
			p.Status = *patch.Status
		}
		if err := s.Products.UpdateAdmin(ctx, *p); err != nil {
			return err
		}
		out = *p
		return nil
	})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.AdminProduct{}, err
	}
	return out, nil
}
