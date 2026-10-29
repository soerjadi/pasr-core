package social

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *socialRepository) RemoveFavorite(ctx context.Context, userID, productID int64) error {
	ctx, span := tracer.Start(ctx, "repository.social.RemoveFavorite")
	defer span.End()

	if _, err := r.db.Exec(ctx, removeFavoriteQuery, userID, productID); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("social.RemoveFavorite exec: %w", err)
	}
	return nil
}
