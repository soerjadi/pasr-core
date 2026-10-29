package storemgmt

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) ListMyProducts(ctx context.Context, ownerID int64, status string, cursor string, limit int) (domain.SellerProductPage, error) {
	ctx, span := tracer.Start(ctx, "service.storemgmt.ListMyProducts")
	defer span.End()

	st := domain.ProductStatus(status)
	if st != "" && !st.Valid() {
		err := invalid("status must be AVAILABLE, OUT_OF_STOCK or HIDDEN")
		tracer.RecordError(span, err)
		return domain.SellerProductPage{}, err
	}
	before, err := domain.DecodeCursor(cursor)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProductPage{}, err
	}
	limit = domain.ClampLimit(limit)
	store, err := s.storeOf(ctx, ownerID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProductPage{}, err
	}

	rows, err := s.products.ListOwned(ctx, domain.SellerProductQuery{StoreID: store.ID, Status: st, BeforeID: before, Limit: limit + 1})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProductPage{}, err
	}
	page := domain.BuildPage(rows, limit, func(r domain.SellerProductRecord) int64 { return r.ID })

	ids := make([]int64, len(page.Items))
	for i, r := range page.Items {
		ids[i] = r.ID
	}
	images, err := s.products.Images(ctx, ids)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProductPage{}, err
	}
	counts, err := s.products.CountOwned(ctx, store.ID)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.SellerProductPage{}, err
	}

	out := domain.SellerProductPage{Items: make([]domain.SellerProduct, 0, len(page.Items)), NextCursor: page.NextCursor, Counts: counts}
	for _, r := range page.Items {
		out.Items = append(out.Items, s.view(r, images[r.ID]))
	}
	return out, nil
}
