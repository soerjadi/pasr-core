package social

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *socialRepository) OwnsStore(ctx context.Context, userID, storeID int64) (bool, error) {
	ctx, span := tracer.Start(ctx, "repository.social.OwnsStore")
	defer span.End()

	var owns bool
	if err := r.db.QueryRow(ctx, ownsStoreQuery, userID, storeID).Scan(&owns); err != nil {
		tracer.RecordError(span, err)
		return false, fmt.Errorf("social.OwnsStore query: %w", err)
	}
	return owns, nil
}
