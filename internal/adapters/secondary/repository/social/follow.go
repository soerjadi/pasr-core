package social

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *socialRepository) Follow(ctx context.Context, userID, storeID int64) error {
	ctx, span := tracer.Start(ctx, "repository.social.Follow")
	defer span.End()

	if _, err := r.db.Exec(ctx, followQuery, userID, storeID); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("social.Follow exec: %w", err)
	}
	return nil
}
