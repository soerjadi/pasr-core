package session

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *sessionRepository) RevokeFamily(ctx context.Context, familyID string, now time.Time) error {
	ctx, span := tracer.Start(ctx, "repository.session.RevokeFamily")
	defer span.End()

	if _, err := r.db.Exec(ctx, revokeFamilyQuery, familyID, now); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("session.RevokeFamily exec: %w", err)
	}
	return nil
}
