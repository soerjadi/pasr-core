package session

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *sessionRepository) Revoke(ctx context.Context, id int64, now time.Time) (bool, error) {
	ctx, span := tracer.Start(ctx, "repository.session.Revoke")
	defer span.End()

	tag, err := r.db.Exec(ctx, revokeQuery, id, now)
	if err != nil {
		tracer.RecordError(span, err)
		return false, fmt.Errorf("session.Revoke exec: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
