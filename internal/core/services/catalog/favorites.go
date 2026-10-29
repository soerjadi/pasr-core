package catalog

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) AddFavorite(ctx context.Context, userID, productID int64) error {
	ctx, span := tracer.Start(ctx, "service.catalog.AddFavorite")
	defer span.End()

	p, err := s.products.GetVisible(ctx, productID, 0)
	if err != nil {
		tracer.RecordError(span, err)
		return err
	}
	if p == nil {
		return notFound("product")
	}
	if err := s.social.AddFavorite(ctx, userID, productID); err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}

func (s *service) RemoveFavorite(ctx context.Context, userID, productID int64) error {
	ctx, span := tracer.Start(ctx, "service.catalog.RemoveFavorite")
	defer span.End()

	if err := s.social.RemoveFavorite(ctx, userID, productID); err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}

func (s *service) Follow(ctx context.Context, userID, storeID int64) error {
	ctx, span := tracer.Start(ctx, "service.catalog.Follow")
	defer span.End()

	seller, err := s.storefront.GetVisible(ctx, storeID, 0)
	if err != nil {
		tracer.RecordError(span, err)
		return err
	}
	if seller == nil {
		return notFound("seller")
	}
	own, err := s.social.OwnsStore(ctx, userID, storeID)
	if err != nil {
		tracer.RecordError(span, err)
		return err
	}
	if own {
		return domain.NewError(domain.ErrConflict, "CANNOT_FOLLOW_OWN_STORE", "you cannot follow your own store")
	}
	if err := s.social.Follow(ctx, userID, storeID); err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}

func (s *service) Unfollow(ctx context.Context, userID, storeID int64) error {
	ctx, span := tracer.Start(ctx, "service.catalog.Unfollow")
	defer span.End()

	if err := s.social.Unfollow(ctx, userID, storeID); err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}
