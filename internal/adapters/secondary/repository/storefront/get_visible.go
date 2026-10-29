package storefront

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *storefrontRepository) GetVisible(ctx context.Context, id, viewerID int64) (*domain.SellerRecord, error) {
	ctx, span := tracer.Start(ctx, "repository.storefront.GetVisible")
	defer span.End()

	var s domain.SellerRecord
	err := r.db.QueryRow(ctx, getVisibleQuery, id, viewerID).Scan(&s.ID, &s.Name, &s.Avatar, &s.Location,
		&s.Rating, &s.RatingCount, &s.PickupAddress, &s.Hours, &s.WhatsApp, &s.Description,
		&s.FollowerCount, &s.IsFollowing)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("storefront.GetVisible query: %w", err)
	}
	return &s, nil
}
