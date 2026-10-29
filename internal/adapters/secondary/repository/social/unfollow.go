package social

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *socialRepository) Unfollow(ctx context.Context, userID, storeID int64) error {
	ctx, span := tracer.Start(ctx, "repository.social.Unfollow")
	defer span.End()

	if _, err := r.db.Exec(ctx, unfollowQuery, userID, storeID); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("social.Unfollow exec: %w", err)
	}
	return nil
}
