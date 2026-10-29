package social

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *socialRepository) ListFavorites(ctx context.Context, userID, beforeID int64, limit int) ([]domain.FavoriteRecord, error) {
	ctx, span := tracer.Start(ctx, "repository.social.ListFavorites")
	defer span.End()

	rows, err := r.db.Query(ctx, listFavoritesQuery, userID, beforeID, limit)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("social.ListFavorites query: %w", err)
	}
	defer rows.Close()

	items := []domain.FavoriteRecord{}
	for rows.Next() {
		var f domain.FavoriteRecord
		p := &f.Product
		if err := rows.Scan(&f.FavoriteID, &p.ID, &p.Name, &p.SellerID, &p.SellerName, &p.Price, &p.Rating,
			&p.RatingCount, &p.Stock, &p.HalalVerified); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("social.ListFavorites scan: %w", err)
		}
		p.IsFavorite = true
		items = append(items, f)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("social.ListFavorites rows: %w", err)
	}
	return items, nil
}
