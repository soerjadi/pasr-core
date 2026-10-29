package social

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *socialRepository) AddFavorite(ctx context.Context, userID, productID int64) error {
	ctx, span := tracer.Start(ctx, "repository.social.AddFavorite")
	defer span.End()

	if _, err := r.db.Exec(ctx, addFavoriteQuery, userID, productID); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("social.AddFavorite exec: %w", err)
	}
	return nil
}
